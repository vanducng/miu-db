package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// oauthSnowflakeConn returns a minimal Snowflake+oauth Connection wired to the
// provided token URL so the refresh path can be tested without a real IdP.
func oauthSnowflakeConn(name, tokenURL string) Connection {
	return Connection{
		Name:   name,
		DBType: "snowflake",
		Options: map[string]any{
			"authenticator":           "oauth",
			"oauth_client_id":         "client-id",
			"oauth_client_secret":     "client-secret",
			"oauth_token_request_url": tokenURL,
		},
	}
}

// fakeTokenServer starts an httptest server that responds with the given body
// and status code to any POST request (simulates a token endpoint).
func fakeTokenServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func farFuture() time.Time  { return time.Now().Add(24 * time.Hour) }
func nearExpiry() time.Time { return time.Now().Add(2 * time.Minute) }
func pastExpiry() time.Time { return time.Now().Add(-1 * time.Minute) }

// ---- OAuthConfigFromOptions -------------------------------------------------

func TestOAuthConfigFromOptions_Defaults(t *testing.T) {
	conn := Connection{
		Options: map[string]any{
			"oauth_client_id":         "cid",
			"oauth_client_secret":     "csec",
			"oauth_authorization_url": "https://idp/auth",
			"oauth_token_request_url": "https://idp/token",
		},
	}
	cfg := OAuthConfigFromOptions(conn)
	if cfg.RedirectURI != "http://localhost:8085/" {
		t.Errorf("default redirect_uri: got %q", cfg.RedirectURI)
	}
	if cfg.ClientID != "cid" {
		t.Errorf("client_id: got %q", cfg.ClientID)
	}
	if cfg.TokenURL != "https://idp/token" {
		t.Errorf("token_url: got %q", cfg.TokenURL)
	}
}

func TestOAuthConfigFromOptions_ScopesSplit(t *testing.T) {
	conn := Connection{
		Options: map[string]any{
			"oauth_scope": "openid, profile,email",
		},
	}
	cfg := OAuthConfigFromOptions(conn)
	want := []string{"openid", "profile", "email"}
	if len(cfg.Scopes) != len(want) {
		t.Fatalf("scopes len: got %d, want %d", len(cfg.Scopes), len(want))
	}
	for i, s := range want {
		if cfg.Scopes[i] != s {
			t.Errorf("scope[%d]: got %q, want %q", i, cfg.Scopes[i], s)
		}
	}
}

func TestOAuthConfigFromOptions_ExplicitRedirectURI(t *testing.T) {
	conn := Connection{
		Options: map[string]any{
			"oauth_redirect_uri": "http://localhost:9999/cb",
		},
	}
	cfg := OAuthConfigFromOptions(conn)
	if cfg.RedirectURI != "http://localhost:9999/cb" {
		t.Errorf("redirect_uri: got %q", cfg.RedirectURI)
	}
}

// ---- resolveOAuth -----------------------------------------------------------

