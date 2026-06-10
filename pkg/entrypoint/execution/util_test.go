package execution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/entrypoint"
)

var (
	testEntryPoint = common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009")
	testSender     = "0x1111111111111111111111111111111111111111"
)

const (
	v09InnerHandleOpSelector  = "0x0042dc53"
	v09UserOpEventTopic       = "0x49628fd1471006c1482da88028e9ce4dbb080b815c9b0344d39e5a8e6ec1419f"
	v09GetCurrentHashSelector = "0xb0a398d1"
)

func testTraceConfig() *TraceConfig {
	return &TraceConfig{
		EntryPoint:                   testEntryPoint,
		InnerHandleOpSelector:        v09InnerHandleOpSelector,
		UserOpEventTopic:             v09UserOpEventTopic,
		GetCurrentUserOpHashSelector: v09GetCurrentHashSelector,
	}
}

func loadCallTracerFixture(t *testing.T) *Call {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "tracer", "testdata", "calltracer_handleops_v09.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	call := &Call{}
	if err := json.Unmarshal(data, call); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return call
}

func TestProcessBundlerExecutionFixture(t *testing.T) {
	call := loadCallTracerFixture(t)

	res, err := ProcessBundlerExecution(call, testTraceConfig())
	if err != nil {
		t.Fatalf("ProcessBundlerExecution: %v", err)
	}

	// 63/64 recursion over the innerHandleOp subtree, hand-computed:
	//   revert frame:     used 0x8000  = 32768, required 32768
	//   staticcall:       used 0x300   = 768,   required 768
	//   account exec:     used 0x18000 = 98304
	//     required = 98304 - 33536 + ceil(33536*64/63) = 98837
	//   innerHandleOp:    used 0x20000 = 131072
	//     required = 131072 - 98304 + ceil(98837*64/63) = 133174
	if res.ExecutionGasLimit != 133174 {
		t.Errorf("ExecutionGasLimit = %v, want 133174", res.ExecutionGasLimit)
	}

	if len(res.Reverts) != 1 {
		t.Fatalf("Reverts = %v, want one entry", res.Reverts)
	}
	if res.Reverts[0][:10] != "0x08c379a0" {
		t.Errorf("revert data = %q, want Error(string) payload", res.Reverts[0])
	}

	if res.ValidationOOG || res.ExecutionOOG {
		t.Errorf("OOG flags = %v/%v, want false/false", res.ValidationOOG, res.ExecutionOOG)
	}

	if res.UserOperationEvent == nil {
		t.Fatal("UserOperationEvent not extracted")
	}
	if res.UserOperationEvent.Topics[0] != v09UserOpEventTopic {
		t.Errorf("event topic0 = %q", res.UserOperationEvent.Topics[0])
	}

	wantHash := "0x1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f"
	if res.CurrentUserOpHash != wantHash {
		t.Errorf("CurrentUserOpHash = %q, want %q", res.CurrentUserOpHash, wantHash)
	}
	if len(res.CurrentUserOpHashes) != 1 {
		t.Errorf("CurrentUserOpHashes = %v, want one entry", res.CurrentUserOpHashes)
	}
}

func TestProcessBundlerExecutionValidationOOG(t *testing.T) {
	ep := "0x433709009b8330fda32311df1c2afa402ed8d009"
	call := &Call{
		Type: "CALL", From: "0x0000000000000000000000000000000000000000", To: ep,
		GasUsed: "0x5000",
		Error:   "execution reverted",
		Calls: []Call{
			{
				Type: "CALL", From: ep, To: testSender,
				Input: "0x19822f7c", GasUsed: "0x4000",
				Error: "out of gas",
			},
		},
	}

	res, err := ProcessBundlerExecution(call, testTraceConfig())
	if err != nil {
		t.Fatalf("ProcessBundlerExecution: %v", err)
	}
	if !res.ValidationOOG {
		t.Error("ValidationOOG not set for OOG in validation phase")
	}
	if res.ExecutionOOG {
		t.Error("ExecutionOOG wrongly set")
	}
	if len(res.Reverts) != 0 {
		t.Errorf("validation-phase failures must not be reverts: %v", res.Reverts)
	}
}

func TestProcessBundlerExecutionExecutionOOG(t *testing.T) {
	ep := "0x433709009b8330fda32311df1c2afa402ed8d009"
	call := &Call{
		Type: "CALL", From: "0x0000000000000000000000000000000000000000", To: ep,
		GasUsed: "0x9000",
		Calls: []Call{
			{
				Type: "CALL", From: ep, To: ep,
				Input: v09InnerHandleOpSelector + "00", GasUsed: "0x8000",
				Calls: []Call{
					{
						Type: "CALL", From: ep, To: testSender,
						Input: "0xb61d27f6", GasUsed: "0x6000",
						Error: "out of gas",
					},
				},
			},
		},
	}

	res, err := ProcessBundlerExecution(call, testTraceConfig())
	if err != nil {
		t.Fatalf("ProcessBundlerExecution: %v", err)
	}
	if res.ExecutionOOG {
		// expected
	} else {
		t.Error("ExecutionOOG not set for OOG in execution phase")
	}
	if res.ValidationOOG {
		t.Error("ValidationOOG wrongly set")
	}
	if len(res.Reverts) != 1 || res.Reverts[0] != "0x" {
		t.Errorf("Reverts = %v, want one empty (0x) entry", res.Reverts)
	}
}

// TestProcessBundlerExecutionSelectorFallback: with no selector configured,
// any EntryPoint self-call is the execution phase.
func TestProcessBundlerExecutionSelectorFallback(t *testing.T) {
	ep := "0x433709009b8330fda32311df1c2afa402ed8d009"
	call := &Call{
		Type: "CALL", From: "0x0000000000000000000000000000000000000000", To: ep,
		GasUsed: "0x9000",
		Calls: []Call{
			{
				Type: "CALL", From: ep, To: ep,
				Input: "0xffffffff", GasUsed: "0x2000",
			},
		},
	}

	cfg := &TraceConfig{EntryPoint: testEntryPoint}
	res, err := ProcessBundlerExecution(call, cfg)
	if err != nil {
		t.Fatalf("ProcessBundlerExecution: %v", err)
	}
	if res.ExecutionGasLimit != 0x2000 {
		t.Errorf("ExecutionGasLimit = %v, want %v", res.ExecutionGasLimit, 0x2000)
	}
}

func TestNewTraceConfigFromABI(t *testing.T) {
	epABI, err := entrypoint.EntrypointMetaData.GetAbi()
	if err != nil {
		t.Fatalf("GetAbi: %v", err)
	}
	cfg := NewTraceConfig(epABI, testEntryPoint)
	if cfg.InnerHandleOpSelector != "0x1d732756" {
		t.Errorf("v0.6 innerHandleOp selector = %q, want 0x1d732756", cfg.InnerHandleOpSelector)
	}
	if cfg.UserOpEventTopic != v09UserOpEventTopic {
		t.Errorf("UserOperationEvent topic = %q", cfg.UserOpEventTopic)
	}
	if cfg.GetCurrentUserOpHashSelector != "" {
		t.Errorf("v0.6 must not have getCurrentUserOpHash, got %q", cfg.GetCurrentUserOpHashSelector)
	}
}
