package tracer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/zarbanio/bundler/pkg/userop"
)

// jsHelpers defines the small helper surface (toHex, toAddress, toWord) that
// the Geth-style tracers depend on. We mimic the parts needed by our tests.
const jsHelpers = `
function toHex(v) {
    if (v === null || v === undefined) return "0x";
    if (typeof v === "string") {
        if (v.startsWith("0x")) return v;
        if (v.length === 0) return "0x";
        // Try hex first (most common EVM value format), then decimal.
        try { return "0x" + BigInt(v).toString(16); } catch (e) {}
        return "0x";
    }
    if (typeof v === "number" || typeof v === "bigint") {
        return "0x" + BigInt(v).toString(16);
    }
    if (Array.isArray(v)) {
        // Byte array (memory slice): join each byte as 2-hex.
        var s = "0x";
        for (var i = 0; i < v.length; i++) {
            var b = Number(v[i]) & 0xff;
            var h = b.toString(16);
            if (h.length < 2) h = "0" + h;
            s += h;
        }
        return s;
    }
    if (typeof v === "object") {
        if (v.hex && typeof v.hex === "string") return v.hex;
        if (v.toString) {
            var s = v.toString();
            if (typeof s === "string") {
                if (s.startsWith("0x")) return s;
                try { return "0x" + BigInt(s).toString(16); } catch (e) {}
            }
        }
    }
    return "0x";
}
function toAddress(v) {
    if (typeof v === "string") {
        if (v.startsWith("0x")) {
            // Pad to 40 hex chars (20 bytes) like a real address.
            var s = v.slice(2);
            while (s.length < 40) s = "0" + s;
            return "0x" + s;
        }
        // BigInt.toString(16) returns the hex without prefix. Wrap as object.
        return { hex: "0x" + v };
    }
    return v;
}
function toWord(v) { return v; }
`

// runCollectorJS loads a v0.6/v0.7/v0.8/v0.9 collector tracer into a fresh
// goja runtime together with the helper stubs. It returns the runtime so the
// caller can drive step/fault/result manually.
func runCollectorJS(t *testing.T, v userop.Version) *goja.Runtime {
	t.Helper()
	ts, err := NewTracers()
	if err != nil {
		t.Fatalf("NewTracers: %v", err)
	}
	set, err := ts.ForVersion(v, "")
	if err != nil {
		t.Fatalf("ForVersion %s: %v", v, err)
	}
	rt := goja.New()
	mustRun(t, rt, jsHelpers)
	mustRun(t, rt, "var tracer = "+set.Collector+";")
	return rt
}

// runExecutionJS loads an execution tracer into a fresh runtime.
func runExecutionJS(t *testing.T, v userop.Version, entryPoint string) *goja.Runtime {
	t.Helper()
	ts, err := NewTracers()
	if err != nil {
		t.Fatalf("NewTracers: %v", err)
	}
	set, err := ts.ForVersion(v, entryPoint)
	if err != nil {
		t.Fatalf("ForVersion %s: %v", v, err)
	}
	rt := goja.New()
	mustRun(t, rt, jsHelpers)
	mustRun(t, rt, "var tracer = "+set.Execution+";")
	return rt
}

func mustRun(t *testing.T, rt *goja.Runtime, src string) {
	t.Helper()
	if _, err := rt.RunString(src); err != nil {
		t.Fatalf("goja: %v\nsource:\n%s", err, src)
	}
}

// stepLog is a minimal stand-in for the Getg EVM step log. We only populate
// the fields that the tracers actually read.
type stepLog struct {
	Op       string
	Gas      int64
	Cost     int64
	Depth    int
	Memory   []byte
	StackTop []string
	Contract string
	Address  string
}

