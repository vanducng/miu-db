package activity

// CaptureMeta carries per-invocation identity threaded from CLI/MCP entry
// points into the query chokepoint. Zero value is safe: logging is skipped.
type CaptureMeta struct {
	SessionID string
	Source    string // "cli" | "mcp"
	MCPClient string // best-effort; client name from the MCP initialize handshake
}
