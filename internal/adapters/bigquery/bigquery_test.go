package bigquery

import (
	"reflect"
	"testing"

	gcbq "cloud.google.com/go/bigquery"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
)

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
