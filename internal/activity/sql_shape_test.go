package activity

import "testing"

func TestShapeNormalizesLiterals(t *testing.T) {
	a := Shape("SELECT * FROM t WHERE id=42 AND name='x'")
	b := Shape("SELECT * FROM t WHERE id=99 AND name='y'")
	if a != b {
		t.Fatalf("expected same shape, got:\n  a=%q\n  b=%q", a, b)
	}
}

func TestShapeCollapseWhitespace(t *testing.T) {
	a := Shape("SELECT  *   FROM   t")
	b := Shape("SELECT * FROM t")
	if a != b {
		t.Fatalf("whitespace not collapsed: %q vs %q", a, b)
	}
}

func TestShapeMultipleStringLiterals(t *testing.T) {
	a := Shape("INSERT INTO t (a,b) VALUES ('foo','bar')")
	b := Shape("INSERT INTO t (a,b) VALUES ('baz','qux')")
	if a != b {
		t.Fatalf("multiple string literals not normalised: %q vs %q", a, b)
	}
}

func TestShapePreservesKeywords(t *testing.T) {
	got := Shape("SELECT id FROM users WHERE active=1")
	want := "SELECT id FROM users WHERE active=?"
	if got != want {
		t.Fatalf("keyword case changed or unexpected output: got %q want %q", got, want)
	}
}

func TestShapeEmptyInput(t *testing.T) {
	if got := Shape(""); got != "" {
		t.Fatalf("empty input should produce empty output, got %q", got)
	}
}
