package mcpserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vanducng/miu-db/internal/core"
)

func New(services *core.Services, opts Options) (*mcp.Server, error) {
	return newServer(services, opts, nil)
}

func Serve(
	ctx context.Context,
	services *core.Services,
	opts Options,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if stdin == nil {
		return fmt.Errorf("stdin is required")
	}
	if stdout == nil {
		return fmt.Errorf("stdout is required")
	}
	if stderr == nil {
		stderr = io.Discard
	}
	opts = opts.withDefaults()
	transport, err := transportFor(opts.Transport, stdin, stdout)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	server, err := newServer(services, opts, logger)
	if err != nil {
		return err
	}
	return server.Run(ctx, transport)
}

func newServer(services *core.Services, opts Options, logger *slog.Logger) (*mcp.Server, error) {
	if services == nil {
		return nil, fmt.Errorf("core services are required")
	}
	opts = opts.withDefaults()
	server := mcp.NewServer(&mcp.Implementation{
		Name:    opts.ImplementationName,
		Version: opts.ImplementationVersion,
	}, &mcp.ServerOptions{
		Logger: logger,
	})
	policy := newSafetyPolicy(opts)
	registerTools(server, services, opts, policy)
	registerResources(server, services, policy)
	return server, nil
}
