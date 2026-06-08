package checks

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/userop"
	"github.com/zarbanio/bundler/pkg/userop/v07"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

// TestNilPaymasterAndData calls checks.ValidatePaymasterAndData with no paymaster set. Expects nil.
func TestNilPaymasterAndData(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = []byte{}
	err := ValidatePaymasterAndData(op, testutils.NonStakedZeroDepositInfo, testutils.MockGetCodeZero)

	if err != nil {
		t.Fatalf("got err %v, want nil", err)
	}
}

// TestBadPaymasterAndData calls checks.ValidatePaymasterAndData with invalid paymaster set. Expects error.
func TestBadPaymasterAndData(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = []byte("1234")
	err := ValidatePaymasterAndData(op, testutils.NonStakedZeroDepositInfo, testutils.MockGetCodeZero)

	if err == nil {
		t.Fatal("got nil, want err")
	}
}

// TestZeroByteCodePaymasterAndData calls checks.ValidatePaymasterAndData with paymaster contract not
// deployed. Expects error.
func TestZeroByteCodePaymasterAndData(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()
	err := ValidatePaymasterAndData(op, testutils.NonStakedZeroDepositInfo, testutils.MockGetCodeZero)

	if err == nil {
		t.Fatal("got nil, want err")
	}
}

// TestNonStakedZeroDepositPaymasterAndData calls checks.ValidatePaymasterAndData with paymaster that is not
// staked with zero deposit. Expects error.
func TestNonStakedZeroDepositPaymasterAndData(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()
	err := ValidatePaymasterAndData(op, testutils.NonStakedZeroDepositInfo, testutils.MockGetCode)

	if err == nil {
		t.Fatal("got nil, want err")
	}
}

// TestZeroDepositPaymasterAndData calls checks.ValidatePaymasterAndData with paymaster that is staked but
// with zero deposit. Expects error.
func TestZeroDepositPaymasterAndData(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()
	err := ValidatePaymasterAndData(op, testutils.StakedZeroDepositInfo, testutils.MockGetCode)

	if err == nil {
		t.Fatal("got nil, want err")
	}
}

// TestNotStakedPaymasterAndData calls checks.ValidatePaymasterAndData with paymaster that is not staked and
// has sufficient deposit. Expects nil.
func TestNotStakedPaymasterAndData(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()
	err := ValidatePaymasterAndData(op, testutils.NonStakedDepositInfo, testutils.MockGetCode)

	if err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

// TestPaymasterAndData calls checks.ValidatePaymasterAndData with paymaster that is staked and has sufficient
// deposit. Expects nil.
func TestPaymasterAndData(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()
	err := ValidatePaymasterAndData(op, testutils.StakedDepositInfo, testutils.MockGetCode)

	if err != nil {
		t.Fatalf("got err %v, want nil", err)
	}
}

// --- ValidatePaymasterSignature tests ---

func TestValidatePaymasterSignatureV06NoPaymaster(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	err := ValidatePaymasterSignature(op, userop.V0_6)
	if err != nil {
		t.Fatalf("v0.6 without paymaster should not error: %v", err)
	}
}

func TestValidatePaymasterSignatureV06WithPaymaster(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = op.Sender.Bytes()
	err := ValidatePaymasterSignature(op, userop.V0_6)
	if err != nil {
		t.Fatalf("v0.6 with paymaster should not error: %v", err)
	}
}

func TestValidatePaymasterSignatureV09NoPaymaster(t *testing.T) {
	op := &v09.UserOperation{
		Sender:   common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Paymaster: common.HexToAddress("0x"),
	}
	err := ValidatePaymasterSignature(op, userop.V0_9)
	if err != nil {
		t.Fatalf("v0.9 without paymaster should not error: %v", err)
	}
}

func TestValidatePaymasterSignatureV09WithPaymasterNoSig(t *testing.T) {
	op := &v09.UserOperation{
		Sender:    common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Paymaster: common.HexToAddress("0x1234567890123456789012345678901234567890"),
	}
	err := ValidatePaymasterSignature(op, userop.V0_9)
	if err == nil {
		t.Fatal("v0.9 with paymaster but no signature should error")
	}
}

func TestValidatePaymasterSignatureV09WithPaymasterAndSig(t *testing.T) {
	op := &v09.UserOperation{
		Sender:             common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Paymaster:          common.HexToAddress("0x1234567890123456789012345678901234567890"),
		PaymasterSignature: []byte("signature"),
	}
	err := ValidatePaymasterSignature(op, userop.V0_9)
	if err != nil {
		t.Fatalf("v0.9 with paymaster and signature should not error: %v", err)
	}
}

func TestValidatePaymasterSignatureV07WithPaymasterNoSig(t *testing.T) {
	op := &v07.UserOperation{
		Sender:    common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Paymaster: common.HexToAddress("0x1234567890123456789012345678901234567890"),
	}
	err := ValidatePaymasterSignature(op, userop.V0_7)
	if err != nil {
		t.Fatalf("v0.7 should not require paymasterSignature: %v", err)
	}
}
