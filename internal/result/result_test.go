package result

import (
	"encoding/json"
	"testing"
)

func TestStatementResultJSONShape(t *testing.T) {
	rows := StatementResult{
		Index:    1,
		Kind:     "exec",
		Result:   &QueryResult{Columns: []Column{{Name: "status"}}, Rows: [][]any{{"ok"}}},
		RowCount: 1,
	}
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"index", "kind", "result", "row_count", "truncated"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("expected key %q in %s", k, b)
		}
	}
	if _, ok := m["error"]; ok {
		t.Fatalf("error must be omitted when nil: %s", b)
	}
}

func TestScriptResultUsesResultsAndErrorsKeys(t *testing.T) {
	// Errors empty -> omitted; key is "results" (NOT "statements").
	b, _ := json.Marshal(ScriptResult{Statements: []StatementResult{{Index: 0, Kind: "rows"}}})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if _, ok := m["results"]; !ok {
		t.Fatalf("expected 'results' key (verbatim protocol surface depends on it): %s", b)
	}
	if _, ok := m["errors"]; ok {
		t.Fatalf("errors must be omitted when empty: %s", b)
	}

	b2, _ := json.Marshal(ScriptResult{
		Statements: []StatementResult{{Index: 0, Kind: "rows"}},
		Errors:     []StatementError{{Index: 1, Code: "query.statement_failed", Message: "boom"}},
	})
	var m2 map[string]any
	_ = json.Unmarshal(b2, &m2)
	if _, ok := m2["errors"]; !ok {
		t.Fatalf("expected 'errors' key when present: %s", b2)
	}
}
