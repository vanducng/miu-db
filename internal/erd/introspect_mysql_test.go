package erd

import (
	"context"
	"database/sql"
	"encoding/json"
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

	ctx := context.Background()
	tables, err := Introspect(ctx, db, "mysql", "cnb_ai", nil)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	Normalize(tables)

	// Load the committed fixture for comparison.
	fixtureBytes := mustRead(t, "testdata/cnb_ai_schema.json")
	var fixture []Table
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	if len(tables) != len(fixture) {
		t.Fatalf("table count: got %d, want %d", len(tables), len(fixture))
	}

	// Deep-compare agent_template_variables (has FKs + composite indexes).
	checkTable(t, tables, "agent_template_variables", func(got Table) {
		if len(got.FKs) < 2 {
			t.Errorf("agent_template_variables: want >=2 FKs, got %d", len(got.FKs))
		}
		if len(got.Indexes) < 2 {
			t.Errorf("agent_template_variables: want >=2 indexes, got %d", len(got.Indexes))
		}
		if len(got.PK) == 0 || got.PK[0] != "id" {
			t.Errorf("agent_template_variables: want pk=[id], got %v", got.PK)
		}
	})

	// Deep-compare agent_templates (referenced by FKs above).
	checkTable(t, tables, "agent_templates", func(got Table) {
		if len(got.Columns) == 0 {
			t.Error("agent_templates: no columns returned")
		}
		if len(got.PK) == 0 {
			t.Error("agent_templates: no PK returned")
		}
	})
}

func checkTable(t *testing.T, tables []Table, name string, check func(Table)) {
	t.Helper()
	for _, tbl := range tables {
		if tbl.Name == name {
			check(tbl)
			return
		}
	}
	t.Errorf("table %q not found in introspected schema", name)
}
