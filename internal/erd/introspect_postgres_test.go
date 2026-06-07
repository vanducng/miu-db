package erd

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestIntrospectPostgresIntegration requires a live Postgres instance.
// Set MIUDB_TEST_PG_DSN to run (e.g. "postgres://user:pass@localhost:5432/mydb").
func TestIntrospectPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("MIUDB_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("MIUDB_TEST_PG_DSN not set; skipping Postgres integration test")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	tables, err := introspectPostgres(ctx, db, "public", nil)
	if err != nil {
		t.Fatalf("introspectPostgres: %v", err)
	}
	Normalize(tables)

	if len(tables) < 1 {
		t.Fatalf("expected at least 1 table, got 0")
	}

	for _, tbl := range tables {
		for _, fk := range tbl.FKs {
			if fk.RefTable == "" {
				t.Errorf("table %s: FK %s has empty RefTable", tbl.Name, fk.Constraint)
			}
			if fk.Constraint == "" {
				t.Errorf("table %s: FK on column %s has empty Constraint", tbl.Name, fk.Column)
			}
			validOnDelete := map[string]bool{
				"CASCADE":     true,
				"SET NULL":    true,
				"SET DEFAULT": true,
				"RESTRICT":    true,
				"NO ACTION":   true,
			}
			if !validOnDelete[fk.OnDelete] {
				t.Errorf("table %s: FK %s has unexpected OnDelete=%q", tbl.Name, fk.Constraint, fk.OnDelete)
			}
		}
	}
}

// TestIntrospectUnsupportedNamesType verifies the error for unknown dbtypes
// includes both the sentinel and the type name. db can be nil — dispatch returns
// before any DB call.
func TestIntrospectUnsupportedNamesType(t *testing.T) {
	cases := []string{"bigquery", "snowflake", "duckdb", "oracle"}
	for _, dbtype := range cases {
		_, err := Introspect(context.Background(), nil, dbtype, "", nil)
		if err == nil {
			t.Errorf("dbtype=%s: expected error, got nil", dbtype)
			continue
		}
		if !errors.Is(err, ErrUnsupported) {
			t.Errorf("dbtype=%s: error does not wrap ErrUnsupported: %v", dbtype, err)
		}
		errStr := err.Error()
		found := false
		for i := 0; i < len(errStr)-len(dbtype)+1; i++ {
			if errStr[i:i+len(dbtype)] == dbtype {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("dbtype=%s: error %q does not mention the type name", dbtype, errStr)
		}
	}
}
