package config

import (
	"testing"
	"time"

	"github.com/99designs/keyring"
	"golang.org/x/oauth2"
)

func openFileBackend(t *testing.T) keyring.Keyring {
	t.Helper()
	ring, err := keyring.Open(keyring.Config{
		AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
		FileDir:          t.TempDir(),
		FilePasswordFunc: func(string) (string, error) { return "test", nil },
	})
	if err != nil {
		t.Fatalf("open file backend: %v", err)
	}
	return ring
}

func withFileBackend(t *testing.T) func() {
	t.Helper()
	prev := oauthKeyring
	oauthKeyring = openFileBackend(t)
	return func() { oauthKeyring = prev }
}

func TestOAuthTokenRoundTrip(t *testing.T) {
	defer withFileBackend(t)()

	expiry := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	tok := &oauth2.Token{
		AccessToken:  "access-abc",
		TokenType:    "Bearer",
		RefreshToken: "refresh-xyz",
		Expiry:       expiry,
	}

	if err := StoreOAuthToken("", "myconn", tok); err != nil {
		t.Fatalf("store: %v", err)
	}

	got, ok, err := LoadOAuthToken("", "myconn")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !ok {
		t.Fatal("expected token to be found")
	}

	if got.AccessToken != tok.AccessToken {
		t.Errorf("access token: got %q, want %q", got.AccessToken, tok.AccessToken)
	}
	if got.RefreshToken != tok.RefreshToken {
		t.Errorf("refresh token: got %q, want %q", got.RefreshToken, tok.RefreshToken)
	}
	if got.TokenType != tok.TokenType {
		t.Errorf("token type: got %q, want %q", got.TokenType, tok.TokenType)
	}
	// JSON round-trip loses sub-second precision; truncate to second.
	if !got.Expiry.Truncate(time.Second).Equal(expiry.Truncate(time.Second)) {
		t.Errorf("expiry: got %v, want %v", got.Expiry, expiry)
	}
}

func TestOAuthTokenLoadMissing(t *testing.T) {
	defer withFileBackend(t)()

	tok, ok, err := LoadOAuthToken("", "no-such-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || tok != nil {
		t.Errorf("expected (nil, false, nil), got (%v, %v, nil)", tok, ok)
	}
}

func TestOAuthTokenDeleteIdempotent(t *testing.T) {
	defer withFileBackend(t)()

	// Delete absent key must return nil.
	if err := DeleteOAuthToken("", "ghost-conn"); err != nil {
		t.Fatalf("delete absent: %v", err)
	}

	// Store then delete, then delete again.
	tok := &oauth2.Token{AccessToken: "tok", RefreshToken: "ref"}
	if err := StoreOAuthToken("", "myconn", tok); err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := DeleteOAuthToken("", "myconn"); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if err := DeleteOAuthToken("", "myconn"); err != nil {
		t.Fatalf("second delete (idempotent): %v", err)
	}

	_, ok, err := LoadOAuthToken("", "myconn")
	if err != nil {
		t.Fatalf("load after delete: %v", err)
	}
	if ok {
		t.Error("expected token absent after delete")
	}
}

func TestOAuthTokenIsolatedByConn(t *testing.T) {
	defer withFileBackend(t)()

	tok1 := &oauth2.Token{AccessToken: "conn1-tok"}
	tok2 := &oauth2.Token{AccessToken: "conn2-tok"}

	if err := StoreOAuthToken("", "conn1", tok1); err != nil {
		t.Fatalf("store conn1: %v", err)
	}
	if err := StoreOAuthToken("", "conn2", tok2); err != nil {
		t.Fatalf("store conn2: %v", err)
	}

	got1, ok1, _ := LoadOAuthToken("", "conn1")
	got2, ok2, _ := LoadOAuthToken("", "conn2")

	if !ok1 || got1.AccessToken != tok1.AccessToken {
		t.Errorf("conn1: got %v ok=%v", got1, ok1)
	}
	if !ok2 || got2.AccessToken != tok2.AccessToken {
		t.Errorf("conn2: got %v ok=%v", got2, ok2)
	}

	if err := DeleteOAuthToken("", "conn1"); err != nil {
		t.Fatalf("delete conn1: %v", err)
	}

	_, ok1After, _ := LoadOAuthToken("", "conn1")
	_, ok2After, _ := LoadOAuthToken("", "conn2")
	if ok1After {
		t.Error("conn1 should be gone after delete")
	}
	if !ok2After {
		t.Error("conn2 should still exist")
	}
}
