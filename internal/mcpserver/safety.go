package mcpserver

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/vanducng/miu-db/internal/config"
)

type SafetyError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *SafetyError) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

type safetyPolicy struct {
	allowedConnections map[string]bool
	allowMutations     bool
	maxBytes           int
	cursorKey          []byte
}

func newSafetyPolicy(opts Options) safetyPolicy {
	allowed := map[string]bool{}
	for _, name := range opts.AllowedConnections {
		name = strings.TrimSpace(name)
		if name != "" {
			allowed[name] = true
		}
	}
	return safetyPolicy{
		allowedConnections: allowed,
		allowMutations:     opts.AllowMutations,
		maxBytes:           opts.MaxBytes,
		cursorKey:          newCursorKey(),
	}
}

func newCursorKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		sum := sha256.Sum256([]byte("miudb-mcp-cursor-fallback"))
		return sum[:]
	}
	return key
}

func (p safetyPolicy) connectionAllowed(name string) bool {
	if len(p.allowedConnections) == 0 {
		return true
	}
	return p.allowedConnections[name]
}

func (p safetyPolicy) filterConnections(conns []config.Connection) []config.Connection {
	out := []config.Connection{}
	for _, conn := range conns {
		if p.connectionAllowed(conn.Name) {
			out = append(out, conn)
		}
	}
	return out
}

func (p safetyPolicy) requireConnection(name string) error {
	if p.connectionAllowed(name) {
		return nil
	}
	return &SafetyError{
		Code:    "connection.not_allowed",
		Message: p.boundMessage(fmt.Sprintf("connection %q is not allowed for this MCP server", name)),
	}
}

func (p safetyPolicy) requireReadOnly(sqlText string) error {
	if p.allowMutations {
		return nil
	}
	if isReadOnlySQL(sqlText) {
		return nil
	}
	return &SafetyError{Code: "query.read_only_violation", Message: "MCP query_run is read-only by default; restart with --allow-mutate to permit mutations"}
}

// requireScriptMutation gates query_script. A read-only script always runs.
// A mutating script needs the server started with --allow-mutate AND an explicit
// per-call allow_mutate=true. It does NOT use isReadOnlySQL, which rejects any
// interior ';' (every multi-statement script has one).
func (p safetyPolicy) requireScriptMutation(sqlText string, allowMutate bool) error {
	if !scriptHasMutation(sqlText) {
		return nil
	}
	if !p.allowMutations {
		return &SafetyError{Code: "query.read_only_violation", Message: "MCP server is read-only; restart with --allow-mutate to permit mutating scripts"}
	}
	if !allowMutate {
		return &SafetyError{Code: "query.mutation_not_acknowledged", Message: "this script modifies data; set allow_mutate=true to run it"}
	}
	return nil
}

// scriptHasMutation conservatively flags a script as mutating if any mutation
// keyword appears as a token anywhere in it. Tokenizing the whole script (';'
// becomes whitespace) sidesteps the isReadOnlySQL ';' short-circuit; over-
// detection is safe-fail for a gate.
func scriptHasMutation(sqlText string) bool {
	return hasMutation(sqlTokens(sqlText))
}

func (p safetyPolicy) enforceBytes(value any) error {
	if p.maxBytes <= 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) <= p.maxBytes {
		return nil
	}
	return &SafetyError{
		Code:    "query.output_too_large",
		Message: p.boundMessage(fmt.Sprintf("MCP tool output is %d bytes, over max %d bytes", len(data), p.maxBytes)),
	}
}

func (p safetyPolicy) toolErr(code string, err error) error {
	if err == nil {
		return nil
	}
	return &SafetyError{Code: code, Message: p.boundMessage(config.RedactString(err.Error()))}
}

func (p safetyPolicy) boundMessage(message string) string {
	if p.maxBytes <= 0 {
		return message
	}
	max := p.maxBytes / 2
	if max <= 0 {
		max = 1
	}
	if max > 512 {
		max = 512
	}
	if len(message) <= max {
		return message
	}
	if max <= len("...[truncated]") {
		return message[:max]
	}
	return message[:max-len("...[truncated]")] + "...[truncated]"
}

func isReadOnlySQL(sqlText string) bool {
	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return false
	}
	trimmed = strings.TrimRight(trimmed, ";")
	if strings.Contains(trimmed, ";") {
		return false
	}
	fields := sqlTokens(trimmed)
	if len(fields) == 0 {
		return false
	}
	if hasMutation(fields) {
		return false
	}
	switch fields[0] {
	case "select", "show", "describe", "desc":
		return true
	case "with":
		return containsToken(fields, "select")
	default:
		return false
	}
}

func sqlTokens(sqlText string) []string {
	var b strings.Builder
	for _, r := range strings.ToLower(sqlText) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte(' ')
	}
	return strings.Fields(b.String())
}

func hasMutation(fields []string) bool {
	for _, field := range fields {
		switch field {
		case "insert", "update", "delete", "drop", "alter", "create", "replace", "truncate", "merge", "grant", "revoke", "call", "copy", "vacuum", "attach", "detach":
			return true
		}
	}
	return false
}

func containsToken(fields []string, token string) bool {
	for _, field := range fields {
		if field == token {
			return true
		}
	}
	return false
}

type mcpCursor struct {
	Connection string `json:"connection"`
	Cursor     string `json:"cursor"`
	Signature  string `json:"signature"`
}

func (p safetyPolicy) encodeToolCursor(connection string, cursor string) string {
	if cursor == "" {
		return ""
	}
	data, err := json.Marshal(mcpCursor{
		Connection: connection,
		Cursor:     cursor,
		Signature:  p.signCursor(connection, cursor),
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func (p safetyPolicy) decodeToolCursor(cursor string) (mcpCursor, error) {
	if cursor == "" {
		return mcpCursor{}, fmt.Errorf("missing cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return mcpCursor{}, fmt.Errorf("invalid cursor: %w", err)
	}
	var decoded mcpCursor
	if err := json.Unmarshal(data, &decoded); err != nil {
		return mcpCursor{}, fmt.Errorf("invalid cursor: %w", err)
	}
	if decoded.Connection == "" || decoded.Cursor == "" || decoded.Signature == "" {
		return mcpCursor{}, fmt.Errorf("invalid cursor")
	}
	if !p.validCursorSignature(decoded.Connection, decoded.Cursor, decoded.Signature) {
		return mcpCursor{}, fmt.Errorf("invalid cursor signature")
	}
	return decoded, nil
}

func (p safetyPolicy) signCursor(connection string, cursor string) string {
	mac := hmac.New(sha256.New, p.cursorKey)
	mac.Write([]byte(connection))
	mac.Write([]byte{0})
	mac.Write([]byte(cursor))
	return hex.EncodeToString(mac.Sum(nil))
}

func (p safetyPolicy) validCursorSignature(connection string, cursor string, sig string) bool {
	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(p.signCursor(connection, cursor))
	if err != nil {
		return false
	}
	return hmac.Equal(got, want)
}
