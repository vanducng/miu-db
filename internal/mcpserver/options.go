package mcpserver

import "time"

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
