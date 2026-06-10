// Native erc7562Tracer types. These mirror the JSON emitted by a node's
// built-in ERC-7562 tracer (a callTracer-style nested frame tree extended with
// per-frame opcode, storage-slot, contract-size and keccak captures).
//
// Field encodings vary slightly between node implementations, so the
// unmarshalers below are tolerant:
//   - usedOpcodes keys may be hex ("0x54") or decimal ("84") strings.
//   - contractSize.opcode may be a JSON number, a hex/decimal string, or a name.
//   - extCodeAccessInfo may be a list of addresses or an address→opcode map.

package tracer

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// OpcodeName is an opcode identifier normalized to its canonical name on
// unmarshal (see OpNameFromKey).
type OpcodeName string

func (o *OpcodeName) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*o = OpcodeName(OpNameFromKey(s))
		return nil
	}
	var n uint64
	if err := json.Unmarshal(data, &n); err == nil {
		if n > 255 {
			return fmt.Errorf("tracer: opcode value out of range: %d", n)
		}
		*o = OpcodeName(OpName(byte(n)))
		return nil
	}
	return fmt.Errorf("tracer: cannot decode opcode: %s", string(data))
}

// Erc7562AccessedSlots holds per-frame storage access captures.
type Erc7562AccessedSlots struct {
	Reads           map[string][]string `json:"reads"`
	Writes          map[string]uint64   `json:"writes"`
	TransientReads  map[string]uint64   `json:"transientReads"`
	TransientWrites map[string]uint64   `json:"transientWrites"`
}

// Erc7562ContractSize reports the deployed code size observed for an address
// and the opcode that touched it.
type Erc7562ContractSize struct {
	ContractSize uint64     `json:"contractSize"`
	Opcode       OpcodeName `json:"opcode"`
}

// Erc7562ExtCodeAccesses maps addresses accessed via EXTCODE* opcodes to the
// opcode name used. Accepts either a JSON array of addresses or an
// address→opcode object.
type Erc7562ExtCodeAccesses map[common.Address]string

func (e *Erc7562ExtCodeAccesses) UnmarshalJSON(data []byte) error {
	out := Erc7562ExtCodeAccesses{}

	var addrs []common.Address
	if err := json.Unmarshal(data, &addrs); err == nil {
		for _, a := range addrs {
			out[a] = "EXTCODESIZE"
		}
		*e = out
		return nil
	}

	var m map[common.Address]OpcodeName
	if err := json.Unmarshal(data, &m); err == nil {
		for a, op := range m {
			out[a] = string(op)
		}
		*e = out
		return nil
	}

	return fmt.Errorf("tracer: cannot decode extCodeAccessInfo: %s", string(data))
}

// Erc7562Log is a LOG capture attached to the emitting frame
// (tracerConfig {"withLog": true}).
type Erc7562Log struct {
	Address common.Address `json:"address"`
	Topics  []string       `json:"topics"`
	Data    string         `json:"data"`
}

// Erc7562Frame is one call frame of the native erc7562Tracer output. The root
// frame is the traced call itself; Calls are nested frames.
type Erc7562Frame struct {
	Type         string         `json:"type"`
	From         common.Address `json:"from"`
	To           common.Address `json:"to"`
	Input        hexutil.Bytes  `json:"input"`
	Output       hexutil.Bytes  `json:"output"`
	Error        string         `json:"error,omitempty"`
	RevertReason string         `json:"revertReason,omitempty"`
	Gas          hexutil.Uint64 `json:"gas"`
	GasUsed      hexutil.Uint64 `json:"gasUsed"`
	Value        *hexutil.Big   `json:"value,omitempty"`

	AccessedSlots     Erc7562AccessedSlots                   `json:"accessedSlots"`
	ExtCodeAccessInfo Erc7562ExtCodeAccesses                 `json:"extCodeAccessInfo"`
	UsedOpcodes       map[string]uint64                      `json:"usedOpcodes"`
	ContractSize      map[common.Address]Erc7562ContractSize `json:"contractSize"`
	OutOfGas          bool                                   `json:"outOfGas"`
	Keccak            []hexutil.Bytes                        `json:"keccak"`
	Logs              []Erc7562Log                           `json:"logs"`
	Calls             []Erc7562Frame                         `json:"calls"`
}

// StorageAddress returns the address whose storage the frame's code operates
// on. DELEGATECALL and CALLCODE run the callee's code in the caller's storage
// context, so storage accesses belong to From, not To.
func (f *Erc7562Frame) StorageAddress() common.Address {
	switch f.Type {
	case "DELEGATECALL", "CALLCODE":
		return f.From
	default:
		return f.To
	}
}
