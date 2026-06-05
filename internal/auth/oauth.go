package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// Config holds the OAuth2 client settings for a single connection.
// Intentionally decoupled from internal/config to avoid import cycles.
type Config struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RedirectURI  string
	Scopes       []string
}

func oauthConfig(cfg Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		},
		RedirectURL: cfg.RedirectURI,
		Scopes:      cfg.Scopes,
	}
}

// Login runs the browser-based Auth-Code + S256-PKCE flow and returns a token.
// A 120-second deadline is imposed if ctx has no deadline.
func Login(ctx context.Context, cfg Config) (*oauth2.Token, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}

	redirectURI := cfg.RedirectURI
	if redirectURI == "" {
		redirectURI = "http://localhost:8085/"
	}

	u, err := url.Parse(redirectURI)
	if err != nil {
		return nil, fmt.Errorf("auth: invalid redirect_uri %q: %w", redirectURI, err)
	}

	verifier := oauth2.GenerateVerifier()
	state, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("auth: generate state: %w", err)
	}

	oc := oauthConfig(cfg)
	oc.RedirectURL = redirectURI

	authURL := oc.AuthCodeURL(state,
		oauth2.S256ChallengeOption(verifier),
		oauth2.AccessTypeOffline,
	)

	codeCh, errCh := startLoopback(ctx, u, state)

	// Open prints the URL itself on failure, so we only need to call it.
	Open(authURL) //nolint:errcheck

	select {
	case code := <-codeCh:
		tok, err := oc.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("auth: token exchange: %w", err)
		}
		return tok, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("auth: login timed out waiting for callback: %w", ctx.Err())
	}
}

// Refresh uses the stored refresh token to obtain a new access token.
// The caller should persist the returned token when it differs from the input.
func Refresh(ctx context.Context, cfg Config, tok *oauth2.Token) (*oauth2.Token, error) {
	oc := oauthConfig(cfg)
	ts := oc.TokenSource(ctx, tok)
	newTok, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("auth: refresh: %w", err)
	}
	return newTok, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
