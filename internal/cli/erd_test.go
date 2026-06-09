package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func TestDefaultErdBase(t *testing.T) {
	t.Run("env var wins", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("VD_VISUALS_PATH", dir)
		got := defaultErdBase()
		if got != dir {
			t.Fatalf("want %q got %q", dir, got)
		}
	})

	t.Run("git-root .work present", func(t *testing.T) {
		root := t.TempDir()
		// Initialize a real git repo so git rev-parse --show-toplevel works.
		if err := runCmd(root, "git", "init", "-q"); err != nil {
			t.Skipf("git init failed: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(root, ".work"), 0o755); err != nil {
			t.Fatal(err)
		}
		sub := filepath.Join(root, "sub")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		orig, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(sub); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(orig) })

		t.Setenv("VD_VISUALS_PATH", "")
		got := defaultErdBase()
		// git rev-parse may resolve symlinks (e.g. /var → /private/var on macOS).
		resolvedRoot, _ := filepath.EvalSymlinks(root)
		if resolvedRoot == "" {
			resolvedRoot = root
		}
		want := filepath.Join(resolvedRoot, ".work", "visuals")
		if got != want {
			t.Fatalf("want %q got %q", want, got)
		}
	})

	t.Run("fallback .diagrams", func(t *testing.T) {
		// No git repo, no VD_VISUALS_PATH.
		dir := t.TempDir()
		orig, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(orig) })

		t.Setenv("VD_VISUALS_PATH", "")
		got := defaultErdBase()
		if got != ".diagrams" {
			t.Fatalf("want .diagrams got %q", got)
		}
	})
}
