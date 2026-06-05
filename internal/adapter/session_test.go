package adapter

import (
	"context"
	"reflect"
	"testing"

	"github.com/vanducng/miu-db/internal/config"
)

// keyedProvider implements Provider + SessionConfigurable with a declared key set.
type keyedProvider struct {
	typ  string
	keys []string
}

func (p keyedProvider) Type() string { return p.typ }
func (p keyedProvider) Open(context.Context, config.Connection) (*Session, error) {
	return nil, nil
}
func (p keyedProvider) Schema(context.Context, *Session) (any, error) { return nil, nil }
func (p keyedProvider) BuildSelect(string, int) string                { return "" }
func (p keyedProvider) SessionKeys() []string                         { return p.keys }

// plainProvider implements Provider but NOT SessionConfigurable.
type plainProvider struct{ typ string }

func (p plainProvider) Type() string { return p.typ }
func (p plainProvider) Open(context.Context, config.Connection) (*Session, error) {
	return nil, nil
}
func (p plainProvider) Schema(context.Context, *Session) (any, error) { return nil, nil }
func (p plainProvider) BuildSelect(string, int) string                { return "" }

func TestApplySessionOverlayDoesNotMutateSavedOptions(t *testing.T) {
	sf := keyedProvider{typ: "snowflake", keys: []string{"role", "warehouse", "schema", "database"}}
	saved := map[string]any{"warehouse": "Y"}
	conn := config.Connection{Name: "c", DBType: "snowflake", Options: saved}

	out, err := ApplySession(sf, conn, map[string]any{"role": "X"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Merge: clone carries both saved and session pairs.
	if out.Options["warehouse"] != "Y" || out.Options["role"] != "X" {
		t.Fatalf("expected merged options {warehouse:Y, role:X}, got %v", out.Options)
	}
	// Non-mutation: the original saved map is untouched (no role written back).
	if _, exists := saved["role"]; exists {
		t.Fatalf("saved options were mutated: %v", saved)
	}
	if len(saved) != 1 {
		t.Fatalf("saved options length changed: %v", saved)
	}
}

func TestApplySessionEmptyReturnsInputUnchanged(t *testing.T) {
	sf := keyedProvider{typ: "snowflake", keys: []string{"role"}}
	saved := map[string]any{"warehouse": "Y"}
	conn := config.Connection{DBType: "snowflake", Options: saved}

	out, err := ApplySession(sf, conn, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Identity: empty session must not clone Options (same underlying map).
	if reflect.ValueOf(out.Options).Pointer() != reflect.ValueOf(conn.Options).Pointer() {
		t.Fatal("empty session should not clone Options")
	}
	if out.Options["warehouse"] != "Y" {
		t.Fatalf("expected unchanged options, got %v", out.Options)
	}
}

func TestApplySessionUnknownKeyOnKeyedProvider(t *testing.T) {
	sf := keyedProvider{typ: "snowflake", keys: []string{"warehouse", "role", "schema", "database"}}
	conn := config.Connection{DBType: "snowflake"}

	_, err := ApplySession(sf, conn, map[string]any{"bogus": "x"})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	usk, ok := err.(*UnsupportedSessionKeyError)
	if !ok {
		t.Fatalf("expected *UnsupportedSessionKeyError, got %T", err)
	}
	if usk.Key != "bogus" || usk.DBType != "snowflake" {
		t.Fatalf("unexpected error fields: %+v", usk)
	}
	want := []string{"database", "role", "schema", "warehouse"} // sorted
	if !reflect.DeepEqual(usk.Supported, want) {
		t.Fatalf("expected sorted Supported %v, got %v", want, usk.Supported)
	}
}

func TestApplySessionProviderWithNoSessionKeys(t *testing.T) {
	lite := plainProvider{typ: "sqlite"}
	conn := config.Connection{DBType: "sqlite"}

	_, err := ApplySession(lite, conn, map[string]any{"role": "x"})
	if err == nil {
		t.Fatal("expected error for provider with no session keys")
	}
	usk, ok := err.(*UnsupportedSessionKeyError)
	if !ok {
		t.Fatalf("expected *UnsupportedSessionKeyError, got %T", err)
	}
	if len(usk.Supported) != 0 {
		t.Fatalf("expected empty Supported, got %v", usk.Supported)
	}
	if got := usk.Error(); got == "" {
		t.Fatal("error string should be non-empty")
	}
}

func TestApplySessionDeterministicUnknownKey(t *testing.T) {
	sf := keyedProvider{typ: "snowflake", keys: []string{"role"}}
	conn := config.Connection{DBType: "snowflake"}
	// Two unknown keys; sorted iteration must always report the first ("aaa").
	for i := 0; i < 20; i++ {
		_, err := ApplySession(sf, conn, map[string]any{"zzz": "1", "aaa": "2"})
		usk, ok := err.(*UnsupportedSessionKeyError)
		if !ok {
			t.Fatalf("expected *UnsupportedSessionKeyError, got %T", err)
		}
		if usk.Key != "aaa" {
			t.Fatalf("expected deterministic first key 'aaa', got %q", usk.Key)
		}
	}
}
