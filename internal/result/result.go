package result

type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Nullable bool   `json:"nullable,omitempty"`
}

type QueryResult struct {
	Columns   []Column `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
}

// StatementResult is one statement's outcome within a query script. Kind is
// "rows" (data columns) or "exec" (status/affected-count set or zero columns).
type StatementResult struct {
	Index     int             `json:"index"`
	Kind      string          `json:"kind"`
	Result    *QueryResult    `json:"result,omitempty"`
	RowCount  int             `json:"row_count"`
	Truncated bool            `json:"truncated"`
	Error     *StatementError `json:"error,omitempty"`
}

// ScriptResult is the multi-statement result. JSON tags are real (not "-") so
// the protocol surface, which marshals the carrier verbatim, emits results/errors.
type ScriptResult struct {
	Statements []StatementResult `json:"results"`
	Errors     []StatementError  `json:"errors,omitempty"`
}

type StatementError struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type QueryPage struct {
	Result     QueryResult `json:"result"`
	NextCursor string      `json:"next_cursor,omitempty"`
}
