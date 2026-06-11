package state

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// WithCodeOverride takes a set and adds (or updates) an override that replaces
// the given address's code, preserving any other overrides on that account.
func WithCodeOverride(acc common.Address, code hexutil.Bytes, os OverrideSet) OverrideSet {
	if os == nil {
		os = OverrideSet{}
	}
	oa := os[acc]
	oa.Code = &code
	os[acc] = oa
	return os
}
