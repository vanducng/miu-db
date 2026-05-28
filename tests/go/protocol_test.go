package go_tests

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/adapters/sqlite"
	"github.com/vanducng/miu-db/internal/config"
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
