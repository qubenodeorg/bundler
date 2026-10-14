// Package factory selects and constructs a store.Store from configuration.
package factory

import (
	"context"
	"fmt"

	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/store/badgerdb"
	"github.com/zarbanio/bundler/pkg/store/memory"
	"github.com/zarbanio/bundler/pkg/store/redis"
)

// Open builds the backend named by cfg.Backend. An empty backend defaults to
// BadgerDB for backward compatibility.
//
// Network backends connect lazily, so Open can succeed even when the backend is
// momentarily unreachable; callers should use Store.Ping to gate readiness.
func Open(_ context.Context, cfg store.Config) (store.Store, error) {
	switch cfg.Backend {
	case "", store.BackendBadger:
		return badgerdb.Open(cfg)
	case store.BackendRedis, store.BackendValkey:
		return redis.New(cfg)
	case store.BackendMemory:
		return memory.New(cfg), nil
	default:
		return nil, fmt.Errorf("store: unknown backend %q", cfg.Backend)
	}
}
