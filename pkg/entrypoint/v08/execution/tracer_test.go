package execution

import (
	"testing"

	"github.com/zarbanio/bundler/pkg/tracer"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestForVersion_ExecutionTracer_ResolvesV08(t *testing.T) {
	set, err := tracer.Loaded.ForVersion(userop.V0_8, "")
	if err != nil {
		t.Fatalf("ForVersion(V0_8): %v", err)
	}
	if set.Execution == "" {
		t.Fatal("V0_8 execution tracer is empty")
	}
	if set.Collector == "" {
		t.Fatal("V0_8 collector tracer is empty")
	}
}

func TestForVersion_ExecutionTracer_ResolvesV07(t *testing.T) {
	set, err := tracer.Loaded.ForVersion(userop.V0_7, "")
	if err != nil {
		t.Fatalf("ForVersion(V0_7): %v", err)
	}
	if set.Execution == "" {
		t.Fatal("V0_7 execution tracer is empty")
	}
	if set.Collector == "" {
		t.Fatal("V0_7 collector tracer is empty")
	}
}
