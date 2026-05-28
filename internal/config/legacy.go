package config

import (
	"encoding/json"
	"fmt"
	"os"
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
