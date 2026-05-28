package schema

type Table struct {
	Schema string `json:"schema,omitempty"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

type Column struct {
	Table  string `json:"table"`
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Schema string `json:"schema,omitempty"`
}

type Tree struct {
	Tables  []Table  `json:"tables"`
	Columns []Column `json:"columns,omitempty"`
}