// callJS invokes tracer.step(log) with a JavaScript object built from the Go
// stepLog. It avoids depending on Getg struct shapes.
func callJS(t *testing.T, rt *goja.Runtime, method string, log *stepLog) {
	t.Helper()
	if log == nil {
		mustRun(t, rt, "tracer."+method+"(null, null);")
		return
	}
	stackJSON := "["
	for i, v := range log.StackTop {
		if i > 0 {
			stackJSON += ","
		}
		stackJSON += fmt.Sprintf("%q", v)
	}
	stackJSON += "]"
	mustRun(t, rt, "var __stack = "+stackJSON+";")

	src := fmt.Sprintf(
		"tracer.%s({op: %q, getDepth: function(){return %d}, getGas: function(){return %d}, getCost: function(){return %d}, stack: {length: function(){return %d}, peek: function(i){return BigInt(__stack[i])}}, memory: {slice: function(o,l){return []}}, contract: {getAddress: function(){return %q}}, getError: function(){return null}}, {getCode: function(a){return ''}});",
		method, log.Op, log.Depth, log.Gas, log.Cost, len(log.StackTop), log.Contract,
	)
	mustRun(t, rt, src)
}

func getInt(t *testing.T, rt *goja.Runtime, expr string) int {
	t.Helper()
	v, err := rt.RunString(expr)
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	return int(v.ToInteger())
}

func getString(t *testing.T, rt *goja.Runtime, expr string) string {
	t.Helper()
	v, err := rt.RunString(expr)
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	return v.String()
}

// ---- Collector tracer tests ----

func TestCollector_StopsAtBeforeExecutionTopic(t *testing.T) {
	rt := runCollectorJS(t, userop.V0_7)
	// Depth-1 LOG1 with the stop-collecting topic.
	callJS(t, rt, "step", &stepLog{
		Op:    "LOG1",
		Depth: 1,
		StackTop: []string{
			"0x0",
			"0x0",
			"0xbb47ee3e183a558b1a2ff0874b079f3fc5478b7454eacf2bfc5af2ff5878f972",
		},
	})
	stop := getInt(t, rt, "tracer.stopCollecting ? 1 : 0")
	if stop != 1 {
		t.Errorf("stopCollecting should be true after BeforeExecution log, got %d", stop)
	}
}

func TestCollector_CountsOpcodes(t *testing.T) {
	for _, v := range []userop.Version{userop.V0_7, userop.V0_8, userop.V0_9} {
		v := v
		t.Run(string(v), func(t *testing.T) {
			rt := runCollectorJS(t, v)

			// Top-level CALL to enter the "current level".
			callJS(t, rt, "step", &stepLog{
				Op:       "CALL",
				Depth:    1,
				Gas:      10000,
				Cost:     100,
				StackTop: []string{"0x0", "0x1111111111111111111111111111111111111111", "0x0", "0x0", "0x0", "0x0", "0x0"},
			})

			// Subsequent steps at depth 2 count opcodes.
			for i := 0; i < 3; i++ {
				callJS(t, rt, "step", &stepLog{
					Op:       "TSTORE",
					Depth:    2,
					Gas:      1000,
					Cost:     100,
					StackTop: []string{"0x0", "0x0"},
				})
			}
			callJS(t, rt, "step", &stepLog{
				Op:       "TLOAD",
				Depth:    2,
				Gas:      1000,
				Cost:     100,
				StackTop: []string{"0x0"},
			})

			tstore := getInt(t, rt, "tracer.currentLevel.opcodes.TSTORE || 0")
			tload := getInt(t, rt, "tracer.currentLevel.opcodes.TLOAD || 0")
			if tstore != 3 {
				t.Errorf("TSTORE: got %d, want 3", tstore)
			}
			if tload != 1 {
				t.Errorf("TLOAD: got %d, want 1", tload)
			}
		})
	}
}

func TestCollector_MarksOOGOnGasUnderrun(t *testing.T) {
	rt := runCollectorJS(t, userop.V0_7)
	// Enter a current level.
	callJS(t, rt, "step", &stepLog{
		Op:       "CALL",
		Depth:    1,
		Gas:      10000,
		Cost:     100,
		StackTop: []string{"0x0", "0x1111111111111111111111111111111111111111", "0x0", "0x0", "0x0", "0x0", "0x0"},
	})
	// Step that runs out of gas.
	callJS(t, rt, "step", &stepLog{
		Op:       "CALL",
		Depth:    2,
		Gas:      50,
		Cost:     100,
		StackTop: []string{"0x0", "0x2222222222222222222222222222222222222222", "0x0", "0x0", "0x0", "0x0", "0x0"},
	})
	oog := getInt(t, rt, "tracer.currentLevel.oog ? 1 : 0")
	if oog != 1 {
		t.Errorf("expected oog=true, got %d", oog)
	}
}

