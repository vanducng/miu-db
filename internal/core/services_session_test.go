package core

import (
	"strings"
	"testing"

	"github.com/vanducng/miu-db/internal/adapter"
)

// No declared --session key may be an auth/secret option — session context is
// runtime-only; credentials must never be overridable per call.
func TestSessionKeysExcludeAuthAndSecrets(t *testing.T) {
	reg := NewRegistry()
	forbidden := []string{"password", "secret", "token", "private_key", "credential", "authenticator"}

	for _, dbType := range []string{"sqlite", "postgresql", "mysql", "snowflake", "bigquery"} {
		p, ok := reg.Get(dbType)
		if !ok {
			t.Fatalf("provider %q not registered", dbType)
		}
		for _, key := range adapter.SupportedSessionKeys(p) {
			lower := strings.ToLower(key)
			for _, bad := range forbidden {
				if strings.Contains(lower, bad) {
					t.Fatalf("%s session key %q looks like an auth/secret key", dbType, key)
				}
			}
		}
	}
}
