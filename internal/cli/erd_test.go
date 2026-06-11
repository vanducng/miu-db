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

func TestSlugifyConn(t *testing.T) {
	cases := []struct{ in, want string }{
		{"shop", "shop"},
		{"prod/orders", "prod-orders"},
		{"prod/orders/v2", "prod-orders-v2"},
		{"my_conn", "my_conn"},
		{"v1.2/db", "v1-2-db"},
		{"a b", "a-b"},
		{"../etc/passwd", "etc-passwd"},
		{"..", "erd"},
		{"/", "erd"},
		{"", "erd"},
		{"/leading", "leading"},
		{"trailing/", "trailing"},
		{"a///b", "a-b"}, // runs of separators collapse to one dash
		{"日本", "erd"},    // all-non-ASCII degrades to the fallback
	}
	for _, c := range cases {
		if got := slugifyConn(c.in); got != c.want {
			t.Errorf("slugifyConn(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Slugification is intentionally lossy: distinct refs that differ only in
// separator chars share one output leaf. Documented in docs/content/erd.md;
// use --out-dir to disambiguate. This locks the accepted-by-design behavior.
func TestSlugifyConnCollisionsAreByDesign(t *testing.T) {
	for _, pair := range [][2]string{
		{"prod/orders", "prod-orders"},
		{"a/b", "a.b"},
	} {
		if slugifyConn(pair[0]) != slugifyConn(pair[1]) {
			t.Errorf("expected %q and %q to collide, got %q vs %q",
				pair[0], pair[1], slugifyConn(pair[0]), slugifyConn(pair[1]))
		}
	}
}

func TestErdOutputDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VD_VISUALS_PATH", dir)

	if got, want := erdOutputDir("prod/orders"), filepath.Join(dir, "prod-orders-erd"); got != want {
		t.Fatalf("want %q got %q", want, got)
	}
	// A slashed ref must stay a single flat segment, never a nested group dir.
	if got := erdOutputDir("prod/orders"); filepath.Dir(got) != dir {
		t.Fatalf("expected flat leaf under %q, got nested %q", dir, got)
	}
}
