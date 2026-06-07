package erd

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// ServeOpts configures the ERD HTTP server.
type ServeOpts struct {
	Port    int              // 0 = auto-pick an available port
	OnReady func(url string) // called once the server is listening
}

// Serve starts a loopback HTTP server that serves html at GET /. It blocks
// until ctx is cancelled, then shuts down gracefully. OnReady is called with
// the actual listen URL before blocking.
func Serve(ctx context.Context, html string, opts ServeOpts) error {
	addr := fmt.Sprintf("127.0.0.1:%d", opts.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("erd serve: bind %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})

	srv := &http.Server{Handler: mux}

	actualURL := fmt.Sprintf("http://%s", ln.Addr().String())
	if opts.OnReady != nil {
		opts.OnReady(actualURL)
	}

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case <-ctx.Done():
		srv.Shutdown(context.Background()) //nolint:errcheck
		return nil
	case err := <-serveErr:
		return err
	}
}