func TestCollector_V09_ResultHasExpectedShape(t *testing.T) {
	rt := runCollectorJS(t, userop.V0_9)
	mustRun(t, rt, "var r = tracer.result({}, {});")
	res := getString(t, rt, "JSON.stringify(r)")
	for _, key := range []string{
		"callsFromEntryPoint",
		"keccak",
		"logs",
		"calls",
		"debug",
	} {
		if !strings.Contains(res, key) {
			t.Errorf("v0.9 result should include key %q; got %s", key, res)
		}
	}
}

// ---- Execution tracer tests ----

func TestExecution_MarkersAdvanceOnNumberV07(t *testing.T) {
	rt := runExecutionJS(t, userop.V0_7, "")
	for i := 0; i < 3; i++ {
		callJS(t, rt, "step", &stepLog{
			Op:    "NUMBER",
			Depth: 1,
			Gas:   100,
			Cost:  2,
		})
	}
	marker := getInt(t, rt, "tracer._marker")
	if marker != 3 {
		t.Errorf("expected _marker=3, got %d", marker)
	}
	if getInt(t, rt, "tracer._isExecution() ? 1 : 0") != 1 {
		t.Errorf("expected _isExecution()=true")
	}
}

func TestExecution_ValidationOOG(t *testing.T) {
	rt := runExecutionJS(t, userop.V0_7, "")
	// One NUMBER -> marker 1, which makes _isValidation() true.
	callJS(t, rt, "step", &stepLog{Op: "NUMBER", Depth: 1, Gas: 100, Cost: 2})
	// Then a step with gas < cost while in validation.
	callJS(t, rt, "step", &stepLog{Op: "CALL", Depth: 2, Gas: 1, Cost: 100})
	if getInt(t, rt, "tracer.validationOOG ? 1 : 0") != 1 {
		t.Errorf("expected validationOOG=true")
	}
}

func TestExecution_ExecutionOOG(t *testing.T) {
	rt := runExecutionJS(t, userop.V0_7, "")
	// Three NUMBERs -> _marker=3 -> _isExecution() true.
	for i := 0; i < 3; i++ {
		callJS(t, rt, "step", &stepLog{Op: "NUMBER", Depth: 1, Gas: 100, Cost: 2})
	}
	callJS(t, rt, "step", &stepLog{Op: "CALL", Depth: 2, Gas: 1, Cost: 100})
	if getInt(t, rt, "tracer.executionOOG ? 1 : 0") != 1 {
		t.Errorf("expected executionOOG=true")
	}
}

func TestExecution_CapturesUserOpEventV09(t *testing.T) {
	rt := runExecutionJS(t, userop.V0_9, "")
	// Move into execution region (3 NUMBERs).
	for i := 0; i < 3; i++ {
		callJS(t, rt, "step", &stepLog{Op: "NUMBER", Depth: 1, Gas: 100, Cost: 2})
	}
	// Build a LOG3 with the userOpEvent topic. The tracer reads ofs, len
	// from peek(0) and peek(1) and topics[0] from peek(2). Provide 5 stack
	// slots so all peeks succeed. The expected topic is
	// 0x49628fd1471006c1482da88028e9ce4dbb080b815c9b0344d39e5a8e6ec1419f.
	callJS(t, rt, "step", &stepLog{
		Op:    "LOG3",
		Depth: 2,
		Gas:   1000,
		Cost:  100,
		StackTop: []string{
			"0x0",
			"0x0",
			"0x49628fd1471006c1482da88028e9ce4dbb080b815c9b0344d39e5a8e6ec1419f",
			"0x1",
			"0x2",
		},
	})
	uop := getString(t, rt, "JSON.stringify(tracer.userOperationEvent || null)")
	if uop == "null" || uop == "" {
		t.Errorf("userOperationEvent not captured; got %s", uop)
	}
}

