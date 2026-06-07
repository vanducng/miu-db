package erd

import (
	"encoding/json"
	"os"
	"testing"
)

func loadTestSchema(t *testing.T, path string) []Table {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema []Table
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	return schema
}

func TestBuildMetaStub_CNBAISchema(t *testing.T) {
	schema := loadTestSchema(t, "testdata/cnb_ai_schema.json")
	meta := BuildMetaStub(schema, "mysql")

	// DatabaseType mapping
	if meta.DatabaseType != "MySQL" {
		t.Errorf("DatabaseType = %q, want %q", meta.DatabaseType, "MySQL")
	}

	// AuditColumns
	wantAudit := []string{"created_at", "updated_at", "deleted_at"}
	if len(meta.AuditColumns) != len(wantAudit) {
		t.Errorf("AuditColumns len = %d, want %d", len(meta.AuditColumns), len(wantAudit))
	} else {
		for i, v := range wantAudit {
			if meta.AuditColumns[i] != v {
				t.Errorf("AuditColumns[%d] = %q, want %q", i, meta.AuditColumns[i], v)
			}
		}
	}

	// FrameworkTables: exactly the Laravel tables present in this schema
	wantFW := []string{
		"cache", "cache_locks", "failed_jobs", "job_batches", "jobs",
		"migrations", "password_reset_tokens", "sessions",
	}
	if len(meta.FrameworkTables) != len(wantFW) {
		t.Errorf("FrameworkTables = %v, want %v", meta.FrameworkTables, wantFW)
	} else {
		for i, v := range wantFW {
			if meta.FrameworkTables[i] != v {
				t.Errorf("FrameworkTables[%d] = %q, want %q", i, meta.FrameworkTables[i], v)
			}
		}
	}

	// Groups initialized as empty map (not nil)
	if meta.Groups == nil {
		t.Error("Groups is nil, want empty map")
	}
	if len(meta.Groups) != 0 {
		t.Errorf("Groups len = %d, want 0", len(meta.Groups))
	}

	// Descriptions contains domain tables only
	wantDomain := []string{
		"agent_template_variables", "agent_templates", "ai_agent_variable_values", "ai_agents", "users",
	}
	if len(meta.Descriptions) != len(wantDomain) {
		t.Errorf("Descriptions keys count = %d, want %d", len(meta.Descriptions), len(wantDomain))
	}
	for _, name := range wantDomain {
		if _, ok := meta.Descriptions[name]; !ok {
			t.Errorf("Descriptions missing domain table %q", name)
		}
	}
	// Framework tables must not appear in Descriptions
	for _, name := range meta.FrameworkTables {
		if _, ok := meta.Descriptions[name]; ok {
			t.Errorf("Descriptions contains framework table %q", name)
		}
	}
}

func TestBuildMetaStub_DBTypeMapping(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"mysql", "MySQL"},
		{"postgres", "PostgreSQL"},
		{"sqlite", "SQLite"},
		{"snowflake", "Snowflake"},
		{"bigquery", "BigQuery"},
		{"duckdb", "DuckDB"},
		{"MYSQL", "MySQL"},     // case-insensitive lookup
		{"unknown", "unknown"}, // passthrough
		{"Oracle", "Oracle"},   // passthrough preserves case
	}
	for _, tc := range cases {
		meta := BuildMetaStub(nil, tc.in)
		if meta.DatabaseType != tc.want {
			t.Errorf("dbtype %q: got %q, want %q", tc.in, meta.DatabaseType, tc.want)
		}
	}
}
