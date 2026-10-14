package memory_test

import (
	"testing"

	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/store/memory"
	"github.com/zarbanio/bundler/pkg/store/storetest"
)

func TestMemoryConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		return memory.New(store.Config{Backend: store.BackendMemory})
	})
}
