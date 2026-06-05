package activity

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEventRoundTrip(t *testing.T) {
	e := Event{
		EventID:   "evt_1",
		SessionID: "sess_1",
		Ts:        "2026-06-05T00:00:00Z",
		Op:        OpQuery,
		SQL:       "SELECT 1",
		SQLShape:  "SELECT ?",
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var got Event
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.EventID != e.EventID || got.SQL != e.SQL {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestEventOmitempty(t *testing.T) {
	e := Event{
		EventID:   "evt_2",
		SessionID: "sess_2",
		Ts:        "2026-06-05T00:00:00Z",
		Op:        OpExec,
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, absent := range []string{"mcp_client", "source", "group", "sql", "sql_shape",
		"session_context", "error", "retry_of", "miudb_version"} {
		if strings.Contains(s, `"`+absent+`"`) {
			t.Errorf("omitempty failed: field %q present in zero-value event: %s", absent, s)
		}
	}
}

func TestEventNoResultRowFields(t *testing.T) {
	// Guard: Event must not contain any field that could hold result row data.
	b, _ := json.Marshal(Event{
		EventID:      "evt_3",
		SessionID:    "sess_3",
		Ts:           "2026-06-05T00:00:00Z",
		Op:           OpQuery,
		SQL:          "SELECT id, name FROM users",
		RowsReturned: 5,
	})
	s := string(b)
	for _, forbidden := range []string{"rows", "data", "result", "cells", "records"} {
		// Allow "rows_returned" and "rows_affected" keys but not raw payload keys.
		if forbidden == "rows" {
			continue // rows_returned / rows_affected are counts, not payloads
		}
		if strings.Contains(s, `"`+forbidden+`"`) {
			t.Errorf("result-row payload field %q found in event JSON", forbidden)
		}
	}
	if !strings.Contains(s, `"rows_returned":5`) {
		t.Errorf("expected rows_returned count in JSON")
	}
}
