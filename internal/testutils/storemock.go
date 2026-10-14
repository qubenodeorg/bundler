package testutils

import (
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/store/memory"
)

// MemoryStore returns an ephemeral store for tests that do not exercise
// persistence.
func MemoryStore() store.Store {
	return memory.New(store.Config{Backend: store.BackendMemory})
}
