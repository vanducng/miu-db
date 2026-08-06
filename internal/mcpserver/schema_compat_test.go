package mcpserver

import (
	"encoding/json"
	"testing"
)

// findBooleanSubschemas reports every position where a client expecting an
// object schema would instead receive a bare boolean. additionalProperties is
// excluded: a boolean is legal and meaningful there.
func findBooleanSubschemas(node any, path string, found *[]string) {
	switch typed := node.(type) {
	case map[string]any:
		for key, value := range typed {
			switch {
			case schemaChildKeys[key]:
				if children, ok := value.(map[string]any); ok {
					for childKey, childValue := range children {
						childPath := path + "." + key + "." + childKey
						if _, isBool := childValue.(bool); isBool {
							*found = append(*found, childPath)
							continue
						}
						findBooleanSubschemas(childValue, childPath, found)
					}
				}
			case schemaValueKeys[key]:
				childPath := path + "." + key
				if _, isBool := value.(bool); isBool {
					*found = append(*found, childPath)
					continue
				}
				findBooleanSubschemas(value, childPath, found)
			case schemaListKeys[key]:
				if items, ok := value.([]any); ok {
					for i, item := range items {
						childPath := path + "." + key
						if _, isBool := item.(bool); isBool {
							*found = append(*found, childPath)
							continue
						}
						findBooleanSubschemas(item, childPath, found)
						_ = i
					}
				}
			}
		}
	}
}

func decode(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// The exact shape the SDK infers for an `any` field, which made every tool
// unloadable in Claude Code even though the server connected.
func TestNormalizeSchemaRewritesBooleanSubschemas(t *testing.T) {
	raw := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"result": true,
			"rows": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "array", "items": true},
			},
		},
		"additionalProperties": false,
	}

	var before []string
	findBooleanSubschemas(decode(t, raw), "root", &before)
	if len(before) == 0 {
		t.Fatal("fixture no longer reproduces the defect")
	}

	var after []string
	findBooleanSubschemas(decode(t, normalizeSchema(raw)), "root", &after)
	if len(after) != 0 {
		t.Fatalf("boolean subschemas survived: %v", after)
	}

	normalized, ok := decode(t, normalizeSchema(raw)).(map[string]any)
	if !ok {
		t.Fatal("normalized schema is not an object")
	}
	if got := normalized["additionalProperties"]; got != false {
		t.Fatalf("additionalProperties must stay false, got %v", got)
	}
	result, ok := normalized["properties"].(map[string]any)["result"].(map[string]any)
	if !ok || len(result) != 0 {
		t.Fatalf("properties.result should be an empty object schema, got %v", normalized["properties"])
	}
}

func TestNormalizeSchemaPreservesNilAndPlainSchemas(t *testing.T) {
	if got := normalizeSchema(nil); got != nil {
		t.Fatalf("nil schema should stay nil, got %v", got)
	}
	plain := map[string]any{"type": "string", "description": "d"}
	out, ok := decode(t, normalizeSchema(plain)).(map[string]any)
	if !ok || out["type"] != "string" || out["description"] != "d" {
		t.Fatalf("plain schema was altered: %v", out)
	}
}

// A whole schema that is just `true` still has to be an object.
func TestNormalizeSchemaHandlesTopLevelBoolean(t *testing.T) {
	out, ok := normalizeSchema(true).(map[string]any)
	if !ok || len(out) != 0 {
		t.Fatalf("top-level true should become {}, got %v", normalizeSchema(true))
	}
}
