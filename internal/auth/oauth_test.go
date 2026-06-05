package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// fakeTokenServer returns an httptest.Server that responds to token requests
// with the provided access token, refresh token, and expiry seconds.
func fakeTokenServer(t *testing.T, accessToken, refreshToken string, expirySeconds int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"access_token":  accessToken,
			"token_type":    "Bearer",
			"refresh_token": refreshToken,
			"expires_in":    expirySeconds,
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	return srv
}

// findFreePort returns an available TCP port on localhost.
func findFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("findFreePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestPKCEChallenge(t *testing.T) {
	verifier := oauth2.GenerateVerifier()
	challenge := oauth2.S256ChallengeFromVerifier(verifier)

	if len(verifier) < 43 {
		t.Fatalf("verifier too short: %d chars", len(verifier))
	}
	if challenge == "" {
		t.Fatal("challenge must not be empty")
	}
	if challenge == verifier {
		t.Fatal("challenge must differ from verifier (must be hashed)")
	}

	// Confirm the challenge appears in an auth URL built with S256ChallengeOption.
	cfg := &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
		},
		RedirectURL: "http://localhost:8085/",
	}
	authURL := cfg.AuthCodeURL("state123", oauth2.S256ChallengeOption(verifier))
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	gotChallenge := u.Query().Get("code_challenge")
	if gotChallenge != challenge {
		t.Fatalf("code_challenge mismatch: want %q got %q", challenge, gotChallenge)
	}
	if u.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("expected code_challenge_method=S256")
	}
}

func TestStateMismatch(t *testing.T) {
	port := findFreePort(t)
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)
	u, _ := url.Parse(redirectURI)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, errCh := startLoopback(ctx, u, "correct-state")

	// Hit the loopback with the wrong state.
	callbackURL := fmt.Sprintf("http://localhost:%d/callback?code=abc&state=wrong-state", port)
	resp, err := http.Get(callbackURL) //nolint:noctx
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	resp.Body.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error on state mismatch, got nil")
		}
		if !strings.Contains(err.Error(), "state mismatch") {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for state-mismatch error")
	}
}

func TestHappyPathLogin(t *testing.T) {
	tokenSrv := fakeTokenServer(t, "access-tok-1", "refresh-tok-1", 3600)
	defer tokenSrv.Close()

	port := findFreePort(t)
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	cfg := Config{
		ClientID:    "client-id",
		AuthURL:     "https://example.com/auth", // never actually called in this test
		TokenURL:    tokenSrv.URL,
		RedirectURI: redirectURI,
		Scopes:      []string{"openid"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Replace browser open with a stub that captures the auth URL (so we can
	// extract state) and immediately fires the loopback callback.
	origOpen := openURL
	defer func() { openURL = origOpen }()

	callbackFired := make(chan struct{})
	openURL = func(rawURL string) error {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		state := parsed.Query().Get("state")
		// Simulate the browser redirect to our loopback in a goroutine.
		go func() {
			cbURL := fmt.Sprintf("http://localhost:%d/callback?code=authcode42&state=%s", port, state)
			resp, err := http.Get(cbURL) //nolint:noctx
			if err == nil {
				resp.Body.Close()
			}
			close(callbackFired)
		}()
		return nil
	}

	tok, err := Login(ctx, cfg)
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if tok.AccessToken != "access-tok-1" {
		t.Fatalf("access token: want %q got %q", "access-tok-1", tok.AccessToken)
	}
	if tok.RefreshToken != "refresh-tok-1" {
		t.Fatalf("refresh token: want %q got %q", "refresh-tok-1", tok.RefreshToken)
	}
	if !tok.Valid() {
		t.Fatal("token should be valid (not expired)")
	}
	<-callbackFired
}

func TestRefresh(t *testing.T) {
	tokenSrv := fakeTokenServer(t, "access-tok-new", "refresh-tok-new", 7200)
	defer tokenSrv.Close()

	cfg := Config{
		ClientID: "client-id",
		AuthURL:  "https://example.com/auth",
		TokenURL: tokenSrv.URL,
	}

	old := &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		Expiry:       time.Now().Add(-time.Hour), // expired
	}

	ctx := context.Background()
	tok, err := Refresh(ctx, cfg, old)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tok.AccessToken != "access-tok-new" {
		t.Fatalf("access token: want %q got %q", "access-tok-new", tok.AccessToken)
	}
	if tok.Expiry.Before(time.Now()) {
		t.Fatal("refreshed token should not be expired")
	}
}

func TestExplicitPort(t *testing.T) {
	port := findFreePort(t)
	redirectURI := fmt.Sprintf("http://localhost:%d/cb", port)
	u, _ := url.Parse(redirectURI)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	codeCh, errCh := startLoopback(ctx, u, "st")

	// Confirm the port is bound by making a request.
	callbackURL := fmt.Sprintf("http://localhost:%d/cb?code=xyz&state=st", port)
	resp, err := http.Get(callbackURL) //nolint:noctx
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	select {
	case code := <-codeCh:
		if code != "xyz" {
			t.Fatalf("code: want %q got %q", "xyz", code)
		}
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestValidateLoopbackHost(t *testing.T) {
	cases := []struct {
		hostport string
		wantErr  bool
	}{
		{"localhost:8085", false},
		{"127.0.0.1:8085", false},
		{"[::1]:8085", false},
		{"", false},
		{"0.0.0.0:8085", true},
		{"192.168.1.1:8085", true},
		{"10.0.0.1:8085", true},
		{"evil.example.com:8085", true},
	}
	for _, tc := range cases {
		err := validateLoopbackHost(tc.hostport)
		if tc.wantErr && err == nil {
			t.Errorf("validateLoopbackHost(%q): expected error, got nil", tc.hostport)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("validateLoopbackHost(%q): unexpected error: %v", tc.hostport, err)
		}
	}
}

func TestNonLoopbackRejectedBeforeListen(t *testing.T) {
	// 0.0.0.0 is not a loopback address; must be rejected before any net.Listen.
	u, _ := url.Parse("http://0.0.0.0:8085/callback")
	ctx := context.Background()
	_, errCh := startLoopback(ctx, u, "state")
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error for non-loopback host, got nil")
		}
		if !strings.Contains(err.Error(), "loopback") {
			t.Fatalf("unexpected error message: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for loopback validation error")
	}
}

func TestPortInUse(t *testing.T) {
	// Bind a port, then try startLoopback on the same port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	u, _ := url.Parse(fmt.Sprintf("http://localhost:%d/", port))
	ctx := context.Background()
	_, errCh := startLoopback(ctx, u, "s")

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error for port-in-use")
		}
		if !strings.Contains(err.Error(), "bind") && !strings.Contains(err.Error(), "in use") && !strings.Contains(err.Error(), "address already") {
			t.Logf("got expected port-bind error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for bind error")
	}
}
