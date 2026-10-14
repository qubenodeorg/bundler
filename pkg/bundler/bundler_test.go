package bundler

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/modules/gasprice"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/userop"
)

// denyLocker simulates another replica holding every lock.
type denyLocker struct{}

func (denyLocker) Lock(context.Context, string, time.Duration) (store.Lock, error) {
	return nil, nil
}

func (denyLocker) TryLock(context.Context, string, time.Duration) (store.Lock, bool, error) {
	return nil, false, nil
}

func newTestBundler(t *testing.T) (*Bundler, store.MempoolStore, common.Address) {
	t.Helper()
	mem := testutils.MemoryStore().Mempool()
	ep := testutils.ValidAddress5
	b := New(mem, testutils.ChainID, []common.Address{ep})
	b.SetGetBaseFeeFunc(func() (*big.Int, error) { return big.NewInt(1), nil })
	b.SetGetGasTipFunc(func() (*big.Int, error) { return big.NewInt(1), nil })
	b.SetGetLegacyGasPriceFunc(func() (*big.Int, error) { return big.NewInt(1), nil })
	return b, mem, ep
}

func TestBundler_Process_EmptyMempool(t *testing.T) {
	b, _, ep := newTestBundler(t)
	ctx, err := b.Process(ep)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if ctx != nil {
		t.Fatalf("expected nil context for empty mempool, got %+v", ctx)
	}
}

func TestBundler_Process_SkipsWhenLocked(t *testing.T) {
	b, mem, ep := newTestBundler(t)
	if err := mem.Add(context.Background(), ep, testutils.MockValidInitUserOp()); err != nil {
		t.Fatalf("Add: %v", err)
	}
	b.SetLocker(denyLocker{})

	called := false
	b.UseModules(func(ctx *modules.BatchHandlerCtx) error {
		called = true
		return nil
	})

	ctx, err := b.Process(ep)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if ctx != nil {
		t.Fatal("expected nil ctx when the bundle lock is held elsewhere")
	}
	if called {
		t.Fatal("batch handler must not run without the lock")
	}
	if all, _ := mem.All(context.Background(), ep); len(all) != 1 {
		t.Fatalf("mempool size = %d, want 1 (unchanged)", len(all))
	}
}

func TestBundler_Process_BatchHandlerInvoked(t *testing.T) {
	b, mem, ep := newTestBundler(t)
	op := testutils.MockValidInitUserOp()
	if err := mem.Add(context.Background(), ep, op); err != nil {
		t.Fatalf("AddOp: %v", err)
	}

	called := false
	b.UseModules(func(ctx *modules.BatchHandlerCtx) error {
		called = true
		if len(ctx.Batch) != 1 {
			t.Fatalf("expected batch size 1, got %d", len(ctx.Batch))
		}
		ctx.Data["txn_hash"] = "0xabc"
		return nil
	})

	ctx, err := b.Process(ep)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !called {
		t.Fatalf("batch handler was not invoked")
	}
	if ctx.Data["txn_hash"] != "0xabc" {
		t.Fatalf("expected txn_hash to be set, got %v", ctx.Data["txn_hash"])
	}

	// Mempool should be drained after a successful Process.
	rem, err := mem.All(context.Background(), ep)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(rem) != 0 {
		t.Fatalf("expected mempool empty after process, got %d", len(rem))
	}
}

func TestBundler_Process_PendingRemoval(t *testing.T) {
	b, mem, ep := newTestBundler(t)
	op := testutils.MockValidInitUserOp()
	op2 := testutils.MockValidInitUserOp()
	op2.Nonce = big.NewInt(1)
	if err := mem.Add(context.Background(), ep, op); err != nil {
		t.Fatalf("AddOp: %v", err)
	}
	if err := mem.Add(context.Background(), ep, op2); err != nil {
		t.Fatalf("AddOp: %v", err)
	}

	b.UseModules(func(ctx *modules.BatchHandlerCtx) error {
		ctx.MarkOpIndexForRemoval(0, "bad op")
		return nil
	})

	ctx, err := b.Process(ep)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(ctx.PendingRemoval) != 1 {
		t.Fatalf("expected one pending removal, got %d", len(ctx.PendingRemoval))
	}
	if ctx.PendingRemoval[0].Reason != "bad op" {
		t.Fatalf("reason mismatch")
	}
}

