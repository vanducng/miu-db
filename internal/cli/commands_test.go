package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/vanducng/miu-db/internal/config"
)

func TestIsMCPOrServeContext(t *testing.T) {
	opts := &options{output: "json", limit: 100}
	root := rootCommand(opts)

	// Find the `mcp serve` subcommand and verify it is detected as non-interactive.
	var serveCmd, mcpCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "mcp" {
			mcpCmd = sub
			for _, s := range sub.Commands() {
				if s.Name() == "serve" {
					serveCmd = s
				}
			}
		}
	}
	if serveCmd == nil {
		t.Fatal("could not find mcp serve subcommand")
	}
	_ = mcpCmd
	if !isMCPOrServeContext(serveCmd) {
		t.Error("isMCPOrServeContext(mcp serve) = false, want true")
	}

	// An ordinary leaf command must not be flagged.
	var connectionsCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "connections" {
			connectionsCmd = sub
			break
		}
	}
	if connectionsCmd != nil {
		if isMCPOrServeContext(connectionsCmd) {
			t.Error("isMCPOrServeContext(connections) = true, want false")
		}
	}
}

func TestVersionStringPrefersInjectedReleaseVersion(t *testing.T) {
	previous := version
	t.Cleanup(func() { version = previous })
	version = "v9.9.9-test"
	if got := versionString(); got != version {
		t.Fatalf("versionString() = %q, want %q", got, version)
	}
}

func TestConnectionsAddAcceptsProviderOptions(t *testing.T) {
	dir := t.TempDir()
	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--config-dir", dir,
		"connections", "add",
		"--name", "sf",
		"--db-type", "snowflake",
		"--host", "account",
		"--username", "USER",
		"--option", "authenticator=snowflake_jwt",
		"--option", "warehouse=DEV_WH",
		"--option", "trusted_connection=false",
		"--extra-option", "sslmode=require",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root config.Root
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	if len(root.Connections) != 1 {
		t.Fatalf("expected one connection, got %d", len(root.Connections))
	}
	conn := root.Connections[0]
	if conn.Options["authenticator"] != "snowflake_jwt" {
		t.Fatal("provider string option was not persisted")
	}
	if conn.Options["trusted_connection"] != false {
		t.Fatal("provider boolean option was not parsed")
	}
	if conn.ExtraOptions["sslmode"] != "require" {
		t.Fatal("extra option was not persisted")
	}
}

func runCLI(t *testing.T, args ...string) map[string]any {
	t.Helper()
	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %v failed: %v", args, err)
	}
	var env map[string]any
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, out.String())
	}
	return env
}

func writeConnectionsFile(t *testing.T, path string, conns ...config.Connection) {
	t.Helper()
	root := config.Root{Version: 2, Connections: conns}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestQueryRunRejectsUnsupportedSessionKey(t *testing.T) {
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath,
		config.Connection{Name: "lite", DBType: "sqlite", Endpoint: config.Endpoint{Kind: "file", Path: filepath.Join(dir, "app.db")}},
	)

	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--config-dir", dir,
		"query", "run",
		"--connection", "lite",
		"--sql", "select 1",
		"--session", "role=x",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported session key")
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != "query.unsupported_session_key" {
		t.Fatalf("expected code query.unsupported_session_key, got %q", cliErr.Code)
	}
	if cliErr.Exit != 2 {
		t.Fatalf("expected exit 2, got %d", cliErr.Exit)
	}
	if !strings.Contains(cliErr.Message, "accepts no session keys") {
		t.Fatalf("expected 'accepts no session keys' in message, got %q", cliErr.Message)
	}
}

func TestQueryScriptRejectsUnsupportedDatasource(t *testing.T) {
	dir := t.TempDir()
	writeConnectionsFile(t, filepath.Join(dir, "connections.json"),
		config.Connection{Name: "lite", DBType: "sqlite", Endpoint: config.Endpoint{Kind: "file", Path: filepath.Join(dir, "app.db")}},
	)
	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config-dir", dir, "query", "script", "--connection", "lite", "--sql", "select 1; select 2"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported datasource")
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *CLIError, got %T: %v", err, err)
	}
	if cliErr.Code != "query.script_unsupported" || cliErr.Exit != 2 {
		t.Fatalf("unexpected: code=%q exit=%d", cliErr.Code, cliErr.Exit)
	}
}

