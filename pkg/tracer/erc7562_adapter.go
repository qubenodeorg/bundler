package tracer

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// TopLevelHook lets version-specific callers enrich a CallFromEntryPointInfo
// from the raw top-level frame (e.g. v0.9 derives the paymasterSignature flag
// from the validatePaymasterUserOp calldata).
type TopLevelHook func(frame *Erc7562Frame, info *CallFromEntryPointInfo)

// FromErc7562 converts a native erc7562Tracer frame tree into the
// BundlerCollectorReturn shape produced by the JS BundlerCollectorTracer, so
// all downstream ERC-7562 rule consumers (ProcessTraceResult, NewKnownEntity,
// NewCallStack, NewStorageSlotsByEntity) work unchanged.
//
// Mapping notes (mirroring the JS tracer semantics):
//   - Each direct child of the EntryPoint root frame becomes one
//     CallFromEntryPointInfo, with opcode/access/contract-size data aggregated
//     over that child's subtree.
//   - Storage access is keyed by the frame's storage context address
//     (From for DELEGATECALL/CALLCODE, To otherwise), matching the JS
//     tracer's log.contract.getAddress().
//   - Keccak preimages are read from the root frame (where the native tracer
//     stores the whole transaction's preimages) plus any per-frame captures.
//   - The flat Calls list is synthesized from the tree in enter/exit order so
//     NewCallStack behaves identically for the JS and native paths.
func FromErc7562(root *Erc7562Frame, entryPoint common.Address, hook TopLevelHook) (*BundlerCollectorReturn, error) {
	if root == nil {
		return nil, fmt.Errorf("tracer: nil erc7562 trace")
	}
	if root.To != entryPoint {
		return nil, fmt.Errorf("tracer: erc7562 trace root targets %s, expected EntryPoint %s", root.To, entryPoint)
	}

	out := &BundlerCollectorReturn{
		CallsFromEntryPoint: []CallFromEntryPointInfo{},
		Keccak:              []string{},
		Calls:               []CallInfo{},
		Logs:                []LogInfo{},
		Debug:               []any{},
	}

	// The native tracer stores the keccak preimages for the whole transaction
	// on the root frame (unlike the JS tracer's per-depth capture).
	for _, k := range root.Keccak {
		out.Keccak = append(out.Keccak, hexutil.Encode(k))
	}

	for i := range root.Calls {
		child := &root.Calls[i]
		info := newCallFromEntryPointInfo(child)
		aggregateSubtree(child, &info, out)
		if hook != nil {
			hook(child, &info)
		}
		out.CallsFromEntryPoint = append(out.CallsFromEntryPoint, info)
	}

	for i := range root.Calls {
		appendFlatCalls(&root.Calls[i], out)
	}

	for _, l := range root.Logs {
		out.Logs = append(out.Logs, LogInfo{Topics: l.Topics, Data: l.Data})
	}

	return out, nil
}

// isPrecompile reports whether addr is in the standard precompile range
// (0x01–0x09). The RIP-7212 precompile at 0x100 is intentionally NOT included:
// whether calls to it are allowed depends on chain support, which the storage
// slots validator decides via IsRIP7212Supported.
func isPrecompile(addr common.Address) bool {
	n := new(big.Int).SetBytes(addr.Bytes())
	return n.Sign() > 0 && n.Cmp(big.NewInt(0x09)) <= 0
}

func newCallFromEntryPointInfo(frame *Erc7562Frame) CallFromEntryPointInfo {
	sig := frame.Input
	if len(sig) > 4 {
		sig = sig[:4]
	}
	return CallFromEntryPointInfo{
		TopLevelMethodSig:     hexutil.Bytes(sig),
		TopLevelTargetAddress: frame.To,
		Opcodes:               Counts{},
		Access:                AccessMap{},
		ContractSize:          ContractSizeMap{},
		ExtCodeAccessInfo:     ExtCodeAccessInfoMap{},
	}
}

