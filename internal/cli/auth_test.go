package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"
	"golang.org/x/oauth2"

	"github.com/vanducng/miu-db/internal/config"
)

// ---- keyring seam -----------------------------------------------------------

func openAuthTestKeyring(t *testing.T) keyring.Keyring {
	t.Helper()
	ring, err := keyring.Open(keyring.Config{
		AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
		FileDir:          t.TempDir(),
		FilePasswordFunc: func(string) (string, error) { return "test", nil },
	})
	if err != nil {
		t.Fatalf("open test keyring: %v", err)
	}
	return ring
}

func withAuthTestKeyring(t *testing.T) {
	t.Helper()
	prev := config.GetOAuthKeyringForTest()
	config.SetOAuthKeyringForTest(openAuthTestKeyring(t))
	t.Cleanup(func() { config.SetOAuthKeyringForTest(prev) })
}

// ---- fixture helpers --------------------------------------------------------

func oauthConn(name string) config.Connection {
	return config.Connection{
		Name:   name,
		DBType: "snowflake",
		Options: map[string]any{
			"authenticator":           "oauth",
			"oauth_client_id":         "cid",
			"oauth_client_secret":     "csec",
			"oauth_authorization_url": "https://idp.example/auth",
			"oauth_token_request_url": "https://idp.example/token",
		},
		Endpoint: config.Endpoint{Kind: "tcp"},
	}
}

func nonOAuthConn(name string) config.Connection {
	return config.Connection{
		Name:    name,
		DBType:  "snowflake",
		Options: map[string]any{"authenticator": "snowflake_jwt"},
		Endpoint: config.Endpoint{
			Kind:     "tcp",
			Host:     "acct.snowflakecomputing.com",
			Username: "USER",
		},
	}
}

// execAuthCmd runs an auth subcommand and returns the decoded envelope.
// Caller must supply the connections.json path via --connections-file.
func execAuthCmd(t *testing.T, connFile string, args ...string) (Envelope, error) {
	t.Helper()
	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(append([]string{"--connections-file", connFile, "--secret-source", "none"}, args...))
	err := cmd.Execute()
	if err != nil {
		return Envelope{}, err
	}
	var env Envelope
	if decErr := json.NewDecoder(&buf).Decode(&env); decErr != nil {
		t.Fatalf("decode envelope: %v (raw: %s)", decErr, buf.String())
	}
	return env, nil
}

// ---- subcommand registration ------------------------------------------------

func TestAuthSubcommandsRegistered(t *testing.T) {
	root := rootCommand(&options{output: "json"})
	for _, sub := range []string{"login", "status", "logout"} {
		found, _, err := root.Find([]string{"auth", sub})
		if err != nil || found == nil || found.Name() != sub {
			t.Errorf("auth %s not registered: %v", sub, err)
		}
	}
}

func TestAuthSubcommandsHaveHelp(t *testing.T) {
	for _, sub := range []string{"login", "status", "logout"} {
		cmd := rootCommand(&options{output: "json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"auth", sub, "--help"})
		_ = cmd.Execute()
		if buf.Len() == 0 {
			t.Errorf("auth %s --help produced no output", sub)
		}
	}
}

// ---- auth login: rejection cases --------------------------------------------

func TestAuthLoginRejectsNonOAuthConnection(t *testing.T) {
	withAuthTestKeyring(t)
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath, nonOAuthConn("sf-jwt"))

	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"--connections-file", connPath, "--secret-source", "none", "auth", "login", "sf-jwt"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-oauth connection")
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != "auth.not_oauth" {
		t.Errorf("code = %q, want auth.not_oauth", cliErr.Code)
	}
}

func TestAuthLoginRejectsUnknownConnection(t *testing.T) {
	withAuthTestKeyring(t)
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath) // empty

	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"--connections-file", connPath, "--secret-source", "none", "auth", "login", "ghost"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown connection")
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != "connection.not_found" {
		t.Errorf("code = %q, want connection.not_found", cliErr.Code)
	}
}

