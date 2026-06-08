package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestContractExamplesValidate(t *testing.T) {
	cases := []struct {
		schema  string
		example string
	}{
		{"tests/testdata/schemas/query-result-v1.schema.json", "tests/testdata/contracts/query-result.json"},
		{"tests/testdata/schemas/script-result-v1.schema.json", "tests/testdata/contracts/script-result.json"},
		{"tests/testdata/schemas/connection-smoke-v1.schema.json", "tests/testdata/contracts/connection-smoke.json"},
		{"tests/testdata/schemas/protocol-jsonrpc-v1.schema.json", "tests/testdata/contracts/protocol-jsonrpc-event.json"},
		{"tests/testdata/schemas/protocol-ndjson-v1.schema.json", "tests/testdata/contracts/protocol-ndjson-event.json"},
	}
	root := repoRoot(t)
	for _, tc := range cases {
		t.Run(tc.example, func(t *testing.T) {
			compiler := jsonschema.NewCompiler()
			schemaPath := filepath.Join(root, tc.schema)
			schema, err := compiler.Compile("file://" + schemaPath)
			if err != nil {
				t.Fatalf("compile schema: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(root, tc.example))
			if err != nil {
				t.Fatalf("read example: %v", err)
			}
			var doc any
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatalf("parse example: %v", err)
			}
			if err := schema.Validate(doc); err != nil {
				t.Fatalf("validate example: %v", err)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("repo root not found")
		}
		dir = next
	}
}
