package snowflake

import (
	"testing"

	"github.com/vanducng/miu-db/internal/adapter"
)

// Compile-time: Snowflake opts into the script capability.
var _ adapter.ScriptRunner = Provider{}

func TestSnowflakeResolvesAsScriptRunner(t *testing.T) {
	runner, err := adapter.ResolveScriptRunner(New())
	if err != nil {
		t.Fatalf("snowflake should support scripts, got: %v", err)
	}
	if runner == nil {
		t.Fatal("expected non-nil ScriptRunner")
	}
}
