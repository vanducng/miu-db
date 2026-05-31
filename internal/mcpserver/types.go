package mcpserver

import (
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/result"
)

type emptyInput struct{}

type connectionsListOutput struct {
	Connections []map[string]any `json:"connections"`
	Count       int              `json:"count"`
	ByType      map[string]int   `json:"by_type"`
	Store       config.StoreInfo `json:"store"`
}

type connectionNameInput struct {
	Name string `json:"name" jsonschema:"The saved miudb connection name."`
}

type connectionDescribeOutput struct {
	Connection map[string]any    `json:"connection"`
	Store      config.StoreInfo  `json:"store"`
	Found      bool              `json:"found"`
	Name       string            `json:"name"`
	DBType     string            `json:"db_type,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type connectionTestOutput struct {
	Name   string `json:"name"`
	DBType string `json:"db_type"`
	OK     bool   `json:"ok"`
}

type connectionsSmokeInput struct {
	Connections []string `json:"connections,omitempty" jsonschema:"Optional saved connection names to smoke-test. Omit to test all connections."`
	SQL         string   `json:"sql,omitempty" jsonschema:"Optional smoke SQL. Defaults to select 1 as one."`
	Limit       int      `json:"limit,omitempty" jsonschema:"Maximum rows returned per smoke query."`
}

type connectionsSmokeOutput struct {
	Results []smokeToolResult         `json:"results"`
	Count   int                       `json:"count"`
	Passed  int                       `json:"passed"`
	Failed  int                       `json:"failed"`
	ByType  map[string]map[string]int `json:"by_type"`
}

type smokeToolResult struct {
	Name   string     `json:"name"`
	DBType string     `json:"db_type"`
	OK     bool       `json:"ok"`
	Rows   int        `json:"rows,omitempty"`
	Error  *toolError `json:"error,omitempty"`
}

type schemaTreeInput struct {
	Connection string `json:"connection" jsonschema:"Saved miudb connection name."`
}

type schemaTreeOutput struct {
	Connection string `json:"connection"`
	DBType     string `json:"db_type"`
	Schema     any    `json:"schema"`
}

type queryRunInput struct {
	Connection string `json:"connection" jsonschema:"Saved miudb connection name."`
	SQL        string `json:"sql" jsonschema:"SQL to execute against the saved connection."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum rows returned inline."`
}

type queryRunOutput struct {
	Connection string `json:"connection"`
	DBType     string `json:"db_type"`
	Result     any    `json:"result"`
	NextCursor string `json:"next_cursor,omitempty"`
	Limit      int    `json:"limit"`
}

type queryFetchPageInput struct {
	Cursor string `json:"cursor" jsonschema:"Continuation cursor returned by query_run or query_fetch_page."`
}

type queryFetchPageOutput struct {
	Result     result.QueryResult `json:"result"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

type toolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
