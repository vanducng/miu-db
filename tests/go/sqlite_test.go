package go_tests

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/adapters/sqlite"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/query"
	"github.com/vanducng/miu-db/internal/result"
)

func TestSQLiteQueryPath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("create table t (id integer primary key, name text); insert into t(name) values ('a'), ('b')"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	reg := adapter.NewRegistry()
	reg.Register(sqlite.New())
	svc := query.Service{Registry: reg, PageStore: result.NewPageStore(t.TempDir())}
	out, err := svc.Run(context.Background(), config.Connection{
		Name:   "local",
		DBType: "sqlite",
		Endpoint: config.Endpoint{
			Kind: "file",
			Path: dbPath,
		},
	}, "select id, name from t order by id", 1)
	if err != nil {
		t.Fatal(err)
	}
	if out.NextCursor == "" {
		t.Fatal("expected cursor for truncated result")
	}
	page, err := svc.FetchPage(out.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Result.Rows) != 1 {
		t.Fatalf("expected second page row, got %d", len(page.Result.Rows))
	}
}

func TestLegacyConfigLoadRedactsCredentials(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "connections.json"), []byte(`{"version":2,"connections":[{"name":"x","db_type":"sqlite","endpoint":{"kind":"file","path":"/tmp/x.db"},"tunnel":{"enabled":false}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "credentials-export.json"), []byte(`{"entries":[{"connection":"x","kind":"db","password":"secret"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := config.NewStore(dir, filepath.Join(dir, "credentials-export.json"))
	if err != nil {
		t.Fatal(err)
	}
	conn, ok := store.Find("x")
	if !ok {
		t.Fatal("missing connection")
	}
	if conn.Endpoint.Password != "secret" {
		t.Fatal("credential not applied")
	}
	redacted := config.RedactedConnection(conn)
	endpoint := redacted["endpoint"].(map[string]any)
	if endpoint["password"] != nil {
		t.Fatal("password leaked in redacted output")
	}
	if endpoint["has_password"] != true {
		t.Fatal("has_password flag missing")
	}
}
