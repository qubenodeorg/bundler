package checks

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/gas"
)

func TestOpMAGLessThanMax_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	max := big.NewInt(0).Add(op.GetMaxGasAvailable(), common.Big1)

	if err := ValidateGasAvailableV07(op, gas.NewDefaultOverhead(), max); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestOpMAGMoreThanMax_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	max := big.NewInt(0).Sub(op.GetMaxGasAvailable(), common.Big1)

	if err := ValidateGasAvailableV07(op, gas.NewDefaultOverhead(), max); err == nil {
		t.Fatalf("got nil, want err")
	}
}

func TestValidateGasAvailableV07_NilMaxGasAvailable(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		nilMaxGasAvail: true,
	}

	err := ValidateGasAvailableV07(op, gas.NewDefaultOverhead(), big.NewInt(1000000))
	if err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidateGasAvailableV07_NilPreVerificationGas(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		nilPVG: true,
	}

	err := ValidateGasAvailableV07(op, gas.NewDefaultOverhead(), big.NewInt(1000000))
	if err == nil {
		t.Fatal("got nil, want err")
	}
}
