package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestQueryRunRejectsMutationsByDefault(t *testing.T) {
	session := newMCPTestSession(t, testServices(t))
	for _, sqlText := range []string{
		"insert into users(name) values ('mallory')",
		"update users set name = 'mallory'",
		"delete from users",
		"drop table users",
		"alter table users add column email text",
		"explain analyze delete from users",
		"pragma user_version = 1",
		"with x as (delete from users returning *) select * from x",
		"select 1; select 2",
	} {
		t.Run(sqlText, func(t *testing.T) {
			res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "query_run",
				Arguments: map[string]any{
					"connection": "local",
					"sql":        sqlText,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !res.IsError {
				t.Fatalf("query_run(%q) succeeded, want read-only error", sqlText)
			}
			if !strings.Contains(mustJSON(t, res.Content), "query.read_only_violation") {
				t.Fatalf("error content missing read-only code: %s", mustJSON(t, res.Content))
			}
		})
	}
}

func TestQueryRunAllowMutateFlag(t *testing.T) {
	session := newMCPTestSessionWithOptions(t, testServices(t), Options{AllowMutations: true})
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "query_run",
		Arguments: map[string]any{
			"connection": "local",
			"sql":        "insert into users(name) values ('mallory')",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("query_run with AllowMutations errored: %s", mustJSON(t, res.Content))
	}
}

func TestConnectionAllowlist(t *testing.T) {
	session := newMCPTestSessionWithOptions(t, testServices(t), Options{AllowedConnections: []string{"missing"}})
	listRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "connections_list"})
	if err != nil {
		t.Fatal(err)
	}
	list := decodeStructured[connectionsListOutput](t, listRes)
	if list.Count != 0 {
		t.Fatalf("allowlisted connections count = %d, want 0", list.Count)
	}

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "schema_tree",
		Arguments: map[string]any{"connection": "local"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("schema_tree for non-allowlisted connection succeeded")
	}
	if !strings.Contains(mustJSON(t, res.Content), "connection.not_allowed") {
		t.Fatalf("error content missing allowlist code: %s", mustJSON(t, res.Content))
	}
}

func TestFetchPageRequiresCursorConnectionAllowlist(t *testing.T) {
	services := testServices(t)
	openSession := newMCPTestSessionWithOptions(t, services, Options{})
	queryRes, err := openSession.CallTool(context.Background(), &mcp.CallToolParams{
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
	cursor := decodeStructured[queryRunOutput](t, queryRes).NextCursor
	if cursor == "" {
		t.Fatal("query_run did not return cursor")
	}

	restrictedSession := newMCPTestSessionWithOptions(t, services, Options{AllowedConnections: []string{"missing"}})
	fetchRes, err := restrictedSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "query_fetch_page",
		Arguments: map[string]any{"cursor": cursor},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fetchRes.IsError {
		t.Fatal("query_fetch_page succeeded for cursor from non-allowlisted connection")
	}
	if !strings.Contains(mustJSON(t, fetchRes.Content), "query.invalid_cursor") {
		t.Fatalf("error content missing invalid cursor code: %s", mustJSON(t, fetchRes.Content))
	}
}

func TestFetchPageRejectsForgedCursorConnection(t *testing.T) {
	session := newMCPTestSessionWithOptions(t, testServices(t), Options{AllowedConnections: []string{"local", "allowed"}})
	queryRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
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
	cursor := decodeStructured[queryRunOutput](t, queryRes).NextCursor
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatal(err)
	}
	var decoded mcpCursor
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	decoded.Connection = "allowed"
	forgedRaw, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	forged := base64.RawURLEncoding.EncodeToString(forgedRaw)

	fetchRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "query_fetch_page",
		Arguments: map[string]any{"cursor": forged},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fetchRes.IsError {
		t.Fatal("query_fetch_page accepted forged cursor connection")
	}
	if !strings.Contains(mustJSON(t, fetchRes.Content), "query.invalid_cursor") {
		t.Fatalf("error content missing invalid cursor code: %s", mustJSON(t, fetchRes.Content))
	}
}

func TestMaxBytesBoundsToolResponses(t *testing.T) {
	session := newMCPTestSessionWithOptions(t, testServices(t), Options{MaxBytes: 16})
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "connections_list"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("connections_list succeeded despite tiny max bytes")
	}
	if !strings.Contains(mustJSON(t, res.Content), "query.output_too_large") {
		t.Fatalf("error content missing output-too-large code: %s", mustJSON(t, res.Content))
	}
}

func TestSafetyRedactsToolErrors(t *testing.T) {
	policy := newSafetyPolicy(Options{MaxBytes: 128})
	err := policy.toolErr("query.failed", assertErr("postgres://app:supersecret@localhost/app password=hunter2"))
	if strings.Contains(err.Error(), "supersecret") || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("tool error leaked secret: %s", err.Error())
	}
}

func TestSafetyBoundsDirectSafetyErrors(t *testing.T) {
	policy := newSafetyPolicy(Options{AllowedConnections: []string{"allowed"}, MaxBytes: 32})
	longName := strings.Repeat("x", 1000)
	err := policy.requireConnection(longName)
	if err == nil {
		t.Fatal("expected disallowed connection error")
	}
	if strings.Contains(err.Error(), strings.Repeat("x", 100)) {
		t.Fatalf("connection error was not bounded: %s", err.Error())
	}
	oversizeErr := policy.enforceBytes(map[string]string{"value": strings.Repeat("x", 1000)})
	if oversizeErr == nil {
		t.Fatal("expected output-too-large error")
	}
	if len(oversizeErr.Error()) > 128 {
		t.Fatalf("output-too-large error too large: %d %q", len(oversizeErr.Error()), oversizeErr.Error())
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