// ---- auth status ------------------------------------------------------------

func TestAuthStatusLoggedOut(t *testing.T) {
	withAuthTestKeyring(t)
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath, oauthConn("sf-oauth"))

	env, err := execAuthCmd(t, connPath, "auth", "status", "sf-oauth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !env.OK {
		t.Errorf("expected ok=true from status command")
	}
	if env.Kind != "auth.status" {
		t.Errorf("kind = %q, want auth.status", env.Kind)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data is %T, want map[string]any", env.Data)
	}
	if data["logged_in"] != false {
		t.Errorf("logged_in = %v, want false", data["logged_in"])
	}
}

func TestAuthStatusLoggedIn(t *testing.T) {
	withAuthTestKeyring(t)
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath, oauthConn("sf-oauth"))

	expiry := time.Now().Add(1 * time.Hour)
	tok := &oauth2.Token{AccessToken: "access-tok", RefreshToken: "refresh-tok", Expiry: expiry}
	if err := config.StoreOAuthToken("", "sf-oauth", tok); err != nil {
		t.Fatalf("store token: %v", err)
	}

	env, err := execAuthCmd(t, connPath, "auth", "status", "sf-oauth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data is %T, want map[string]any", env.Data)
	}
	if data["logged_in"] != true {
		t.Errorf("logged_in = %v, want true", data["logged_in"])
	}
	if data["expired"] != false {
		t.Errorf("expired = %v, want false", data["expired"])
	}
	if _, has := data["expires_at"]; !has {
		t.Error("missing expires_at in status output")
	}
}

func TestAuthStatusExpiredToken(t *testing.T) {
	withAuthTestKeyring(t)
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath, oauthConn("sf-oauth"))

	past := time.Now().Add(-1 * time.Hour)
	tok := &oauth2.Token{AccessToken: "stale", Expiry: past}
	if err := config.StoreOAuthToken("", "sf-oauth", tok); err != nil {
		t.Fatalf("store: %v", err)
	}

	env, err := execAuthCmd(t, connPath, "auth", "status", "sf-oauth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := env.Data.(map[string]any)
	if data["logged_in"] != true {
		t.Errorf("logged_in = %v, want true (token present even if expired)", data["logged_in"])
	}
	if data["expired"] != true {
		t.Errorf("expired = %v, want true", data["expired"])
	}
}

// ---- auth logout ------------------------------------------------------------

func TestAuthLogout(t *testing.T) {
	withAuthTestKeyring(t)
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath, oauthConn("sf-oauth"))

	if err := config.StoreOAuthToken("", "sf-oauth", &oauth2.Token{AccessToken: "tok"}); err != nil {
		t.Fatalf("store: %v", err)
	}

	env, err := execAuthCmd(t, connPath, "auth", "logout", "sf-oauth")
	if err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	if env.Kind != "auth.logout" {
		t.Errorf("kind = %q, want auth.logout", env.Kind)
	}
	data := env.Data.(map[string]any)
	if data["cleared"] != true {
		t.Errorf("cleared = %v, want true", data["cleared"])
	}

	_, still, _ := config.LoadOAuthToken("", "sf-oauth")
	if still {
		t.Error("token still present after logout")
	}
}

func TestAuthLogoutIdempotent(t *testing.T) {
	withAuthTestKeyring(t)
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath, oauthConn("sf-oauth"))

	_, err := execAuthCmd(t, connPath, "auth", "logout", "sf-oauth")
	if err != nil {
		t.Fatalf("logout without prior login failed: %v", err)
	}
}

// ---- maybeLazyLogin ---------------------------------------------------------

