package activity_test

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/vanducng/miu-db/internal/activity"
	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/adapters/sqlite"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/query"
	"github.com/vanducng/miu-db/internal/result"
)

func makeTestDB(t *testing.T) (dbPath string, conn config.Connection) {
	t.Helper()
	dir := t.TempDir()
	dbPath = filepath.Join(dir, "int.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`create table rows (id integer primary key, val text);
		insert into rows values (1,'alpha'),(2,'beta'),(3,'gamma')`)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	conn = config.Connection{
		Name:     "int-sqlite",
		DBType:   "sqlite",
		Group:    "test",
		Endpoint: config.Endpoint{Kind: "file", Path: dbPath},
	}
	return dbPath, conn
}

func makeTestSvc(t *testing.T, logger *activity.Logger, dbPath string) query.Service {
	t.Helper()
	reg := adapter.NewRegistry()
	reg.Register(sqlite.New())
	return query.Service{
		Registry:  reg,
		PageStore: result.NewPageStore(t.TempDir()),
		Logger:    logger,
	}
}

func allJSONLLines(t *testing.T, root string) []string {
	t.Helper()
	var lines []string
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(root, e.Name(), "*.jsonl"))
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				if l != "" {
					lines = append(lines, l)
				}
			}
		}
	}
	return lines
}

// TestIntegrationSelectEventAndQuery runs a real SELECT, asserts the event file
// exists with correct fields, and confirms activity.Query can retrieve it.
func TestIntegrationSelectEventAndQuery(t *testing.T) {
	actRoot := filepath.Join(t.TempDir(), "activity")
	_, conn := makeTestDB(t)
	logger := activity.New(activity.Options{Root: actRoot, Enabled: true})
	svc := makeTestSvc(t, logger, conn.Endpoint.Path)
	sessionID := "int_sess_select"
	meta := activity.CaptureMeta{SessionID: sessionID, Source: "cli"}

	out, err := svc.Run(context.Background(), conn, "select id, val from rows", 100, meta)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	qr, ok := out.Result.(result.QueryResult)
	if !ok || len(qr.Rows) == 0 {
		t.Fatal("expected rows in result")
	}

	lines := allJSONLLines(t, actRoot)
	if len(lines) != 1 {
		t.Fatalf("expected 1 JSONL line, got %d", len(lines))
	}

	var ev activity.Event
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if ev.Op != activity.OpQuery {
		t.Errorf("op = %q, want query", ev.Op)
	}
	if ev.Connection != "int-sqlite" {
		t.Errorf("connection = %q, want int-sqlite", ev.Connection)
	}
	if ev.Group != "test" {
		t.Errorf("group = %q, want test", ev.Group)
	}
	if ev.DBType != "sqlite" {
		t.Errorf("db_type = %q, want sqlite", ev.DBType)
	}
	if ev.SQL == "" {
		t.Error("sql must be set")
	}
	if ev.SQLShape == "" {
		t.Error("sql_shape must be set")
	}
	if ev.RowsReturned != 3 {
		t.Errorf("rows_returned = %d, want 3", ev.RowsReturned)
	}
	if ev.LatencyMs < 0 {
		t.Error("latency_ms must be >= 0")
	}
	if ev.Error != nil {
		t.Errorf("unexpected error field: %+v", ev.Error)
	}

	// activity.Query must return the same event.
	events, qErr := activity.Query(actRoot, activity.Filter{Session: sessionID})
	if qErr != nil {
		t.Fatalf("activity.Query: %v", qErr)
	}
	if len(events) != 1 {
		t.Fatalf("activity.Query returned %d events, want 1", len(events))
	}
	if events[0].EventID != ev.EventID {
		t.Errorf("event_id mismatch: %s vs %s", events[0].EventID, ev.EventID)
	}
}

// TestIntegrationFailedQueryFilter runs a failing query then asserts --failed
// returns it but the success session does not appear.
func TestIntegrationFailedQueryFilter(t *testing.T) {
	actRoot := filepath.Join(t.TempDir(), "activity")
	_, conn := makeTestDB(t)
	logger := activity.New(activity.Options{Root: actRoot, Enabled: true})
	svc := makeTestSvc(t, logger, conn.Endpoint.Path)

	successSession := "int_sess_ok"
	_, err := svc.Run(context.Background(), conn, "select id from rows", 100,
		activity.CaptureMeta{SessionID: successSession, Source: "cli"})
	if err != nil {
		t.Fatalf("success run: %v", err)
	}

	failSession := "int_sess_fail"
	_, err = svc.Run(context.Background(), conn, "select * from no_such_table", 10,
		activity.CaptureMeta{SessionID: failSession, Source: "cli"})
	if err == nil {
		t.Fatal("expected error for bad query")
	}

	// --failed filter should return only the failed event.
	failed, qErr := activity.Query(actRoot, activity.Filter{FailedOnly: true})
	if qErr != nil {
		t.Fatal(qErr)
	}
	if len(failed) != 1 {
		t.Fatalf("failed filter: got %d events, want 1", len(failed))
	}
	if failed[0].SessionID != failSession {
		t.Errorf("wrong session in failed result: %s", failed[0].SessionID)
	}

	// Success session must not appear in the failed filter.
	for _, e := range failed {
		if e.SessionID == successSession {
			t.Error("success session leaked into --failed filter")
		}
	}

	// Without filter both events are present.
	all, _ := activity.Query(actRoot, activity.Filter{})
	if len(all) != 2 {
		t.Fatalf("all filter: got %d events, want 2", len(all))
	}
}

// TestIntegrationLeakCanary runs a real multi-row SELECT with known sentinel cell values
// and asserts those values are absent from every written JSONL line (HIGH leak guard).
func TestIntegrationLeakCanary(t *testing.T) {
	actRoot := filepath.Join(t.TempDir(), "activity")
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "canary.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// Sentinel values that must never appear in the activity log.
	_, err = db.Exec(`create table secrets (id integer primary key, secret text);
		insert into secrets values (1,'LEAK_CANARY_42'),(2,'LEAK_CANARY_99'),(3,'LEAK_CANARY_77')`)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	conn := config.Connection{
		Name:     "canary-sqlite",
		DBType:   "sqlite",
		Group:    "test",
		Endpoint: config.Endpoint{Kind: "file", Path: dbPath},
	}
	logger := activity.New(activity.Options{Root: actRoot, Enabled: true})
	svc := makeTestSvc(t, logger, dbPath)
	meta := activity.CaptureMeta{SessionID: "int_canary_sess", Source: "cli"}

	out, err := svc.Run(context.Background(), conn, "select id, secret from secrets", 100, meta)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	qr, ok := out.Result.(result.QueryResult)
	if !ok {
		t.Fatal("expected QueryResult")
	}
	// Confirm the query actually returned the sentinel rows (result is correct).
	if len(qr.Rows) != 3 {
		t.Fatalf("expected 3 sentinel rows in result, got %d", len(qr.Rows))
	}

	// Scan every JSONL byte and assert sentinel strings are absent.
	sentinels := []string{"LEAK_CANARY_42", "LEAK_CANARY_99", "LEAK_CANARY_77"}
	lines := allJSONLLines(t, actRoot)
	if len(lines) == 0 {
		t.Fatal("no JSONL lines written — event not recorded")
	}
	for _, line := range lines {
		for _, s := range sentinels {
			if strings.Contains(line, s) {
				t.Errorf("sentinel %q leaked into JSONL: %s", s, line)
			}
		}
		// Also assert no "rows" key (would indicate result payload).
		if strings.Contains(line, `"rows"`) {
			t.Errorf("'rows' key found in JSONL (result payload leak): %s", line)
		}
	}

	// rows_returned count is present and correct (metadata only).
	var ev activity.Event
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.RowsReturned != 3 {
		t.Errorf("rows_returned = %d, want 3", ev.RowsReturned)
	}
}

// TestIntegrationConcurrentSmoke runs N concurrent queries sharing one session ID
// and asserts every written line is valid JSON (-race compatible).
func TestIntegrationConcurrentSmoke(t *testing.T) {
	actRoot := filepath.Join(t.TempDir(), "activity")
	_, conn := makeTestDB(t)
	logger := activity.New(activity.Options{Root: actRoot, Enabled: true})
	svc := makeTestSvc(t, logger, conn.Endpoint.Path)

	const n = 20
	sessionID := "int_smoke_sess"
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			meta := activity.CaptureMeta{SessionID: sessionID, Source: "cli"}
			_, _ = svc.Run(context.Background(), conn, "select id from rows", 100, meta)
			_ = i
		}(i)
	}
	wg.Wait()

	// Every written line must be valid JSON.
	entries, err := os.ReadDir(actRoot)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(actRoot, e.Name(), "*.jsonl"))
		for _, f := range files {
			fh, err := os.Open(f)
			if err != nil {
				t.Fatal(err)
			}
			sc := bufio.NewScanner(fh)
			lineN := 0
			for sc.Scan() {
				l := sc.Text()
				if l == "" {
					continue
				}
				var v any
				if err := json.Unmarshal([]byte(l), &v); err != nil {
					t.Errorf("line %d in %s is not valid JSON: %v — %s", lineN, f, err, l)
				}
				lineN++
			}
			_ = fh.Close()
			if lineN != n {
				t.Errorf("expected %d lines in %s, got %d", n, f, lineN)
			}
		}
	}
}
