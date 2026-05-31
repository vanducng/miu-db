package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/vanducng/miu-db/internal/config"
)

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
