package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/core"
)

func TestNewServerInitializesClient(t *testing.T) {
	services := core.NewServices(&config.Store{}, 0)
	server, err := New(services, Options{
		ImplementationName:    "miudb-test",
		ImplementationVersion: "v0.0.0-test",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clientSession.Close() })

	init := clientSession.InitializeResult()
	if init == nil || init.ServerInfo == nil {
		t.Fatal("missing initialize result server metadata")
	}
	if init.ServerInfo.Name != "miudb-test" {
		t.Fatalf("server name = %q, want miudb-test", init.ServerInfo.Name)
	}
	if init.ServerInfo.Version != "v0.0.0-test" {
		t.Fatalf("server version = %q, want v0.0.0-test", init.ServerInfo.Version)
	}
}

func TestServeRejectsUnsupportedTransportWithoutStdout(t *testing.T) {
	services := core.NewServices(&config.Store{}, 0)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Serve(context.Background(), services, Options{Transport: "bad"}, strings.NewReader(""), &stdout, &stderr)
	var unsupported *UnsupportedTransportError
	if !errors.As(err, &unsupported) {
		t.Fatalf("Serve error = %v, want UnsupportedTransportError", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty for startup diagnostics, got %q", stdout.String())
	}
}
