package erd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func loadFixture(t *testing.T) ([]Table, Meta) {
	t.Helper()
	var schema []Table
	if err := json.Unmarshal(mustRead(t, "testdata/cnb_ai_schema.json"), &schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	var meta Meta
	if err := json.Unmarshal(mustRead(t, "testdata/cnb_ai_meta.json"), &meta); err != nil {
		t.Fatalf("meta: %v", err)
	}
	return schema, meta
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return b
}

// EmitDBML must be byte-identical to the diagram skill's er_html.py emit_dbml.
func TestEmitDBMLGoldenParity(t *testing.T) {
	schema, meta := loadFixture(t)
	got := EmitDBML(schema, meta)
	want := string(mustRead(t, "testdata/cnb_ai_expected.dbml"))
	if got != want {
		t.Fatalf("DBML mismatch with golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestEmitDBMLCompositeAndInferred(t *testing.T) {
	schema := []Table{{
		Name: "order_items",
		PK:   []string{"order_id", "sku"},
		Columns: []Column{
			{Name: "order_id", UDT: "bigint", Nullable: "NO", Ord: 1},
			{Name: "sku", UDT: "varchar(64)", Nullable: "NO", Ord: 2},
			{Name: "warehouse", UDT: "varchar(64)", Nullable: "YES", Ord: 3},
		},
		FKs: []FK{
			{Column: "order_id", RefTable: "orders", RefColumn: "id", OnDelete: "CASCADE", Constraint: "fk_o"},
			{Column: "sku", RefTable: "orders", RefColumn: "ref", OnDelete: "CASCADE", Constraint: "fk_o"},
			{Column: "warehouse", RefTable: "warehouses", RefColumn: "code", Constraint: "fk_w", Inferred: true},
		},
	}}
	got := EmitDBML(schema, Meta{Title: "shop", DatabaseType: "MySQL"})

	checks := []string{
		"database_type: 'MySQL'",
		"order_id bigint [pk]",  // pk wins over not null
		"warehouse varchar(64)", // nullable -> no attr
		"Ref: order_items.(order_id, sku) > orders.(id, ref) [delete: cascade]", // composite tuple
		"Ref: order_items.warehouse > warehouses.code [note: 'inferred']",       // inferred -> note
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("missing %q in:\n%s", c, got)
		}
	}
}
