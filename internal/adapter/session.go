package adapter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vanducng/miu-db/internal/config"
)

// SupportedSessionKeys returns the per-call session keys a provider declares,
// or nil if the provider does not implement SessionConfigurable.
func SupportedSessionKeys(p Provider) []string {
	if sc, ok := p.(SessionConfigurable); ok {
		return sc.SessionKeys()
	}
	return nil
}

// UnsupportedSessionKeyError is returned when a --session key is not declared by
// the target datasource's provider.
type UnsupportedSessionKeyError struct {
	Key       string
	DBType    string
	Supported []string
}

func (e *UnsupportedSessionKeyError) Error() string {
	if len(e.Supported) == 0 {
		return fmt.Sprintf("datasource %q accepts no session keys (got %q)", e.DBType, e.Key)
	}
	return fmt.Sprintf("datasource %q does not support session key %q (supported: %s)",
		e.DBType, e.Key, strings.Join(e.Supported, ", "))
}

// ApplySession validates session keys against the provider's declared set and
// returns a copy of conn with the validated pairs overlaid onto a clone of
// conn.Options. The input conn.Options map is never mutated, so the saved
// connection config is preserved and the overlay applies only to this call.
// Values are scalars (string/bool) per parseOptionValue, so a shallow map copy
// is sufficient.
func ApplySession(p Provider, conn config.Connection, session map[string]any) (config.Connection, error) {
	if len(session) == 0 {
		return conn, nil
	}

	supported := SupportedSessionKeys(p)
	allowed := make(map[string]bool, len(supported))
	for _, k := range supported {
		allowed[k] = true
	}

	sortedSupported := append([]string(nil), supported...)
	sort.Strings(sortedSupported)

	// Validate in sorted key order so the reported key is deterministic.
	keys := make([]string, 0, len(session))
	for k := range session {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !allowed[key] {
			return conn, &UnsupportedSessionKeyError{Key: key, DBType: p.Type(), Supported: sortedSupported}
		}
	}

	clone := make(map[string]any, len(conn.Options)+len(session))
	for k, v := range conn.Options {
		clone[k] = v
	}
	for _, k := range keys {
		clone[k] = session[k]
	}
	conn.Options = clone
	return conn, nil
}
