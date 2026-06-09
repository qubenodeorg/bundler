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

func TestValidateEIP7702AuthStandalone_BadAuthFails(t *testing.T) {
	auth := &userop.Authorization{
		ChainID: big.NewInt(999),
		Address: common.HexToAddress("0xABCD"),
		Nonce:   big.NewInt(0),
		R:       big.NewInt(1),
		S:       big.NewInt(1),
	}
	op := &mockAuthOp{auth: auth, sender: common.HexToAddress("0x1234")}
	if err := ValidateEIP7702AuthStandalone(op, big.NewInt(1)); err == nil {
		t.Fatal("expected chain ID mismatch error")
	}
}
