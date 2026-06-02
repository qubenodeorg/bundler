// These are the same BundlerCollectorTracer types from github.com/eth-infinitism/bundler ported for Go.
// Extended for ERC-4337 v0.7, v0.8 and v0.9 to capture:
//   - EIP-7702 auth presence on top-level calls (v0.8+)
//   - paymasterSignature presence on paymaster calls (v0.9+)
//   - TSTORE / TLOAD counts per top-level call (OP-070)
//   - getCurrentUserOpHash() captures during execution (v0.9)

package tracer

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type HexMap = map[string]string
type Counts = map[string]float64

// AccessInfo provides context on read and write counts by storage slots.
type AccessInfo struct {
	Reads  HexMap `json:"reads"`
	Writes Counts `json:"writes"`
}
type AccessMap = map[common.Address]AccessInfo

// ContractSizeInfo provides context on the code size and call type used to access upstream contracts.
type ContractSizeInfo struct {
	ContractSize float64 `json:"contractSize"`
	Opcode       string  `json:"opcode"`
}
type ContractSizeMap map[common.Address]ContractSizeInfo

// ExtCodeAccessInfoMap provides context on potentially illegal use of EXTCODESIZE.
type ExtCodeAccessInfoMap map[common.Address]string

// CallFromEntryPoint provides context on opcodes and storage access made via the EntryPoint to UserOperation
// entities.
type CallFromEntryPointInfo struct {
	TopLevelMethodSig     hexutil.Bytes        `json:"topLevelMethodSig"`
	TopLevelTargetAddress common.Address       `json:"topLevelTargetAddress"`
	Opcodes               Counts               `json:"opcodes"`
	Access                AccessMap            `json:"access"`
	ContractSize          ContractSizeMap      `json:"contractSize"`
	ExtCodeAccessInfo     ExtCodeAccessInfoMap `json:"extCodeAccessInfo"`
	OOG                   bool                 `json:"oog"`

	// v0.7+ tracking
	TSTORE float64 `json:"tstore,omitempty"`
	TLOAD  float64 `json:"tload,omitempty"`

	// v0.8+ EIP-7702 flag set on the top-level handleOps call when present.
	EIP7702Auth bool `json:"eip7702Auth,omitempty"`

	// v0.9+ paymasterSignature presence on the paymaster top-level call.
	PaymasterSignature bool `json:"paymasterSignature,omitempty"`
}

// CallInfo provides context on internal calls made during tracing.
type CallInfo struct {
	// Common
	Type string `json:"type"`

	// Method info
	From   common.Address `json:"from"`
	To     common.Address `json:"to"`
	Method string         `json:"method"`
	Value  string         `json:"value"`
	Gas    float64        `json:"gas"`

	// Exit info
	GasUsed float64 `json:"gasUsed"`
	Data    any     `json:"data"`
}

// LogInfo provides context from LOG opcodes during each step in the EVM trace.
type LogInfo struct {
	Topics []string `json:"topics"`
	Data   string   `json:"data"`
}

// UserOpEventInfo captures the UserOperationEvent log emitted by the EntryPoint
// when the tracer observes it during execution. It is populated by the v0.7+
// BundlerExecutionTracer variants.
type UserOpEventInfo struct {
	Topics []string `json:"topics"`
	Data   string   `json:"data"`
}

// BundlerCollectorReturn is the return value from performing an EVM trace with BundlerCollectorTracer.js.
type BundlerCollectorReturn struct {
	CallsFromEntryPoint []CallFromEntryPointInfo `json:"callsFromEntryPoint"`
	Keccak              []string                 `json:"keccak"`
	Calls               []CallInfo               `json:"calls"`
	Logs                []LogInfo                `json:"logs"`
	Debug               []any                    `json:"debug"`
}

// BundlerExecutionReturn is the return value from performing an EVM trace with BundlerExecutionTracer.js.
type BundlerExecutionReturn struct {
	Reverts           []string        `json:"reverts"`
	ValidationOOG     bool            `json:"validationOOG"`
	ExecutionOOG      bool            `json:"executionOOG"`
	ExecutionGasLimit float64         `json:"executionGasLimit"`
	UserOperationEvent *UserOpEventInfo `json:"userOperationEvent,omitempty"`

	// v0.9+ captures the getCurrentUserOpHash() result when the EntryPoint
	// exposes one during execution. When more than one capture occurs,
	// CurrentUserOpHashes holds the full list.
	CurrentUserOpHash   string   `json:"currentUserOpHash,omitempty"`
	CurrentUserOpHashes []string `json:"currentUserOpHashes,omitempty"`

	Output string `json:"output"`
	Error  string `json:"error"`
}
