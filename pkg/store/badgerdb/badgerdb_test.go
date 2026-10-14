package badgerdb_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/store/badgerdb"
	"github.com/zarbanio/bundler/pkg/store/storetest"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestBadgerConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		st, err := badgerdb.Open(store.Config{
			Backend:       store.BackendBadger,
			DataDirectory: t.TempDir(),
		})
		if err != nil {
			t.Fatalf("badgerdb.Open: %v", err)
		}
		return st
	})
}

// TestBadgerPersistsAcrossReopen verifies the mempool survives a restart, which
// is the single-node analogue of a rolling deploy without losing ops.
func TestBadgerPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{Backend: store.BackendBadger, DataDirectory: dir}
	ctx := context.Background()

	first, err := badgerdb.Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	op := &userop.UserOperation{
		Sender:               common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Nonce:                big.NewInt(1),
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
	ep := common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032")
	if err := first.Mempool().Add(ctx, ep, op); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := badgerdb.Open(cfg)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = second.Close() }()

	got, err := second.Mempool().All(ctx, ep)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("reloaded %d ops, want 1", len(got))
	}
}

// TestBadgerArrivalOrderAcrossReopen verifies arrival order survives a restart
// instead of falling back to key iteration order.
func TestBadgerArrivalOrderAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	cfg := store.Config{Backend: store.BackendBadger, DataDirectory: dir}
	ctx := context.Background()
	ep := common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032")

	// Senders chosen so arrival order (30,10,20) differs from lexicographic key
	// order (10,20,30).
	mk := func(sender string, nonce int64) *userop.UserOperation {
		return &userop.UserOperation{
			Sender:               common.HexToAddress(sender),
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
	}
	first := mk("0x3000000000000000000000000000000000000000", 1)
	secondOp := mk("0x1000000000000000000000000000000000000000", 2)
	third := mk("0x2000000000000000000000000000000000000000", 3)

	write, err := badgerdb.Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, op := range []*userop.UserOperation{first, secondOp, third} {
		if err := write.Mempool().Add(ctx, ep, op); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if err := write.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	read, err := badgerdb.Open(cfg)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = read.Close() }()

	got, err := read.Mempool().All(ctx, ep)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("reloaded %d ops, want 3", len(got))
	}
	for i, want := range []*userop.UserOperation{first, secondOp, third} {
		if got[i].Sender != want.Sender {
			t.Fatalf("op[%d] sender = %s, want %s", i, got[i].Sender, want.Sender)
		}
	}
}
