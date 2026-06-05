package activity

import (
	"regexp"
	"testing"
)

var reAllowed = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func TestSanitizeIDRemovesUnsafe(t *testing.T) {
	got := SanitizeID("../a b/..")
	if !reAllowed.MatchString(got) {
		t.Fatalf("SanitizeID output %q contains forbidden chars or wrong length", got)
	}
	for _, bad := range []byte{'/', '.', ' '} {
		for _, c := range []byte(got) {
			if c == bad {
				t.Fatalf("SanitizeID output %q still contains %q", got, bad)
			}
		}
	}
}

func TestSanitizeIDTruncates(t *testing.T) {
	long := make([]byte, 100)
	for i := range long {
		long[i] = 'a'
	}
	got := SanitizeID(string(long))
	if len(got) != 64 {
		t.Fatalf("expected length 64, got %d", len(got))
	}
}

func TestSanitizeIDPassthrough(t *testing.T) {
	in := "valid-ID_123"
	if got := SanitizeID(in); got != in {
		t.Fatalf("expected passthrough %q, got %q", in, got)
	}
}

func TestNewSessionIDIsClean(t *testing.T) {
	id := NewSessionID("mcp")
	if !reAllowed.MatchString(id) {
		t.Fatalf("NewSessionID %q fails charset/length constraint", id)
	}
	// Must be stable under another sanitize pass.
	if SanitizeID(id) != id {
		t.Fatalf("NewSessionID output %q is not sanitize-stable", id)
	}
}

func TestNewSessionIDUniqueness(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewSessionID("mcp")
		if seen[id] {
			t.Fatalf("duplicate session id generated: %q", id)
		}
		seen[id] = true
	}
}
