package simulation

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/tracer"
)

// TraceAccessesAddress reports whether a validation trace touched the given
// address — either its own storage directly or storage slots associated with
// it (keccak(addr || x), per ERC-7562 associated-storage rules). Used for
// STO-041: a UserOperation may not access the sender of another UserOperation
// in the same bundle.
func TraceAccessesAddress(trace *tracer.BundlerCollectorReturn, addr common.Address) bool {
	if trace == nil {
		return false
	}

	assocSlots := NewStorageSlotsByEntity(EntityStakes{addr: nil}, trace.Keccak)[addr]

	for _, cep := range trace.CallsFromEntryPoint {
		for accessAddr, access := range cep.Access {
			if accessAddr == addr {
				return true
			}
			if assocSlots == nil {
				continue
			}
			for slot := range access.Reads {
				if isAssociatedWith(assocSlots, slot) {
					return true
				}
			}
			for slot := range access.Writes {
				if isAssociatedWith(assocSlots, slot) {
					return true
				}
			}
		}
	}

	return false
}