func TestConnectionsImportBacksUpAndOverwrites(t *testing.T) {
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath,
		config.Connection{Name: "keep", DBType: "sqlite", Endpoint: config.Endpoint{Kind: "file", Path: "/tmp/keep.db"}},
		config.Connection{Name: "shared", DBType: "postgresql", Endpoint: config.Endpoint{Kind: "tcp", Host: "old-host", Username: "old"}},
	)
	srcPath := filepath.Join(dir, "incoming.json")
	writeConnectionsFile(t, srcPath,
		config.Connection{Name: "shared", DBType: "postgresql", Endpoint: config.Endpoint{Kind: "tcp", Host: "new-host", Username: "new", Password: "pw"}},
		config.Connection{Name: "fresh", DBType: "mysql", Endpoint: config.Endpoint{Kind: "tcp", Host: "fresh-host", Username: "u", Password: "pw"}},
	)

	env := runCLI(t, "--config-dir", dir, "connections", "import", srcPath)
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("import not ok: %v", env)
	}
	summary, _ := env["summary"].(map[string]any)
	if summary["added"].(float64) != 1 || summary["overwritten"].(float64) != 1 || summary["imported"].(float64) != 2 {
		t.Fatalf("unexpected summary: %v", summary)
	}
	backup, _ := summary["backup_path"].(string)
	if backup == "" {
		t.Fatal("expected backup_path in summary")
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	raw, err := os.ReadFile(connPath)
	if err != nil {
		t.Fatal(err)
	}
	var root config.Root
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	if len(root.Connections) != 3 {
		t.Fatalf("expected 3 connections after merge, got %d", len(root.Connections))
	}
	byName := map[string]config.Connection{}
	for _, c := range root.Connections {
		byName[c.Name] = c
	}
	if byName["shared"].Endpoint.Host != "new-host" {
		t.Fatalf("shared connection not overwritten: %+v", byName["shared"].Endpoint)
	}
	if _, ok := byName["keep"]; !ok {
		t.Fatal("existing connection 'keep' was dropped")
	}
	if _, ok := byName["fresh"]; !ok {
		t.Fatal("new connection 'fresh' was not added")
	}
}

func TestConnectionsImportDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	connPath := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, connPath,
		config.Connection{Name: "keep", DBType: "sqlite", Endpoint: config.Endpoint{Kind: "file", Path: "/tmp/keep.db"}},
	)
	before, err := os.ReadFile(connPath)
	if err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(dir, "incoming.json")
	writeConnectionsFile(t, srcPath,
		config.Connection{Name: "fresh", DBType: "mysql", Endpoint: config.Endpoint{Kind: "tcp", Host: "h", Username: "u", Password: "pw"}},
	)

	env := runCLI(t, "--config-dir", dir, "connections", "import", srcPath, "--dry-run")
	summary, _ := env["summary"].(map[string]any)
	if summary["dry_run"].(bool) != true || summary["added"].(float64) != 1 {
		t.Fatalf("unexpected dry-run summary: %v", summary)
	}
	if _, ok := summary["backup_path"]; ok {
		t.Fatal("dry-run should not produce a backup")
	}
	after, err := os.ReadFile(connPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("dry-run modified connections.json")
	}
}

func TestConnectionsImportRejectsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(srcPath, []byte(`{"version":2,"connections":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config-dir", dir, "connections", "import", srcPath})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for empty import file")
	}
}

func TestMCPServeRejectsUnsupportedTransport(t *testing.T) {
	dir := t.TempDir()
	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--config-dir", dir,
		"mcp", "serve",
		"--transport", "bad",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unsupported transport error")
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("error type = %T, want *CLIError", err)
	}
	if cliErr.Code != "mcp.unsupported_transport" {
		t.Fatalf("error code = %q, want mcp.unsupported_transport", cliErr.Code)
	}
	if out.Len() != 0 {
		t.Fatalf("unsupported transport should not write MCP frames, got %q", out.String())
	}
}

func TestExecuteMCPServeWritesStartupErrorsToStderr(t *testing.T) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		stdoutReader.Close()
		stderrReader.Close()
	})

	execErr := Execute([]string{"mcp", "serve", "--transport", "bad"})
	if closeErr := stdoutWriter.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if closeErr := stderrWriter.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if execErr == nil {
		t.Fatal("expected unsupported transport error")
	}
	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatal(err)
	}
	if len(stdout) != 0 {
		t.Fatalf("mcp serve startup error wrote to stdout: %q", string(stdout))
	}
	if !bytes.Contains(stderr, []byte(`"code":"mcp.unsupported_transport"`)) {
		t.Fatalf("stderr missing structured MCP startup error: %q", string(stderr))
	}
}
