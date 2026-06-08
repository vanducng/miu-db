package erd

import (
	"encoding/json"
	"testing"
)

// The fixture must survive a decode->encode->decode cycle without field loss.
func TestSchemaJSONRoundTrip(t *testing.T) {
	var schema []Table
	if err := json.Unmarshal(mustRead(t, "testdata/sample_schema.json"), &schema); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(schema) != 13 {
		t.Fatalf("want 13 tables, got %d", len(schema))
	}
	b, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var again []Table
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	// spot-check order_items (junction: has 2 FKs + 2 indexes) survived round-trip
	var oi *Table
	for i := range again {
		if again[i].Name == "order_items" {
			oi = &again[i]
		}
	}
	if oi == nil {
		t.Fatal("order_items missing after round-trip")
	}
	if len(oi.PK) == 0 || len(oi.Columns) == 0 || len(oi.FKs) == 0 || len(oi.Indexes) == 0 {
		t.Fatalf("lost fields: pk=%d cols=%d fks=%d idx=%d",
			len(oi.PK), len(oi.Columns), len(oi.FKs), len(oi.Indexes))
	}
	// spot-check categories (self-referential FK: parent_id -> categories.id)
	var cats *Table
	for i := range again {
		if again[i].Name == "categories" {
			cats = &again[i]
		}
	}
	if cats == nil {
		t.Fatal("categories missing after round-trip")
	}
	if len(cats.FKs) == 0 || cats.FKs[0].RefTable != "categories" {
		t.Fatalf("categories self-ref FK missing or wrong: %+v", cats.FKs)
	}
}

func TestNormalizeDeterministic(t *testing.T) {
	schema := []Table{
		{Name: "b", Columns: []Column{{Name: "y", Ord: 2}, {Name: "x", Ord: 1}}},
		{Name: "a", Columns: []Column{{Name: "q", Ord: 3}, {Name: "p", Ord: 1}}},
	}
	Normalize(schema)
	if schema[0].Name != "a" || schema[1].Name != "b" {
		t.Fatalf("tables not sorted: %s,%s", schema[0].Name, schema[1].Name)
	}
	if schema[1].Columns[0].Ord != 1 || schema[1].Columns[1].Ord != 2 {
		t.Fatalf("columns not sorted by ord: %+v", schema[1].Columns)
	}
	// idempotent
	before := schema[1].Columns[0].Name
	Normalize(schema)
	if schema[1].Columns[0].Name != before {
		t.Fatal("Normalize not idempotent")
	}
}
