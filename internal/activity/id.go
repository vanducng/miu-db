package activity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"
)

var reUnsafe = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// NewSessionID mints a collision-resistant session id using prefix, unix-nano,
// and 4 bytes of crypto/rand hex. Output is always SanitizeID-clean.
func NewSessionID(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	raw := fmt.Sprintf("%s_%d_%s", prefix, time.Now().UnixNano(), hex.EncodeToString(b))
	return SanitizeID(raw)
}

// SanitizeID keeps [A-Za-z0-9_-], replaces other chars with '-', truncates to 64.
// Use before any filepath.Join involving user-supplied ids.
func SanitizeID(s string) string {
	clean := reUnsafe.ReplaceAllString(s, "-")
	if len(clean) > 64 {
		clean = clean[:64]
	}
	return clean
}
