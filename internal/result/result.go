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

type QueryPage struct {
	Result     QueryResult `json:"result"`
	NextCursor string      `json:"next_cursor,omitempty"`
}
