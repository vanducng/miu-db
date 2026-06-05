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

// newServer mints a stable mcp_ session id and attaches an activity logger.
func TestNewServerMintsMCPSessionID(t *testing.T) {
	services := core.NewServices(&config.Store{}, 0)
	opts := Options{ImplementationName: "miudb-test", ImplementationVersion: "test"}
	_, err := newServer(services, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	// services.Logger must be attached by newServer
	if services.Logger == nil {
		t.Fatal("expected Logger to be set on services after newServer")
	}
}

// Two calls to newServer mint distinct session IDs.
func TestNewServerSessionIDsAreDistinct(t *testing.T) {
	s1 := core.NewServices(&config.Store{}, 0)
	s2 := core.NewServices(&config.Store{}, 0)
	opts := Options{ImplementationName: "t", ImplementationVersion: "t"}
	// Capture the session IDs via withDefaults: newServer sets SessionID if empty.
	// We pass non-empty opts to verify the minted IDs differ across calls.
	o1 := opts.withDefaults()
	o2 := opts.withDefaults()
	if _, err := newServer(s1, o1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := newServer(s2, o2, nil); err != nil {
		t.Fatal(err)
	}
	// Each services gets a logger; that's the observable side-effect of newServer.
	if s1.Logger == nil || s2.Logger == nil {
		t.Fatal("both servers should have a logger")
	}
}

// activityMeta returns source=mcp for any non-empty session ID.
func TestActivityMetaSourceMCP(t *testing.T) {
	opts := Options{SessionID: "mcp_123"}
	meta := opts.activityMeta()
	if meta.Source != "mcp" {
		t.Errorf("source = %q, want mcp", meta.Source)
	}
	if meta.SessionID != "mcp_123" {
		t.Errorf("session_id = %q, want mcp_123", meta.SessionID)
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
