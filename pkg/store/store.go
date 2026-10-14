package store

import (
	"context"
	"time"
)

// Store bundles the three persistence concerns used by the bundler behind a
// single handle. Backends return the same Store value for each accessor so
// callers can share one connection pool.
type Store interface {
	// Mempool returns the pending UserOperation store.
	Mempool() MempoolStore
	// Reputation returns the entity reputation store.
	Reputation() ReputationStore
	// CodeHash returns the code-hash cache.
	CodeHash() CodeHashStore
	// Locker returns the backend's cross-replica lock manager. Single-node
	// backends return a no-op locker so callers can use it unconditionally.
	Locker() Locker

	// Ping verifies the backend is reachable and usable. Readiness probes call
	// this with a short timeout; a failure must fail readiness, not liveness.
	Ping(ctx context.Context) error
	// Close releases all backend resources.
	Close() error
}

// Lock is a held distributed lock. Unlock must be safe to call more than once.
type Lock interface {
	Unlock(ctx context.Context) error
}

// NoopLocker is a Locker for single-node backends. Locks are always granted
// immediately.
type NoopLocker struct{}

type noopLock struct{}

func (noopLock) Unlock(context.Context) error { return nil }

// Lock always succeeds.
func (NoopLocker) Lock(context.Context, string, time.Duration) (Lock, error) {
	return noopLock{}, nil
}

// TryLock always reports the lock as acquired.
func (NoopLocker) TryLock(context.Context, string, time.Duration) (Lock, bool, error) {
	return noopLock{}, true, nil
}

// Locker is implemented by backends that can coordinate replicas. Backends
// that cannot (memory, single-node Badger) return a no-op Locker whose Lock
// always succeeds, so callers can depend on the interface unconditionally.
type Locker interface {
	// Lock blocks until the named lock is acquired or ctx is done. The lock is
	// automatically released after ttl even if Unlock is never called, which is
	// what lets a crashed replica's claims be reclaimed.
	Lock(ctx context.Context, name string, ttl time.Duration) (Lock, error)
	// TryLock is the non-blocking form: it reports acquired=false instead of
	// waiting when the lock is already held.
	TryLock(ctx context.Context, name string, ttl time.Duration) (Lock, bool, error)
}
