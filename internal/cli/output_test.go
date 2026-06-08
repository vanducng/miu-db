package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteJSONScrubsSecrets(t *testing.T) {
	var buf bytes.Buffer
	err := writeJSON(&buf, Envelope{
		OK:      true,
		Kind:    "x",
		Command: "x",
		Summary: map[string]any{"password": "SUMMARYPW", "has_password": true, "host": "h"},
		Data: map[string]any{
			"endpoint":  map[string]any{"password": "ENDPOINTPW", "username": "u"},
			"api_key":   "APIKEYVAL",
			"token":     "TOKENVAL",
			"key_path":  "/home/u/id_rsa", // not a secret value — keep
			"note":      "plain text",
			"conn":      "mysql://u:URLPW@host/db",
			"rows":      []any{[]any{"rowsecretlike", "normal"}}, // positional query data — keep
			"row_count": 3,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, leaked := range []string{"SUMMARYPW", "ENDPOINTPW", "APIKEYVAL", "TOKENVAL", "u:URLPW@"} {
		if strings.Contains(out, leaked) {
			t.Errorf("secret %q leaked into output: %s", leaked, out)
		}
	}
	for _, kept := range []string{`"has_password":true`, `"host":"h"`, `"username":"u"`, "/home/u/id_rsa", "plain text", "rowsecretlike", "normal", `"row_count":3`} {
		if !strings.Contains(out, kept) {
			t.Errorf("expected %q preserved, missing from: %s", kept, out)
		}
	}
}
