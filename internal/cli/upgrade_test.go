package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestUpgradeAssetName(t *testing.T) {
	cases := map[[2]string]string{
		{"darwin", "amd64"}:  "miudb_darwin_x86_64.tar.gz",
		{"darwin", "arm64"}:  "miudb_darwin_arm64.tar.gz",
		{"linux", "amd64"}:   "miudb_linux_x86_64.tar.gz",
		{"linux", "arm64"}:   "miudb_linux_arm64.tar.gz",
		{"windows", "amd64"}: "miudb_windows_x86_64.zip",
		{"windows", "arm64"}: "miudb_windows_arm64.zip",
		{"linux", "386"}:     "",
		{"plan9", "amd64"}:   "",
	}
	for in, want := range cases {
		if got := upgradeAssetName(in[0], in[1]); got != want {
			t.Errorf("upgradeAssetName(%q,%q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "aaa111  miudb_darwin_arm64.tar.gz\nBBB222  miudb_linux_x86_64.tar.gz\n"
	if got := checksumFor(sums, "miudb_linux_x86_64.tar.gz"); got != "bbb222" {
		t.Errorf("checksumFor = %q, want bbb222", got)
	}
	if got := checksumFor(sums, "miudb_windows_arm64.zip"); got != "" {
		t.Errorf("checksumFor for missing asset = %q, want empty", got)
	}
}

func TestNormalizeTag(t *testing.T) {
	if normalizeTag("v0.2.9") != normalizeTag(" V0.2.9 ") {
		t.Error("normalizeTag should ignore leading v and surrounding space/case")
	}
	if normalizeTag("v0.2.9") == normalizeTag("v0.3.0") {
		t.Error("different tags must not normalize equal")
	}
}

func TestBinaryName(t *testing.T) {
	if binaryName("windows") != "miudb.exe" {
		t.Error("windows binary must be miudb.exe")
	}
	if binaryName("linux") != "miudb" {
		t.Error("non-windows binary must be miudb")
	}
}

func TestUpgradeCommandRegistered(t *testing.T) {
	opts := &options{output: "json", limit: 100}
	root := rootCommand(opts)
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "upgrade" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("upgrade command not registered on root")
	}
	inCatalog := false
	for _, info := range catalog() {
		if info.Name == "upgrade" {
			inCatalog = true
		}
	}
	if !inCatalog {
		t.Fatal("upgrade not present in command catalog")
	}
}

func TestOutputPrettyFlagIndentsJSON(t *testing.T) {
	t.Cleanup(func() { prettyOutput = false })
	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-o", "pretty", "version"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "{\n  \"ok\": true") {
		t.Fatalf("expected indented JSON, got: %s", out.String())
	}
}

func TestOutputDefaultIsCompact(t *testing.T) {
	t.Cleanup(func() { prettyOutput = false })
	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("expected single-line compact JSON, got: %s", out.String())
	}
}

func TestOutputInvalidFormatErrors(t *testing.T) {
	t.Cleanup(func() { prettyOutput = false })
	opts := &options{output: "json", limit: 100}
	cmd := rootCommand(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-o", "yaml", "version"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != "output.invalid_format" {
		t.Fatalf("expected output.invalid_format CLIError, got %v", err)
	}
}
