package erd

import (
	"sort"
	"strings"
)

// Postgres UDT names normalize to DBML types; unknown names (incl. MySQL's
// "bigint unsigned", "varchar(255)") pass through verbatim.
var dbmlType = map[string]string{
	"int4": "int", "int8": "bigint", "int2": "smallint", "bool": "boolean", "varchar": "varchar",
	"text": "text", "float8": "float", "float4": "float", "numeric": "numeric", "jsonb": "jsonb",
	"json": "json", "uuid": "uuid", "bytea": "bytea", "timestamp": "timestamp", "timestamptz": "timestamptz",
	"date": "date", "time": "time", "vector": "vector", "tsvector": "tsvector",
}

var dbmlDelete = map[string]string{
	"CASCADE": "cascade", "SET NULL": "set null", "SET DEFAULT": "set default", "RESTRICT": "restrict",
}

type fkGroup struct {
	cols, rcols []string
	ref, od     string
	inferred    bool
}

// EmitDBML renders the schema as DBML (dbdiagram.io / dbdocs.io). Byte-compatible
// with the diagram skill's er_html.py emit_dbml: tables sorted by name, columns and
// composite-FK columns kept in input order, TableGroups by group name.
func EmitDBML(schema []Table, meta Meta) string {
	dbType := meta.DatabaseType
	if dbType == "" {
		dbType = "PostgreSQL"
	}
	title := meta.Title
	if title == "" {
		title = "database"
	}

	tables := make([]Table, len(schema))
	copy(tables, schema)
	sort.SliceStable(tables, func(i, j int) bool { return tables[i].Name < tables[j].Name })

	out := []string{"Project \"" + title + "\" {\n  database_type: '" + dbType + "'\n}\n"}

	for _, t := range tables {
		pk := make(map[string]bool, len(t.PK))
		for _, c := range t.PK {
			pk[c] = true
		}
		out = append(out, "Table "+t.Name+" {")
		for _, c := range t.Columns {
			ty, ok := dbmlType[c.UDT] // comma-ok mirrors Python .get(key, default): a future ""-mapping stays intentional
			if !ok {
				ty = firstNonEmpty(c.UDT, c.Type, "text")
			}
			seg := "  " + c.Name + " " + ty
			var attrs []string
			if pk[c.Name] {
				attrs = append(attrs, "pk")
			} else if c.Nullable == "NO" {
				attrs = append(attrs, "not null")
			}
			if len(attrs) > 0 {
				seg += " [" + strings.Join(attrs, ", ") + "]"
			}
			out = append(out, seg)
		}
		out = append(out, "}\n")
	}

	for _, t := range tables {
		var order []string
		groups := map[string]*fkGroup{}
		for _, f := range t.FKs {
			key := firstNonEmpty(f.Constraint, f.Column)
			g, ok := groups[key]
			if !ok {
				g = &fkGroup{ref: f.RefTable, od: f.OnDelete, inferred: f.Inferred}
				groups[key] = g
				order = append(order, key)
			}
			g.cols = append(g.cols, f.Column)
			g.rcols = append(g.rcols, f.RefColumn)
		}
		for _, key := range order {
			g := groups[key]
			var settings []string
			if d := dbmlDelete[strings.ToUpper(g.od)]; d != "" {
				settings = append(settings, "delete: "+d)
			}
			if g.inferred {
				settings = append(settings, "note: 'inferred'")
			}
			setting := ""
			if len(settings) > 0 {
				setting = " [" + strings.Join(settings, ", ") + "]"
			}
			lhs, rhs := t.Name+"."+g.cols[0], g.ref+"."+g.rcols[0]
			if len(g.cols) > 1 {
				lhs = t.Name + ".(" + strings.Join(g.cols, ", ") + ")"
				rhs = g.ref + ".(" + strings.Join(g.rcols, ", ") + ")"
			}
			out = append(out, "Ref: "+lhs+" > "+rhs+setting)
		}
	}

	if len(meta.Groups) > 0 {
		present := make(map[string]bool, len(tables))
		for _, t := range tables {
			present[t.Name] = true
		}
		names := make([]string, 0, len(meta.Groups))
		for n := range meta.Groups {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			var tbls []string
			for _, x := range meta.Groups[n].Tables {
				if present[x] {
					tbls = append(tbls, x)
				}
			}
			if len(tbls) > 0 {
				out = append(out, "\nTableGroup \""+n+"\" {\n  "+strings.Join(tbls, "\n  ")+"\n}")
			}
		}
	}

	return strings.Join(out, "\n") + "\n"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
