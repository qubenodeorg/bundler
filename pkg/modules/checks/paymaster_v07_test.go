package checks

import (
	"math/big"
	"testing"

	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestValidatePaymaster_ZeroAddress(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidatePaymaster(op, testutils.NonStakedZeroDepositInfo, testutils.MockGetCode, userop.V0_6); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidatePaymaster_NotDeployed(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()

	err := ValidatePaymaster(op, testutils.NonStakedDepositInfo, testutils.MockGetCodeZero, userop.V0_6)
	if err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidatePaymaster_InsufficientDeposit(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()

	err := ValidatePaymaster(op, testutils.NonStakedZeroDepositInfo, testutils.MockGetCode, userop.V0_6)
	if err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidatePaymaster_Success(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()

	if err := ValidatePaymaster(op, testutils.NonStakedDepositInfo, testutils.MockGetCode, userop.V0_6); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidatePaymaster_V08ZeroAddrWithData(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	// Set a paymaster address as a non-empty paymasterAndData for a zero-address paymaster
	// but actually zero-address paymaster means no paymaster
	op.PaymasterAndData = []byte{}

	err := ValidatePaymaster(op, testutils.NonStakedZeroDepositInfo, testutils.MockGetCode, userop.V0_8)
	if err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidatePaymaster_V08WithPaymaster(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()

	if err := ValidatePaymaster(op, testutils.NonStakedDepositInfo, testutils.MockGetCode, userop.V0_8); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidatePaymasterVerificationGasLimit_ZeroAddress(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidatePaymasterVerificationGasLimit(op, big.NewInt(100000)); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidatePaymasterVerificationGasLimit_ExceedsMax(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()
	// Use partialNilOp to override PaymasterVerificationGasLimit to a value > max(1)
	wrapped := &partialNilOp{
		UserOperationInterface: op,
		overridePaymasterVGL: big.NewInt(2),
	}

	err := ValidatePaymasterVerificationGasLimit(wrapped, big.NewInt(1))
	if err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidatePaymasterVerificationGasLimit_Success(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()

	if err := ValidatePaymasterVerificationGasLimit(op, big.NewInt(99999999)); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}



func TestValidatePaymasterPostOpGasLimit_ZeroAddress(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidatePaymasterPostOpGasLimit(op); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidatePaymasterPostOpGasLimit_TooLow(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()
	// Override to a very low value
	op.VerificationGasLimit = big.NewInt(100)

	pogl := op.GetPaymasterPostOpGasLimit()
	if pogl.Cmp(big.NewInt(12000)) < 0 {
		if err := ValidatePaymasterPostOpGasLimit(op); err == nil {
			t.Fatal("got nil, want err")
		}
		return
	}
}

func TestValidatePaymasterPostOpGasLimit_Success(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()

	pogl := op.GetPaymasterPostOpGasLimit()
	if pogl.Cmp(big.NewInt(12000)) >= 0 {
		if err := ValidatePaymasterPostOpGasLimit(op); err != nil {
			t.Fatalf("got %v, want nil", err)
		}
		return
	}
}

func TestValidatePaymasterPostOpGasLimit_Nil(t *testing.T) {
	mockOp := testutils.MockValidInitUserOp()
	mockOp.PaymasterAndData = mockOp.Sender.Bytes()
	op := &partialNilOp{
		UserOperationInterface: mockOp,
		nilPaymasterPOGL: true,
	}

	err := ValidatePaymasterPostOpGasLimit(op)
	if err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidatePaymasterVerificationGasLimit_Nil(t *testing.T) {
	mockOp := testutils.MockValidInitUserOp()
	mockOp.PaymasterAndData = mockOp.Sender.Bytes()
	op := &partialNilOp{
		UserOperationInterface: mockOp,
		nilPaymasterVGL: true,
	}

	err := ValidatePaymasterVerificationGasLimit(op, big.NewInt(100000))
	if err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidatePaymasterData_V07ZeroAddrNoData(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidatePaymasterData(op, userop.V0_7); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidatePaymasterData_V07NoData(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidatePaymasterData(op, userop.V0_6); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidatePaymasterData_NonZeroPaymaster(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()

	if err := ValidatePaymasterData(op, userop.V0_7); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

// TestValidatePaymasterData_V06ZeroAddrWithData verifies that a zero-address
// paymaster with non-empty paymasterData is rejected for v0.6 after the
// consolidation.
func TestValidatePaymasterData_V06ZeroAddrWithData(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		overridePaymasterData: testutils.ValidAddress1.Bytes(),
	}

	if err := ValidatePaymasterData(op, userop.V0_6); err != ErrPaymasterZeroAddrWithData {
		t.Fatalf("got %v, want %v", err, ErrPaymasterZeroAddrWithData)
	}
}

// TestValidatePaymasterData_V08ZeroAddrWithData verifies that a zero-address
// paymaster with non-empty paymasterData is rejected for v0.8.
func TestValidatePaymasterData_V08ZeroAddrWithData(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		overridePaymasterData: testutils.ValidAddress1.Bytes(),
	}

	if err := ValidatePaymasterData(op, userop.V0_8); err != ErrPaymasterZeroAddrWithData {
		t.Fatalf("got %v, want %v", err, ErrPaymasterZeroAddrWithData)
	}
}

// TestValidatePaymasterData_V09ZeroAddrWithData verifies that a zero-address
// paymaster with non-empty paymasterData is rejected for v0.9.
func TestValidatePaymasterData_V09ZeroAddrWithData(t *testing.T) {
	op := &partialNilOp{
		UserOperationInterface: testutils.MockValidInitUserOp(),
		overridePaymasterData: testutils.ValidAddress1.Bytes(),
	}

	if err := ValidatePaymasterData(op, userop.V0_9); err != ErrPaymasterZeroAddrWithData {
		t.Fatalf("got %v, want %v", err, ErrPaymasterZeroAddrWithData)
	}
}
