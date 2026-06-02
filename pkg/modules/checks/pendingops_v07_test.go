package checks

import (
	"errors"
	"math/big"
	"testing"

	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestNoPendingOps_V07(t *testing.T) {
	penOps := []*userop.UserOperation{}
	op := testutils.MockValidInitUserOp()

	if err := ValidatePendingOpsV07(op, penOps); err != nil {
		t.Fatalf("got err %v, want nil", err)
	}
}

func TestPendingOpsWithNewOp_V07(t *testing.T) {
	penOp := testutils.MockValidInitUserOp()
	penOps := []*userop.UserOperation{penOp}
	op := testutils.MockValidInitUserOp()
	op.Nonce = big.NewInt(1)

	if err := ValidatePendingOpsV07(op, penOps); err != nil {
		t.Fatalf("got err %v, want nil", err)
	}
}

func TestPendingOpsWithNoGasFeeReplacement_V07(t *testing.T) {
	penOp := testutils.MockValidInitUserOp()
	penOps := []*userop.UserOperation{penOp}
	op := testutils.MockValidInitUserOp()

	if err := ValidatePendingOpsV07(op, penOps); !errors.Is(err, ErrReplacementOpUnderpriced) {
		t.Fatalf("got %v, want ErrReplacementOpUnderpriced", err)
	}
}

func TestPendingOpsWithOnlyMaxFeeReplacement_V07(t *testing.T) {
	penOp := testutils.MockValidInitUserOp()
	penOps := []*userop.UserOperation{penOp}
	op := testutils.MockValidInitUserOp()
	op.MaxFeePerGas, _ = calcNewThresholds(op.MaxFeePerGas, op.MaxPriorityFeePerGas)

	if err := ValidatePendingOpsV07(op, penOps); !errors.Is(err, ErrReplacementOpUnderpriced) {
		t.Fatalf("got %v, want ErrReplacementOpUnderpriced", err)
	}
}

func TestPendingOpsWithOnlyMaxPriorityFeeReplacement_V07(t *testing.T) {
	penOp := testutils.MockValidInitUserOp()
	penOps := []*userop.UserOperation{penOp}
	op := testutils.MockValidInitUserOp()
	_, op.MaxPriorityFeePerGas = calcNewThresholds(op.MaxFeePerGas, op.MaxPriorityFeePerGas)

	if err := ValidatePendingOpsV07(op, penOps); !errors.Is(err, ErrReplacementOpUnderpriced) {
		t.Fatalf("got %v, want ErrReplacementOpUnderpriced", err)
	}
}

func TestPendingOpsWithOkGasFeeReplacement_V07(t *testing.T) {
	penOp := testutils.MockValidInitUserOp()
	penOps := []*userop.UserOperation{penOp}
	op := testutils.MockValidInitUserOp()
	op.MaxFeePerGas, op.MaxPriorityFeePerGas = calcNewThresholds(op.MaxFeePerGas, op.MaxPriorityFeePerGas)

	if err := ValidatePendingOpsV07(op, penOps); err != nil {
		t.Fatalf("got err %v, want nil", err)
	}
}

func TestValidatePendingOpsV07_NilNonce(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		nilNonce: true,
	}

	if err := ValidatePendingOpsV07(op, []*userop.UserOperation{testutils.MockValidInitUserOp()}); err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidatePendingOpsV07_NilMaxFee(t *testing.T) {
	penOp := testutils.MockValidInitUserOp()
	penOps := []*userop.UserOperation{penOp}
	mockOp := testutils.MockValidInitUserOp()
	mockOp.Nonce = big.NewInt(0)
	op := &partialNilOp{
		UserOperationInterface: mockOp,
		nilMaxFee: true,
	}

	if err := ValidatePendingOpsV07(op, penOps); err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidatePendingOpsV07_NilMaxPriorityFee(t *testing.T) {
	penOp := testutils.MockValidInitUserOp()
	penOps := []*userop.UserOperation{penOp}
	mockOp := testutils.MockValidInitUserOp()
	mockOp.Nonce = big.NewInt(0)
	op := &partialNilOp{
		UserOperationInterface: mockOp,
		nilMaxPriority: true,
	}

	if err := ValidatePendingOpsV07(op, penOps); err == nil {
		t.Fatal("got nil, want err")
	}
}
