// Package erd builds an interactive entity-relationship diagram from a database
// schema: introspect -> schema.json IR -> DBML export + self-contained HTML.
package erd

import "sort"

// Table is one entity in the IR. JSON tags match the schema.json contract the
// renderer consumes (shared with the diagram skill's er_html.py).
type Table struct {
	Name    string   `json:"table"`
	Rows    int64    `json:"rows"`
	PK      []string `json:"pk"`
	Columns []Column `json:"columns"`
	FKs     []FK     `json:"fks"`
	Indexes []Index  `json:"indexes"`
}

type Column struct {
	Name     string  `json:"column"`
	Type     string  `json:"type"` // DATA_TYPE / data_type
	UDT      string  `json:"udt"`  // COLUMN_TYPE / udt_name — what cards display
	Nullable string  `json:"nullable"`
	Default  *string `json:"default"`
	Ord      int     `json:"ord"`
}

type FK struct {
	Column     string `json:"column"`
	RefTable   string `json:"ref_table"`
	RefColumn  string `json:"ref_column"`
	OnDelete   string `json:"on_delete"`
	Constraint string `json:"constraint"`
	Inferred   bool   `json:"inferred,omitempty"` // declared-but-unenforced (Snowflake/BigQuery/heuristic)
}

type Index struct {
	Name string `json:"name"`
	Def  string `json:"def"`
}

type Group struct {
	Color  string   `json:"color"`
	Tables []string `json:"tables"`
}

// Meta is the optional agentic-polish layer; every field is optional.
type Meta struct {
	Title           string            `json:"title,omitempty"`
	DatabaseType    string            `json:"database_type,omitempty"`
	Groups          map[string]Group  `json:"groups,omitempty"`
	FrameworkTables []string          `json:"framework_tables,omitempty"`
	AuditColumns    []string          `json:"audit_columns,omitempty"`
	Classifications map[string]string `json:"classifications,omitempty"`
	Descriptions    map[string]string `json:"descriptions,omitempty"`
}

// Payload is the {schema, meta} document the renderer ingests.
type Payload struct {
	Schema []Table `json:"schema"`
	Meta   Meta    `json:"meta"`
}

// Normalize makes serialization deterministic regardless of introspection order:
// tables by name, columns by ordinal, indexes by name, FKs by (constraint, column)
// while preserving composite-key column order within a constraint.
func Normalize(schema []Table) {
	sort.SliceStable(schema, func(i, j int) bool { return schema[i].Name < schema[j].Name })
	for i := range schema {
		t := &schema[i]
		sort.SliceStable(t.Columns, func(a, b int) bool { return t.Columns[a].Ord < t.Columns[b].Ord })
		sort.SliceStable(t.Indexes, func(a, b int) bool { return t.Indexes[a].Name < t.Indexes[b].Name })
		sort.SliceStable(t.FKs, func(a, b int) bool {
			if t.FKs[a].Constraint != t.FKs[b].Constraint {
				return t.FKs[a].Constraint < t.FKs[b].Constraint
			}
			return false
		})
	}
}
