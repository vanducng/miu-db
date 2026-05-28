package cli

import "testing"

func TestVersionStringPrefersInjectedReleaseVersion(t *testing.T) {
	previous := version
	t.Cleanup(func() { version = previous })
	version = "v9.9.9-test"
	if got := versionString(); got != version {
		t.Fatalf("versionString() = %q, want %q", got, version)
	}
}