func TestExecution_CapturesGetCurrentUserOpHashV09(t *testing.T) {
	ep := "0x433709009b8330fda32311df1c2afa402ed8d009"
	rt := runExecutionJS(t, userop.V0_9, ep)
	// Move into execution region.
	for i := 0; i < 3; i++ {
		callJS(t, rt, "step", &stepLog{Op: "NUMBER", Depth: 1, Gas: 100, Cost: 2})
	}

	// Build a small memory that contains the getCurrentUserOpHash selector
	// (0x60e6b9fb) at offset 0 and is large enough for the slice.
	mustRun(t, rt, `
var __mem = [0x60, 0xe6, 0xb9, 0xfb];
for (var i = 4; i < 64; i++) __mem.push(0);
`)

	// Staticcall to the EntryPoint with selector getCurrentUserOpHash.
	// STATICCALL is executed in the *caller* frame (depth 1 in our trace
	// after the 3 NUMBERs above). The callee then runs at depth 2.
	mustRun(t, rt, fmt.Sprintf(`
tracer.step({
  op: "STATICCALL",
  getDepth: function(){return 1},
  getGas: function(){return 1000},
  getCost: function(){return 100},
  stack: {
    length: function(){return 7},
    peek: function(i){
      var s = ["0x64", %q, "0x0", "0x4", "0x0", "0x20", "0x0"];
      return s[i];
    }
  },
  memory: {slice: function(o,l){
    var out = [];
    for (var i = o; i < o + l && i < __mem.length; i++) out.push(__mem[i]);
    return out;
  }},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`, ep))

	// The tracer pushes a pending capture; now simulate a RETURN at
	// depth 2 (callee frame) that consumes it. We use the same
	// STATICCALL return path that _consumePendingGetCurrentUserOpHash
	// expects.
	mustRun(t, rt, `
tracer.step({
  op: "RETURN",
  getDepth: function(){return 2},
  getGas: function(){return 100},
  getCost: function(){return 0},
  stack: {length: function(){return 2}, peek: function(i){return i === 0 ? "0x0" : "0x20"}},
  memory: {slice: function(o,l){
    var out = [];
    for (var i = 0; i < 32; i++) out.push(0xaa);
    return out;
  }},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`)

	count := getInt(t, rt, "(tracer.currentUserOpHashes || []).length")
	if count < 1 {
		t.Errorf("expected at least one currentUserOpHash capture, got %d", count)
	}
}

func TestExecution_V06V07V08_NoCurrentUserOpHash(t *testing.T) {
	for _, v := range []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8} {
		v := v
		t.Run(string(v), func(t *testing.T) {
			rt := runExecutionJS(t, v, "")
			if rt == nil {
				t.Fatal("runtime is nil")
			}
			// The field should be undefined.
			has := getInt(t, rt, "typeof tracer.currentUserOpHashes === 'undefined' ? 1 : 0")
			if has != 1 {
				t.Errorf("%s should not define currentUserOpHashes", v)
			}
		})
	}
}

// ---- Regression tests for PR #25 review comments ----

// TestCollector_HandlesOOGBeforeFirstCall ensures the v0.7+ collector
// tracer does not crash with a TypeError when a gas-underrun happens at
// depth 1 before any CALL/STATICCALL initialises currentLevel.
func TestCollector_HandlesOOGBeforeFirstCall(t *testing.T) {
	for _, v := range []userop.Version{userop.V0_7, userop.V0_8, userop.V0_9} {
		v := v
		t.Run(string(v), func(t *testing.T) {
			rt := runCollectorJS(t, v)
			// Drive a depth-1 step with gas < cost. Before the fix this
			// threw "Cannot set property 'oog' of null".
			mustRun(t, rt, `
tracer.step({
  op: "ADD",
  getDepth: function(){return 1},
  getGas: function(){return 1},
  getCost: function(){return 100},
  stack: {length: function(){return 0}, peek: function(i){return BigInt(0)}},
  memory: {slice: function(o,l){return []}},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`)
		})
	}
}

