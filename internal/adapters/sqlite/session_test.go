package sqlite

import (
	"testing"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
)

func TestSQLiteRejectsAllSessionKeys(t *testing.T) {
	_, err := adapter.ApplySession(New(), config.Connection{DBType: "sqlite"}, map[string]any{"role": "x"})
	usk, ok := err.(*adapter.UnsupportedSessionKeyError)
	if !ok {
		t.Fatalf("expected *adapter.UnsupportedSessionKeyError, got %T", err)
	}
	if len(usk.Supported) != 0 {
		t.Fatalf("sqlite should declare no session keys, got %v", usk.Supported)
	}
}
