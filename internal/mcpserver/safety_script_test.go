package mcpserver

import "testing"

func TestRequireScriptMutationGate(t *testing.T) {
	readOnly := safetyPolicy{}                    // server NOT started with --allow-mutate
	mutable := safetyPolicy{allowMutations: true} // server started with --allow-mutate

	// Read-only multi-statement script always runs — interior ';' must NOT block
	// it (the isReadOnlySQL bug query_script must avoid).
	if err := readOnly.requireScriptMutation("select 1; show tables; describe t", false); err != nil {
		t.Fatalf("read-only multi must pass on a read-only server: %v", err)
	}
	if err := mutable.requireScriptMutation("select 1; select 2", false); err != nil {
		t.Fatalf("read-only multi must pass regardless of ack: %v", err)
	}

	// Mutating script on a read-only server is blocked outright.
	if err := readOnly.requireScriptMutation("select 1; delete from t", false); err == nil {
		t.Fatal("mutating script must be blocked on a read-only server")
	}

	// Mutating script on a mutable server still needs the per-call ack.
	if err := mutable.requireScriptMutation("delete from t", false); err == nil {
		t.Fatal("mutating script without allow_mutate=true must be blocked")
	}
	if err := mutable.requireScriptMutation("delete from t", true); err != nil {
		t.Fatalf("mutating script with server allow + ack must pass: %v", err)
	}
}
