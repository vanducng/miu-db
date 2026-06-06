package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func snowflakeOAuthConn() Connection {
	return Connection{
		Name:   "sf-oauth",
		DBType: "snowflake",
		Endpoint: Endpoint{
			Kind:     "tcp",
			Host:     "acct.snowflakecomputing.com",
			Username: "analyst",
		},
		Options: map[string]any{
			"authenticator":           "oauth",
			"oauth_client_id":         "my-client-id",
			"oauth_client_secret":     "super-secret-value",
			"oauth_authorization_url": "https://acct.snowflakecomputing.com/oauth/authorize",
			"oauth_token_request_url": "https://acct.snowflakecomputing.com/oauth/token-request",
			"__oauth_access_token":    "live-bearer-token",
		},
	}
}

func TestRedactedConnectionOAuthSecretAbsent(t *testing.T) {
	conn := snowflakeOAuthConn()
	red := RedactedConnection(conn)
	encoded, err := json.Marshal(red)
	if err != nil {
		t.Fatal(err)
	}
	output := string(encoded)
	if strings.Contains(output, "super-secret-value") {
		t.Errorf("oauth_client_secret value must not appear in redacted output: %s", output)
	}
}

func TestRedactedConnectionOAuthSecretAppearsRedacted(t *testing.T) {
	conn := snowflakeOAuthConn()
	red := RedactedConnection(conn)
	opts, _ := red["options"].(map[string]any)
	if opts == nil {
		t.Fatal("redacted options must not be nil")
	}
	secretEntry, ok := opts["oauth_client_secret"]
	if !ok {
		t.Fatal("oauth_client_secret key must be present (as redacted marker) when the value is set")
	}
	marker, ok := secretEntry.(map[string]any)
	if !ok || marker["redacted"] != true {
		t.Errorf("oauth_client_secret must be {redacted:true}, got %v", secretEntry)
	}
}

func TestRedactedConnectionTransientTokenAbsent(t *testing.T) {
	conn := snowflakeOAuthConn()
	red := RedactedConnection(conn)
	opts, _ := red["options"].(map[string]any)
	if opts == nil {
		t.Fatal("redacted options must not be nil")
	}
	if _, found := opts["__oauth_access_token"]; found {
		t.Error("__oauth_access_token must be absent from redacted output (transient key)")
	}
	encoded, _ := json.Marshal(red)
	if strings.Contains(string(encoded), "__oauth_access_token") {
		t.Error("__oauth_access_token key must not appear anywhere in redacted output")
	}
	if strings.Contains(string(encoded), "live-bearer-token") {
		t.Error("live-bearer-token value must not appear in redacted output")
	}
}

func TestSensitiveTargetsOAuthClientSecret(t *testing.T) {
	conn := snowflakeOAuthConn()
	targets := SensitiveTargets(conn)
	found := false
	for _, t_ := range targets {
		if t_ == "options.oauth_client_secret" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SensitiveTargets must include options.oauth_client_secret; got %v", targets)
	}
}

func TestSensitiveTargetsTransientTokenNotListed(t *testing.T) {
	conn := snowflakeOAuthConn()
	for _, tgt := range SensitiveTargets(conn) {
		if strings.Contains(tgt, "__oauth_access_token") {
			t.Errorf("SensitiveTargets must not list transient key %q", tgt)
		}
	}
}

func TestRedactOptionsDoubleUnderscoreSkipped(t *testing.T) {
	opts := map[string]any{
		"__runtime_key":  "runtime-value",
		"normal_key":     "visible",
		"password":       "hidden",
		"__another_temp": "also-gone",
	}
	out := RedactOptions(opts)
	if _, found := out["__runtime_key"]; found {
		t.Error("__runtime_key must be absent from RedactOptions output")
	}
	if _, found := out["__another_temp"]; found {
		t.Error("__another_temp must be absent from RedactOptions output")
	}
	if out["normal_key"] != "visible" {
		t.Errorf("normal_key should be visible, got %v", out["normal_key"])
	}
	if _, ok := out["password"].(map[string]any); !ok {
		t.Errorf("password must be redacted marker, got %v", out["password"])
	}
}

func TestRedactOptionsNoLeakInJSONMarshal(t *testing.T) {
	opts := map[string]any{
		"__oauth_access_token": "bearer-xyz",
		"oauth_client_secret":  "client-secret-abc",
		"authenticator":        "oauth",
	}
	out := RedactOptions(opts)
	encoded, _ := json.Marshal(out)
	s := string(encoded)
	if strings.Contains(s, "bearer-xyz") {
		t.Error("access token value must not appear in marshaled RedactOptions output")
	}
	if strings.Contains(s, "client-secret-abc") {
		t.Error("client secret value must not appear in marshaled RedactOptions output")
	}
}
