package activity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFixture(t *testing.T, root, date, session string, events []Event) {
	t.Helper()
	dir := filepath.Join(root, date)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, session+".jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
}

func TestQueryFilters(t *testing.T) {
	root := t.TempDir()

	writeFixture(t, root, "2026-06-05", "sess_a", []Event{
		{EventID: "e1", SessionID: "sess_a", Ts: "2026-06-05T10:00:00Z", Op: OpQuery, Connection: "demo-conn", Group: "demo-group"},
		{EventID: "e2", SessionID: "sess_a", Ts: "2026-06-05T11:00:00Z", Op: OpExec, Connection: "demo-conn", Group: "demo-group",
			Error: &EventError{Class: "sql", Message: "syntax error"}},
	})
	writeFixture(t, root, "2026-06-05", "sess_b", []Event{
		{EventID: "e3", SessionID: "sess_b", Ts: "2026-06-05T09:00:00Z", Op: OpQuery, Connection: "other", Group: "grp"},
	})

	t.Run("no filter returns all", func(t *testing.T) {
		evts, err := Query(root, Filter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(evts) != 3 {
			t.Fatalf("want 3, got %d", len(evts))
		}
		// ts-desc: e2 > e1 > e3
		if evts[0].EventID != "e2" || evts[1].EventID != "e1" || evts[2].EventID != "e3" {
			t.Errorf("unexpected order: %v %v %v", evts[0].EventID, evts[1].EventID, evts[2].EventID)
		}
	})

	t.Run("filter by connection bare", func(t *testing.T) {
		evts, err := Query(root, Filter{Connection: "demo-conn"})
		if err != nil {
			t.Fatal(err)
		}
		if len(evts) != 2 {
			t.Fatalf("want 2, got %d", len(evts))
		}
	})

	t.Run("filter by connection group/connection", func(t *testing.T) {
		evts, err := Query(root, Filter{Connection: "demo-group/demo-conn"})
		if err != nil {
			t.Fatal(err)
		}
		if len(evts) != 2 {
			t.Fatalf("want 2, got %d", len(evts))
		}
	})

	t.Run("filter by group", func(t *testing.T) {
		evts, err := Query(root, Filter{Group: "grp"})
		if err != nil {
			t.Fatal(err)
		}
		if len(evts) != 1 || evts[0].EventID != "e3" {
			t.Fatalf("unexpected: %v", evts)
		}
	})

	t.Run("filter by session", func(t *testing.T) {
		evts, err := Query(root, Filter{Session: "sess_a"})
		if err != nil {
			t.Fatal(err)
		}
		if len(evts) != 2 {
			t.Fatalf("want 2, got %d", len(evts))
		}
	})

	t.Run("failed only", func(t *testing.T) {
		evts, err := Query(root, Filter{FailedOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(evts) != 1 || evts[0].EventID != "e2" {
			t.Fatalf("unexpected: %v", evts)
		}
	})

	t.Run("limit", func(t *testing.T) {
		evts, err := Query(root, Filter{Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(evts) != 2 {
			t.Fatalf("want 2, got %d", len(evts))
		}
	})
}

func TestQuerySinceWindow(t *testing.T) {
	root := t.TempDir()

	writeFixture(t, root, "2026-06-01", "old", []Event{
		{EventID: "old1", SessionID: "old", Ts: "2026-06-01T10:00:00Z", Op: OpQuery},
	})
	// Use a recent timestamp within "1h" window.
	recentTs := time.Now().UTC().Add(-30 * time.Minute).Format(time.RFC3339)
	writeFixture(t, root, time.Now().UTC().Format("2006-01-02"), "new", []Event{
		{EventID: "new1", SessionID: "new", Ts: recentTs, Op: OpQuery},
	})

	evts, err := Query(root, Filter{Since: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evts {
		if e.EventID == "old1" {
			t.Error("old event should have been filtered by Since window")
		}
	}
	found := false
	for _, e := range evts {
		if e.EventID == "new1" {
			found = true
		}
	}
	if !found {
		t.Error("recent event should be included")
	}
}

func TestQueryUnparseableLinesSkipped(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2026-06-05")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sess.jsonl")
	content := `{"event_id":"e1","session_id":"sess","ts":"2026-06-05T10:00:00Z","op":"query"}
{NOT VALID JSON
{"event_id":"e2","session_id":"sess","ts":"2026-06-05T11:00:00Z","op":"exec"}
` // trailing partial line
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	evts, err := Query(root, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(evts) != 2 {
		t.Fatalf("want 2 valid events, got %d", len(evts))
	}
}

func TestQuerySessionAcrossDateDirs(t *testing.T) {
	root := t.TempDir()
	const sid = "trace_sess"

	writeFixture(t, root, "2026-06-03", sid, []Event{
		{EventID: "d3e1", SessionID: sid, Ts: "2026-06-03T08:00:00Z", Op: OpQuery},
	})
	writeFixture(t, root, "2026-06-04", sid, []Event{
		{EventID: "d4e1", SessionID: sid, Ts: "2026-06-04T08:00:00Z", Op: OpExec},
	})
	writeFixture(t, root, "2026-06-05", sid, []Event{
		{EventID: "d5e1", SessionID: sid, Ts: "2026-06-05T08:00:00Z", Op: OpSmoke},
	})
	// noise: different session
	writeFixture(t, root, "2026-06-05", "other", []Event{
		{EventID: "other1", SessionID: "other", Ts: "2026-06-05T09:00:00Z", Op: OpQuery},
	})

	evts, err := Query(root, Filter{Session: sid})
	if err != nil {
		t.Fatal(err)
	}
	if len(evts) != 3 {
		t.Fatalf("want 3, got %d: %v", len(evts), evts)
	}
}

func TestQueryMissingRoot(t *testing.T) {
	evts, err := Query("/nonexistent/path/xyz", Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if evts != nil {
		t.Fatalf("expected nil, got %v", evts)
	}
}

func TestPruneRemovesStale(t *testing.T) {
	root := t.TempDir()

	// old dir: 40 days ago
	old := time.Now().UTC().AddDate(0, 0, -40).Format("2006-01-02")
	writeFixture(t, root, old, "s", []Event{
		{EventID: "old", SessionID: "s", Ts: old + "T10:00:00Z", Op: OpQuery},
	})
	// fresh dir: today
	today := time.Now().UTC().Format("2006-01-02")
	writeFixture(t, root, today, "s", []Event{
		{EventID: "new", SessionID: "s", Ts: today + "T10:00:00Z", Op: OpQuery},
	})

	removed, dirs, err := Prune(root, 30*24*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("want 1 removed, got %d", removed)
	}
	if len(dirs) != 1 {
		t.Fatalf("want 1 dir reported, got %d", len(dirs))
	}
	if _, err := os.Stat(filepath.Join(root, old)); !os.IsNotExist(err) {
		t.Error("stale dir should be removed")
	}
	if _, err := os.Stat(filepath.Join(root, today)); err != nil {
		t.Error("fresh dir should be kept")
	}
}

func TestPruneDryRun(t *testing.T) {
	root := t.TempDir()

	old := time.Now().UTC().AddDate(0, 0, -40).Format("2006-01-02")
	writeFixture(t, root, old, "s", []Event{
		{EventID: "old", SessionID: "s", Ts: old + "T10:00:00Z", Op: OpQuery},
	})

	removed, _, err := Prune(root, 30*24*time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("want 1 would-remove, got %d", removed)
	}
	// dir must still exist
	if _, err := os.Stat(filepath.Join(root, old)); err != nil {
		t.Error("dry-run must not delete")
	}
}

func TestParseSince(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{"24h", 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"", 0, false},
		{"0d", 0, true},
		{"xd", 0, true},
		{"bad", 0, true},
	}
	for _, c := range cases {
		got, err := ParseSince(c.in)
		if c.err {
			if err == nil {
				t.Errorf("ParseSince(%q) expected error, got %v", c.in, got)
			}
		} else {
			if err != nil {
				t.Errorf("ParseSince(%q) unexpected error: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseSince(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}
