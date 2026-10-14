package store

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
)

// CodeHash records the code hash of a contract that a UserOperation's
// validation interacted with. The bundler re-checks these hashes at bundle time
// and drops ops whose dependencies changed.
type CodeHash struct {
	Address common.Address `json:"address"`
	Hash    common.Hash    `json:"hash"`
}

// CodeHashStore caches the code hashes captured during simulation, keyed by
// UserOperation hash.
type CodeHashStore interface {
	// Save stores the code hashes for a UserOperation hash, replacing any
	// previous snapshot.
	Save(ctx context.Context, userOpHash common.Hash, hashes []CodeHash) error
	// Get returns the saved code hashes, or ErrNotFound if none were saved.
	Get(ctx context.Context, userOpHash common.Hash) ([]CodeHash, error)
	// Remove deletes the saved code hashes for the given UserOperation hashes.
	Remove(ctx context.Context, userOpHashes ...common.Hash) error
	// Clear removes every cached code-hash snapshot.
	Clear(ctx context.Context) error
}