func TestResolveOAuth_ValidTokenFarFuture(t *testing.T) {
	defer withFileBackend(t)()

	conn := oauthSnowflakeConn("myconn", "http://unused")
	tok := &oauth2.Token{
		AccessToken:  "access-valid",
		RefreshToken: "ref",
		Expiry:       farFuture(),
	}
	if err := StoreOAuthToken("", conn.Name, tok); err != nil {
		t.Fatal(err)
	}

	if err := resolveOAuth("", &conn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Options["__oauth_access_token"] != "access-valid" {
		t.Errorf("injected token: got %v", conn.Options["__oauth_access_token"])
	}
}

func TestResolveOAuth_NoCachedToken(t *testing.T) {
	defer withFileBackend(t)()

	conn := oauthSnowflakeConn("notoken", "http://unused")
	err := resolveOAuth("", &conn)
	if !errors.Is(err, ErrOAuthLoginRequired) {
		t.Errorf("expected ErrOAuthLoginRequired, got %v", err)
	}
	if _, set := conn.Options["__oauth_access_token"]; set {
		t.Error("__oauth_access_token must not be set when token absent")
	}
}

func TestResolveOAuth_NearExpiryRefreshesAndPersists(t *testing.T) {
	defer withFileBackend(t)()

	srv := fakeTokenServer(t, http.StatusOK, `{
		"access_token":"fresh-access",
		"token_type":"Bearer",
		"expires_in":3600,
		"refresh_token":"new-refresh"
	}`)

	conn := oauthSnowflakeConn("nearconn", srv.URL)
	tok := &oauth2.Token{
		AccessToken:  "stale-access",
		RefreshToken: "old-refresh",
		Expiry:       nearExpiry(),
	}
	if err := StoreOAuthToken("", conn.Name, tok); err != nil {
		t.Fatal(err)
	}

	if err := resolveOAuth("", &conn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Options["__oauth_access_token"] != "fresh-access" {
		t.Errorf("injected token: got %v", conn.Options["__oauth_access_token"])
	}

	// Persisted token must be the rotated one.
	stored, ok, err := LoadOAuthToken("", conn.Name)
	if err != nil || !ok {
		t.Fatalf("load persisted: ok=%v err=%v", ok, err)
	}
	if stored.AccessToken != "fresh-access" {
		t.Errorf("persisted access token: got %q", stored.AccessToken)
	}
}

func TestResolveOAuth_ExpiredTokenRefreshes(t *testing.T) {
	defer withFileBackend(t)()

	srv := fakeTokenServer(t, http.StatusOK, `{
		"access_token":"refreshed-access",
		"token_type":"Bearer",
		"expires_in":3600,
		"refresh_token":"new-ref"
	}`)

	conn := oauthSnowflakeConn("expiredconn", srv.URL)
	tok := &oauth2.Token{
		AccessToken:  "expired-access",
		RefreshToken: "old-ref",
		Expiry:       pastExpiry(),
	}
	if err := StoreOAuthToken("", conn.Name, tok); err != nil {
		t.Fatal(err)
	}

	if err := resolveOAuth("", &conn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Options["__oauth_access_token"] != "refreshed-access" {
		t.Errorf("injected token: got %v", conn.Options["__oauth_access_token"])
	}
}

func TestResolveOAuth_InvalidGrantDeletesTokenAndReturnsLoginRequired(t *testing.T) {
	defer withFileBackend(t)()

	srv := fakeTokenServer(t, http.StatusBadRequest, `{
		"error":"invalid_grant",
		"error_description":"refresh token expired"
	}`)

	conn := oauthSnowflakeConn("badgrant", srv.URL)
	tok := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "bad-refresh",
		Expiry:       pastExpiry(),
	}
	if err := StoreOAuthToken("", conn.Name, tok); err != nil {
		t.Fatal(err)
	}

	err := resolveOAuth("", &conn)
	if !errors.Is(err, ErrOAuthLoginRequired) {
		t.Errorf("expected ErrOAuthLoginRequired, got %v", err)
	}

	// Token must be deleted after invalid_grant.
	_, ok, loadErr := LoadOAuthToken("", conn.Name)
	if loadErr != nil {
		t.Fatalf("load after invalid_grant: %v", loadErr)
	}
	if ok {
		t.Error("token must be deleted after invalid_grant")
	}
}

func TestResolveOAuth_RefreshErrorNotInvalidGrant_ReturnsLoginRequired(t *testing.T) {
	defer withFileBackend(t)()

	// 500 not invalid_grant — token must NOT be deleted.
	srv := fakeTokenServer(t, http.StatusInternalServerError, `{"error":"server_error"}`)

	conn := oauthSnowflakeConn("serverr", srv.URL)
	tok := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "ref",
		Expiry:       pastExpiry(),
	}
	if err := StoreOAuthToken("", conn.Name, tok); err != nil {
		t.Fatal(err)
	}

	err := resolveOAuth("", &conn)
	if !errors.Is(err, ErrOAuthLoginRequired) {
		t.Errorf("expected ErrOAuthLoginRequired, got %v", err)
	}

	_, ok, loadErr := LoadOAuthToken("", conn.Name)
	if loadErr != nil {
		t.Fatalf("load after server_error: %v", loadErr)
	}
	if !ok {
		t.Error("token must remain after non-invalid_grant refresh failure")
	}
}

// ---- Map isolation (critical regression) ------------------------------------

func TestResolveOAuth_DoesNotMutateStoredOptions(t *testing.T) {
	defer withFileBackend(t)()

	tok := &oauth2.Token{
		AccessToken:  "tok",
		RefreshToken: "ref",
		Expiry:       farFuture(),
	}

	originalOptions := map[string]any{
		"authenticator":           "oauth",
		"oauth_client_id":         "cid",
		"oauth_client_secret":     "csec",
		"oauth_token_request_url": "http://unused",
	}
	storedConn := Connection{
		Name:    "isolated",
		DBType:  "snowflake",
		Options: originalOptions,
	}

	if err := StoreOAuthToken("", storedConn.Name, tok); err != nil {
		t.Fatal(err)
	}

	// Value-copy shares the same Options map pointer — resolveOAuth must clone.
	resolvedConn := storedConn
	if err := resolveOAuth("", &resolvedConn); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if _, found := storedConn.Options["__oauth_access_token"]; found {
		t.Error("__oauth_access_token leaked into storedConn.Options")
	}
	if _, found := originalOptions["__oauth_access_token"]; found {
		t.Error("__oauth_access_token leaked into originalOptions map")
	}
	if resolvedConn.Options["__oauth_access_token"] != "tok" {
		t.Errorf("resolved Options missing token: %v", resolvedConn.Options["__oauth_access_token"])
	}
}

// ---- Store-level FindResolved integration -----------------------------------

func makeTestStore(t *testing.T, conns []Connection) *Store {
	t.Helper()
	dir := t.TempDir()
	root := Root{Version: 1, Connections: conns}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/connections.json", data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestFindResolved_OAuthInjectsToken(t *testing.T) {
	defer withFileBackend(t)()

	conn := Connection{
		Name:   "sf-oauth",
		DBType: "snowflake",
		Options: map[string]any{
			"authenticator":           "oauth",
			"oauth_client_id":         "cid",
			"oauth_client_secret":     "csec",
			"oauth_token_request_url": "http://unused",
		},
	}
	store := makeTestStore(t, []Connection{conn})

	tok := &oauth2.Token{
		AccessToken:  "injected-tok",
		RefreshToken: "ref",
		Expiry:       farFuture(),
	}
	if err := StoreOAuthToken(store.KeyringService, conn.Name, tok); err != nil {
		t.Fatal(err)
	}

	resolved, found, err := store.FindResolved(conn.Name)
	if err != nil {
		t.Fatalf("FindResolved: %v", err)
	}
	if !found {
		t.Fatal("connection not found")
	}
	if resolved.Options["__oauth_access_token"] != "injected-tok" {
		t.Errorf("token not injected: %v", resolved.Options["__oauth_access_token"])
	}

	// Stored raw connection must never carry the ephemeral key.
	raw, ok := store.FindRaw(conn.Name)
	if !ok {
		t.Fatal("raw not found")
	}
	if _, found := raw.Options["__oauth_access_token"]; found {
		t.Error("__oauth_access_token must not appear in stored connection")
	}
}

func TestFindResolved_OAuthLoginRequired_WhenNoToken(t *testing.T) {
	defer withFileBackend(t)()

	conn := Connection{
		Name:   "sf-no-token",
		DBType: "snowflake",
		Options: map[string]any{
			"authenticator": "oauth",
		},
	}
	store := makeTestStore(t, []Connection{conn})

	_, found, err := store.FindResolved(conn.Name)
	if !found {
		t.Fatal("connection not found")
	}
	if !errors.Is(err, ErrOAuthLoginRequired) {
		t.Errorf("expected ErrOAuthLoginRequired, got %v", err)
	}
}

func TestFindResolved_NonOAuthUnchanged(t *testing.T) {
	defer withFileBackend(t)()

	conn := Connection{
		Name:   "pg-conn",
		DBType: "postgres",
		Endpoint: Endpoint{
			Host:     "localhost",
			Username: "user",
			Password: "pass",
		},
	}
	store := makeTestStore(t, []Connection{conn})

	resolved, found, err := store.FindResolved(conn.Name)
	if err != nil {
		t.Fatalf("unexpected error for non-oauth conn: %v", err)
	}
	if !found {
		t.Fatal("connection not found")
	}
	if _, set := resolved.Options["__oauth_access_token"]; set {
		t.Error("non-oauth conn must not have __oauth_access_token")
	}
}

// ---- endpointPasswordRequired -----------------------------------------------

func TestEndpointPasswordRequired(t *testing.T) {
	tests := []struct {
		name string
		conn Connection
		want bool
	}{
		{
			name: "oauth => false",
			conn: Connection{DBType: "snowflake", Options: map[string]any{"authenticator": "oauth"}},
			want: false,
		},
		{
			name: "OAuth mixed case => false",
			conn: Connection{DBType: "snowflake", Options: map[string]any{"authenticator": "OAuth"}},
			want: false,
		},
		{
			name: "snowflake_jwt => false",
			conn: Connection{DBType: "snowflake", Options: map[string]any{"authenticator": "snowflake_jwt"}},
			want: false,
		},
		{
			name: "snowflake password auth => true",
			conn: Connection{DBType: "snowflake", Endpoint: Endpoint{Username: "user"}},
			want: true,
		},
		{
			name: "bigquery => false",
			conn: Connection{DBType: "bigquery"},
			want: false,
		},
		{
			name: "postgres with username => true",
			conn: Connection{DBType: "postgres", Endpoint: Endpoint{Username: "user"}},
			want: true,
		},
		{
			name: "postgres no username => false",
			conn: Connection{DBType: "postgres"},
			want: false,
		},
		{
			name: "password_command overrides oauth => true",
			conn: Connection{
				DBType:   "snowflake",
				Options:  map[string]any{"authenticator": "oauth"},
				Endpoint: Endpoint{PasswordCommand: "echo secret"},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := endpointPasswordRequired(tc.conn)
			if got != tc.want {
				t.Errorf("endpointPasswordRequired = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---- isOAuthConnection ------------------------------------------------------

func TestIsOAuthConnection(t *testing.T) {
	tests := []struct {
		name string
		conn Connection
		want bool
	}{
		{
			name: "snowflake+oauth => true",
			conn: Connection{DBType: "snowflake", Options: map[string]any{"authenticator": "oauth"}},
			want: true,
		},
		{
			name: "snowflake+OAuth case-insensitive => true",
			conn: Connection{DBType: "snowflake", Options: map[string]any{"authenticator": "OAuth"}},
			want: true,
		},
		{
			name: "snowflake+snowflake_jwt => false",
			conn: Connection{DBType: "snowflake", Options: map[string]any{"authenticator": "snowflake_jwt"}},
			want: false,
		},
		{
			name: "snowflake no authenticator => false",
			conn: Connection{DBType: "snowflake"},
			want: false,
		},
		{
			name: "bigquery+oauth => false (only snowflake supported)",
			conn: Connection{DBType: "bigquery", Options: map[string]any{"authenticator": "oauth"}},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isOAuthConnection(tc.conn)
			if got != tc.want {
				t.Errorf("isOAuthConnection = %v, want %v", got, tc.want)
			}
		})
	}
}
