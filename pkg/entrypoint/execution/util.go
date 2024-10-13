package execution

import (
	"math/big"
	"strings"

	"github.com/zarbanio/bundler/pkg/tracer"
)

type BundlerExecutionReturn struct {
	Reverts           []string `json:"reverts"`
	ValidationOOG     bool     `json:"validationOOG"`
	ExecutionOOG      bool     `json:"executionOOG"`
	ExecutionGasLimit float64  `json:"executionGasLimit"`
	Output            string   `json:"output"`
	Error             string   `json:"error"`
}

type LogInfo struct {
	Topics []string `json:"topics"`
	Data   string   `json:"data"`
}

// Structs for parsing the input JSON
type Call struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Gas     string `json:"gas"`
	GasUsed string `json:"gasUsed"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
	Calls   []Call `json:"calls,omitempty"`
	Type    string `json:"type"`
	Value   string `json:"value"`
}

// ProcessBundlerExecution processes the JSON input and returns BundlerExecutionReturn
func ProcessBundlerExecution(call *Call) (*tracer.BundlerExecutionReturn, error) {
	entryPointAddress := strings.ToLower("0x5ff137d4b0fdcd49dca30c7cf57e578a026d2789")
	// Start traversal
	context := &CallContext{
		IsValidationPhase: false,
		IsExecutionPhase:  false,
		EntryPointAddress: entryPointAddress,
		SenderAddress:     "", // Set this if known
	}
	result := &tracer.BundlerExecutionReturn{
		Reverts:           []string{},
		Output:            call.Output,
		Error:             call.Error,
		ExecutionGasLimit: hexToFloat64(call.Gas),
	}

	traverseCalls(call, result, context)

	return result, nil
}

// CallContext keeps track of traversal context
type CallContext struct {
	IsValidationPhase bool
	IsExecutionPhase  bool
	EntryPointAddress string
	SenderAddress     string
}

// traverseCalls recursively traverses the call stack to extract information
func traverseCalls(call *Call, result *tracer.BundlerExecutionReturn, context *CallContext) {
	// Check for reverts
	if call.Error != "" {
		// Collect the revert reason if available
		if call.Output != "" {
			result.Reverts = append(result.Reverts, call.Output)
		} else {
			result.Reverts = append(result.Reverts, call.Error)
		}

		// Check for Out of Gas errors
		if strings.Contains(strings.ToLower(call.Error), "out of gas") {
			if context.IsValidationPhase {
				result.ValidationOOG = true
			}
			if context.IsExecutionPhase {
				result.ExecutionOOG = true
			}
		}
	}

	// Identify phases based on call addresses or types
	toAddress := strings.ToLower(call.To)

	// Check if we're in the EntryPoint contract
	if toAddress == context.EntryPointAddress {
		context.IsValidationPhase = true
	}

	// Logic to switch from validation to execution phase
	// For this example, we'll assume that when we exit the EntryPoint contract, execution starts
	if context.IsValidationPhase && toAddress != context.EntryPointAddress {
		context.IsValidationPhase = false
		context.IsExecutionPhase = true
	}

	// Recurse into nested calls
	for _, nestedCall := range call.Calls {
		// Create a copy of the context for nested calls
		nestedContext := *context
		traverseCalls(&nestedCall, result, &nestedContext)
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
