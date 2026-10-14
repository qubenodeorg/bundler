// Package storetest provides a backend-agnostic conformance suite. Every
// store backend must pass the same suite so they can be swapped freely.
package storetest

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/userop"
)

// Factory opens a fresh, empty store for a single test.
type Factory func(t *testing.T) store.Store

// Ep returns the canonical EntryPoint address used by the suite.
func Ep() common.Address { return ep }

// Op returns a fresh, valid UserOperation tied to the suite's entities.
func Op() *userop.UserOperation { return mkOp(sender1, 1, factoryA, paymaster) }

var (
	ep        = common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032")
	sender1   = common.HexToAddress("0x1111111111111111111111111111111111111111")
	sender2   = common.HexToAddress("0x2222222222222222222222222222222222222222")
	factoryA  = common.HexToAddress("0xaaaa000000000000000000000000000000000000")
	factoryB  = common.HexToAddress("0xbbbb000000000000000000000000000000000000")
	paymaster = common.HexToAddress("0xcccc000000000000000000000000000000000000")
)

// Run executes the full conformance suite against a backend.
func Run(t *testing.T, newStore Factory) {
	t.Run("Ping", func(t *testing.T) { testPing(t, newStore(t)) })
	t.Run("MempoolAddGetAllRemove", func(t *testing.T) { testMempoolAddGetAllRemove(t, newStore(t)) })
	t.Run("MempoolArrivalOrder", func(t *testing.T) { testMempoolArrivalOrder(t, newStore(t)) })
	t.Run("MempoolReplacementDropsStaleIndexes", func(t *testing.T) { testMempoolReplacement(t, newStore(t)) })
	t.Run("MempoolClear", func(t *testing.T) { testMempoolClear(t, newStore(t)) })
	t.Run("ReputationApplyStatus", func(t *testing.T) { testReputationApplyStatus(t, newStore(t)) })
	t.Run("ReputationOverrideDumpClear", func(t *testing.T) { testReputationOverrideDumpClear(t, newStore(t)) })
	t.Run("CodeHashRoundTrip", func(t *testing.T) { testCodeHashRoundTrip(t, newStore(t)) })
}

