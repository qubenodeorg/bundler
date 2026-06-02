package checks

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
)

func TestMFLessThanBF_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	gbf := testutils.GetMockBaseFeeFunc(common.Big2)
	op.MaxFeePerGas = common.Big1
	op.MaxPriorityFeePerGas = common.Big0

	if err := ValidateFeePerGasV07(op, gbf); err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestMFEqualBF_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	gbf := testutils.GetMockBaseFeeFunc(common.Big1)
	op.MaxFeePerGas = common.Big1
	op.MaxPriorityFeePerGas = common.Big0

	if err := ValidateFeePerGasV07(op, gbf); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestMFMoreThanBF_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	gbf := testutils.GetMockBaseFeeFunc(common.Big1)
	op.MaxFeePerGas = common.Big2
	op.MaxPriorityFeePerGas = common.Big0

	if err := ValidateFeePerGasV07(op, gbf); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestMPFMoreThanMF_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	gbf := testutils.GetMockBaseFeeFunc(common.Big1)
	op.MaxFeePerGas = common.Big2
	op.MaxPriorityFeePerGas = common.Big3

	if err := ValidateFeePerGasV07(op, gbf); err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestMPFEqualMF_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	gbf := testutils.GetMockBaseFeeFunc(common.Big1)
	op.MaxFeePerGas = common.Big2
	op.MaxPriorityFeePerGas = common.Big2

	if err := ValidateFeePerGasV07(op, gbf); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestLegacyFeeMode_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.MaxFeePerGas = common.Big2
	op.MaxPriorityFeePerGas = common.Big2

	gbf := func() (*big.Int, error) {
		return nil, nil
	}

	if err := ValidateFeePerGasV07(op, gbf); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestLegacyFeeModeFail_V07(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.MaxFeePerGas = common.Big2
	op.MaxPriorityFeePerGas = common.Big1

	gbf := func() (*big.Int, error) {
		return nil, nil
	}

	if err := ValidateFeePerGasV07(op, gbf); err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidateFeePerGasV07_NilMaxPriorityFee(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		nilMaxPriority: true,
	}

	err := ValidateFeePerGasV07(op, testutils.GetMockBaseFeeFunc(common.Big1))
	if err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidateFeePerGasV07_NilMaxFee(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		nilMaxFee: true,
	}

	err := ValidateFeePerGasV07(op, testutils.GetMockBaseFeeFunc(common.Big1))
	if err == nil {
		t.Fatal("got nil, want err")
	}
}
