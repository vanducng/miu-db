package erd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadPayloadFromPath reads a Payload from a directory or a .json file.
// Directory: reads <dir>/schema.json (required) and <dir>/meta.json (optional).
// File: reads the file as schema; looks for a sibling meta.json.
func LoadPayloadFromPath(path string) (Payload, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Payload{}, fmt.Errorf("erd: stat %q: %w", path, err)
	}

	var schemaPath, metaPath string
	if info.IsDir() {
		schemaPath = filepath.Join(path, "schema.json")
		metaPath = filepath.Join(path, "meta.json")
	} else {
		schemaPath = path
		metaPath = filepath.Join(filepath.Dir(path), "meta.json")
	}

	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return Payload{}, fmt.Errorf("erd: read schema.json: %w", err)
	}
	var tables []Table
	if err := json.Unmarshal(schemaData, &tables); err != nil {
		return Payload{}, fmt.Errorf("erd: parse schema.json: %w", err)
	}

	var meta Meta
	if metaData, err := os.ReadFile(metaPath); err == nil {
		if err := json.Unmarshal(metaData, &meta); err != nil {
			return Payload{}, fmt.Errorf("erd: parse meta.json: %w", err)
		}
	}

	return Payload{Schema: tables, Meta: meta}, nil
}
