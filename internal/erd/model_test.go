package erd

import (
	"encoding/json"
	"testing"
)

// The fixture must survive a decode->encode->decode cycle without field loss.
func TestSchemaJSONRoundTrip(t *testing.T) {
	var schema []Table
	if err := json.Unmarshal(mustRead(t, "testdata/cnb_ai_schema.json"), &schema); err != nil {
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
	// spot-check a table with FKs + indexes survived
	var atv *Table
	for i := range again {
		if again[i].Name == "agent_template_variables" {
			atv = &again[i]
		}
	}
	if atv == nil {
		t.Fatal("agent_template_variables missing after round-trip")
	}
	if len(atv.PK) == 0 || len(atv.Columns) == 0 || len(atv.FKs) == 0 || len(atv.Indexes) == 0 {
		t.Fatalf("lost fields: pk=%d cols=%d fks=%d idx=%d",
			len(atv.PK), len(atv.Columns), len(atv.FKs), len(atv.Indexes))
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
