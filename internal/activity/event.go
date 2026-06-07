package activity

// OpKind classifies the operation recorded in an Event.
type OpKind string

const (
	OpQuery  OpKind = "query"
	OpExec   OpKind = "exec"
	OpSchema OpKind = "schema"
	OpSmoke  OpKind = "smoke"
	OpERD    OpKind = "erd"
)

// EventError carries structured error info; never contains result rows.
type EventError struct {
	Class   string `json:"class,omitempty"`
	Message string `json:"message,omitempty"`
}

// Event is the immutable per-operation record written to JSONL.
// It captures SQL text + metadata only — result rows are NEVER stored.
type Event struct {
	EventID          string            `json:"event_id"`
	SessionID        string            `json:"session_id"`
	Ts               string            `json:"ts"`
	Source           string            `json:"source,omitempty"`
	MCPClient        string            `json:"mcp_client,omitempty"`
	Op               OpKind            `json:"op"`
	Connection       string            `json:"connection,omitempty"`
	Group            string            `json:"group,omitempty"`
	DBType           string            `json:"db_type,omitempty"`
	SQL              string            `json:"sql,omitempty"`
	SQLShape         string            `json:"sql_shape,omitempty"`
	SessionContext   map[string]string `json:"session_context,omitempty"`
	Limit            int               `json:"limit,omitempty"`
	LatencyMs        int64             `json:"latency_ms,omitempty"`
	RowsReturned     int               `json:"rows_returned,omitempty"`
	RowsAffected     int64             `json:"rows_affected,omitempty"`
	NextCursorIssued bool              `json:"next_cursor_issued,omitempty"`
	Error            *EventError       `json:"error,omitempty"`
	RetryOf          string            `json:"retry_of,omitempty"`
	MiuDBVersion     string            `json:"miudb_version,omitempty"`
}
