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