// TestCollector_PopulatesTSTOREAndTLOADFields ensures the v0.7+ collector
// tracer populates the dedicated tstore / tload fields on the current
// CallFromEntryPointInfo (and not just the generic opcodes map).
func TestCollector_PopulatesTSTOREAndTLOADFields(t *testing.T) {
	for _, v := range []userop.Version{userop.V0_7, userop.V0_8, userop.V0_9} {
		v := v
		t.Run(string(v), func(t *testing.T) {
			rt := runCollectorJS(t, v)
			// Open a depth-1 CALL so currentLevel is initialised.
			mustRun(t, rt, `
tracer.step({
  op: "CALL",
  getDepth: function(){return 1},
  getGas: function(){return 1000},
  getCost: function(){return 100},
  stack: {
    length: function(){return 7},
    peek: function(i){
      var s = ["0x64", "0x1111111111111111111111111111111111111111", "0x0", "0x0", "0x0", "0x0", "0x0"];
      return s[i];
    }
  },
  memory: {slice: function(o,l){
    var out = [];
    for (var i = o; i < o + l && i < 4; i++) out.push(0x80 + i);
    return out;
  }},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`)
			// TSTORE/TLOAD happen inside the called code, at depth 2.
			for i := 0; i < 2; i++ {
				mustRun(t, rt, `
tracer.step({
  op: "TSTORE",
  getDepth: function(){return 2},
  getGas: function(){return 1000},
  getCost: function(){return 100},
  stack: {length: function(){return 0}, peek: function(i){return BigInt(0)}},
  memory: {slice: function(o,l){return []}},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`)
			}
			mustRun(t, rt, `
tracer.step({
  op: "TLOAD",
  getDepth: function(){return 2},
  getGas: function(){return 1000},
  getCost: function(){return 100},
  stack: {length: function(){return 0}, peek: function(i){return BigInt(0)}},
  memory: {slice: function(o,l){return []}},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`)
			ts := getInt(t, rt, "tracer.currentLevel.tstore")
			tl := getInt(t, rt, "tracer.currentLevel.tload")
			if ts != 2 {
				t.Errorf("%s tstore: got %d, want 2", v, ts)
			}
			if tl != 1 {
				t.Errorf("%s tload: got %d, want 1", v, tl)
			}
		})
	}
}

// TestCollector_V08_StandardHandleOpsHasNoTopLevelEIP7702Auth documents the
// v0.8 contract: standard handleOps has only two parameters
// (PackedUserOperation[] ops, address beneficiary), so the JS tracer
// cannot detect eip7702Auth from the calldata alone. The flag is therefore
// always false at this level; the Go simulation pipeline decodes auth from
// the PackedUserOperation tail instead.
func TestCollector_V08_StandardHandleOpsHasNoTopLevelEIP7702Auth(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{
			// Realistic v0.8 handleOps: selector + 2 head words (ops offset,
			// beneficiary) + minimal PackedUserOperation array length of 0.
			name: "standard_handleOps",
			payload: "0x1a5d1cd7" +
				"0000000000000000000000000000000000000000000000000000000000000020" +
				"0000000000000000000000001111111111111111111111111111111111111111" +
				"0000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			// Custom 3-head-word calldata with a 1-element auth tuple must
			// still return false under the v0.8/v0.9 standard 2-param ABI.
			name: "decoy_3head_with_auth",
			payload: "0x1a5d1cd7" +
				"0000000000000000000000000000000000000000000000000000000000000020" +
				"0000000000000000000000001111111111111111111111111111111111111111" +
				"0000000000000000000000000000000000000000000000000000000000000080" +
				"0000000000000000000000000000000000000000000000000000000000000001" +
				"0000000000000000000000002222222222222222222222222222222222222222",
		},
		{
			name:    "non_handleOps",
			payload: "0xdeadbeef00000000000000000000000000000000000000000000000000000000",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			rt := runCollectorJS(t, userop.V0_8)
			mustRun(t, rt, `
tracer.step({
  op: "CALL",
  getDepth: function(){return 1},
  getGas: function(){return 1000},
  getCost: function(){return 100},
  stack: {
    length: function(){return 7},
    peek: function(i){
      var s = ["0x64", "0x1111111111111111111111111111111111111111", "0x0", "0x0", "0x0", "0x0", "0x0"];
      return s[i];
    }
  },
  memory: {slice: function(o,l){return [0x80, 0xc5, 0xc7, 0xd0];}},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`)
			got := getInt(t, rt, fmt.Sprintf(`
(function(){
  var out = tracer.result({input: %q}, {getCode: function(a){return '';}});
  return out.callsFromEntryPoint[0].eip7702Auth ? 1 : 0;
})()
`, c.payload))
			if got != 0 {
				t.Errorf("v0.8 eip7702Auth must be false for standard handleOps; got %d", got)
			}
		})
	}
}

