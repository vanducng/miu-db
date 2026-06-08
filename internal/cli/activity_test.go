package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeActivityFixture creates JSONL events under root/{date}/{session}.jsonl.
func writeActivityFixture(t *testing.T, root, date, session string, events []map[string]any) {
	t.Helper()
	dir := filepath.Join(root, date)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, session+".jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
}

func runActivity(t *testing.T, root string, args ...string) Envelope {
	t.Helper()
	allArgs := append([]string{"activity"}, args...)
	allArgs = append(allArgs, "--config-dir", filepath.Dir(root)) // root is configDir/activity
	var buf bytes.Buffer
	opts := &options{output: "json", limit: 100, timeout: 30 * time.Second, configDir: filepath.Dir(root)}
	cmd := rootCommand(opts)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(allArgs)
	if err := cmd.Execute(); err != nil {
		_ = writeError(&buf, "activity", err)
	}
	var env Envelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v\noutput: %s", err, buf.String())
	}
	return env
}

func TestActivityCLIEmitsValidJSONEnvelope(t *testing.T) {
	root := t.TempDir()
	actRoot := filepath.Join(root, "activity")

	writeActivityFixture(t, actRoot, "2026-06-05", "s1", []map[string]any{
		{"event_id": "e1", "session_id": "s1", "ts": "2026-06-05T10:00:00Z", "op": "query", "connection": "demo-conn"},
	})

	env := runActivity(t, actRoot)
	if !env.OK {
		t.Fatalf("envelope not OK: %v", env.Error)
	}
	if env.Kind != "activity.events" {
		t.Errorf("kind = %q, want activity.events", env.Kind)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatal("data is not map")
	}
	events, ok := data["events"].([]any)
	if !ok {
		t.Fatal("data.events missing or wrong type")
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
}

func TestActivityCLIFailedFlag(t *testing.T) {
	root := t.TempDir()
	actRoot := filepath.Join(root, "activity")

	writeActivityFixture(t, actRoot, "2026-06-05", "s1", []map[string]any{
		{"event_id": "e1", "session_id": "s1", "ts": "2026-06-05T10:00:00Z", "op": "query"},
		{"event_id": "e2", "session_id": "s1", "ts": "2026-06-05T11:00:00Z", "op": "exec",
			"error": map[string]any{"class": "sql", "message": "syntax error"}},
	})

	env := runActivity(t, actRoot, "--failed")
	if !env.OK {
		t.Fatalf("not OK: %v", env.Error)
	}
	data := env.Data.(map[string]any)
	events := data["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("want 1 failed event, got %d", len(events))
	}
	e := events[0].(map[string]any)
	if e["event_id"] != "e2" {
		t.Errorf("expected e2, got %v", e["event_id"])
	}
}

func TestActivityCLISessionFlag(t *testing.T) {
	root := t.TempDir()
	actRoot := filepath.Join(root, "activity")

	const sid = "trace_session"
	writeActivityFixture(t, actRoot, "2026-06-03", sid, []map[string]any{
		{"event_id": "d3", "session_id": sid, "ts": "2026-06-03T08:00:00Z", "op": "query"},
	})
	writeActivityFixture(t, actRoot, "2026-06-04", sid, []map[string]any{
		{"event_id": "d4", "session_id": sid, "ts": "2026-06-04T08:00:00Z", "op": "exec"},
	})
	writeActivityFixture(t, actRoot, "2026-06-05", "other", []map[string]any{
		{"event_id": "noise", "session_id": "other", "ts": "2026-06-05T08:00:00Z", "op": "query"},
	})

	env := runActivity(t, actRoot, "--session", sid)
	if !env.OK {
		t.Fatalf("not OK: %v", env.Error)
	}
	data := env.Data.(map[string]any)
	events := data["events"].([]any)
	if len(events) != 2 {
		t.Fatalf("want 2 events across date dirs, got %d", len(events))
	}
	for _, ev := range events {
		m := ev.(map[string]any)
		if m["session_id"] != sid {
			t.Errorf("unexpected session_id %v", m["session_id"])
		}
	}
}

func TestActivityPruneCLIDryRun(t *testing.T) {
	root := t.TempDir()
	actRoot := filepath.Join(root, "activity")

	old := time.Now().UTC().AddDate(0, 0, -40).Format("2006-01-02")
	writeActivityFixture(t, actRoot, old, "s", []map[string]any{
		{"event_id": "old1", "session_id": "s", "ts": old + "T10:00:00Z", "op": "query"},
	})

	env := runActivity(t, actRoot, "prune", "--older-than", "30d", "--dry-run")
	if !env.OK {
		t.Fatalf("not OK: %v", env.Error)
	}
	if env.Kind != "activity.pruned" {
		t.Errorf("kind = %q, want activity.pruned", env.Kind)
	}
	// dry-run: dir still exists
	if _, err := os.Stat(filepath.Join(actRoot, old)); err != nil {
		t.Error("dry-run must not delete the directory")
	}
	if env.Summary["dry_run"] != true {
		t.Errorf("dry_run not set in summary")
	}
}

func TestActivityPruneCLIRemoves(t *testing.T) {
	root := t.TempDir()
	actRoot := filepath.Join(root, "activity")

	old := time.Now().UTC().AddDate(0, 0, -40).Format("2006-01-02")
	today := time.Now().UTC().Format("2006-01-02")
	writeActivityFixture(t, actRoot, old, "s", []map[string]any{
		{"event_id": "old1", "session_id": "s", "ts": old + "T10:00:00Z", "op": "query"},
	})
	writeActivityFixture(t, actRoot, today, "s", []map[string]any{
		{"event_id": "new1", "session_id": "s", "ts": today + "T10:00:00Z", "op": "query"},
	})

	env := runActivity(t, actRoot, "prune", "--older-than", "30d")
	if !env.OK {
		t.Fatalf("not OK: %v", env.Error)
	}
	if env.Summary["removed"].(float64) != 1 {
		t.Errorf("want removed=1, got %v", env.Summary["removed"])
	}
	if _, err := os.Stat(filepath.Join(actRoot, old)); !os.IsNotExist(err) {
		t.Error("stale dir should be deleted")
	}
	if _, err := os.Stat(filepath.Join(actRoot, today)); err != nil {
		t.Error("fresh dir should be kept")
	}
}

func TestActivityPruneInvalidOlderThan(t *testing.T) {
	root := t.TempDir()
	actRoot := filepath.Join(root, "activity")

	env := runActivity(t, actRoot, "prune", "--older-than", "bad")
	if env.OK {
		t.Error("expected not-OK for bad --older-than")
	}
	if env.Error == nil || env.Error.Code != "activity.invalid_older_than" {
		t.Errorf("expected activity.invalid_older_than error, got: %v", env.Error)
	}
}
