package query

import (
	"context"
	"errors"
	"testing"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/adapters/sqlite"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/result"
)

func TestRunScriptRejectsNonScriptProviderBeforeOpen(t *testing.T) {
	reg := adapter.NewRegistry()
	reg.Register(sqlite.New()) // sqlite does not implement ScriptRunner
	svc := Service{Registry: reg, PageStore: result.NewPageStore(t.TempDir())}

	// Path is intentionally bogus: a correct strict-opt-in dispatch rejects
	// BEFORE Open, so this file is never touched.
	conn := config.Connection{Name: "x", DBType: "sqlite", Endpoint: config.Endpoint{Kind: "file", Path: "/nonexistent/must-not-open.db"}}
	_, err := svc.RunScript(context.Background(), conn, "select 1; select 2", 100, adapter.ScriptOptions{})

	var u *adapter.UnsupportedScriptError
	if !errors.As(err, &u) {
		t.Fatalf("expected *adapter.UnsupportedScriptError, got %v", err)
	}
	if u.DBType != "sqlite" {
		t.Fatalf("expected dbtype sqlite, got %q", u.DBType)
	}
}
