package mcpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	_ "modernc.org/sqlite"

	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/core"
)

func TestToolsListIncludesV1Tools(t *testing.T) {
	session := newMCPTestSession(t, testServices(t))
	list, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(list.Tools))
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	for _, want := range []string{
		"connections_list",
		"connection_describe",
		"connection_test",
		"connections_smoke",
		"schema_tree",
		"query_run",
		"query_fetch_page",
	} {
		if !slices.Contains(names, want) {
			t.Fatalf("tools missing %q in %v", want, names)
		}
	}
}

func TestConnectionToolsReturnRedactedConnections(t *testing.T) {
	session := newMCPTestSession(t, testServices(t))
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "connections_list"})
	if err != nil {
		t.Fatal(err)
	}
	out := decodeStructured[connectionsListOutput](t, res)
	if out.Count != 1 {
		t.Fatalf("count = %d, want 1", out.Count)
	}
	raw := mustJSON(t, out)
	if strings.Contains(raw, "supersecret") {
		t.Fatalf("connections_list leaked secret: %s", raw)
	}

	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "connection_describe",
		Arguments: map[string]any{"name": "local"},
	})
	if err != nil {
		t.Fatal(err)
	}
	describe := decodeStructured[connectionDescribeOutput](t, res)
	if describe.Name != "local" || describe.DBType != "sqlite" {
		t.Fatalf("unexpected connection description: %+v", describe)
	}
	if strings.Contains(mustJSON(t, describe), "supersecret") {
		t.Fatalf("connection_describe leaked secret: %s", mustJSON(t, describe))
	}
}

func TestSQLiteToolFlow(t *testing.T) {
	session := newMCPTestSession(t, testServices(t))
	ctx := context.Background()

	testRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "connection_test",
		Arguments: map[string]any{"name": "local"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeStructured[connectionTestOutput](t, testRes); !got.OK {
		t.Fatalf("connection_test OK = false: %+v", got)
	}

	schemaRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "schema_tree",
		Arguments: map[string]any{"connection": "local"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mustJSON(t, decodeStructured[schemaTreeOutput](t, schemaRes)), `"users"`) {
		t.Fatalf("schema_tree missing users table: %s", mustJSON(t, schemaRes.StructuredContent))
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
	queryOut := decodeStructured[queryRunOutput](t, queryRes)
	if queryOut.NextCursor == "" {
		t.Fatalf("query_run did not return cursor: %+v", queryOut)
	}
	if !strings.Contains(mustJSON(t, queryOut.Result), `"alice"`) {
		t.Fatalf("query_run missing first row: %s", mustJSON(t, queryOut.Result))
	}

	pageRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query_fetch_page",
		Arguments: map[string]any{"cursor": queryOut.NextCursor},
	})
	if err != nil {
		t.Fatal(err)
	}
	pageOut := decodeStructured[queryFetchPageOutput](t, pageRes)
	if len(pageOut.Result.Rows) != 1 || pageOut.Result.Rows[0][1] != "bob" {
		t.Fatalf("query_fetch_page rows = %+v, want bob row", pageOut.Result.Rows)
	}
}

func testServices(t *testing.T) *core.Services {
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
					"kind":     "file",
					"path":     dbPath,
					"password": "supersecret",
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
	store, err := config.NewStoreWithOptions(config.StoreOptions{
		Source:        config.SourceFile,
		ConfigDir:     configDir,
		SecretSources: []string{"file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	services := core.NewServices(store, 0)
	services.PageStore = nil
	return services
}

func newMCPTestSession(t *testing.T, services *core.Services) *mcp.ClientSession {
	t.Helper()
	return newMCPTestSessionWithOptions(t, services, Options{})
}

func newMCPTestSessionWithOptions(t *testing.T, services *core.Services, opts Options) *mcp.ClientSession {
	t.Helper()
	opts.ImplementationName = "miudb-test"
	opts.ImplementationVersion = "test"
	server, err := New(services, opts)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clientSession.Close() })
	return clientSession
}

func decodeStructured[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool returned error: %s", mustJSON(t, res.Content))
	}
	var out T
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode structured content %s: %v", string(data), err)
	}
	return out
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
