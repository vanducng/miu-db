package config

import (
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// The file backend must always have a FileDir; without it, CGO-disabled release
// builds (no OS keychain backend) fail with "no directory provided for file keyring".
func TestKeyringConfigFileDir(t *testing.T) {
	cfg := keyringConfig("")
	if cfg.ServiceName != "miudb" {
		t.Fatalf("service default = %q, want miudb", cfg.ServiceName)
	}
	if cfg.FileDir == "" {
		t.Fatal("FileDir must be set so CGO-disabled builds have a working file backend")
	}
	if cfg.FilePasswordFunc == nil {
		t.Fatal("FilePasswordFunc must be set for the file backend")
	}
}

// Exercises the real openKeyring file backend (no test seam). Gated + isolated so
// it never touches the OS keychain in CI; run with CGO_ENABLED=0 MIUDB_KEYRING_E2E=1.
func TestOAuthTokenFileBackendRoundTrip(t *testing.T) {
	if os.Getenv("MIUDB_KEYRING_E2E") != "1" {
		t.Skip("set CGO_ENABLED=0 MIUDB_KEYRING_E2E=1 to run the real file-backend round-trip")
	}
	prev := GetOAuthKeyringForTest()
	SetOAuthKeyringForTest(nil)
	defer SetOAuthKeyringForTest(prev)
	t.Setenv("MIUDB_CONFIG_DIR", t.TempDir())

	tok := &oauth2.Token{AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour).Round(time.Second)}
	if err := StoreOAuthToken("miudb", "conn1", tok); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, ok, err := LoadOAuthToken("miudb", "conn1")
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if got.AccessToken != "a" || got.RefreshToken != "r" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if err := DeleteOAuthToken("miudb", "conn1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, _ := LoadOAuthToken("miudb", "conn1"); ok {
		t.Fatal("token should be gone after delete")
	}
}
