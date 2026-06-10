package execution

import (
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/zarbanio/bundler/pkg/tracer"
)

// CallLog is a LOG entry attached to its emitting frame by the native
// callTracer when run with tracerConfig {"withLog": true}.
type CallLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

// Call is one frame of a native callTracer trace.
type Call struct {
	From    string    `json:"from"`
	To      string    `json:"to"`
	Gas     string    `json:"gas"`
	GasUsed string    `json:"gasUsed"`
	Input   string    `json:"input"`
	Output  string    `json:"output"`
	Error   string    `json:"error,omitempty"`
	Logs    []CallLog `json:"logs,omitempty"`
	Calls   []Call    `json:"calls,omitempty"`
	Type    string    `json:"type"`
	Value   string    `json:"value"`
}

// TraceConfig carries the version-specific constants ProcessBundlerExecution
// needs to interpret a handleOps callTracer trace.
type TraceConfig struct {
	EntryPoint common.Address

	// InnerHandleOpSelector identifies the EntryPoint's self-call that runs
	// the execution phase (innerHandleOp). When empty, any EntryPoint
	// self-call is treated as the execution phase.
	InnerHandleOpSelector string

	// UserOpEventTopic is topics[0] of the version's UserOperationEvent.
	UserOpEventTopic string

	// GetCurrentUserOpHashSelector, when set (v0.9), captures the output of
	// EntryPoint.getCurrentUserOpHash() STATICCALLs made during execution.
	GetCurrentUserOpHashSelector string
}

// NewTraceConfig derives the version-specific trace constants from an
// EntryPoint ABI. Selectors/topics absent from the ABI are left empty and the
// corresponding capture is skipped (e.g. getCurrentUserOpHash before v0.9).
func NewTraceConfig(epABI *abi.ABI, entryPoint common.Address) *TraceConfig {
	cfg := &TraceConfig{EntryPoint: entryPoint}
	if m, ok := epABI.Methods["innerHandleOp"]; ok {
		cfg.InnerHandleOpSelector = hexutil.Encode(m.ID)
	}
	if e, ok := epABI.Events["UserOperationEvent"]; ok {
		cfg.UserOpEventTopic = e.ID.Hex()
	}
	if m, ok := epABI.Methods["getCurrentUserOpHash"]; ok {
		cfg.GetCurrentUserOpHashSelector = hexutil.Encode(m.ID)
	}
	return cfg
}

// ProcessBundlerExecution interprets a callTracer trace of
// EntryPoint.handleOps and reproduces the JS BundlerExecutionTracer output.
//
// The validation/execution split is structural: the EntryPoint runs the
// execution phase inside a self-call to innerHandleOp, so that subtree is the
// execution phase and everything else under the root is validation. The
// executionGasLimit applies the EIP-150 63/64 rule bottom-up over the
// execution subtree instead of summing gasUsed.
func ProcessBundlerExecution(call *Call, cfg *TraceConfig) (*tracer.BundlerExecutionReturn, error) {
	result := &tracer.BundlerExecutionReturn{
		Reverts: []string{},
		Output:  call.Output,
		Error:   call.Error,
	}

	ep := strings.ToLower(cfg.EntryPoint.Hex())
	if isOutOfGas(call.Error) {
		result.ValidationOOG = true
	}

	for i := range call.Calls {
		child := &call.Calls[i]
		if isInnerHandleOp(child, ep, cfg.InnerHandleOpSelector) {
			walkExecution(child, ep, cfg, result)
			_, required := requiredGas(child)
			result.ExecutionGasLimit += required
			scanUserOpEvent(child.Logs, cfg.UserOpEventTopic, result)
		} else {
			walkValidation(child, result)
		}
	}

	scanUserOpEvent(call.Logs, cfg.UserOpEventTopic, result)

	if len(result.CurrentUserOpHashes) == 1 {
		result.CurrentUserOpHash = result.CurrentUserOpHashes[0]
	}

	return result, nil
}

func isInnerHandleOp(call *Call, ep string, selector string) bool {
	if strings.ToLower(call.From) != ep || strings.ToLower(call.To) != ep {
		return false
	}
	return selector == "" || strings.HasPrefix(strings.ToLower(call.Input), strings.ToLower(selector))
}

func isOutOfGas(err string) bool {
	return err != "" && strings.Contains(strings.ToLower(err), "out of gas")
}

func walkValidation(call *Call, result *tracer.BundlerExecutionReturn) {
	if isOutOfGas(call.Error) {
		result.ValidationOOG = true
	}
	for i := range call.Calls {
		walkValidation(&call.Calls[i], result)
	}
}

// walkExecution collects reverts, OOG flags and getCurrentUserOpHash captures
// from the execution subtree in frame-exit (post-) order, matching the JS
// tracer.
func walkExecution(call *Call, ep string, cfg *TraceConfig, result *tracer.BundlerExecutionReturn) {
	for i := range call.Calls {
		walkExecution(&call.Calls[i], ep, cfg, result)
	}

	if call.Error != "" {
		out := call.Output
		if out == "" {
			out = "0x"
		}
		result.Reverts = append(result.Reverts, out)
		if isOutOfGas(call.Error) {
			result.ExecutionOOG = true
		}
	}

	if cfg.GetCurrentUserOpHashSelector != "" &&
		call.Error == "" &&
		strings.EqualFold(call.Type, "STATICCALL") &&
		strings.ToLower(call.To) == ep &&
		strings.HasPrefix(strings.ToLower(call.Input), strings.ToLower(cfg.GetCurrentUserOpHashSelector)) {
		result.CurrentUserOpHashes = append(result.CurrentUserOpHashes, call.Output)
	}
}

// requiredGas computes the gas a caller must hold to complete the frame under
// the EIP-150 63/64 forwarding rule:
//
//	required(f) = gasUsed(f) − Σ gasUsed(child) + ceil(64/63 × Σ required(child))
//
// This reproduces the JS tracer's bottom-up _executionGasStack recursion.
func requiredGas(call *Call) (used float64, required float64) {
	used = hexToFloat64(call.GasUsed)

	var childrenUsed, childrenRequired float64
	for i := range call.Calls {
		cu, cr := requiredGas(&call.Calls[i])
		childrenUsed += cu
		childrenRequired += cr
	}

	required = used - childrenUsed + math.Ceil(childrenRequired*64/63)
	return used, required
}

func scanUserOpEvent(logs []CallLog, topic string, result *tracer.BundlerExecutionReturn) {
	if topic == "" {
		return
	}
	for _, l := range logs {
		if len(l.Topics) > 0 && strings.EqualFold(l.Topics[0], topic) {
			result.UserOperationEvent = &tracer.UserOpEventInfo{
				Topics: l.Topics,
				Data:   l.Data,
			}
		}
	}
}

// hexToFloat64 converts a hex string to float64
func hexToFloat64(hexStr string) float64 {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	bi := new(big.Int)
	bi.SetString(hexStr, 16)
	f, _ := new(big.Float).SetInt(bi).Float64()
	return f
}
