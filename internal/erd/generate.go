package erd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateOpts controls what Generate writes to disk.
type GenerateOpts struct {
	OutputDir string   // destination directory; created if absent
	Formats   []string // "json", "dbml"; "html" rejected until phase 3
	Meta      *Meta    // optional agentic polish layer
	Schema    string   // database/schema name; "" = auto-detect
	Tables    []string // restrict to these table names; nil = all
}

// Result carries the output paths and stats from a Generate call.
type Result struct {
	OutputDir  string   `json:"output_dir"`
	Files      []string `json:"files"`
	TableCount int      `json:"table_count"`
}

// Generate introspects db, normalises the schema, and writes artifacts to
// opts.OutputDir. Caller supplies a ready *sql.DB so this stays decoupled from
// the adapter/core layers.
func Generate(ctx context.Context, db *sql.DB, dbtype string, opts GenerateOpts) (Result, error) {
	for _, f := range opts.Formats {
		if f == "html" {
			return Result{}, fmt.Errorf("html rendering lands in phase 3; use --format json,dbml")
		}
	}

	tables, err := Introspect(ctx, db, dbtype, opts.Schema, opts.Tables)
	if err != nil {
		return Result{}, err
	}
	Normalize(tables)

	meta := Meta{}
	if opts.Meta != nil {
		meta = *opts.Meta
	}

	payload := Payload{Schema: tables, Meta: meta}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output dir: %w", err)
	}
	if err := writeGitIgnore(opts.OutputDir); err != nil {
		return Result{}, err
	}

	var written []string

	wantFormat := func(f string) bool {
		if len(opts.Formats) == 0 {
			return false
		}
		for _, v := range opts.Formats {
			if strings.EqualFold(v, f) {
				return true
			}
		}
		return false
	}

	// schema.json is always written regardless of format selection.
	jsonPath := filepath.Join(opts.OutputDir, "schema.json")
	if err := writeJSON(jsonPath, payload.Schema); err != nil {
		return Result{}, err
	}
	written = append(written, jsonPath)

	if opts.Meta != nil {
		metaPath := filepath.Join(opts.OutputDir, "meta.json")
		if err := writeJSON(metaPath, meta); err != nil {
			return Result{}, err
		}
		written = append(written, metaPath)
	}

	if wantFormat("dbml") {
		dbmlPath := filepath.Join(opts.OutputDir, "schema.dbml")
		if err := os.WriteFile(dbmlPath, []byte(EmitDBML(tables, meta)), 0o644); err != nil {
			return Result{}, fmt.Errorf("write schema.dbml: %w", err)
		}
		written = append(written, dbmlPath)
	}

	return Result{
		OutputDir:  opts.OutputDir,
		Files:      written,
		TableCount: len(tables),
	}, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeGitIgnore(dir string) error {
	p := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(p); err == nil {
		return nil // already present
	}
	return os.WriteFile(p, []byte("*\n!.gitignore\n"), 0o644)
}
