package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// JSON Schema 2020-12 lets a subschema be the bare boolean `true` (accept
// anything), and the SDK emits exactly that for Go fields typed `any`. MCP
// clients validate tools/list against a stricter model that requires an object
// in those positions, so a single `any` field makes the whole tool list fail to
// load — Claude Code reports "tools fetch failed — Invalid input (at
// tools.5.outputSchema.properties.result)" and the server exposes no tools at
// all despite connecting.
//
// `true` and `{}` accept the same instances, so rewriting one to the other is
// behaviour-preserving. Doing it in middleware rather than per-tool keeps the
// schemas generated from the Go types and covers any `any` field added later.
//
// `additionalProperties` is deliberately left alone: a boolean is both valid
// and meaningful there, and `false` (the common case here) means "no extra
// properties" — replacing it with `{}` would invert that into "any extra
// property is allowed".
var schemaChildKeys = map[string]bool{
	"properties":           true,
	"$defs":                true,
	"definitions":          true,
	"patternProperties":    true,
	"dependentSchemas":     true,
}

var schemaValueKeys = map[string]bool{
	"items":            true,
	"additionalItems":  true,
	"contains":         true,
	"propertyNames":    true,
	"if":               true,
	"then":             true,
	"else":             true,
	"not":              true,
	"unevaluatedItems": true,
}

var schemaListKeys = map[string]bool{
	"prefixItems": true,
	"allOf":       true,
	"anyOf":       true,
	"oneOf":       true,
}

// normalizeSchemaNode rewrites boolean subschemas to empty object schemas in
// every position where a client expects an object.
func normalizeSchemaNode(node any) any {
	switch typed := node.(type) {
	case bool:
		// Reached only through a position listed above, never through
		// additionalProperties. `false` here means "accept nothing", which has no
		// object spelling, so keep the permissive reading rather than invent one.
		return map[string]any{}
	case []any:
		for i, item := range typed {
			typed[i] = normalizeSchemaNode(item)
		}
		return typed
	case map[string]any:
		for key, value := range typed {
			switch {
			case schemaChildKeys[key]:
				if children, ok := value.(map[string]any); ok {
					for childKey, childValue := range children {
						children[childKey] = normalizeSchemaNode(childValue)
					}
				}
			case schemaValueKeys[key]:
				typed[key] = normalizeSchemaNode(value)
			case schemaListKeys[key]:
				if items, ok := value.([]any); ok {
					for i, item := range items {
						items[i] = normalizeSchemaNode(item)
					}
				}
			}
		}
		return typed
	default:
		return node
	}
}

// normalizeSchema round-trips a schema through JSON so it can be walked as
// plain maps regardless of the concrete type the SDK inferred. On any error the
// original is returned untouched: a schema we cannot parse is still better than
// no schema.
func normalizeSchema(schema any) any {
	if schema == nil {
		return nil
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return schema
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return schema
	}
	// A whole schema that is just `true` still has to be an object for clients
	// that require type "object" at the top level.
	if _, isBool := decoded.(bool); isBool {
		return map[string]any{}
	}
	return normalizeSchemaNode(decoded)
}

// schemaCompatMiddleware rewrites tool schemas on the way out of tools/list.
func schemaCompatMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, req)
		if err != nil || method != "tools/list" {
			return result, err
		}
		listed, ok := result.(*mcp.ListToolsResult)
		if !ok {
			return result, err
		}
		for _, tool := range listed.Tools {
			if tool == nil {
				continue
			}
			tool.InputSchema = normalizeSchema(tool.InputSchema)
			tool.OutputSchema = normalizeSchema(tool.OutputSchema)
		}
		return listed, nil
	}
}
