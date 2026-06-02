package checks

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/gas"
)

func TestOpCGLessThanOH_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	ov := gas.NewDefaultOverhead()
	cg := ov.NonZeroValueCall()
	op.CallGasLimit = big.NewInt(0).Sub(cg, common.Big1)

	if err := ValidateCallGasLimitV07(op, ov); err == nil {
		t.Fatalf("got nil, want err")
	}
}

func TestOpCGEqualOH_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	ov := gas.NewDefaultOverhead()
	cg := ov.NonZeroValueCall()
	op.CallGasLimit = big.NewInt(0).Add(cg, common.Big0)

	if err := ValidateCallGasLimitV07(op, ov); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestOpCGMoreThanOH_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	ov := gas.NewDefaultOverhead()
	cg := ov.NonZeroValueCall()
	op.CallGasLimit = big.NewInt(0).Add(cg, common.Big1)

	if err := ValidateCallGasLimitV07(op, ov); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidateCallGasLimitV07_Nil(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		nilCallGasLimit: true,
	}

	err := ValidateCallGasLimitV07(op, gas.NewDefaultOverhead())
	if err == nil {
		t.Fatal("got nil, want err")
	}
}
