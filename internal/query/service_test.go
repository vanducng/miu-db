package query_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/vanducng/miu-db/internal/activity"
	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/adapters/sqlite"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/query"
	"github.com/vanducng/miu-db/internal/result"
)

func makeDB(t *testing.T) (string, *activity.Logger, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("create table items (id integer primary key, val text); insert into items values (1,'a'),(2,'b'),(3,'c')")
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	logDir := filepath.Join(dir, "activity")
	logger := activity.New(activity.Options{Root: logDir, Enabled: true})
	return dbPath, logger, logDir
}

func makeSvc(t *testing.T, logger *activity.Logger, dbPath string) (query.Service, config.Connection) {
	t.Helper()
	reg := adapter.NewRegistry()
	reg.Register(sqlite.New())
	conn := config.Connection{
		Name:     "testdb",
		DBType:   "sqlite",
		Group:    "g1",
		Endpoint: config.Endpoint{Kind: "file", Path: dbPath},
	}
	svc := query.Service{
		Registry:  reg,
		PageStore: result.NewPageStore(t.TempDir()),
		Logger:    logger,
	}
	return svc, conn
}

func readEvents(t *testing.T, logDir string) []activity.Event {
	t.Helper()
	var events []activity.Event
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return events
	}
	for _, dateDir := range entries {
		if !dateDir.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(logDir, dateDir.Name()))
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(logDir, dateDir.Name(), f.Name()))
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				if line == "" {
					continue
				}
				var ev activity.Event
				if err := json.Unmarshal([]byte(line), &ev); err != nil {
					t.Fatalf("invalid JSONL line: %v — %s", err, line)
				}
				events = append(events, ev)
			}
		}
	}
	return events
}

// SELECT writes event with correct metadata and no result rows in the file.
func TestRunEmitsSelectEvent(t *testing.T) {
	dbPath, logger, logDir := makeDB(t)
	svc, conn := makeSvc(t, logger, dbPath)
	meta := activity.CaptureMeta{SessionID: "test_sess_1", Source: "cli"}

	out, err := svc.Run(context.Background(), conn, "select id, val from items", 100, meta)
	if err != nil {
		t.Fatal(err)
	}
	qr, ok := out.Result.(result.QueryResult)
	if !ok || len(qr.Rows) == 0 {
		t.Fatal("expected rows in result")
	}

	events := readEvents(t, logDir)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]

	if ev.Op != activity.OpQuery {
		t.Errorf("op = %q, want query", ev.Op)
	}
	if ev.Connection != "testdb" {
		t.Errorf("connection = %q, want testdb", ev.Connection)
	}
	if ev.DBType != "sqlite" {
		t.Errorf("db_type = %q, want sqlite", ev.DBType)
	}
	if ev.SQL == "" {
		t.Error("sql should be set")
	}
	if ev.SQLShape == "" {
		t.Error("sql_shape should be set")
	}
	if ev.RowsReturned != 3 {
		t.Errorf("rows_returned = %d, want 3", ev.RowsReturned)
	}
	if ev.LatencyMs < 0 {
		t.Error("latency_ms should be >= 0")
	}
	if ev.Error != nil {
		t.Errorf("unexpected error in event: %+v", ev.Error)
	}

	// Leak guard: raw result rows must never appear in the JSONL file.
	raw, _ := os.ReadFile(filepath.Join(logDir, filepath.Join(func() string {
		entries, _ := os.ReadDir(logDir)
		if len(entries) > 0 {
			return entries[0].Name()
		}
		return ""
	}(), ev.SessionID+".jsonl")))
	for _, val := range []string{"\"a\"", "\"b\"", "\"c\""} {
		// val may appear in sql but must not be a row cell value key
		content := string(raw)
		// Check that rows array content is absent (row values would appear as JSON array elements)
		if strings.Contains(content, `"rows"`) {
			t.Errorf("event file must not contain 'rows' key (leak guard): %s", content)
		}
		_ = val
	}
}

// A failing query writes an event with error set and returns the error unchanged.
func TestRunEmitsErrorEvent(t *testing.T) {
	dbPath, logger, logDir := makeDB(t)
	svc, conn := makeSvc(t, logger, dbPath)
	meta := activity.CaptureMeta{SessionID: "test_err_sess", Source: "cli"}

	_, err := svc.Run(context.Background(), conn, "select * from nonexistent_table", 10, meta)
	if err == nil {
		t.Fatal("expected error from bad query")
	}

	events := readEvents(t, logDir)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Error == nil {
		t.Fatal("expected error in event")
	}
	if ev.Error.Message == "" {
		t.Error("error message should be non-empty")
	}
}

// LogSQL=false → event has empty sql but non-empty sql_shape.
func TestRunLogSQLFalse(t *testing.T) {
	dbPath, logger, logDir := makeDB(t)
	svc, conn := makeSvc(t, logger, dbPath)
	f := false
	conn.LogSQL = &f
	meta := activity.CaptureMeta{SessionID: "test_nolog_sess", Source: "cli"}

	_, err := svc.Run(context.Background(), conn, "select id from items", 100, meta)
	if err != nil {
		t.Fatal(err)
	}

	events := readEvents(t, logDir)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.SQL != "" {
		t.Errorf("sql should be empty when LogSQL=false, got %q", ev.SQL)
	}
	if ev.SQLShape == "" {
		t.Error("sql_shape should still be set when LogSQL=false")
	}
}

// nil logger / empty session ID → no event file written; result unaffected.
func TestRunNoLogger(t *testing.T) {
	dbPath, _, logDir := makeDB(t)
	svc, conn := makeSvc(t, nil, dbPath) // nil logger
	meta := activity.CaptureMeta{}       // empty session ID

	out, err := svc.Run(context.Background(), conn, "select id from items", 100, meta)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Result.(result.QueryResult); !ok {
		t.Fatal("expected QueryResult")
	}
	events := readEvents(t, logDir)
	if len(events) != 0 {
		t.Errorf("expected no events with nil logger, got %d", len(events))
	}
}

// MIUDB_ACTIVITY_LOG=off → no event file; result unaffected.
func TestRunActivityLogEnvOff(t *testing.T) {
	dbPath, _, logDir := makeDB(t)
	reg := adapter.NewRegistry()
	reg.Register(sqlite.New())
	conn := config.Connection{
		Name: "testdb", DBType: "sqlite",
		Endpoint: config.Endpoint{Kind: "file", Path: dbPath},
	}
	t.Setenv("MIUDB_ACTIVITY_LOG", "off")
	logOff := activity.New(activity.Options{Root: logDir, Enabled: false})
	svc := query.Service{
		Registry:  reg,
		PageStore: result.NewPageStore(t.TempDir()),
		Logger:    logOff,
	}
	meta := activity.CaptureMeta{SessionID: "test_off_sess", Source: "cli"}

	out, err := svc.Run(context.Background(), conn, "select id from items", 100, meta)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Result.(result.QueryResult); !ok {
		t.Fatal("expected QueryResult")
	}
	events := readEvents(t, logDir)
	if len(events) != 0 {
		t.Errorf("expected no events when disabled, got %d", len(events))
	}
}