// TestCollector_V09_StandardHandleOpsHasNoTopLevelEIP7702Auth mirrors the
// v0.8 contract for v0.9: standard handleOps is (PackedUserOperation[] ops,
// address beneficiary), so the JS tracer returns false and lets the Go
// pipeline decode the auth from the userOp tail.
func TestCollector_V09_StandardHandleOpsHasNoTopLevelEIP7702Auth(t *testing.T) {
	payload := "0x1a5d1cd7" +
		"0000000000000000000000000000000000000000000000000000000000000020" +
		"0000000000000000000000001111111111111111111111111111111111111111" +
		"0000000000000000000000000000000000000000000000000000000000000000"
	rt := runCollectorJS(t, userop.V0_9)
	mustRun(t, rt, `
tracer.step({
  op: "CALL",
  getDepth: function(){return 1},
  getGas: function(){return 1000},
  getCost: function(){return 100},
  stack: {
    length: function(){return 7},
    peek: function(i){
      var s = ["0x64", "0x1111111111111111111111111111111111111111", "0x0", "0x0", "0x0", "0x0", "0x0"];
      return s[i];
    }
  },
  memory: {slice: function(o,l){return [0x80, 0xc5, 0xc7, 0xd0];}},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`)
	got := getInt(t, rt, fmt.Sprintf(`
(function(){
  var out = tracer.result({input: %q}, {getCode: function(a){return '';}});
  return out.callsFromEntryPoint[0].eip7702Auth ? 1 : 0;
})()
`, payload))
	if got != 0 {
		t.Errorf("v0.9 eip7702Auth must be false for standard handleOps; got %d", got)
	}
}

// TestCollector_V09_DetectsPaymasterSignature exercises the v0.9 collector's
// _hasPaymasterSignature helper directly with realistic ABI-encoded
// validatePaymasterUserOp calldata. The v0.9 PackedUserOperation's 13th
// head word (chars 768-832 of the payload) is the offset of the
// paymasterSignature tail relative to the start of the PackedUserOperation
// struct (which begins at byte 96 of the payload). We set that offset to
// zero in the test and abuse the sender word (chars 192-256) as the
// signature length, so the parser picks up the value it would read at
// payload[(96 + 0) * 2 : ...] = payload[192:256].
func TestCollector_V09_DetectsPaymasterSignature(t *testing.T) {
	// hexWord builds a 32-byte (64-hex-char) big-endian word.
	hexWord := func(v uint64) string {
		return fmt.Sprintf("%064x", v)
	}
	zeroWord := "0000000000000000000000000000000000000000000000000000000000000000"
	// pad builds `n` zero words of 64 hex chars each.
	pad := func(n int) string {
		out := ""
		for i := 0; i < n; i++ {
			out += zeroWord
		}
		return out
	}

	// validatePaymasterUserOp selector.
	const selector = "c04a6310"

	// 3 head words of validatePaymasterUserOp, then 10 PackedUserOperation
	// head words, then 12 zero words representing the variable-length
	// tail offsets. Total payload after selector = 3 + 10 + 12 = 25 words
	// = 1600 hex chars.
	build := func(senderLen uint64) string {
		// Word 12 (chars 768-832) of the payload is the 10th
		// PackedUserOperation head word = paymasterSignature offset.
		// Setting it to 0 means the parser reads the length from
		// payload[(96+0)*2 : (96+0)*2 + 64] = payload[192:256] =
		// the first PackedUserOperation head word (sender). The
		// sender field therefore doubles as the signature length
		// indicator in this synthetic test.
		return "0x" + selector +
			hexWord(0x20) + // head0: userOp offset
			hexWord(0x1122334455667788) + // head1: userOpHash
			hexWord(0) + // head2: missingAccountFunds
			hexWord(senderLen) + // sender / (parser-visible sig length)
			pad(9) + // remaining 9 PackedUserOperation head words
			pad(12) // tail offsets (zeros)
	}

	cases := []struct {
		name    string
		payload string
		want    int
	}{
		// Realistic 65-byte ECDSA signature.
		{name: "with_sig", payload: build(65), want: 1},
		// No signature — sender is zero, parser reads length=0.
		{name: "no_sig", payload: build(0), want: 0},
		// Wrong selector — must short-circuit to false.
		{name: "non_validatePaymasterUserOp", payload: "0xdeadbeef00", want: 0},
		// Truncated before the 13th head word — must short-circuit.
		{name: "truncated_payload", payload: "0xc04a6310" + pad(12), want: 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			rt := runCollectorJS(t, userop.V0_9)
			got := getInt(t, rt, fmt.Sprintf(`tracer._hasPaymasterSignature(%q) ? 1 : 0`, c.payload))
			if got != c.want {
				t.Errorf("paymasterSignature: got %d, want %d", got, c.want)
			}
		})
	}
}

