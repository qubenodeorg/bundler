package redis_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/store/redis"
	"github.com/zarbanio/bundler/pkg/store/storetest"
)

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	st, err := redis.New(store.Config{
		Backend:   store.BackendRedis,
		Addrs:     []string{mr.Addr()},
		Namespace: "test",
		ChainID:   big.NewInt(1),
		OpTTL:     30 * time.Second,
	})
	if err != nil {
		t.Fatalf("redis.New: %v", err)
	}
	return st
}

func TestRedisConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		return newTestStore(t)
	})
}

// TestRedisSharedAcrossReplicas simulates two bundler processes (two store
// handles) sharing one backend: state written by one is visible to the other,
// and removal propagates.
func TestRedisSharedAcrossReplicas(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	cfg := store.Config{Backend: store.BackendRedis, Addrs: []string{mr.Addr()}, Namespace: "test", ChainID: big.NewInt(1)}
	replicaA, err := redis.New(cfg)
	if err != nil {
		t.Fatalf("new A: %v", err)
	}
	defer func() { _ = replicaA.Close() }()
	replicaB, err := redis.New(cfg)
	if err != nil {
		t.Fatalf("new B: %v", err)
	}
	defer func() { _ = replicaB.Close() }()

	ctx := context.Background()
	ep, op := storetest.Ep(), storetest.Op()

	if err := replicaA.Mempool().Add(ctx, ep, op); err != nil {
		t.Fatalf("A add: %v", err)
	}
	got, err := replicaB.Mempool().All(ctx, ep)
	if err != nil {
		t.Fatalf("B all: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("replica B sees %d ops, want 1", len(got))
	}

	if err := replicaB.Mempool().Remove(ctx, ep, op); err != nil {
		t.Fatalf("B remove: %v", err)
	}
	if got, _ := replicaA.Mempool().All(ctx, ep); len(got) != 0 {
		t.Fatalf("replica A still sees %d ops after B removed", len(got))
	}
}

// TestRedisLockMutualExclusionAndExpiry verifies the distributed lock excludes
// a second replica and that an unreleased lock expires so a crashed replica's
// claim can be reclaimed.
func TestRedisLockMutualExclusionAndExpiry(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	cfg := store.Config{Backend: store.BackendRedis, Addrs: []string{mr.Addr()}, Namespace: "test", ChainID: big.NewInt(1)}
	st, err := redis.New(cfg)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	lk := st.Locker()

	held, acquired, err := lk.TryLock(ctx, "ep:0xabc", 30*time.Second)
	if err != nil || !acquired {
		t.Fatalf("first TryLock: acquired=%v err=%v", acquired, err)
	}

	if _, acquired, err := lk.TryLock(ctx, "ep:0xabc", 30*time.Second); err != nil || acquired {
		t.Fatalf("second TryLock: acquired=%v err=%v, want false", acquired, err)
	}

	// A crashed replica never unlocks; the lease must expire on its own.
	mr.FastForward(31 * time.Second)
	if _, acquired, err := lk.TryLock(ctx, "ep:0xabc", 30*time.Second); err != nil || !acquired {
		t.Fatalf("TryLock after expiry: acquired=%v err=%v, want true", acquired, err)
	}

	if err := held.Unlock(ctx); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

// TestRedisOpTTL verifies pending ops carry a bounded lifetime so a crashed
// replica cannot leave state behind forever.
func TestRedisOpTTL(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	st, err := redis.New(store.Config{
		Backend:   store.BackendRedis,
		Addrs:     []string{mr.Addr()},
		Namespace: "test",
		ChainID:   big.NewInt(1),
		OpTTL:     30 * time.Second,
	})
	if err != nil {
		t.Fatalf("redis.New: %v", err)
	}
	defer func() { _ = st.Close() }()

	if err := st.Mempool().Add(context.Background(), storetest.Ep(), storetest.Op()); err != nil {
		t.Fatalf("Add: %v", err)
	}
	mr.FastForward(31 * time.Second)

	got, err := st.Mempool().All(context.Background(), storetest.Ep())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("All after TTL = %d, want 0", len(got))
	}
}
