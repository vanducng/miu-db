package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
)

const closeTabHTML = `<html><body><p>Authentication successful. You may close this tab.</p></body></html>`

// startLoopback binds to the host:port from redirectURI, waits for exactly one
// callback hit on redirectURI.Path, validates state, then shuts down.
// Returns channels for code and error; exactly one will receive a value.
func startLoopback(ctx context.Context, redirectURI *url.URL, expectedState string) (<-chan string, <-chan error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	host := redirectURI.Host
	if host == "" {
		host = "localhost:8085"
	}
	callbackPath := redirectURI.Path
	if callbackPath == "" {
		callbackPath = "/"
	}

	// Security: only allow loopback addresses to prevent redirect capture by LAN hosts.
	if err := validateLoopbackHost(host); err != nil {
		errCh <- err
		return codeCh, errCh
	}

	ln, err := net.Listen("tcp", host)
	if err != nil {
		errCh <- fmt.Errorf("auth: cannot bind loopback %s (port in use?): %w", host, err)
		return codeCh, errCh
	}

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}

	// once ensures only the first callback validates/sends/shuts-down;
	// a second concurrent hit returns immediately without blocking.
	var once sync.Once

	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		handled := false
		once.Do(func() {
			handled = true
			q := r.URL.Query()

			if idpErr := q.Get("error"); idpErr != "" {
				desc := q.Get("error_description")
				http.Error(w, "Authentication failed: "+idpErr, http.StatusBadRequest)
				errCh <- fmt.Errorf("auth: IdP error %q: %s", idpErr, desc)
				go srv.Shutdown(context.Background()) //nolint:errcheck
				return
			}

			if q.Get("state") != expectedState {
				http.Error(w, "state mismatch", http.StatusBadRequest)
				errCh <- fmt.Errorf("auth: state mismatch — possible CSRF")
				go srv.Shutdown(context.Background()) //nolint:errcheck
				return
			}

			code := q.Get("code")
			if code == "" {
				http.Error(w, "missing code", http.StatusBadRequest)
				errCh <- fmt.Errorf("auth: callback missing code parameter")
				go srv.Shutdown(context.Background()) //nolint:errcheck
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, closeTabHTML)
			codeCh <- code
			go srv.Shutdown(context.Background()) //nolint:errcheck
		})
		if !handled {
			http.Error(w, "already handled", http.StatusGone)
		}
	})

	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			select {
			case errCh <- fmt.Errorf("auth: loopback server: %w", serveErr):
			default:
			}
		}
	}()

	// honour ctx cancellation
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background()) //nolint:errcheck
	}()

	return codeCh, errCh
}

// validateLoopbackHost ensures the host portion (no port) is a loopback address.
func validateLoopbackHost(hostport string) error {
	h, _, err := net.SplitHostPort(hostport)
	if err != nil {
		// No port separator — treat the whole thing as the host.
		h = hostport
	}
	if h == "" || h == "localhost" {
		return nil
	}
	ip := net.ParseIP(h)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("auth: oauth_redirect_uri must use a loopback host (localhost/127.0.0.1/::1), got %q", h)
}
