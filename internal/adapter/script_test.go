package adapter

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestStatementErrorRedactsMessage(t *testing.T) {
	e := statementError(2, errors.New("connect failed password=hunter2 to host"))
	if strings.Contains(e.Message, "hunter2") {
		t.Fatalf("expected password redacted, got %q", e.Message)
	}
	if e.Index != 2 || e.Code != "query.statement_failed" {
		t.Fatalf("unexpected statement error: %+v", e)
	}
}

func openScriptSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "script.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestUnsupportedScriptErrorMessage(t *testing.T) {
	e := &UnsupportedScriptError{DBType: "postgresql", Reason: "no NextResultSet"}
	if !strings.Contains(e.Error(), "postgresql") || !strings.Contains(e.Error(), "NextResultSet") {
		t.Fatalf("unexpected: %q", e.Error())
	}
	if got := (&UnsupportedScriptError{DBType: "x"}).Error(); !strings.Contains(got, "x") || strings.Contains(got, ":") {
		t.Fatalf("no-reason variant unexpected: %q", got)
	}
}

func TestRunScriptSQLSingleStatement(t *testing.T) {
	db := openScriptSQLite(t)
	sr, err := RunScriptSQL(context.Background(), db, "select 1 as a, 2 as b", 100, ScriptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sr.Statements) != 1 {
		t.Fatalf("want 1 statement, got %d", len(sr.Statements))
	}
	st := sr.Statements[0]
	if st.Kind != "rows" || st.RowCount != 1 || st.Index != 0 {
		t.Fatalf("unexpected statement: %+v", st)
	}
	if st.Result == nil || len(st.Result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %+v", st.Result)
	}
}

func TestRunScriptSQLTruncatesPerStatement(t *testing.T) {
	db := openScriptSQLite(t)
	if _, err := db.Exec("create table t(id integer); insert into t values (1),(2),(3)"); err != nil {
		t.Fatal(err)
	}
	sr, err := RunScriptSQL(context.Background(), db, "select id from t order by id", 2, ScriptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	st := sr.Statements[0]
	if !st.Truncated || st.RowCount != 2 {
		t.Fatalf("expected truncated at 2, got %+v", st)
	}
}

func TestRunScriptSQLStatementFailureIsCarriedInErrors(t *testing.T) {
	db := openScriptSQLite(t)
	sr, err := RunScriptSQL(context.Background(), db, "select * from does_not_exist", 100, ScriptOptions{})
	if err != nil {
		t.Fatalf("statement failure must NOT be a Go error: %v", err)
	}
	if len(sr.Errors) != 1 || sr.Errors[0].Code != "query.statement_failed" {
		t.Fatalf("expected one statement error, got %+v", sr.Errors)
	}
	if len(sr.Statements) != 0 {
		t.Fatalf("expected no statements collected, got %d", len(sr.Statements))
	}
}
