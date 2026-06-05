package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/core"
	"github.com/vanducng/miu-db/internal/result"
)

func registerTools(server *mcp.Server, services *core.Services, opts Options, policy safetyPolicy) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "connections_list",
		Description: "List saved miudb connections with secrets redacted. Use this before selecting a connection for schema or query tools.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in emptyInput) (*mcp.CallToolResult, connectionsListOutput, error) {
		out := listConnections(services, policy)
		return nil, out, policy.enforceBytes(out)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "connection_describe",
		Description: "Describe one saved connection using redacted configuration metadata. Never returns passwords, credential file content, or raw secret values.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in connectionNameInput) (*mcp.CallToolResult, connectionDescribeOutput, error) {
		if err := policy.requireConnection(in.Name); err != nil {
			return nil, connectionDescribeOutput{}, err
		}
		conn, ok := services.Store.FindRaw(in.Name)
		if !ok {
			return nil, connectionDescribeOutput{}, policy.toolErr("connection.not_found", fmt.Errorf("connection %q not found", in.Name))
		}
		out := connectionDescribeOutput{
			Connection: config.RedactedConnection(conn),
			Store:      services.Store.Info(),
			Found:      true,
			Name:       conn.Name,
			DBType:     conn.DBType,
		}
		return nil, out, policy.enforceBytes(out)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "connection_test",
		Description: "Open one saved connection and report whether it is reachable. This may create tunnels but does not run user SQL.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in connectionNameInput) (*mcp.CallToolResult, connectionTestOutput, error) {
		if err := policy.requireConnection(in.Name); err != nil {
			return nil, connectionTestOutput{}, err
		}
		conn, err := services.TestConnection(ctx, in.Name)
		if err != nil {
			return nil, connectionTestOutput{}, policy.toolErr("connection.test_failed", err)
		}
		out := connectionTestOutput{Name: conn.Name, DBType: conn.DBType, OK: true}
		return nil, out, policy.enforceBytes(out)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "connections_smoke",
		Description: "Run a small bounded smoke query across saved connections or a named subset. Use sparingly because it may open several connections.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in connectionsSmokeInput) (*mcp.CallToolResult, connectionsSmokeOutput, error) {
		out := smokeConnections(ctx, services, opts, policy, in)
		return nil, out, policy.enforceBytes(out)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "schema_tree",
		Description: "Inspect tables, views, and columns for one saved connection. Schema payloads can be large; call only after choosing a connection.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in schemaTreeInput) (*mcp.CallToolResult, schemaTreeOutput, error) {
		if err := policy.requireConnection(in.Connection); err != nil {
			return nil, schemaTreeOutput{}, err
		}
		conn, data, err := services.SchemaTreeWithMeta(ctx, in.Connection, opts.activityMeta())
		if err != nil {
			return nil, schemaTreeOutput{}, policy.toolErr("schema.failed", err)
		}
		out := schemaTreeOutput{Connection: conn.Name, DBType: conn.DBType, Schema: data}
		return nil, out, policy.enforceBytes(out)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_run",
		Description: "Run SQL against one saved connection and return compact columns plus row arrays. Prefer small limits; use query_fetch_page for continuation cursors.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in queryRunInput) (*mcp.CallToolResult, queryRunOutput, error) {
		if err := policy.requireConnection(in.Connection); err != nil {
			return nil, queryRunOutput{}, err
		}
		if err := policy.requireReadOnly(in.SQL); err != nil {
			return nil, queryRunOutput{}, err
		}
		limit := normalizeLimit(opts, in.Limit)
		conn, outcome, err := services.RunQueryWithMeta(ctx, in.Connection, in.SQL, limit, nil, opts.activityMeta())
		if err != nil {
			return nil, queryRunOutput{}, policy.toolErr("query.failed", err)
		}
		out := queryRunOutput{
			Connection: conn.Name,
			DBType:     conn.DBType,
			Result:     outcome.Result,
			NextCursor: policy.encodeToolCursor(conn.Name, outcome.NextCursor),
			Limit:      limit,
		}
		return nil, out, policy.enforceBytes(out)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_fetch_page",
		Description: "Fetch the next page for a continuation cursor returned by query_run or a previous query_fetch_page call.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in queryFetchPageInput) (*mcp.CallToolResult, queryFetchPageOutput, error) {
		cursor, err := policy.decodeToolCursor(in.Cursor)
		if err != nil {
			return nil, queryFetchPageOutput{}, policy.toolErr("query.invalid_cursor", err)
		}
		if err := policy.requireConnection(cursor.Connection); err != nil {
			return nil, queryFetchPageOutput{}, err
		}
		page, err := services.FetchPage(cursor.Cursor)
		if err != nil {
			return nil, queryFetchPageOutput{}, policy.toolErr("query.fetch_page_failed", err)
		}
		out := queryFetchPageOutput{Result: page.Result, NextCursor: policy.encodeToolCursor(cursor.Connection, page.NextCursor)}
		return nil, out, policy.enforceBytes(out)
	})
}

