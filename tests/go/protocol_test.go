package go_tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/adapters/sqlite"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/core"
	"github.com/vanducng/miu-db/internal/protocol"
	"github.com/vanducng/miu-db/internal/result"
)

func TestProtocolServerInfoJSONRPC(t *testing.T) {
	store := &config.Store{}
	reg := adapter.NewRegistry()
	reg.Register(sqlite.New())
	server := protocol.Server{Store: store, Registry: reg, PageStore: result.NewPageStore(t.TempDir()), Protocol: "jsonrpc"}
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"server.info"}` + "\n")
	var out bytes.Buffer
	if err := server.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"jsonrpc":"2.0"`) {
		t.Fatalf("expected jsonrpc frame, got %s", out.String())
	}
}

func TestProtocolServerQueryAndFetchPage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("create table t (id integer primary key, name text); insert into t(name) values ('a'), ('b')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	configDir := t.TempDir()
	payload := `{"version":1,"connections":[{"name":"local","db_type":"sqlite","endpoint":{"kind":"file","path":` + strconvQuote(dbPath) + `}}]}`
	if err := os.WriteFile(filepath.Join(configDir, "connections.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := config.NewStoreWithOptions(config.StoreOptions{
		Source:        config.SourceFile,
		ConfigDir:     configDir,
		SecretSources: []string{"file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	services := core.NewServices(store, 0)
	services.PageStore = result.NewPageStore(t.TempDir())
	server := protocol.Server{Services: services, Protocol: "jsonrpc"}

	listResp := serveOne(t, server, `{"jsonrpc":"2.0","id":1,"method":"connection.list"}`)
	if listResp.Error != nil {
		t.Fatalf("connection.list error: %+v", listResp.Error)
	}
	listBytes, err := json.Marshal(listResp.Result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(listBytes), `"name":"local"`) {
		t.Fatalf("connection.list missing local connection: %s", string(listBytes))
	}

	queryResp := serveOne(t, server, `{"jsonrpc":"2.0","id":2,"method":"query.run","params":{"connection":"local","sql":"select id, name from t order by id","limit":1}}`)
	if queryResp.Error != nil {
		t.Fatalf("query.run error: %+v", queryResp.Error)
	}
	var queryResult struct {
		NextCursor string `json:"next_cursor"`
	}
	queryBytes, err := json.Marshal(queryResp.Result)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(queryBytes, &queryResult); err != nil {
		t.Fatal(err)
	}
	if queryResult.NextCursor == "" {
		t.Fatalf("query.run did not return a cursor: %s", string(queryBytes))
	}

	fetchResp := serveOne(t, server, `{"jsonrpc":"2.0","id":3,"method":"call.fetch_page","params":{"cursor":`+strconvQuote(queryResult.NextCursor)+`}}`)
	if fetchResp.Error != nil {
		t.Fatalf("call.fetch_page error: %+v", fetchResp.Error)
	}
	fetchBytes, err := json.Marshal(fetchResp.Result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fetchBytes), `"b"`) {
		t.Fatalf("fetch page missing second row: %s", string(fetchBytes))
	}
}

func serveOne(t *testing.T, server protocol.Server, frame string) protocol.Response {
	t.Helper()
	var out bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(frame+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	var resp protocol.Response
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response %q: %v", out.String(), err)
	}
	return resp
}

func strconvQuote(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}
