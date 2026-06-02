package checks

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestOpVGTotalLessThanMaxVG_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	mvg := big.NewInt(0).Add(op.VerificationGasLimit, common.Big1)

	if err := ValidateVerificationGasV07(op, gas.NewDefaultOverhead(), mvg); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestOpVGTotalMoreThanMaxVG_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	// GetPaymasterVerificationGasLimit returns 0 for v0.6 base struct, so total = VGL + 0 = VGL
	mvg := big.NewInt(0).Sub(op.VerificationGasLimit, common.Big1)

	if err := ValidateVerificationGasV07(op, gas.NewDefaultOverhead(), mvg); err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestOpPVGMoreThanOH_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	ov := gas.NewDefaultOverhead()
	pvg, _ := ov.CalcPreVerificationGas(op)
	op.PreVerificationGas = big.NewInt(0).Add(pvg, common.Big1)

	if err := ValidateVerificationGasV07(op, ov, big.NewInt(9999999)); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestOpPVGLessThanOH_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	ov := gas.NewDefaultOverhead()
	pvg, _ := ov.CalcPreVerificationGas(op)
	op.PreVerificationGas = big.NewInt(0).Sub(pvg, common.Big1)

	if err := ValidateVerificationGasV07(op, ov, big.NewInt(9999999)); err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidateVerificationGasV07_NilVerificationGasLimit(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		nilVG: true,
	}

	err := ValidateVerificationGasV07(op, gas.NewDefaultOverhead(), big.NewInt(1000000))
	if err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidateVerificationGasV07_NilPaymasterVGL(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		nilPaymasterVGL: true,
	}

	err := ValidateVerificationGasV07(op, gas.NewDefaultOverhead(), big.NewInt(1000000))
	if err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidateVerificationGasV07_NilPreVerificationGas(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		nilPVG: true,
	}

	ov := gas.NewDefaultOverhead()
	pvg, _ := ov.CalcPreVerificationGas(op.UserOperationInterface)
	if v, ok := op.UserOperationInterface.(*userop.UserOperation); ok {
		v.PreVerificationGas = pvg
	}

	err := ValidateVerificationGasV07(op, ov, big.NewInt(9999999))
	if err == nil {
		t.Fatal("got nil, want err")
	}
}
