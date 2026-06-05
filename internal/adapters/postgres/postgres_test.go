package postgres

import (
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
)

func TestBuildDSNAppliesSessionOptions(t *testing.T) {
	conn := config.Connection{
		DBType:   "postgresql",
		Endpoint: config.Endpoint{Host: "h", Username: "u", Password: "p", Database: "db"},
		Options:  map[string]any{"role": "DBT", "search_path": "public", "application_name": "miudb"},
	}
	u, err := url.Parse(buildDSN(conn, "h", "5432"))
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	q := u.Query()
	if q.Get("application_name") != "miudb" {
		t.Fatalf("expected application_name=miudb, got query %v", q)
	}
	opts := q.Get("options")
	if !strings.Contains(opts, "-c role=DBT") || !strings.Contains(opts, "-c search_path=public") {
		t.Fatalf("expected -c role/search_path GUCs, got options=%q", opts)
	}
}

func TestBuildDSNEscapesSpacesInGUC(t *testing.T) {
	conn := config.Connection{
		DBType:   "postgresql",
		Endpoint: config.Endpoint{Host: "h", Username: "u", Password: "p", Database: "db"},
		Options:  map[string]any{"search_path": "schema one, schema two"},
	}
	u, err := url.Parse(buildDSN(conn, "h", "5432"))
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	opts := u.Query().Get("options")
	if !strings.Contains(opts, `search_path=schema\ one,`) {
		t.Fatalf("expected backslash-escaped space in GUC value, got %q", opts)
	}
	if strings.Count(opts, "-c ") != 1 {
		t.Fatalf("expected exactly one -c directive (no injection), got %q", opts)
	}
}

func TestBuildDSNOverlaysSessionOntoConnectionURL(t *testing.T) {
	conn := config.Connection{
		DBType:        "postgresql",
		ConnectionURL: "postgres://u@h:5432/db?sslmode=require",
		Options:       map[string]any{"role": "DBT", "application_name": "miudb"},
	}
	u, err := url.Parse(buildDSN(conn, "h", "5432"))
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	q := u.Query()
	if q.Get("application_name") != "miudb" {
		t.Fatalf("expected session applied onto connection_url, got %v", q)
	}
	if !strings.Contains(q.Get("options"), "-c role=DBT") {
		t.Fatalf("expected role GUC, got %q", q.Get("options"))
	}
	if q.Get("sslmode") != "require" {
		t.Fatalf("expected existing sslmode preserved, got %q", q.Get("sslmode"))
	}
}

func TestBuildDSNReturnsConnectionURLUnchangedWithoutSession(t *testing.T) {
	raw := "postgres://u@h:5432/db?sslmode=require"
	conn := config.Connection{DBType: "postgresql", ConnectionURL: raw}
	if got := buildDSN(conn, "h", "5432"); got != raw {
		t.Fatalf("expected raw connection_url unchanged, got %q", got)
	}
}

func TestPostgresRejectsUnknownSessionKey(t *testing.T) {
	_, err := adapter.ApplySession(New(), config.Connection{DBType: "postgresql"}, map[string]any{"bogus": "x"})
	usk, ok := err.(*adapter.UnsupportedSessionKeyError)
	if !ok {
		t.Fatalf("expected *adapter.UnsupportedSessionKeyError, got %T", err)
	}
	want := []string{"application_name", "role", "search_path", "statement_timeout"}
	if !reflect.DeepEqual(usk.Supported, want) {
		t.Fatalf("expected %v, got %v", want, usk.Supported)
	}
}