// aggregateSubtree folds one frame (and recursively its children) into the
// top-level call info and the global collector return.
func aggregateSubtree(frame *Erc7562Frame, info *CallFromEntryPointInfo, out *BundlerCollectorReturn) {
	storageAddr := frame.StorageAddress()
	for key, count := range frame.UsedOpcodes {
		name := OpNameFromKey(key)
		info.Opcodes[name] += float64(count)
		if name == "CREATE" || name == "CREATE2" {
			if info.CreateOpcodes == nil {
				info.CreateOpcodes = map[common.Address]Counts{}
			}
			if info.CreateOpcodes[storageAddr] == nil {
				info.CreateOpcodes[storageAddr] = Counts{}
			}
			info.CreateOpcodes[storageAddr][name] += float64(count)
		}
	}

	ensureAccess := func() AccessInfo {
		access, ok := info.Access[storageAddr]
		if !ok {
			access = AccessInfo{Reads: HexMap{}, Writes: Counts{}}
			info.Access[storageAddr] = access
		}
		return access
	}
	if len(frame.AccessedSlots.Reads) > 0 || len(frame.AccessedSlots.Writes) > 0 {
		access := ensureAccess()
		for slot, values := range frame.AccessedSlots.Reads {
			if _, seen := access.Reads[slot]; !seen {
				v := ""
				if len(values) > 0 {
					v = values[0]
				}
				access.Reads[slot] = v
			}
		}
		for slot, count := range frame.AccessedSlots.Writes {
			access.Writes[slot] += float64(count)
		}
	}

	// OP-070: transient storage (TLOAD/TSTORE) follows the exact same
	// validation rules as persistent storage, so transient slots join the
	// regular access map alongside the per-entity opcode counts.
	for slot, count := range frame.AccessedSlots.TransientReads {
		info.TLOAD += float64(count)
		access := ensureAccess()
		if _, seen := access.Reads[slot]; !seen {
			access.Reads[slot] = ""
		}
	}
	for slot, count := range frame.AccessedSlots.TransientWrites {
		info.TSTORE += float64(count)
		ensureAccess().Writes[slot] += float64(count)
	}

	for addr, cs := range frame.ContractSize {
		// The JS tracer skipped precompiles when recording contract sizes
		// (calling one is always allowed per OP-062, despite size 0).
		if isPrecompile(addr) {
			continue
		}
		if _, seen := info.ContractSize[addr]; !seen {
			info.ContractSize[addr] = ContractSizeInfo{
				ContractSize: float64(cs.ContractSize),
				Opcode:       string(cs.Opcode),
			}
		}
	}

	for addr, opcode := range frame.ExtCodeAccessInfo {
		info.ExtCodeAccessInfo[addr] = opcode
	}

	// Precompiles execute no opcodes, so the JS tracer never attributed an
	// out-of-gas inside one to the calling entity; a precompile call with too
	// little gas simply returns failure (e.g. probing allowed precompiles per
	// OP-062). Only propagate OOG from regular frames.
	if frame.OutOfGas && !isPrecompile(frame.To) {
		info.OOG = true
	}

	for _, k := range frame.Keccak {
		out.Keccak = append(out.Keccak, hexutil.Encode(k))
	}

	for i := range frame.Calls {
		aggregateSubtree(&frame.Calls[i], info, out)
	}
}

// appendFlatCalls reproduces the JS tracer's flat calls list: an entry per
// frame enter, then its children depth-first, then an exit entry (RETURN, or
// REVERT when the frame errored).
func appendFlatCalls(frame *Erc7562Frame, out *BundlerCollectorReturn) {
	method := hexutil.Encode(frame.Input)
	if len(method) > 10 {
		method = method[:10]
	}
	value := "0x0"
	if frame.Value != nil {
		value = frame.Value.String()
	}
	out.Calls = append(out.Calls, CallInfo{
		Type:   frame.Type,
		From:   frame.From,
		To:     frame.To,
		Method: method,
		Value:  value,
		Gas:    float64(frame.Gas),
	})

	for i := range frame.Calls {
		appendFlatCalls(&frame.Calls[i], out)
	}

	exitType := "RETURN"
	if frame.Error != "" {
		exitType = "REVERT"
	}
	out.Calls = append(out.Calls, CallInfo{
		Type:    exitType,
		GasUsed: float64(frame.GasUsed),
		Data:    hexutil.Encode(frame.Output),
	})
}
