package activity

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func makeEvent(sessionID, ts string, op OpKind) Event {
	return Event{
		EventID:   "evt_" + sessionID,
		SessionID: sessionID,
		Ts:        ts,
		Op:        op,
		SQL:       "SELECT 1",
		SQLShape:  "SELECT ?",
	}
}

func TestLogWritesSingleEvent(t *testing.T) {
	root := t.TempDir()
	l := New(Options{Root: root, Enabled: true})

	ts := "2026-06-05T12:00:00Z"
	l.Log(makeEvent("sess1", ts, OpQuery))

	path := filepath.Join(root, "2026-06-05", "sess1.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	var got Event
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.SessionID != "sess1" {
		t.Errorf("session_id mismatch: %s", got.SessionID)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode: want 0o600, got %o", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Errorf("dir mode: want 0o700, got %o", dirInfo.Mode().Perm())
	}
}

func TestLogConcurrency(t *testing.T) {
	root := t.TempDir()
	l := New(Options{Root: root, Enabled: true})

	ts := "2026-06-05T10:00:00Z"
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			e := makeEvent("shared", ts, OpQuery)
			e.EventID = fmt.Sprintf("evt_%d", i)
			l.Log(e)
		}(i)
	}
	wg.Wait()

	path := filepath.Join(root, "2026-06-05", "shared.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	defer f.Close()

	count := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("line %d unmarshal failed: %v — content: %s", count, err, line)
		}
		count++
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if count != n {
		t.Errorf("expected %d lines, got %d", n, count)
	}
}

func TestLogDatePartition(t *testing.T) {
	root := t.TempDir()
	l := New(Options{Root: root, Enabled: true})

	l.Log(makeEvent("sess2", "2026-06-04T23:59:59Z", OpQuery))
	l.Log(makeEvent("sess2", "2026-06-05T00:00:00Z", OpQuery))

	path04 := filepath.Join(root, "2026-06-04", "sess2.jsonl")
	path05 := filepath.Join(root, "2026-06-05", "sess2.jsonl")

	if _, err := os.Stat(path04); err != nil {
		t.Errorf("expected file for 2026-06-04: %v", err)
	}
	if _, err := os.Stat(path05); err != nil {
		t.Errorf("expected file for 2026-06-05: %v", err)
	}
}

func TestLogDisabled(t *testing.T) {
	root := t.TempDir()
	l := New(Options{Root: root, Enabled: false})

	l.Log(makeEvent("sess3", time.Now().UTC().Format(time.RFC3339), OpExec))

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("disabled logger wrote %d entries", len(entries))
	}
}

func readEvents(t *testing.T, path string) []Event {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []Event
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		out = append(out, e)
	}
	return out
}

func TestLogRetryOfChaining(t *testing.T) {
	root := t.TempDir()
	l := New(Options{Root: root, Enabled: true})
	ts := "2026-06-05T10:00:00Z"

	ev := func(id, conn string, failed bool) Event {
		e := Event{EventID: id, SessionID: "s", Ts: ts, Op: OpQuery, Connection: conn}
		if failed {
			e.Error = &EventError{Class: "query_error", Message: "boom"}
		}
		return e
	}

	// connection "c": success, error, error, success, success
	l.Log(ev("e1", "c", false))
	l.Log(ev("e2", "c", true))
	l.Log(ev("e3", "c", true))
	l.Log(ev("e4", "c", false))
	l.Log(ev("e5", "c", false))
	// a different connection must not cross-link
	l.Log(ev("f1", "c2", true))
	l.Log(ev("e6", "c", false))

	want := map[string]string{
		"e1": "", "e2": "", "e3": "e2", "e4": "e3", "e5": "", "f1": "", "e6": "",
	}
	for _, e := range readEvents(t, filepath.Join(root, "2026-06-05", "s.jsonl")) {
		if w, ok := want[e.EventID]; ok && e.RetryOf != w {
			t.Errorf("%s: retry_of = %q, want %q", e.EventID, e.RetryOf, w)
		}
	}
}

func TestLogSanitizesTraversal(t *testing.T) {
	root := t.TempDir()
	l := New(Options{Root: root, Enabled: true})

	malicious := "../../../etc/passwd"
	ts := "2026-06-05T08:00:00Z"
	l.Log(makeEvent(malicious, ts, OpQuery))

	// Walk all created files and assert every one is under root.
	err := filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			t.Errorf("file escaped root: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Confirm the sanitized file actually exists under root.
	sanitized := SanitizeID(malicious)
	path := filepath.Join(root, "2026-06-05", sanitized+".jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("sanitized file not found at %s: %v", path, err)
	}
}
