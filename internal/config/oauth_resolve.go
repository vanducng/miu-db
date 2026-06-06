package config

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/vanducng/miu-db/internal/auth"
	"golang.org/x/oauth2"
)

// ErrOAuthLoginRequired is returned when no valid token exists and interactive
// login is needed (resolveOAuth is non-interactive).
var ErrOAuthLoginRequired = errors.New("oauth login required")

// OAuthConfigFromOptions builds an auth.Config from Connection.Options using
// the standard snake_case option keys that mirror dbt/SnowSQL conventions.
func OAuthConfigFromOptions(conn Connection) auth.Config {
	get := func(key string) string {
		v, _ := conn.Options[key].(string)
		return v
	}
	redirectURI := get("oauth_redirect_uri")
	if redirectURI == "" {
		redirectURI = "http://localhost:8085/"
	}
	var scopes []string
	if raw := get("oauth_scope"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				scopes = append(scopes, s)
			}
		}
	}
	return auth.Config{
		ClientID:     get("oauth_client_id"),
		ClientSecret: get("oauth_client_secret"),
		AuthURL:      get("oauth_authorization_url"),
		TokenURL:     get("oauth_token_request_url"),
		RedirectURI:  redirectURI,
		Scopes:       scopes,
	}
}

// resolveOAuth loads the cached token for conn, refreshes if near-expiry, and
// injects the fresh access token into a cloned Options map on conn. It never
// opens a browser; missing or unrefreshable token => ErrOAuthLoginRequired.
func resolveOAuth(service string, conn *Connection) error {
	tok, ok, err := LoadOAuthToken(service, conn.Name)
	if err != nil {
		return err
	}
	if !ok {
		return ErrOAuthLoginRequired
	}

	const expiryBuffer = 5 * time.Minute
	needsRefresh := !tok.Valid() ||
		(!tok.Expiry.IsZero() && time.Until(tok.Expiry) < expiryBuffer)

	if needsRefresh {
		cfg := OAuthConfigFromOptions(*conn)
		// Strip expiry so oauth2.TokenSource considers the token expired and
		// unconditionally exchanges the refresh token for a new access token.
		stripped := &oauth2.Token{RefreshToken: tok.RefreshToken}
		newTok, refreshErr := auth.Refresh(context.Background(), cfg, stripped)
		if refreshErr != nil {
			if isInvalidGrant(refreshErr) {
				_ = DeleteOAuthToken(service, conn.Name)
				return ErrOAuthLoginRequired
			}
			// Transient IdP/network failure: preserve token, wrap cause without secrets.
			return fmt.Errorf("%w: refresh failed: %s", ErrOAuthLoginRequired, redactRefreshErr(refreshErr))
		}
		if err := StoreOAuthToken(service, conn.Name, newTok); err != nil {
			return err
		}
		tok = newTok
	}

	// Clone before injecting so s.root.Connections[i].Options is never mutated.
	m := maps.Clone(conn.Options)
	if m == nil {
		m = make(map[string]any)
	}
	m["__oauth_access_token"] = tok.AccessToken
	conn.Options = m
	return nil
}

func isInvalidGrant(err error) bool {
	if err == nil {
		return false
	}
	var rErr *oauth2.RetrieveError
	if errors.As(err, &rErr) {
		return strings.EqualFold(rErr.ErrorCode, "invalid_grant")
	}
	return strings.Contains(strings.ToLower(err.Error()), "invalid_grant")
}

// redactRefreshErr returns a diagnostic string that omits token/secret material.
// For oauth2.RetrieveError we expose only the HTTP status and error code.
func redactRefreshErr(err error) string {
	var rErr *oauth2.RetrieveError
	if errors.As(err, &rErr) {
		return fmt.Sprintf("HTTP %d: %s", rErr.Response.StatusCode, rErr.ErrorCode)
	}
	// For network/other errors strip the URL (may contain query params with tokens).
	msg := err.Error()
	if idx := strings.Index(msg, "://"); idx != -1 {
		// Remove the URL portion; keep only the surrounding description.
		if end := strings.IndexAny(msg[idx:], " \t"); end != -1 {
			msg = msg[:idx] + msg[idx+end:]
		} else {
			msg = msg[:idx]
		}
	}
	return strings.TrimSpace(msg)
}