func TestBundler_Process_BatchHandlerError(t *testing.T) {
	b, mem, ep := newTestBundler(t)
	op := testutils.MockValidInitUserOp()
	if err := mem.Add(context.Background(), ep, op); err != nil {
		t.Fatalf("AddOp: %v", err)
	}

	wantErr := errors.New("boom")
	b.UseModules(func(ctx *modules.BatchHandlerCtx) error {
		return wantErr
	})

	_, err := b.Process(ep)
	if !errors.Is(err, wantErr) {
		t.Fatalf("got err %v, want %v", err, wantErr)
	}
}

func TestBundler_MaxBatchAdjusts(t *testing.T) {
	b, mem, ep := newTestBundler(t)
	b.SetMaxBatch(1)

	for i := int64(0); i < 3; i++ {
		op := testutils.MockValidInitUserOp()
		op.Nonce = big.NewInt(i)
		if err := mem.Add(context.Background(), ep, op); err != nil {
			t.Fatalf("AddOp: %v", err)
		}
	}

	var seen int
	b.UseModules(func(ctx *modules.BatchHandlerCtx) error {
		seen = len(ctx.Batch)
		return nil
	})

	if _, err := b.Process(ep); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if seen != 1 {
		t.Fatalf("expected batch size 1 after maxBatch, got %d", seen)
	}
}

func TestBundler_GasPriceErrorsPropagate(t *testing.T) {
	b, mem, ep := newTestBundler(t)
	if err := mem.Add(context.Background(), ep, testutils.MockValidInitUserOp()); err != nil {
		t.Fatalf("AddOp: %v", err)
	}

	wantErr := errors.New("basefee err")
	b.SetGetBaseFeeFunc(func() (*big.Int, error) { return nil, wantErr })
	if _, err := b.Process(ep); !errors.Is(err, wantErr) {
		t.Fatalf("base fee err mismatch: got %v", err)
	}

	// Reset state for tip / gas price error checks.
	if err := mem.Add(context.Background(), ep, testutils.MockValidInitUserOp()); err != nil {
		t.Fatalf("AddOp: %v", err)
	}
	b.SetGetBaseFeeFunc(func() (*big.Int, error) { return big.NewInt(1), nil })
	tipErr := errors.New("tip err")
	b.SetGetGasTipFunc(func() (*big.Int, error) { return nil, tipErr })
	if _, err := b.Process(ep); !errors.Is(err, tipErr) {
		t.Fatalf("tip err mismatch: got %v", err)
	}

	if err := mem.Add(context.Background(), ep, testutils.MockValidInitUserOp()); err != nil {
		t.Fatalf("AddOp: %v", err)
	}
	b.SetGetGasTipFunc(func() (*big.Int, error) { return big.NewInt(1), nil })
	gpErr := errors.New("gp err")
	b.SetGetLegacyGasPriceFunc(func() (*big.Int, error) { return nil, gpErr })
	if _, err := b.Process(ep); !errors.Is(err, gpErr) {
		t.Fatalf("gp err mismatch: got %v", err)
	}
}

func TestBundler_RunStop(t *testing.T) {
	b, _, _ := newTestBundler(t)
	if err := b.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Second Run is a no-op.
	if err := b.Run(); err != nil {
		t.Fatalf("Run (second): %v", err)
	}
	b.Stop()
	// Second Stop is a no-op.
	b.Stop()
}

func TestBundler_UseLoggerAndUseModules(t *testing.T) {
	b, _, _ := newTestBundler(t)
	b.UseLogger(b.logger)
	b.UseModules(func(ctx *modules.BatchHandlerCtx) error { return nil })
	// Sanity: gasprice noop helpers compile and run.
	if _, err := gasprice.NoopGetBaseFeeFunc()(); err != nil {
		t.Fatalf("noop base fee returned err: %v", err)
	}
	// Sanity: userop default still in canonical packed v0.6 shape.
	_ = (*userop.UserOperation)(testutils.MockValidInitUserOp())
}
