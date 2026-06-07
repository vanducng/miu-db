package erd

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestServe_RootReturnsHTML(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	html := "<html><body>test</body></html>"
	ready := make(chan string, 1)

	go func() {
		if err := Serve(ctx, html, ServeOpts{
			Port:    0,
			OnReady: func(url string) { ready <- url },
		}); err != nil {
			t.Errorf("Serve: %v", err)
		}
	}()

	var url string
	select {
	case url = <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not become ready")
	}

	resp, err := http.Get(url + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != html {
		t.Fatalf("body = %q, want %q", string(body), html)
	}
}

func TestServe_NonRootReturns404(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan string, 1)
	go func() {
		Serve(ctx, "hi", ServeOpts{Port: 0, OnReady: func(url string) { ready <- url }}) //nolint:errcheck
	}()

	url := <-ready

	resp, err := http.Get(url + "/other")
	if err != nil {
		t.Fatalf("GET /other: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestServe_CancelStopsServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, "hi", ServeOpts{Port: 0, OnReady: func(url string) { ready <- url }})
	}()

	<-ready
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned error after cancel: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after ctx cancel")
	}
}
