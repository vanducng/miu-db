package mcpserver

import (
	"os"
	"time"

	"github.com/vanducng/miu-db/internal/activity"
)

const TransportStdio = "stdio"

type Options struct {
	Transport             string
	ImplementationName    string
	ImplementationVersion string
	AllowedConnections    []string
	DefaultLimit          int
	MaxLimit              int
	MaxBytes              int
	Timeout               time.Duration
	AllowMutations        bool
	// SessionID is minted once per server start; zero = no activity logging.
	SessionID string
}

// activityMeta returns a CaptureMeta for this server session.
func (o Options) activityMeta() activity.CaptureMeta {
	return activity.CaptureMeta{SessionID: o.SessionID, Source: "mcp"}
}

// activityEnabled reports whether activity logging is on for this server.
func activityEnabled() bool {
	return os.Getenv("MIUDB_ACTIVITY_LOG") != "off"
}

func (o Options) withDefaults() Options {
	if o.Transport == "" {
		o.Transport = TransportStdio
	}
	if o.ImplementationName == "" {
		o.ImplementationName = "miudb"
	}
	if o.ImplementationVersion == "" {
		o.ImplementationVersion = "dev"
	}
	if o.DefaultLimit <= 0 {
		o.DefaultLimit = 100
	}
	if o.MaxLimit <= 0 {
		o.MaxLimit = 1000
	}
	if o.DefaultLimit > o.MaxLimit {
		o.DefaultLimit = o.MaxLimit
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = 1 << 20
	}
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	return o
}
