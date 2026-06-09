package checks

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestValidateEIP7702AuthStandalone_NilAuth(t *testing.T) {
	op := &mockAuthOp{auth: nil}
	if err := ValidateEIP7702AuthStandalone(op, big.NewInt(1)); err != nil {
		t.Fatalf("nil auth must pass: %v", err)
	}
}

func TestValidateEIP7702AuthStandalone_ChainIDMismatch(t *testing.T) {
	// Sender matches auth.Address so the only mismatch is the chain ID; this guarantees
	// the test fails specifically when chain-ID validation regresses, not for an
	// unrelated signer-mismatch reason.
	delegate := common.HexToAddress("0xABCD000000000000000000000000000000000000")
	auth := &userop.Authorization{
		ChainID: big.NewInt(999),
		Address: delegate,
		Nonce:   big.NewInt(0),
		R:       big.NewInt(1),
		S:       big.NewInt(1),
	}
	op := &mockAuthOp{auth: auth, sender: delegate}
	if err := ValidateEIP7702AuthStandalone(op, big.NewInt(1)); err == nil {
		t.Fatal("expected chain ID mismatch error")
	}
}
