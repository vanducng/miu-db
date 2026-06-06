package go_tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	_ "modernc.org/sqlite"

	"github.com/vanducng/miu-db/internal/activity"
)

func TestMCPCommandTransportSQLiteFlow(t *testing.T) {
	bin := buildMiudb(t)
	configDir := writeMCPTestConfig(t)

	cmd := exec.Command(bin,
		"--config-dir", configDir,
		"--secret-source", "file",
		"mcp", "serve",
		"--transport", "stdio",
		"--connection", "local",
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "miudb-test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect mcp server: %v; stderr=%s", err, stderr.String())
	}
	t.Cleanup(func() { session.Close() })

	if init := session.InitializeResult(); init == nil || init.ServerInfo == nil || init.ServerInfo.Name != "miudb" {
		t.Fatalf("unexpected initialize result: %+v", init)
	}

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	gotNames := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		gotNames = append(gotNames, tool.Name)
	}
	wantNames := readToolsContract(t)
	slices.Sort(gotNames)
	slices.Sort(wantNames)
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("MCP tool contract mismatch\ngot:  %v\nwant: %v", gotNames, wantNames)
	}

	listRes, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "connections_list"})
	if err != nil {
		t.Fatal(err)
	}
	if listRes.IsError {
		t.Fatalf("connections_list error: %s", mustMarshalString(t, listRes.Content))
	}
	if !strings.Contains(mustMarshalString(t, listRes.StructuredContent), `"local"`) {
		t.Fatalf("connections_list missing local connection: %s", mustMarshalString(t, listRes.StructuredContent))
	}

	schemaRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "schema_tree",
		Arguments: map[string]any{"connection": "local"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if schemaRes.IsError || !strings.Contains(mustMarshalString(t, schemaRes.StructuredContent), `"users"`) {
		t.Fatalf("schema_tree unexpected result: %s", mustMarshalString(t, schemaRes))
	}

	queryRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "query_run",
		Arguments: map[string]any{
			"connection": "local",
			"sql":        "select id, name from users order by id",
			"limit":      1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if queryRes.IsError {
		t.Fatalf("query_run error: %s", mustMarshalString(t, queryRes.Content))
	}
	query := decodeMap(t, queryRes.StructuredContent)
	cursor, _ := query["next_cursor"].(string)
	if cursor == "" {
		t.Fatalf("query_run did not return next_cursor: %s", mustMarshalString(t, query))
	}
	assertQueryContract(t, query)

	pageRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query_fetch_page",
		Arguments: map[string]any{"cursor": cursor},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pageRes.IsError || !strings.Contains(mustMarshalString(t, pageRes.StructuredContent), `"bob"`) {
		t.Fatalf("query_fetch_page unexpected result: %s", mustMarshalString(t, pageRes))
	}

	assertActivityCaptured(t, configDir, "miudb-test-client")
}

// assertActivityCaptured verifies the MCP server wrote activity events under the
// --config-dir (not the real user dir), attributed them to the connected client,
// and never leaked result rows (the users table holds 'alice'/'bob').
func assertActivityCaptured(t *testing.T, configDir, wantClient string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(configDir, "activity", "*", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatalf("no activity jsonl written under %s/activity", configDir)
	}
	sawClient, sawQuery := false, false
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "alice") || strings.Contains(string(data), "bob") {
			t.Errorf("result rows leaked into activity log %s", m)
		}
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				continue
			}
			var e activity.Event
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				t.Fatalf("activity line not JSON: %v — %s", err, line)
			}
			if e.Source != "mcp" {
				t.Errorf("event source = %q, want mcp", e.Source)
			}
			if e.MCPClient == wantClient {
				sawClient = true
			}
			if e.Op == activity.OpQuery {
				sawQuery = true
			}
		}
	}
	if !sawClient {
		t.Errorf("no activity event carried mcp_client=%q", wantClient)
	}
	if !sawQuery {
		t.Errorf("expected at least one query event")
	}
}

func buildMiudb(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "miudb")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", bin, "./cmd/miudb")
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, string(out))
	}
	return bin
}

func writeMCPTestConfig(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "app.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("create table users (id integer primary key, name text); insert into users(name) values ('alice'), ('bob')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	configDir := t.TempDir()
	payload := map[string]any{
		"version": 1,
		"connections": []any{
			map[string]any{
				"name":    "local",
				"db_type": "sqlite",
				"endpoint": map[string]any{
					"kind": "file",
					"path": dbPath,
				},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "connections.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return configDir
}

func readToolsContract(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "contracts", "mcp-tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tools []string
	if err := json.Unmarshal(data, &tools); err != nil {
		t.Fatal(err)
	}
	return tools
}

func assertQueryContract(t *testing.T, got map[string]any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "contracts", "mcp-query-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	got["next_cursor"] = nil
	want["next_cursor"] = nil
	if mustMarshalString(t, got) != mustMarshalString(t, want) {
		t.Fatalf("query result contract mismatch\ngot:  %s\nwant: %s", mustMarshalString(t, got), mustMarshalString(t, want))
	}
}

func decodeMap(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func mustMarshalString(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
