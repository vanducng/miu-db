package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type credentialExport struct {
	Entries []credentialEntry `json:"entries"`
}

type credentialEntry struct {
	Connection string `json:"connection"`
	Kind       string `json:"kind"`
	Password   string `json:"password"`
}

func LoadCredentialExport(path string) (map[string]string, error) {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	var payload credentialExport
	if err := json.Unmarshal(data, &payload); err != nil {
		return out, fmt.Errorf("parse credentials export: %w", err)
	}
	for _, entry := range payload.Entries {
		if entry.Connection == "" || entry.Kind == "" || entry.Password == "" {
			continue
		}
		out[credentialKey(entry.Connection, entry.Kind)] = entry.Password
	}
	return out, nil
}

func LoadCredentialFile(path string) (map[string]string, error) {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	var export credentialExport
	if err := json.Unmarshal(data, &export); err == nil && len(export.Entries) > 0 {
		for _, entry := range export.Entries {
			if entry.Connection == "" || entry.Kind == "" || entry.Password == "" {
				continue
			}
			out[credentialKey(entry.Connection, entry.Kind)] = entry.Password
		}
		return out, nil
	}
	var flat map[string]string
	if err := json.Unmarshal(data, &flat); err != nil {
		return out, fmt.Errorf("parse credential file: %w", err)
	}
	for key, value := range flat {
		if strings.TrimSpace(key) == "" {
			continue
		}
		out[key] = value
	}
	return out, nil
}

func StoreCredentialFileSecret(path, connection, kind, value string) error {
	if value == "" {
		return nil
	}
	path = expandHome(path)
	items, err := LoadCredentialFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		items = map[string]string{}
	}
	items[credentialKey(connection, kind)] = value
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

type staticSecretResolver struct {
	source string
	items  map[string]string
}

func newStaticSecretResolver(source string, items map[string]string) *staticSecretResolver {
	if items == nil {
		items = map[string]string{}
	}
	return &staticSecretResolver{source: source, items: items}
}

func (r *staticSecretResolver) Source() string { return r.source }

func (r *staticSecretResolver) Resolve(ctx context.Context, conn Connection, kind string) (string, bool, error) {
	value, ok := r.items[credentialKey(conn.Name, kind)]
	if !ok {
		return "", false, nil
	}
	return value, true, nil
}
