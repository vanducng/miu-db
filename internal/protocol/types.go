package protocol

type Request struct {
	JSONRPC string         `json:"jsonrpc,omitempty"`
	ID      any            `json:"id,omitempty"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc,omitempty"`
	ID      any         `json:"id,omitempty"`
	Result  any         `json:"result,omitempty"`
	Error   *ErrorReply `json:"error,omitempty"`
}

type ErrorReply struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Event struct {
	Type   string `json:"type,omitempty"`
	Event  string `json:"event"`
	CallID string `json:"call_id"`
	Seq    int    `json:"seq"`
	Data   any    `json:"data,omitempty"`
}
