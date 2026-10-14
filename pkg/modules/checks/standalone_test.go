package checks

import (
	"context"
	"math/big"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/altmempools"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/modules/entities"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/store/memory"
	"github.com/zarbanio/bundler/pkg/userop"
)

func newStandalone(t *testing.T) (*Standalone, store.CodeHashStore, *httptest.Server) {
	t.Helper()
	codeHashes := memory.New(store.Config{Backend: store.BackendMemory}).CodeHash()

	srv := httptest.NewServer(nil)
	t.Cleanup(srv.Close)
	r, err := rpc.Dial(srv.URL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	t.Cleanup(r.Close)

	ovr := gas.NewOverheadResolver(userop.V0_6, userop.V0_7, userop.V0_8, userop.V0_9)
	alt, err := altmempools.New(big.NewInt(1), nil)
	if err != nil {
		t.Fatalf("altmempools: %v", err)
	}
	rep := &entities.ReputationConstants{}
	s := New(codeHashes, r, ovr, alt, big.NewInt(1_000_000), big.NewInt(10_000_000), false, "callTracer", rep)
	return s, codeHashes, srv
}

func TestStandalone_NewSetsFields(t *testing.T) {
	s, _, _ := newStandalone(t)
	if s == nil {
		t.Fatal("nil standalone")
	}
	// The resolver must serve every configured version; the v0.6 Overhead is
	// always present as the fallback.
	if s.ovr.For(userop.V0_6).GetVersion() != userop.V0_6 {
		t.Fatalf("expected v0.6 overhead from resolver")
	}
	if s.ovr.For(userop.V0_9).GetVersion() != userop.V0_9 {
		t.Fatalf("expected v0.9 overhead from resolver")
	}
	if s.tracer != "callTracer" {
		t.Fatalf("tracer not set")
	}
}

func TestStandalone_Clean_RemovesHashes(t *testing.T) {
	s, codeHashes, _ := newStandalone(t)
	ctx := context.Background()

	// Pre-populate with code hashes for two userOpHashes.
	mk := func(nonce int64, sender string) userop.UserOperation {
		return userop.UserOperation{
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
	op1 := mk(1, "0xaa")
	op2 := mk(2, "0xbb")
	hash1 := op1.GetUserOpHash(common.Address{}, big.NewInt(1))
	hash2 := op2.GetUserOpHash(common.Address{}, big.NewInt(1))

	if err := codeHashes.Save(ctx, hash1, []store.CodeHash{}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := codeHashes.Save(ctx, hash2, []store.CodeHash{}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	batchCtx := modules.NewBatchHandlerContext(
		[]*userop.UserOperation{&op1},
		common.Address{},
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
	)
	batchCtx.MarkOpIndexForRemoval(0, "remove")
	// Put op2 back into batch so both ops get cleaned.
	batchCtx.Batch = append(batchCtx.Batch, &op2)

	if err := s.Clean()(batchCtx); err != nil {
		t.Fatalf("Clean: %v", err)
	}

	if _, err := codeHashes.Get(ctx, hash1); !store.IsNotFound(err) {
		t.Fatalf("expected hash1 to be removed, got %v", err)
	}
	if _, err := codeHashes.Get(ctx, hash2); !store.IsNotFound(err) {
		t.Fatalf("expected hash2 to be removed, got %v", err)
	}
}

func TestStandalone_SimulateBatch_NoOp(t *testing.T) {
	s, _, _ := newStandalone(t)
	ctx := modules.NewBatchHandlerContext(nil, common.Address{}, big.NewInt(1), big.NewInt(1), big.NewInt(1), big.NewInt(1))
	if err := s.SimulateBatch()(ctx); err != nil {
		t.Fatalf("SimulateBatch (placeholder) should not error: %v", err)
	}
}

func TestStandalone_PaymasterDeposit_NoPaymasterEarlyReturn(t *testing.T) {
	s, _, _ := newStandalone(t)
	op := userop.UserOperation{
		Sender:               common.HexToAddress("0xaa"),
		Nonce:                big.NewInt(1),
		InitCode:             []byte{},
		CallData:             []byte{},
		CallGasLimit:         big.NewInt(0),
		VerificationGasLimit: big.NewInt(0),
		PreVerificationGas:   big.NewInt(0),
		MaxFeePerGas:         big.NewInt(0),
		MaxPriorityFeePerGas: big.NewInt(0),
		PaymasterAndData:     []byte{}, // no paymaster
		Signature:            []byte{},
	}
	ctx := modules.NewBatchHandlerContext(
		[]*userop.UserOperation{&op},
		common.Address{},
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
	)
	if err := s.PaymasterDeposit()(ctx); err != nil {
		t.Fatalf("PaymasterDeposit no-paymaster path: %v", err)
	}
}
