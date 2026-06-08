package erd

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// TestBuildIndexDef is pure-unit: no DB required.
func TestBuildIndexDef(t *testing.T) {
	cases := []struct {
		name   string
		cols   []string
		unique bool
		want   string
	}{
		{"idx_email", []string{"email"}, false, "INDEX idx_email (email)"},
		{"uq_user_slug", []string{"user_id", "slug"}, true, "UNIQUE INDEX uq_user_slug (user_id, slug)"},
		{"multi_col", []string{"a", "b", "c"}, false, "INDEX multi_col (a, b, c)"},
	}
	for _, c := range cases {
		got := buildIndexDef(c.name, c.cols, c.unique)
		if got != c.want {
			t.Errorf("buildIndexDef(%q, %v, %v) = %q, want %q", c.name, c.cols, c.unique, got, c.want)
		}
	}
}

// TestIntrospectMySQLIntegration requires a live MySQL instance.
// Set MIUDB_TEST_MYSQL_DSN to run (e.g. "root:pass@tcp(127.0.0.1:3306)/").
// Optionally set MIUDB_TEST_MYSQL_SCHEMA to target a specific schema name
// (default: empty string, which lets introspectMySQL resolve via DATABASE()).
func TestIntrospectMySQLIntegration(t *testing.T) {
	dsn := os.Getenv("MIUDB_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("MIUDB_TEST_MYSQL_DSN not set; skipping MySQL integration test")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	schema := os.Getenv("MIUDB_TEST_MYSQL_SCHEMA")
	ctx := context.Background()
	tables, err := Introspect(ctx, db, "mysql", schema, nil)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	Normalize(tables)

	// Structural sanity: at least one table was returned.
	if len(tables) < 1 {
		t.Fatal("expected at least 1 table from introspection")
	}

	// Every table must have at least one column.
	for _, tbl := range tables {
		if len(tbl.Columns) == 0 {
			t.Errorf("table %q has no columns", tbl.Name)
		}
	}

	// Every FK must carry non-empty Column, RefTable, RefColumn, and OnDelete.
	for _, tbl := range tables {
		for _, fk := range tbl.FKs {
			if fk.Column == "" || fk.RefTable == "" || fk.RefColumn == "" {
				t.Errorf("table %q: FK missing required fields: %+v", tbl.Name, fk)
			}
			if fk.OnDelete == "" {
				t.Errorf("table %q FK %q: OnDelete is empty", tbl.Name, fk.Constraint)
			}
		}
	}

	// PK detection: at least one table must have a non-empty PK slice.
	hasPK := false
	for _, tbl := range tables {
		if len(tbl.PK) > 0 {
			hasPK = true
			break
		}
	}
	if !hasPK {
		t.Error("no table had a PK detected — pk detection may be broken")
	}
}