// TestExecution_ConsumePendingGetCurrentUserOpHash_NullLog ensures the v0.9
// execution tracer does not crash when _consumePendingGetCurrentUserOpHash
// is called with a stub log (no stack) on error exit.
func TestExecution_ConsumePendingGetCurrentUserOpHash_NullLog(t *testing.T) {
	rt := runExecutionJS(t, userop.V0_9, "0x433709009b8330fda32311df1c2afa402ed8d009")
	mustRun(t, rt, `tracer._pendingGetCurrentUserOpHash = true;`)
	// Should be a no-op, not a TypeError.
	mustRun(t, rt, `tracer._consumePendingGetCurrentUserOpHash({memory: {slice: function(){return "";}}});`)
	if getInt(t, rt, "tracer._pendingGetCurrentUserOpHash ? 1 : 0") != 0 {
		t.Errorf("pending flag should be cleared after a null-log consume")
	}
}

// TestExecution_PendingClearedOnFrameExit exercises the v0.9 execution
// tracer's depth-aware clear: a STATICCALL to getCurrentUserOpHash sets
// the pending flag at the call's depth, and if the call frame ends
// without a matching RETURN (revert / OOG / etc.), the next step at a
// shallower or equal depth must clear the flag so a subsequent
// unrelated RETURN does not consume stale memory.
func TestExecution_PendingClearedOnFrameExit(t *testing.T) {
	ep := "0x433709009b8330fda32311df1c2afa402ed8d009"
	rt := runExecutionJS(t, userop.V0_9, ep)

	// Move into execution region (3 NUMBERs).
	for i := 0; i < 3; i++ {
		callJS(t, rt, "step", &stepLog{Op: "NUMBER", Depth: 1, Gas: 100, Cost: 2})
	}

	// STATICCALL to EntryPoint with selector getCurrentUserOpHash. The
	// opcode itself runs in the *caller* frame (depth 1 in our trace).
	mustRun(t, rt, fmt.Sprintf(`
tracer.step({
  op: "STATICCALL",
  getDepth: function(){return 1},
  getGas: function(){return 1000},
  getCost: function(){return 100},
  stack: {
    length: function(){return 7},
    peek: function(i){
      var s = ["0x64", %q, "0x0", "0x4", "0x0", "0x20", "0x0"];
      return s[i];
    }
  },
  memory: {slice: function(o,l){
    var all = [0x60, 0xe6, 0xb9, 0xfb, 0, 0, 0, 0];
    var out = [];
    for (var i = o; i < o + l && i < all.length; i++) out.push(all[i]);
    return out;
  }},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`, ep))

	if getInt(t, rt, "tracer._pendingGetCurrentUserOpHash ? 1 : 0") != 1 {
		t.Fatalf("pending flag should be set after STATICCALL to EntryPoint with getCurrentUserOpHash selector")
	}
	if getInt(t, rt, "tracer._pendingGetCurrentUserOpHashDepth") != 1 {
		t.Fatalf("pending depth should be 1, got %d", getInt(t, rt, "tracer._pendingGetCurrentUserOpHashDepth"))
	}

	// A non-RETURN step at depth 2 (the callee frame) does not clear the
	// flag — the STATICCALL is still in flight. The clear check is
	// `depth <= pendingDepth` (1), so depth 2 is above the bar.
	callJS(t, rt, "step", &stepLog{Op: "ADD", Depth: 2, Gas: 100, Cost: 3})
	if getInt(t, rt, "tracer._pendingGetCurrentUserOpHash ? 1 : 0") != 1 {
		t.Errorf("pending flag should NOT be cleared at depth 2 while call is in flight")
	}

	// Now drive a step at depth 1 (back in the caller frame): the call
	// ended without a RETURN (it reverted / ran OOG / etc.), so the
	// tracer must clear the pending flag.
	callJS(t, rt, "step", &stepLog{Op: "ADD", Depth: 1, Gas: 100, Cost: 3})
	if getInt(t, rt, "tracer._pendingGetCurrentUserOpHash ? 1 : 0") != 0 {
		t.Errorf("pending flag should be cleared when EVM returns to a shallower frame without consuming the capture")
	}

	// An unrelated RETURN at depth 1 must not be captured: it is not
	// associated with the getCurrentUserOpHash STATICCALL.
	mustRun(t, rt, `
tracer.step({
  op: "RETURN",
  getDepth: function(){return 1},
  getGas: function(){return 100},
  getCost: function(){return 0},
  stack: {length: function(){return 2}, peek: function(i){return i === 0 ? "0x0" : "0x20"}},
  memory: {slice: function(o,l){
    var out = [];
    for (var i = 0; i < 32; i++) out.push(0xaa);
    return out;
  }},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`)
	if count := getInt(t, rt, "(tracer.currentUserOpHashes || []).length"); count != 0 {
		t.Errorf("currentUserOpHashes should be empty after a stale-pending clear; got %d", count)
	}
}

