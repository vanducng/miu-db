package go_tests

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
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
	if err := os.WriteFile(filepath.Join(dir, "connections.json"), []byte(`{"version":2,"connections":[{"name":"x","db_type":"postgresql","endpoint":{"kind":"tcp","host":"localhost","port":"5432","database":"x","username":"x"},"tunnel":{"enabled":false}}]}`), 0o600); err != nil {
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

func TestDefaultConfigDirUsesMiuProductPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("MIUDB_CONFIG_DIR", "")
	t.Setenv("MIU_DB_CONFIG_DIR", "")
	want := filepath.Join(dir, ".config", "miu", "db")
	if got := config.DefaultConfigDir(); got != want {
		t.Fatalf("DefaultConfigDir() = %q, want %q", got, want)
	}
}

func TestLegacyCredentialExportFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "connections.json"), []byte(`{"version":2,"connections":[{"name":"x","db_type":"postgresql","endpoint":{"kind":"tcp","host":"localhost","port":"5432","database":"x","username":"x"},"tunnel":{"enabled":false}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "credentials-export.json"), []byte(`{"entries":[{"connection":"x","kind":"db","password":"secret"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := config.NewStore(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	conn, ok := store.Find("x")
	if !ok {
		t.Fatal("missing connection")
	}
	if conn.Endpoint.Password != "secret" {
		t.Fatal("legacy credential export fallback not applied")
	}
	if store.Info().CredentialsPath != filepath.Join(dir, "credentials-export.json") {
		t.Fatal("store info should report legacy credential export path")
	}
}

func TestConnectionAddStoresSensitiveFieldsOutsideConnectionFile(t *testing.T) {
	dir := t.TempDir()
	store, err := config.NewWritableStore(config.StoreOptions{
		Source:        config.SourceFile,
		ConfigDir:     dir,
		SecretSources: []string{"file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.Add(config.Connection{
		Name:   "pg",
		DBType: "postgresql",
		Endpoint: config.Endpoint{
			Kind:     "tcp",
			Host:     "localhost",
			Port:     "5432",
			Database: "app",
			Username: "app",
			Password: "supersecret",
		},
	}, config.AddOptions{SecretStore: "file"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Endpoint.Password != "" {
		t.Fatal("password should not be persisted inline")
	}
	if len(config.SecretRefsFor(saved, "db")) != 1 {
		t.Fatal("expected database secret ref")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "supersecret") {
		t.Fatal("connection file leaked password")
	}
	creds, err := config.LoadCredentialFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	if creds["pg:db"] != "supersecret" {
		t.Fatal("credential file missing password")
	}
	if !strings.Contains(strings.Join(store.Info().SecretSources, ","), "file") {
		t.Fatal("store info should include file secret source after file secret add")
	}
}

func TestRedactStringMasksSecrets(t *testing.T) {
	input := "postgres://app:supersecret@localhost/app password=hunter2 token=abc"
	got := config.RedactString(input)
	if strings.Contains(got, "supersecret") || strings.Contains(got, "hunter2") || strings.Contains(got, "abc") {
		t.Fatalf("secret leaked after redaction: %s", got)
	}
}
