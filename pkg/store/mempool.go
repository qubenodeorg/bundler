package store

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop"
)

// MempoolStore persists pending UserOperations that have passed all client
// checks and are waiting to be bundled.
//
// Ops are uniquely identified by (entryPoint, sender, nonce); adding an op with
// the same triple replaces the previous one. Implementations must index each op
// by every entity role it uses (sender, factory, paymaster) so Ops can return
// the pending set for any entity.
type MempoolStore interface {
	// Add inserts or replaces a UserOperation in the pool.
	Add(ctx context.Context, entryPoint common.Address, op *userop.UserOperation) error
	// Remove deletes the given UserOperations from the pool.
	Remove(ctx context.Context, entryPoint common.Address, ops ...*userop.UserOperation) error
	// Ops returns every pending op in which entity appears as sender, factory,
	// or paymaster.
	Ops(ctx context.Context, entryPoint common.Address, entity common.Address) ([]*userop.UserOperation, error)
	// All returns every pending op for the EntryPoint in arrival order.
	All(ctx context.Context, entryPoint common.Address) ([]*userop.UserOperation, error)
	// Clear removes every pending op across all EntryPoints.
	Clear(ctx context.Context) error
}