func listConnections(services *core.Services, policy safetyPolicy) connectionsListOutput {
	items := []map[string]any{}
	byType := map[string]int{}
	for _, conn := range policy.filterConnections(services.Connections()) {
		items = append(items, config.RedactedConnection(conn))
		byType[conn.DBType]++
	}
	return connectionsListOutput{
		Connections: items,
		Count:       len(items),
		ByType:      byType,
		Store:       services.Store.Info(),
	}
}

func smokeConnections(ctx context.Context, services *core.Services, opts Options, policy safetyPolicy, in connectionsSmokeInput) connectionsSmokeOutput {
	sqlText := strings.TrimSpace(in.SQL)
	if sqlText == "" {
		sqlText = "select 1 as one"
	}
	limit := normalizeLimit(opts, in.Limit)
	wanted := map[string]bool{}
	for _, name := range in.Connections {
		wanted[name] = true
	}
	results := []smokeToolResult{}
	byType := map[string]map[string]int{}
	for _, raw := range services.Connections() {
		if len(wanted) > 0 && !wanted[raw.Name] {
			continue
		}
		if !policy.connectionAllowed(raw.Name) {
			continue
		}
		if err := policy.requireReadOnly(sqlText); err != nil {
			results = append(results, smokeFailure(policy, raw, err))
			continue
		}
		conn, err := services.Store.Resolve(raw)
		if err != nil {
			results = append(results, smokeFailure(policy, raw, err))
			continue
		}
		outcome, err := services.RunQueryConnectionMeta(ctx, conn, sqlText, limit, opts.activityMeta())
		if err != nil {
			results = append(results, smokeFailure(policy, conn, err))
			continue
		}
		rows := 0
		if qr, ok := outcome.Result.(result.QueryResult); ok {
			rows = len(qr.Rows)
		}
		results = append(results, smokeToolResult{Name: conn.Name, DBType: conn.DBType, OK: true, Rows: rows})
	}
	passed := 0
	for _, res := range results {
		if byType[res.DBType] == nil {
			byType[res.DBType] = map[string]int{"passed": 0, "failed": 0}
		}
		if res.OK {
			passed++
			byType[res.DBType]["passed"]++
		} else {
			byType[res.DBType]["failed"]++
		}
	}
	return connectionsSmokeOutput{
		Results: results,
		Count:   len(results),
		Passed:  passed,
		Failed:  len(results) - passed,
		ByType:  byType,
	}
}

func smokeFailure(policy safetyPolicy, conn config.Connection, err error) smokeToolResult {
	return smokeToolResult{
		Name:   conn.Name,
		DBType: conn.DBType,
		Error:  toolErrorFromError(policy, err),
	}
}

func toolErrorFromError(policy safetyPolicy, err error) *toolError {
	code := "internal.error"
	message := config.RedactString(err.Error())
	var safetyErr *SafetyError
	if errors.As(err, &safetyErr) {
		code = safetyErr.Code
		message = safetyErr.Message
	}
	return &toolError{Code: code, Message: policy.boundMessage(message)}
}

func normalizeLimit(opts Options, requested int) int {
	if requested <= 0 {
		return opts.DefaultLimit
	}
	if opts.MaxLimit > 0 && requested > opts.MaxLimit {
		return opts.MaxLimit
	}
	return requested
}
