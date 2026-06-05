package snowflake

import (
	"reflect"
	"strings"
	"testing"

	"github.com/snowflakedb/gosnowflake"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
)

func TestBuildConfigSetsRoleFromOptions(t *testing.T) {
	conn := config.Connection{
		DBType:   "snowflake",
		Endpoint: config.Endpoint{Host: "acct", Username: "U", Password: "P"},
		Options:  map[string]any{"role": "DBT_ANALYTICS_ROLE"},
	}
	cfg, err := buildConfig(conn)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Role != "DBT_ANALYTICS_ROLE" {
		t.Fatalf("expected role on config, got %q", cfg.Role)
	}
	// Role must reach the driver via the DSN (connect-time), not a statement.
	dsn, err := gosnowflake.DSN(cfg)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	if !strings.Contains(dsn, "DBT_ANALYTICS_ROLE") {
		t.Fatalf("expected DSN to carry the role, got %q", dsn)
	}
}

func TestBuildConfigPreservesWarehouseAndSetsRole(t *testing.T) {
	// Mirrors the saved-options-preserved case: ApplySession merges saved
	// warehouse with the per-call role, and buildConfig reads both.
	merged, err := adapter.ApplySession(New(),
		config.Connection{DBType: "snowflake", Options: map[string]any{"warehouse": "Y"}},
		map[string]any{"role": "X"},
	)
	if err != nil {
		t.Fatalf("ApplySession: %v", err)
	}
	cfg, err := buildConfig(merged)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Warehouse != "Y" || cfg.Role != "X" {
		t.Fatalf("expected warehouse=Y role=X, got warehouse=%q role=%q", cfg.Warehouse, cfg.Role)
	}
}

func TestBuildConfigDatabaseOverride(t *testing.T) {
	base := config.Connection{DBType: "snowflake", Endpoint: config.Endpoint{Database: "ENDPOINT_DB"}}

	// No override: endpoint database wins.
	cfg, _ := buildConfig(base)
	if cfg.Database != "ENDPOINT_DB" {
		t.Fatalf("expected endpoint database, got %q", cfg.Database)
	}

	// Override present: session/option database wins.
	base.Options = map[string]any{"database": "SESSION_DB"}
	cfg, _ = buildConfig(base)
	if cfg.Database != "SESSION_DB" {
		t.Fatalf("expected overridden database, got %q", cfg.Database)
	}
}

func TestSnowflakeRejectsUnknownSessionKey(t *testing.T) {
	conn := config.Connection{DBType: "snowflake"}
	_, err := adapter.ApplySession(New(), conn, map[string]any{"bogus": "x"})
	usk, ok := err.(*adapter.UnsupportedSessionKeyError)
	if !ok {
		t.Fatalf("expected *adapter.UnsupportedSessionKeyError, got %T (%v)", err, err)
	}
	want := []string{"database", "role", "schema", "warehouse"} // sorted
	if !reflect.DeepEqual(usk.Supported, want) {
		t.Fatalf("expected supported %v, got %v", want, usk.Supported)
	}
}

func TestSnowflakeSessionNonPersistence(t *testing.T) {
	saved := config.Connection{
		DBType:   "snowflake",
		Endpoint: config.Endpoint{Database: "DB"},
		Options:  map[string]any{}, // no saved role
	}

	// Call 1: role overlaid for this call only.
	c1, err := adapter.ApplySession(New(), saved, map[string]any{"role": "DBT_ANALYTICS_ROLE"})
	if err != nil {
		t.Fatalf("call1 ApplySession: %v", err)
	}
	cfg1, _ := buildConfig(c1)
	if cfg1.Role != "DBT_ANALYTICS_ROLE" {
		t.Fatalf("call1 expected role set, got %q", cfg1.Role)
	}

	// Call 2: no session → reverts to saved/default (empty), proving call 1
	// did not persist.
	c2, err := adapter.ApplySession(New(), saved, nil)
	if err != nil {
		t.Fatalf("call2 ApplySession: %v", err)
	}
	cfg2, _ := buildConfig(c2)
	if cfg2.Role != "" {
		t.Fatalf("call2 expected default role, got %q (role leaked across calls)", cfg2.Role)
	}

	// And the saved config itself was never mutated.
	if _, leaked := saved.Options["role"]; leaked {
		t.Fatalf("saved options were mutated: %v", saved.Options)
	}
}
