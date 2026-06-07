package erd

import "strings"

var dbtypeNames = map[string]string{
	"mysql":     "MySQL",
	"postgres":  "PostgreSQL",
	"sqlite":    "SQLite",
	"snowflake": "Snowflake",
	"bigquery":  "BigQuery",
	"duckdb":    "DuckDB",
}

// BuildMetaStub produces a Meta scaffold an agent can fill in.
// FrameworkTables and Descriptions are seeded deterministically from the schema.
func BuildMetaStub(schema []Table, dbtype string) Meta {
	dbName, ok := dbtypeNames[strings.ToLower(dbtype)]
	if !ok {
		dbName = dbtype
	}

	fw := DetectFrameworkTables(schema)
	fwSet := make(map[string]struct{}, len(fw))
	for _, n := range fw {
		fwSet[n] = struct{}{}
	}

	descs := make(map[string]string)
	for _, t := range schema {
		if _, skip := fwSet[t.Name]; !skip {
			descs[t.Name] = ""
		}
	}

	return Meta{
		DatabaseType:    dbName,
		AuditColumns:    []string{"created_at", "updated_at", "deleted_at"},
		FrameworkTables: fw,
		Groups:          map[string]Group{},
		Descriptions:    descs,
	}
}
