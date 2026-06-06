package core

import (
	"errors"
	"testing"

	"github.com/vanducng/miu-db/internal/adapter"
)

func TestScriptCapabilityMatrix(t *testing.T) {
	reg := NewRegistry()

	for _, dbType := range []string{"postgresql", "sqlite", "bigquery"} {
		p, ok := reg.Get(dbType)
		if !ok {
			t.Fatalf("provider %q not registered", dbType)
		}
		_, err := adapter.ResolveScriptRunner(p)
		var u *adapter.UnsupportedScriptError
		if !errors.As(err, &u) {
			t.Fatalf("%s: expected *UnsupportedScriptError, got %v", dbType, err)
		}
		if u.Reason == "" {
			t.Fatalf("%s: expected an actionable rejection reason", dbType)
		}
	}

	for _, dbType := range []string{"snowflake", "mysql"} {
		p, ok := reg.Get(dbType)
		if !ok {
			t.Fatalf("provider %q not registered", dbType)
		}
		if _, err := adapter.ResolveScriptRunner(p); err != nil {
			t.Fatalf("%s should support scripts, got: %v", dbType, err)
		}
	}
}
