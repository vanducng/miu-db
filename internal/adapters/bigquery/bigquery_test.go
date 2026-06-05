package bigquery

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	gcbq "cloud.google.com/go/bigquery"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
)

func conn(opts map[string]any) config.Connection {
	return config.Connection{Options: opts}
}

// optionTypeName returns the unexported type name of a ClientOption for
// asserting which option was appended without importing internal packages.
func optionTypeName(o interface{}) string {
	return fmt.Sprintf("%T", o)
}

// TestClientOptionsADCNoPath: default path — no credential option appended.
func TestClientOptionsADCNoPath(t *testing.T) {
	opts, err := clientOptions(conn(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 0 {
		t.Fatalf("expected 0 opts for plain ADC, got %d", len(opts))
	}
}

// TestClientOptionsADCExplicit: auth_method=adc behaves like the default.
func TestClientOptionsADCExplicit(t *testing.T) {
	opts, err := clientOptions(conn(map[string]any{"auth_method": "adc"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 0 {
		t.Fatalf("expected 0 opts for auth_method=adc without path, got %d", len(opts))
	}
}

// TestClientOptionsWithCredentialsPath: credential path appended via WithAuthCredentialsFile.
func TestClientOptionsWithCredentialsPath(t *testing.T) {
	opts, err := clientOptions(conn(map[string]any{
		"bigquery_credentials_path": "/tmp/sa.json",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("expected 1 opt, got %d", len(opts))
	}
	if !strings.Contains(optionTypeName(opts[0]), "AuthCredentialsFile") {
		t.Fatalf("expected WithAuthCredentialsFile option, got %s", optionTypeName(opts[0]))
	}
}

// TestClientOptionsServiceAccountRequiresPath: missing path ⇒ error.
func TestClientOptionsServiceAccountRequiresPath(t *testing.T) {
	_, err := clientOptions(conn(map[string]any{"auth_method": "service_account"}))
	if err == nil {
		t.Fatal("expected error when service_account has no path")
	}
	if !strings.Contains(err.Error(), "bigquery_credentials_path") {
		t.Fatalf("error should mention bigquery_credentials_path, got: %v", err)
	}
}

// TestClientOptionsServiceAccountWithPath: valid service_account ⇒ one cred option.
func TestClientOptionsServiceAccountWithPath(t *testing.T) {
	opts, err := clientOptions(conn(map[string]any{
		"auth_method":               "service_account",
		"bigquery_credentials_path": "/tmp/sa.json",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("expected 1 opt, got %d", len(opts))
	}
	if !strings.Contains(optionTypeName(opts[0]), "AuthCredentialsFile") {
		t.Fatalf("expected WithAuthCredentialsFile option, got %s", optionTypeName(opts[0]))
	}
}

// TestClientOptionsUnknownAuthMethod: clear error for bad auth_method value.
func TestClientOptionsUnknownAuthMethod(t *testing.T) {
	_, err := clientOptions(conn(map[string]any{"auth_method": "magic"}))
	if err == nil {
		t.Fatal("expected error for unknown auth_method")
	}
	if !strings.Contains(err.Error(), "magic") {
		t.Fatalf("error should contain the unknown value, got: %v", err)
	}
}

// TestClientOptionsQuotaProject: bigquery_quota_project appended as WithQuotaProject.
func TestClientOptionsQuotaProject(t *testing.T) {
	opts, err := clientOptions(conn(map[string]any{
		"bigquery_quota_project": "my-billing-project",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("expected 1 opt (QuotaProject), got %d", len(opts))
	}
	if !strings.Contains(optionTypeName(opts[0]), "withQuotaProject") {
		t.Fatalf("expected withQuotaProject option, got %s", optionTypeName(opts[0]))
	}
}

// TestClientOptionsQuotaProjectWithCreds: both cred path and quota project ⇒ 2 opts.
func TestClientOptionsQuotaProjectWithCreds(t *testing.T) {
	opts, err := clientOptions(conn(map[string]any{
		"bigquery_credentials_path": "/tmp/sa.json",
		"bigquery_quota_project":    "billing-project",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 2 {
		t.Fatalf("expected 2 opts (creds + quota), got %d", len(opts))
	}
}

// TestWrapCredentialError: missing-ADC errors get the gcloud hint.
func TestWrapCredentialError(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantHit bool
	}{
		{"google SDK phrasing", "could not find default credentials", true},
		{"alternate phrasing", "application default credentials not found", true},
		{"ADC caps variant", "Application Default Credentials error", true},
		{"unrelated error", "connection refused", false},
		{"nil error", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.input == "" {
				if got := wrapCredentialError(nil); got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			wrapped := wrapCredentialError(errors.New(tc.input))
			if tc.wantHit {
				if !strings.Contains(wrapped.Error(), "gcloud auth application-default login") {
					t.Fatalf("expected gcloud hint in error, got: %v", wrapped)
				}
				if !errors.Is(wrapped, errors.New(tc.input)) {
					// verify original is wrapped
					unwrapped := errors.Unwrap(wrapped)
					if unwrapped == nil || unwrapped.Error() != tc.input {
						t.Fatalf("original error not wrapped: %v", wrapped)
					}
				}
			} else {
				if wrapped.Error() != tc.input {
					t.Fatalf("unrelated error should be unchanged, got: %v", wrapped)
				}
			}
		})
	}
}

// TestApplySessionSetsLocationAndMaxBytes: existing test kept.
func TestApplySessionSetsLocationAndMaxBytes(t *testing.T) {
	q := &gcbq.Query{}
	if err := applySession(q, map[string]any{"bigquery_location": "US", "bigquery_maximum_bytes_billed": "1000000"}); err != nil {
		t.Fatalf("applySession: %v", err)
	}
	if q.Location != "US" {
		t.Fatalf("expected location US, got %q", q.Location)
	}
	if q.MaxBytesBilled != 1000000 {
		t.Fatalf("expected MaxBytesBilled 1000000, got %d", q.MaxBytesBilled)
	}
}

func TestApplySessionInvalidMaxBytes(t *testing.T) {
	q := &gcbq.Query{}
	if err := applySession(q, map[string]any{"bigquery_maximum_bytes_billed": "abc"}); err == nil {
		t.Fatal("expected error for non-integer maximum_bytes_billed")
	}
}

func TestBigQueryRejectsUnknownSessionKey(t *testing.T) {
	_, err := adapter.ApplySession(New(), config.Connection{DBType: "bigquery"}, map[string]any{"bogus": "x"})
	usk, ok := err.(*adapter.UnsupportedSessionKeyError)
	if !ok {
		t.Fatalf("expected *adapter.UnsupportedSessionKeyError, got %T", err)
	}
	want := []string{"bigquery_location", "bigquery_maximum_bytes_billed"}
	if !reflect.DeepEqual(usk.Supported, want) {
		t.Fatalf("expected %v, got %v", want, usk.Supported)
	}
}
