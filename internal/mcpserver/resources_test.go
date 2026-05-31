package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResourceListingIncludesDocumentedURIs(t *testing.T) {
	session := newMCPTestSession(t, testServices(t))
	ctx := context.Background()

	resources, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources.Resources) != 1 || resources.Resources[0].URI != "miudb://connections" {
		t.Fatalf("resources = %+v, want miudb://connections", resources.Resources)
	}

	templates, err := session.ListResourceTemplates(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tmpl := range templates.ResourceTemplates {
		got[tmpl.URITemplate] = true
	}
	for _, want := range []string{
		"miudb://connections/{name}",
		"miudb://connections/{name}/schema",
		"miudb://queries/{cursor}",
	} {
		if !got[want] {
			t.Fatalf("resource templates missing %q in %+v", want, templates.ResourceTemplates)
		}
	}
}

func TestReadConnectionResourcesAreRedacted(t *testing.T) {
	session := newMCPTestSession(t, testServices(t))
	for _, uri := range []string{"miudb://connections", "miudb://connections/local"} {
		t.Run(uri, func(t *testing.T) {
			res, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
			if err != nil {
				t.Fatal(err)
			}
			text := resourceText(t, res)
			if strings.Contains(text, "supersecret") {
				t.Fatalf("resource leaked secret: %s", text)
			}
			if !strings.Contains(text, `"local"`) {
				t.Fatalf("resource missing connection name: %s", text)
			}
		})
	}
}

func TestReadSchemaAndQueryResources(t *testing.T) {
	session := newMCPTestSession(t, testServices(t))
	ctx := context.Background()

	schema, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "miudb://connections/local/schema"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resourceText(t, schema), `"users"`) {
		t.Fatalf("schema resource missing users table: %s", resourceText(t, schema))
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
	cursor := decodeStructured[queryRunOutput](t, queryRes).NextCursor
	if cursor == "" {
		t.Fatal("query_run did not return cursor")
	}
	page, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "miudb://queries/" + cursor})
	if err != nil {
		t.Fatal(err)
	}
	var out queryFetchPageOutput
	if err := json.Unmarshal([]byte(resourceText(t, page)), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result.Rows) != 1 || out.Result.Rows[0][1] != "bob" {
		t.Fatalf("query resource rows = %+v, want bob row", out.Result.Rows)
	}
}

func resourceText(t *testing.T, res *mcp.ReadResourceResult) string {
	t.Helper()
	if len(res.Contents) != 1 {
		t.Fatalf("contents length = %d, want 1", len(res.Contents))
	}
	return res.Contents[0].Text
}
