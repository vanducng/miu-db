package mysql

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
)

func TestBuildConfigAppliesSessionParams(t *testing.T) {
	conn := config.Connection{
		DBType:   "mysql",
		Endpoint: config.Endpoint{Host: "h", Username: "u", Password: "p", Database: "db"},
		Options:  map[string]any{"time_zone": "+00:00", "sql_mode": "TRADITIONAL"},
	}
	cfg := buildConfig(conn, "h", "3306")
	if cfg.Params["time_zone"] != "+00:00" || cfg.Params["sql_mode"] != "TRADITIONAL" {
		t.Fatalf("expected session params, got %v", cfg.Params)
	}
	if !strings.Contains(cfg.FormatDSN(), "time_zone") {
		t.Fatalf("expected time_zone in DSN, got %s", cfg.FormatDSN())
	}
}

func TestMySQLRejectsUnknownSessionKey(t *testing.T) {
	_, err := adapter.ApplySession(New(), config.Connection{DBType: "mysql"}, map[string]any{"bogus": "x"})
	usk, ok := err.(*adapter.UnsupportedSessionKeyError)
	if !ok {
		t.Fatalf("expected *adapter.UnsupportedSessionKeyError, got %T", err)
	}
	want := []string{"max_execution_time", "sql_mode", "time_zone"}
	if !reflect.DeepEqual(usk.Supported, want) {
		t.Fatalf("expected %v, got %v", want, usk.Supported)
	}
}
