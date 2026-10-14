package checks

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/zarbanio/bundler/pkg/store"
)

// codeHash is the on-wire snapshot of a contract's code hash, shared with the
// persistent code-hash store.
type codeHash = store.CodeHash

func getCodeHashes(ic []common.Address, gc GetCodeFunc) ([]codeHash, error) {
	ret := []codeHash{}

	for _, addr := range ic {
		bytecode, err := gc(addr)
		if err != nil {
			return ret, err
		}

		ret = append(ret, codeHash{
			Address: addr,
			Hash:    crypto.Keccak256Hash(bytecode),
		})
	}

	return ret, nil
}

func hasCodeHashChanges(chs []codeHash, gc GetCodeFunc) (bool, error) {
	prev := map[common.Address]common.Hash{}
	ic := []common.Address{}
	for _, ch := range chs {
		prev[ch.Address] = ch.Hash
		ic = append(ic, ch.Address)
	}

	curr, err := getCodeHashes(ic, gc)
	if err != nil {
		return false, err
	}

	for _, ch := range curr {
		if ch.Hash != prev[ch.Address] {
			return true, nil
		}
	}
	return false, nil
}
