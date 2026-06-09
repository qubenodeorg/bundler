package checks

import (
	"math/big"
	"net/http/httptest"
	"testing"

	"github.com/dgraph-io/badger/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/altmempools"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/modules/entities"
	"github.com/zarbanio/bundler/pkg/userop"
)

func newStandalone(t *testing.T) (*Standalone, *badger.DB, *httptest.Server) {
	t.Helper()
	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true).WithLoggingLevel(badger.ERROR))
	if err != nil {
		t.Fatalf("badger: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	srv := httptest.NewServer(nil)
	t.Cleanup(srv.Close)
	r, err := rpc.Dial(srv.URL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}

	ov := gas.NewDefaultOverhead()
	alt, err := altmempools.New(big.NewInt(1), nil)
	if err != nil {
		t.Fatalf("altmempools: %v", err)
	}
	rep := &entities.ReputationConstants{}
	s := New(db, r, ov, alt, big.NewInt(1_000_000), big.NewInt(10_000_000), false, "callTracer", rep)
	return s, db, srv
}

func TestStandalone_NewSetsFields(t *testing.T) {
	s, _, _ := newStandalone(t)
	if s == nil {
		t.Fatal("nil standalone")
	}
	if s.version != userop.V0_6 {
		t.Fatalf("expected v0.6 version, got %s", s.version)
	}
	if s.tracer != "callTracer" {
		t.Fatalf("tracer not set")
	}
}

func TestStandalone_Clean_RemovesHashes(t *testing.T) {
	s, db, _ := newStandalone(t)

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

	_ = db.Update(func(txn *badger.Txn) error {
		_ = txn.Set(getCodeHashesKey(hash1), []byte("[]"))
		_ = txn.Set(getCodeHashesKey(hash2), []byte("[]"))
		return nil
	})

	ctx := modules.NewBatchHandlerContext(
		[]*userop.UserOperation{&op1},
		common.Address{},
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
	)
	ctx.MarkOpIndexForRemoval(0, "remove")
	// Put op2 back into batch so both ops get cleaned.
	ctx.Batch = append(ctx.Batch, &op2)

	if err := s.Clean()(ctx); err != nil {
		t.Fatalf("Clean: %v", err)
	}

	if _, err := getSavedCodeHashes(db, hash1); err == nil {
		t.Fatal("expected hash1 to be removed")
	}
	if _, err := getSavedCodeHashes(db, hash2); err == nil {
		t.Fatal("expected hash2 to be removed")
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
