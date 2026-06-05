package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/99designs/keyring"
	"golang.org/x/oauth2"
)

// oauthKeyring is overridden in tests to avoid OS-keychain prompts.
var oauthKeyring keyring.Keyring

func oauthRing(service string) (keyring.Keyring, error) {
	if oauthKeyring != nil {
		return oauthKeyring, nil
	}
	return openKeyring(service)
}

// StoreOAuthToken serializes tok as JSON and persists it under the oauth credential key.
func StoreOAuthToken(service, conn string, tok *oauth2.Token) error {
	ring, err := oauthRing(service)
	if err != nil {
		return err
	}
	data, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("marshal oauth token: %w", err)
	}
	return ring.Set(keyring.Item{
		Key:         credentialKey(conn, "oauth"),
		Data:        data,
		Label:       fmt.Sprintf("%s oauth token", conn),
		Description: "Stored by miudb",
	})
}

// LoadOAuthToken retrieves and deserializes the stored token.
// Returns (nil, false, nil) when no token exists for conn.
func LoadOAuthToken(service, conn string) (*oauth2.Token, bool, error) {
	ring, err := oauthRing(service)
	if err != nil {
		return nil, false, err
	}
	item, err := ring.Get(credentialKey(conn, "oauth"))
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) || errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(item.Data, &tok); err != nil {
		return nil, false, fmt.Errorf("unmarshal oauth token: %w", err)
	}
	return &tok, true, nil
}

// DeleteOAuthToken removes the stored token; idempotent when key is absent.
func DeleteOAuthToken(service, conn string) error {
	ring, err := oauthRing(service)
	if err != nil {
		return err
	}
	err = ring.Remove(credentialKey(conn, "oauth"))
	if errors.Is(err, keyring.ErrKeyNotFound) || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
