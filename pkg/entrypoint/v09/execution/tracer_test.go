package execution

import (
	"testing"

	"github.com/zarbanio/bundler/pkg/tracer"
	"github.com/zarbanio/bundler/pkg/userop"
)

// TestForVersion_ExecutionTracer_Resolves ensures that the v0.9 execution
// tracer can be resolved and is non-empty. This is a smoke test that catches
// accidental regressions in the version-aware loader.
func TestForVersion_ExecutionTracer_Resolves(t *testing.T) {
	set, err := tracer.Loaded.ForVersion(userop.V0_9, "0x433709009b8330fda32311df1c2afa402ed8d009")
	if err != nil {
		t.Fatalf("ForVersion(V0_9): %v", err)
	}
	if set.Execution == "" {
		t.Fatal("V0_9 execution tracer is empty")
	}
	if set.Collector == "" {
		t.Fatal("V0_9 collector tracer is empty")
	}
}

// TestForVersion_EntryPointInjection_Execution ensures that supplying a
// non-empty EntryPoint address injects the placeholder in the v0.9 tracer.
func TestForVersion_EntryPointInjection_Execution(t *testing.T) {
	ep := "0x433709009b8330fda32311df1c2afa402ed8d009"
	set, err := tracer.Loaded.ForVersion(userop.V0_9, ep)
	if err != nil {
		t.Fatalf("ForVersion: %v", err)
	}
	if got := containsPlaceholder(set.Execution); got {
		t.Errorf("V0_9 execution tracer still contains entry-point placeholder")
	}
	if !containsHex(set.Execution, ep) {
		t.Errorf("V0_9 execution tracer missing entry-point address %q", ep)
	}
}

// TestForVersion_EntryPointInjection_Collector ensures the v0.9 collector
// tracer loads (it does not embed the EntryPoint address because it has no
// STATICCALL detection logic).
func TestForVersion_EntryPointInjection_Collector(t *testing.T) {
	set, err := tracer.Loaded.ForVersion(userop.V0_9, "0x433709009b8330fda32311df1c2afa402ed8d009")
	if err != nil {
		t.Fatalf("ForVersion: %v", err)
	}
	if set.Collector == "" {
		t.Fatal("V0_9 collector tracer is empty")
	}
}

const entryPointPlaceholder = "__ENTRY_POINT_ADDRESS_HEX__"

func containsPlaceholder(s string) bool {
	for i := 0; i+len(entryPointPlaceholder) <= len(s); i++ {
		if s[i:i+len(entryPointPlaceholder)] == entryPointPlaceholder {
			return true
		}
	}
	return false
}

func containsHex(s, h string) bool {
	for i := 0; i+len(h) <= len(s); i++ {
		if s[i:i+len(h)] == h {
			return true
		}
	}
	return false
}
