package activity

import (
	"regexp"
	"strings"
)

var (
	reStringLit = regexp.MustCompile(`'[^']*'`)
	reNumLit    = regexp.MustCompile(`\b\d+(\.\d+)?\b`)
	reSpace     = regexp.MustCompile(`\s+`)
)

// Shape normalises sql into a grouping key by replacing string/number literals
// with ? and collapsing whitespace. Best-effort only — not a full SQL parser;
// edge cases (e.g. escaped quotes, hex literals) are left as-is.
func Shape(sql string) string {
	s := reStringLit.ReplaceAllString(sql, "?")
	s = reNumLit.ReplaceAllString(s, "?")
	s = reSpace.ReplaceAllString(strings.TrimSpace(s), " ")
	return s
}
