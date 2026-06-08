package checks

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

// TestSenderExistAndInitCodeDNE calls checks.ValidateSender where sender contract exist and initCode does
// not. Expect nil.
func TestSenderExistAndInitCodeDNE(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.InitCode = []byte{}
	err := ValidateSender(op, testutils.MockGetCode)

	if err != nil {
		t.Fatalf(`got err %v, want nil`, err)
	}
}

// TestSenderAndInitCodeExist calls checks.ValidateSender where sender contract and initCode exist. Expect
// error.
func TestSenderAndInitCodeExist(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	err := ValidateSender(op, testutils.MockGetCode)

	if err == nil {
		t.Fatalf(`got nil, want err`)
	}
}

// TestSenderDNEAndInitCodeExist calls checks.ValidateSender where sender contract does not exist and
// initCode does. Expect nil.
func TestSenderDNEAndInitCodeExist(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	err := ValidateSender(op, testutils.MockGetCodeZero)

	if err != nil {
		t.Fatalf(`got err %v, want nil`, err)
	}
}

// TestSenderAndInitCodeDNE calls checks.ValidateSender where sender contract and initCode does not exist.
// Expect error.
func TestSenderAndInitCodeDNE(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.InitCode = []byte{}
	err := ValidateSender(op, testutils.MockGetCodeZero)

	if err == nil {
		t.Fatalf(`got nil, want err`)
	}
}

// --- ValidateSenderV09 tests ---

// TestSenderV09ExistingAccountWithInitCode allows initCode with existing account in v0.9.
func TestSenderV09ExistingAccountWithInitCode(t *testing.T) {
	op := &v09.UserOperation{
		Sender:  common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Factory: common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
	}
	err := ValidateSenderV09(op, testutils.MockGetCode)
	if err != nil {
		t.Fatalf("v0.9 should allow initCode with existing account: %v", err)
	}
}

// TestSenderV09ExistingAccountNoInitCode allows existing account without initCode in v0.9.
func TestSenderV09ExistingAccountNoInitCode(t *testing.T) {
	op := &v09.UserOperation{
		Sender:  common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Factory: common.HexToAddress("0x"),
	}
	err := ValidateSenderV09(op, testutils.MockGetCode)
	if err != nil {
		t.Fatalf("v0.9 should allow existing account without initCode: %v", err)
	}
}

// TestSenderV09NewAccountWithInitCode allows new account with initCode in v0.9.
func TestSenderV09NewAccountWithInitCode(t *testing.T) {
	op := &v09.UserOperation{
		Sender:  common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Factory: common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
	}
	err := ValidateSenderV09(op, testutils.MockGetCodeZero)
	if err != nil {
		t.Fatalf("v0.9 should allow new account with initCode: %v", err)
	}
}

// TestSenderV09NewAccountNoInitCode rejects new account without initCode in v0.9.
func TestSenderV09NewAccountNoInitCode(t *testing.T) {
	op := &v09.UserOperation{
		Sender:  common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Factory: common.HexToAddress("0x"),
	}
	err := ValidateSenderV09(op, testutils.MockGetCodeZero)
	if err == nil {
		t.Fatal("v0.9 should reject new account without initCode")
	}
}