// TestExecution_AddressComparisonIsCaseInsensitive exercises the v0.9
// execution tracer's case-insensitive EntryPoint address comparison:
// even if the bytecode / runtime emits a checksummed or mixed-case
// address on the STATICCALL stack, the tracer must still match
// getCurrentUserOpHash() calls to the real EntryPoint address (which
// load.go forces to lowercase).
func TestExecution_AddressComparisonIsCaseInsensitive(t *testing.T) {
	ep := "0x433709009b8330fda32311df1c2afa402ed8d009"
	mixedCase := "0x433709009B8330FDA32311DF1C2AFA402ED8D009"
	rt := runExecutionJS(t, userop.V0_9, ep)

	// Move into execution region.
	for i := 0; i < 3; i++ {
		callJS(t, rt, "step", &stepLog{Op: "NUMBER", Depth: 1, Gas: 100, Cost: 2})
	}

	mustRun(t, rt, fmt.Sprintf(`
tracer.step({
  op: "STATICCALL",
  getDepth: function(){return 1},
  getGas: function(){return 1000},
  getCost: function(){return 100},
  stack: {
    length: function(){return 7},
    peek: function(i){
      var s = ["0x64", %q, "0x0", "0x4", "0x0", "0x20", "0x0"];
      return s[i];
    }
  },
  memory: {slice: function(o,l){
    var all = [0x60, 0xe6, 0xb9, 0xfb, 0, 0, 0, 0];
    var out = [];
    for (var i = o; i < o + l && i < all.length; i++) out.push(all[i]);
    return out;
  }},
  contract: {getAddress: function(){return "0x"}},
  getError: function(){return null}
}, {getCode: function(a){return ""}});
`, mixedCase))

	if getInt(t, rt, "tracer._pendingGetCurrentUserOpHash ? 1 : 0") != 1 {
		t.Errorf("pending flag should be set even when the on-stack address is mixed-case")
	}
}