func testPing(t *testing.T, st store.Store) {
	t.Helper()
	defer closeStore(t, st)
	if err := st.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func testMempoolAddGetAllRemove(t *testing.T, st store.Store) {
	t.Helper()
	defer closeStore(t, st)
	ctx := context.Background()
	m := st.Mempool()

	op := mkOp(sender1, 1, factoryA, paymaster)
	if err := m.Add(ctx, ep, op); err != nil {
		t.Fatalf("Add: %v", err)
	}

	assertOps(t, "sender", m, ctx, sender1, op)
	assertOps(t, "factory", m, ctx, factoryA, op)
	assertOps(t, "paymaster", m, ctx, paymaster, op)

	all, err := m.All(ctx, ep)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 1 || !opsEqual(all[0], op) {
		t.Fatalf("All = %v, want the single added op", all)
	}

	if err := m.Remove(ctx, ep, op); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	for _, entity := range []common.Address{sender1, factoryA, paymaster} {
		got, err := m.Ops(ctx, ep, entity)
		if err != nil {
			t.Fatalf("Ops(%s): %v", entity, err)
		}
		if len(got) != 0 {
			t.Fatalf("Ops(%s) after remove = %d, want 0", entity, len(got))
		}
	}
	if all, _ := m.All(ctx, ep); len(all) != 0 {
		t.Fatalf("All after remove = %d, want 0", len(all))
	}
}

func testMempoolArrivalOrder(t *testing.T, st store.Store) {
	t.Helper()
	defer closeStore(t, st)
	ctx := context.Background()
	m := st.Mempool()

	op1 := mkOp(sender1, 1, common.Address{}, common.Address{})
	op2 := mkOp(sender2, 2, common.Address{}, common.Address{})
	for _, op := range []*userop.UserOperation{op1, op2} {
		if err := m.Add(ctx, ep, op); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	all, err := m.All(ctx, ep)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 2 || !opsEqual(all[0], op1) || !opsEqual(all[1], op2) {
		t.Fatalf("All not in arrival order: %v", all)
	}
}

func testMempoolReplacement(t *testing.T, st store.Store) {
	t.Helper()
	defer closeStore(t, st)
	ctx := context.Background()
	m := st.Mempool()

	old := mkOp(sender1, 7, factoryA, common.Address{})
	replacement := mkOp(sender1, 7, factoryB, common.Address{})

	if err := m.Add(ctx, ep, old); err != nil {
		t.Fatalf("Add old: %v", err)
	}
	if err := m.Add(ctx, ep, replacement); err != nil {
		t.Fatalf("Add replacement: %v", err)
	}

	// The stale factory index entry must be gone.
	if got, _ := m.Ops(ctx, ep, factoryA); len(got) != 0 {
		t.Fatalf("stale factory index has %d ops, want 0", len(got))
	}
	assertOps(t, "new factory", m, ctx, factoryB, replacement)
	all, _ := m.All(ctx, ep)
	if len(all) != 1 || !opsEqual(all[0], replacement) {
		t.Fatalf("All = %v, want only the replacement", all)
	}
}

func testMempoolClear(t *testing.T, st store.Store) {
	t.Helper()
	defer closeStore(t, st)
	ctx := context.Background()
	m := st.Mempool()

	if err := m.Add(ctx, ep, mkOp(sender1, 1, factoryA, paymaster)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := m.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if all, _ := m.All(ctx, ep); len(all) != 0 {
		t.Fatalf("All after clear = %d, want 0", len(all))
	}
}

func testReputationApplyStatus(t *testing.T, st store.Store) {
	t.Helper()
	defer closeStore(t, st)
	ctx := context.Background()
	r := st.Reputation()
	params := store.ReputationParams{MinInclusionRateDenominator: 100, ThrottlingSlack: 10, BanSlack: 20}
	addr := common.HexToAddress("0x9999999999999999999999999999999999999999")

	if status, _ := r.Status(ctx, addr, params); status != store.StatusOK {
		t.Fatalf("fresh status = %v, want OK", status)
	}

	if err := r.Apply(ctx, store.ReputationDelta{Address: addr, OpsSeen: 1500}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	seen, included, err := r.OpsCount(ctx, addr)
	if err != nil {
		t.Fatalf("OpsCount: %v", err)
	}
	if seen != 1500 || included != 0 {
		t.Fatalf("OpsCount = (%d,%d), want (1500,0)", seen, included)
	}
	if status, _ := r.Status(ctx, addr, params); status != store.StatusThrottled {
		t.Fatalf("status = %v, want Throttled", status)
	}

	if err := r.Apply(ctx, store.ReputationDelta{Address: addr, OpsSeen: 1000}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if status, _ := r.Status(ctx, addr, params); status != store.StatusBanned {
		t.Fatalf("status = %v, want Banned", status)
	}

	if err := r.Apply(ctx, store.ReputationDelta{Address: addr, OpsSeen: -100000}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if seen, _, _ := r.OpsCount(ctx, addr); seen != 0 {
		t.Fatalf("seen after negative delta = %d, want 0", seen)
	}
}

func testReputationOverrideDumpClear(t *testing.T, st store.Store) {
	t.Helper()
	defer closeStore(t, st)
	ctx := context.Background()
	r := st.Reputation()
	addr := common.HexToAddress("0x8888888888888888888888888888888888888888")

	if err := r.Override(ctx, []store.ReputationEntry{{Address: addr, OpsSeen: 5, OpsIncluded: 2}}); err != nil {
		t.Fatalf("Override: %v", err)
	}
	seen, included, _ := r.OpsCount(ctx, addr)
	if seen != 5 || included != 2 {
		t.Fatalf("OpsCount = (%d,%d), want (5,2)", seen, included)
	}

	dump, err := r.Dump(ctx)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(dump) != 1 || dump[0].Address != addr || dump[0].OpsSeen != 5 || dump[0].OpsIncluded != 2 {
		t.Fatalf("Dump = %+v, want one entry for %s", dump, addr)
	}

	if err := r.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if dump, _ := r.Dump(ctx); len(dump) != 0 {
		t.Fatalf("Dump after clear = %d, want 0", len(dump))
	}
}

func testCodeHashRoundTrip(t *testing.T, st store.Store) {
	t.Helper()
	defer closeStore(t, st)
	ctx := context.Background()
	c := st.CodeHash()
	h := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	want := []store.CodeHash{
		{Address: common.HexToAddress("0xaa"), Hash: common.HexToHash("0xbb")},
		{Address: common.HexToAddress("0xcc"), Hash: common.HexToHash("0xdd")},
	}

	if _, err := c.Get(ctx, h); !store.IsNotFound(err) {
		t.Fatalf("Get on empty = %v, want ErrNotFound", err)
	}
	if err := c.Save(ctx, h, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := c.Get(ctx, h)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Get len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if err := c.Remove(ctx, h); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := c.Get(ctx, h); !store.IsNotFound(err) {
		t.Fatalf("Get after remove = %v, want ErrNotFound", err)
	}
}

// --- helpers ---------------------------------------------------------------

func assertOps(t *testing.T, label string, m store.MempoolStore, ctx context.Context, entity common.Address, want *userop.UserOperation) {
	t.Helper()
	got, err := m.Ops(ctx, ep, entity)
	if err != nil {
		t.Fatalf("Ops(%s): %v", label, err)
	}
	if len(got) != 1 || !opsEqual(got[0], want) {
		t.Fatalf("Ops(%s) = %v, want the added op", label, got)
	}
}

func closeStore(t *testing.T, st store.Store) {
	t.Helper()
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
}

// mkOp builds a minimal but round-trippable UserOperation. The factory and
// paymaster are derived from InitCode/PaymasterAndData by the userop type.
func mkOp(sender common.Address, nonce int64, factory, paymaster common.Address) *userop.UserOperation {
	op := &userop.UserOperation{
		Sender:               sender,
		Nonce:                big.NewInt(nonce),
		InitCode:             []byte{},
		CallData:             []byte{},
		CallGasLimit:         big.NewInt(1),
		VerificationGasLimit: big.NewInt(1),
		PreVerificationGas:   big.NewInt(1),
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
		PaymasterAndData:     []byte{},
		Signature:            []byte{},
	}
	if factory != (common.Address{}) {
		op.InitCode = append(factory.Bytes(), 0x01, 0x02, 0x03)
	}
	if paymaster != (common.Address{}) {
		op.PaymasterAndData = append(paymaster.Bytes(), 0x01)
	}
	return op
}

func opsEqual(a, b *userop.UserOperation) bool {
	if a == nil || b == nil {
		return a == b
	}
	aj, err := a.MarshalJSON()
	if err != nil {
		return false
	}
	bj, err := b.MarshalJSON()
	if err != nil {
		return false
	}
	return string(aj) == string(bj)
}
