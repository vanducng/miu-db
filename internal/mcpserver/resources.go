package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vanducng/miu-db/internal/core"
)

const resourceMIMEJSON = "application/json"

func registerResources(server *mcp.Server, services *core.Services, policy safetyPolicy) {
	handler := resourceHandler(services, policy)
	server.AddResource(&mcp.Resource{
		URI:         "miudb://connections",
		Name:        "connections",
		Title:       "miudb connections",
		MIMEType:    resourceMIMEJSON,
		Description: "Redacted list of MCP-visible saved miudb connections.",
	}, handler)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "miudb://connections/{name}",
		Name:        "connection",
		Title:       "miudb connection",
		MIMEType:    resourceMIMEJSON,
		Description: "Redacted saved connection metadata by name.",
	}, handler)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "miudb://connections/{name}/schema",
		Name:        "connection schema",
		Title:       "miudb connection schema",
		MIMEType:    resourceMIMEJSON,
		Description: "Schema tree for one saved connection. Prefer schema_tree for bounded interactive calls.",
	}, handler)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "miudb://queries/{cursor}",
		Name:        "query page",
		Title:       "miudb query page",
		MIMEType:    resourceMIMEJSON,
		Description: "Continuation page for a cursor returned by query_run.",
	}, handler)
}

func resourceHandler(services *core.Services, policy safetyPolicy) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		switch {
		case uri == "miudb://connections":
			return jsonResource(uri, listConnections(services, policy), policy)
		case strings.HasPrefix(uri, "miudb://connections/") && strings.HasSuffix(uri, "/schema"):
			name, err := connectionNameFromSchemaURI(uri)
			if err != nil {
				return nil, policy.toolErr("resource.invalid_uri", err)
			}
			if err := policy.requireConnection(name); err != nil {
				return nil, err
			}
			conn, schema, err := services.SchemaTree(ctx, name)
			if err != nil {
				return nil, policy.toolErr("schema.failed", err)
			}
			return jsonResource(uri, schemaTreeOutput{Connection: conn.Name, DBType: conn.DBType, Schema: schema}, policy)
		case strings.HasPrefix(uri, "miudb://connections/"):
			name, err := connectionNameFromURI(uri)
			if err != nil {
				return nil, policy.toolErr("resource.invalid_uri", err)
			}
			if err := policy.requireConnection(name); err != nil {
				return nil, err
			}
			conn, ok := services.Store.FindRaw(name)
			if !ok {
				return nil, policy.toolErr("connection.not_found", fmt.Errorf("connection %q not found", name))
			}
			out := connectionDescribeOutput{
				Connection: redactedConnectionByName(services, policy, name),
				Store:      services.Store.Info(),
				Found:      true,
				Name:       conn.Name,
				DBType:     conn.DBType,
			}
			return jsonResource(uri, out, policy)
		case strings.HasPrefix(uri, "miudb://queries/"):
			cursorText, err := queryCursorFromURI(uri)
			if err != nil {
				return nil, policy.toolErr("resource.invalid_uri", err)
			}
			cursor, err := policy.decodeToolCursor(cursorText)
			if err != nil {
				return nil, policy.toolErr("query.invalid_cursor", err)
			}
			if err := policy.requireConnection(cursor.Connection); err != nil {
				return nil, err
			}
			page, err := services.FetchPage(cursor.Cursor)
			if err != nil {
				return nil, policy.toolErr("query.fetch_page_failed", err)
			}
			out := queryFetchPageOutput{Result: page.Result, NextCursor: policy.encodeToolCursor(cursor.Connection, page.NextCursor)}
			return jsonResource(uri, out, policy)
		default:
			return nil, mcp.ResourceNotFoundError(uri)
		}
	}
}

func jsonResource(uri string, value any, policy safetyPolicy) (*mcp.ReadResourceResult, error) {
	if err := policy.enforceBytes(value); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: resourceMIMEJSON,
			Text:     string(data),
		}},
	}, nil
}

func connectionNameFromURI(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	if name == "" || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid connection resource URI")
	}
	value, err := url.PathUnescape(name)
	if err != nil {
		return "", err
	}
	return value, nil
}

func connectionNameFromSchemaURI(raw string) (string, error) {
	trimmed := strings.TrimSuffix(raw, "/schema")
	return connectionNameFromURI(trimmed)
}

func queryCursorFromURI(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	cursor := strings.TrimPrefix(parsed.Path, "/")
	if cursor == "" || strings.Contains(cursor, "/") {
		return "", fmt.Errorf("invalid query resource URI")
	}
	value, err := url.PathUnescape(cursor)
	if err != nil {
		return "", err
	}
	return value, nil
}

func redactedConnectionByName(services *core.Services, policy safetyPolicy, name string) map[string]any {
	for _, conn := range listConnections(services, policy).Connections {
		if conn["name"] == name {
			return conn
		}
	}
	return map[string]any{}
}