func TestMaybeLazyLoginNonTTYReturnsHintError(t *testing.T) {
	// isInteractive() is false in CI / piped test output, so this always
	// returns the hint CLIError without launching a browser.
	retried := false
	err := maybeLazyLogin(context.Background(), &options{}, "sf-oauth", config.ErrOAuthLoginRequired, &retried, false)

	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != "auth.login_required" {
		t.Errorf("code = %q, want auth.login_required", cliErr.Code)
	}
	if cliErr.Exit != 2 {
		t.Errorf("exit = %d, want 2", cliErr.Exit)
	}
	if !strings.Contains(cliErr.Hint, "auth login") {
		t.Errorf("hint should contain 'auth login', got %q", cliErr.Hint)
	}
}

func TestMaybeLazyLoginMCPContextAlwaysReturnsHintError(t *testing.T) {
	retried := false
	// isMCPOrServe=true blocks browser launch even if stdout were a TTY.
	err := maybeLazyLogin(context.Background(), &options{}, "sf-oauth", config.ErrOAuthLoginRequired, &retried, true)

	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != "auth.login_required" {
		t.Errorf("code = %q, want auth.login_required", cliErr.Code)
	}
}

func TestMaybeLazyLoginPassesThroughUnrelatedError(t *testing.T) {
	retried := false
	orig := errors.New("dial tcp: connection refused")
	err := maybeLazyLogin(context.Background(), &options{}, "sf-oauth", orig, &retried, false)
	if !errors.Is(err, orig) {
		t.Errorf("expected original error passthrough, got %v", err)
	}
}

func TestMaybeLazyLoginAlreadyRetriedReturnsHintError(t *testing.T) {
	retried := true // second oauth failure after a login attempt
	err := maybeLazyLogin(context.Background(), &options{}, "sf-oauth", config.ErrOAuthLoginRequired, &retried, false)

	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != "auth.login_required" {
		t.Errorf("code = %q, want auth.login_required", cliErr.Code)
	}
}

// ---- isMCPOrServeContext ----------------------------------------------------

func TestIsMCPOrServeContextDetectsServeCommands(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"mcp", "serve"}, true},
		{[]string{"serve"}, true},
		{[]string{"query", "run"}, false},
		{[]string{"auth", "login"}, false},
		{[]string{"connections", "list"}, false},
	}
	for _, tc := range cases {
		root := rootCommand(&options{output: "json"})
		cmd, _, _ := root.Find(tc.args)
		if cmd == nil {
			continue
		}
		got := isMCPOrServeContext(cmd)
		if got != tc.want {
			t.Errorf("args=%v: isMCPOrServeContext=%v, want %v (path=%q)", tc.args, got, tc.want, cmd.CommandPath())
		}
	}
}

// ---- catalog ----------------------------------------------------------------

func TestCatalogContainsAuthEntries(t *testing.T) {
	need := map[string]bool{"auth login": false, "auth status": false, "auth logout": false}
	for _, info := range catalog() {
		if _, ok := need[info.Name]; ok {
			need[info.Name] = true
		}
	}
	for name, found := range need {
		if !found {
			t.Errorf("catalog missing entry %q", name)
		}
	}
}

func TestCatalogAuthLoginMetadata(t *testing.T) {
	for _, info := range catalog() {
		if info.Name != "auth login" {
			continue
		}
		if info.Stability != "experimental" {
			t.Errorf("auth login stability = %q, want experimental", info.Stability)
		}
		if !info.Mutates {
			t.Error("auth login should have Mutates=true")
		}
		hasBrowser := false
		for _, se := range info.SideEffects {
			if se == "opens_browser" {
				hasBrowser = true
			}
		}
		if !hasBrowser {
			t.Error("auth login missing opens_browser side effect")
		}
		return
	}
	t.Error("auth login not found in catalog")
}

func TestCatalogAuthLogoutMetadata(t *testing.T) {
	for _, info := range catalog() {
		if info.Name != "auth logout" {
			continue
		}
		if !info.Mutates {
			t.Error("auth logout should have Mutates=true")
		}
		return
	}
	t.Error("auth logout not found in catalog")
}
