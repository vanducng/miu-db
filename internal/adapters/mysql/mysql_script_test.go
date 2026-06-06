package mysql

import (
	"testing"

	"github.com/vanducng/miu-db/internal/adapter"
)

// Compile-time: MySQL opts into the script capability.
var _ adapter.ScriptRunner = Provider{}

func TestMySQLResolvesAsScriptRunner(t *testing.T) {
	runner, err := adapter.ResolveScriptRunner(New())
	if err != nil {
		t.Fatalf("mysql should support scripts, got: %v", err)
	}
	if runner == nil {
		t.Fatal("expected non-nil ScriptRunner")
	}
}
