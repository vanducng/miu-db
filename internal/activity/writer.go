package activity

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vanducng/miu-db/internal/config"
)

// Logger appends one compact JSON line per Event to {root}/{date}/{session_id}.jsonl.
// All operations are best-effort: errors are swallowed, panics are recovered.
type Logger struct {
	root    string
	enabled bool
	mu      sync.Mutex
}

// Options configures a Logger.
type Options struct {
	// Root overrides the default activity directory.
	Root string
	// Enabled can disable writing without removing the Logger from call sites.
	Enabled bool
}

// New returns a Logger. Enabled defaults to true.
func New(opts Options) *Logger {
	root := opts.Root
	if root == "" {
		root = filepath.Join(config.DefaultConfigDir(), "activity")
	}
	return &Logger{root: root, enabled: opts.Enabled}
}

// NewDefault returns a Logger with default root and enabled=true.
func NewDefault() *Logger {
	return New(Options{Enabled: true})
}

// SetEnabled toggles writing without replacing the Logger.
func (l *Logger) SetEnabled(v bool) {
	l.mu.Lock()
	l.enabled = v
	l.mu.Unlock()
}

// Log appends e as one JSON line. Best-effort: never panics, never returns an error
// that would affect the caller. Uses the event's own timestamp for date partitioning.
func (l *Logger) Log(e Event) {
	defer func() {
		if r := recover(); r != nil {
			slog.Debug("activity logger recovered from panic", "recover", r)
		}
	}()

	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.enabled {
		return
	}

	ts, err := time.Parse(time.RFC3339Nano, e.Ts)
	if err != nil {
		ts, err = time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			ts = time.Now().UTC()
		}
	}
	date := ts.Format("2006-01-02")

	sid := SanitizeID(e.SessionID)
	dir := filepath.Join(l.root, date)
	path := filepath.Join(dir, sid+".jsonl")

	line, err := json.Marshal(e)
	if err != nil {
		return
	}
	line = append(line, '\n')

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(line)
	_ = f.Close()
}

// Close is a no-op placeholder for future handle-cache cleanup.
func (l *Logger) Close() {}
