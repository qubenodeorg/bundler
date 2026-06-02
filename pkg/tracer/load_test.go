package tracer

import (
	"strings"
	"testing"

	"github.com/zarbanio/bundler/pkg/userop"
)

func TestNewTracers_LoadsAllVersions(t *testing.T) {
	ts, err := NewTracers()
	if err != nil {
		t.Fatalf("NewTracers: %v", err)
	}
	if ts.V06.Collector == "" {
		t.Error("V06.Collector is empty")
	}
	if ts.V06.Execution == "" {
		t.Error("V06.Execution is empty")
	}
	if ts.V07.Collector == "" {
		t.Error("V07.Collector is empty")
	}
	if ts.V07.Execution == "" {
		t.Error("V07.Execution is empty")
	}
	if ts.V08.Collector == "" {
		t.Error("V08.Collector is empty")
	}
	if ts.V08.Execution == "" {
		t.Error("V08.Execution is empty")
	}
	if ts.V09.Collector == "" {
		t.Error("V09.Collector is empty")
	}
	if ts.V09.Execution == "" {
		t.Error("V09.Execution is empty")
	}
}

func TestParse_StripsCommentsAndWhitespace(t *testing.T) {
	in := "var tracer = {a:1, b:2};\n// trailing comment\n"
	out := parse(in)
	if strings.Contains(out, "//") {
		t.Errorf("parse did not strip comments: %s", out)
	}
	if strings.Contains(out, "\n") {
		t.Errorf("parse did not strip newlines: %s", out)
	}
	if strings.Contains(out, "  ") {
		t.Errorf("parse did not collapse whitespace: %s", out)
	}
	if !strings.HasPrefix(out, "{a:1,") {
		t.Errorf("parse did not trim prefix; got: %s", out)
	}
	if !strings.HasSuffix(out, "b:2}") {
		t.Errorf("parse did not trim suffix; got: %s", out)
	}
}

func TestParse_StripsWholeLineComment(t *testing.T) {
	in := "var tracer = {a:1};\n// whole-line comment\n"
	out := parse(in)
	if strings.Contains(out, "comment") {
		t.Errorf("parse did not strip whole-line comment: %q", out)
	}
}

func TestParse_KeepsTrailingSemicolonIntact(t *testing.T) {
	in := "var tracer = {a:1} ;"
	out := parse(in)
	if !strings.HasSuffix(out, "}") {
		t.Errorf("parse: expected trailing brace, got %q", out)
	}
}

func TestParse_Empty(t *testing.T) {
	if got := parse(""); got != "" {
		t.Errorf("parse empty: got %q, want %q", got, "")
	}
}

func TestForVersion_KnownVersions(t *testing.T) {
	ts, err := NewTracers()
	if err != nil {
		t.Fatalf("NewTracers: %v", err)
	}
	tests := []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8, userop.V0_9}
	for _, v := range tests {
		set, err := ts.ForVersion(v, "")
		if err != nil {
			t.Errorf("ForVersion(%s): unexpected error %v", v, err)
			continue
		}
		if set.Collector == "" {
			t.Errorf("ForVersion(%s): Collector is empty", v)
		}
		if set.Execution == "" {
			t.Errorf("ForVersion(%s): Execution is empty", v)
		}
	}
}

func TestForVersion_UnknownVersion(t *testing.T) {
	ts, _ := NewTracers()
	_, err := ts.ForVersion(userop.Version("v0.99"), "")
	if err == nil {
		t.Fatal("ForVersion(v0.99): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "v0.99") {
		t.Errorf("ForVersion(v0.99): error %q should mention the version", err)
	}
}

func TestForVersion_V09InjectsEntryPointAddress(t *testing.T) {
	ts, _ := NewTracers()
	ep := "0x433709009B8330FDa32311DF1C2AFA402eD8D009"
	set, err := ts.ForVersion(userop.V0_9, ep)
	if err != nil {
		t.Fatalf("ForVersion v0.9: %v", err)
	}
	if strings.Contains(set.Execution, entryPointPlaceholder) {
		t.Errorf("v0.9 execution tracer still has placeholder after injection")
	}
	if !strings.Contains(set.Execution, strings.ToLower(ep)) {
		t.Errorf("v0.9 execution tracer should contain the EntryPoint address %q", ep)
	}
}

func TestForVersion_V09NoInjectionWithoutAddress(t *testing.T) {
	ts, _ := NewTracers()
	set, err := ts.ForVersion(userop.V0_9, "")
	if err != nil {
		t.Fatalf("ForVersion v0.9 no addr: %v", err)
	}
	if !strings.Contains(set.Execution, entryPointPlaceholder) {
		t.Errorf("v0.9 execution tracer placeholder should remain when no address is passed")
	}
}

func TestForVersion_V06V07V08DoNotInjectAddress(t *testing.T) {
	ts, _ := NewTracers()
	ep := "0x433709009B8330FDa32311DF1C2AFA402eD8D009"
	for _, v := range []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8} {
		set, err := ts.ForVersion(v, ep)
		if err != nil {
			t.Fatalf("ForVersion %s: %v", v, err)
		}
		if strings.Contains(set.Collector, entryPointPlaceholder) {
			t.Errorf("%s collector should not contain entry-point placeholder", v)
		}
		if strings.Contains(set.Execution, entryPointPlaceholder) {
			t.Errorf("%s execution should not contain entry-point placeholder", v)
		}
	}
}

func TestBundlerCollectorTracer_CountsOpcodesAtRuntime(t *testing.T) {
	ts, _ := NewTracers()
	for _, v := range []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8, userop.V0_9} {
		set, err := ts.ForVersion(v, "")
		if err != nil {
			t.Fatalf("ForVersion %s: %v", v, err)
		}
		// Each collector should reference the stop-collecting topic and opcode.
		if !strings.Contains(set.Collector, "stopCollectingTopic") {
			t.Errorf("%s collector should declare stopCollectingTopic", v)
		}
		if !strings.Contains(set.Collector, "countSlot") {
			t.Errorf("%s collector should define countSlot helper", v)
		}
	}
}

func TestBundlerExecutionTracer_V09MentionsCurrentUserOpHash(t *testing.T) {
	ts, _ := NewTracers()
	set, _ := ts.ForVersion(userop.V0_9, "")
	if !strings.Contains(set.Execution, "currentUserOpHashes") {
		t.Errorf("v0.9 execution tracer should capture currentUserOpHashes")
	}
	if !strings.Contains(set.Execution, "getCurrentUserOpHash") {
		t.Errorf("v0.9 execution tracer should reference getCurrentUserOpHash")
	}
}

func TestBundlerExecutionTracer_AllVersionsHaveUserOpEvent(t *testing.T) {
	ts, _ := NewTracers()
	for _, v := range []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8, userop.V0_9} {
		set, _ := ts.ForVersion(v, "")
		if !strings.Contains(set.Execution, "_userOperationEventTopics0") {
			t.Errorf("%s execution should track UserOperationEvent topic", v)
		}
	}
}
