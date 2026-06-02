package checks

import (
	"testing"

	"github.com/zarbanio/bundler/internal/testutils"
)

func TestValidateFactory_ZeroAddress(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidateFactory(op); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidateFactory_NonZeroAddress(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.InitCode = op.Sender.Bytes()

	if err := ValidateFactory(op); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidateFactoryData_ZeroAddress(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidateFactoryData(op); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidateFactoryData_NonZeroAddress(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.InitCode = op.Sender.Bytes()

	if err := ValidateFactoryData(op); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidateSenderV07_ExistAndNoInitCode(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.InitCode = []byte{}

	if err := ValidateSenderV07(op, testutils.MockGetCode); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidateSenderV07_NotExistWithInitCode(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidateSenderV07(op, testutils.MockGetCodeZero); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestValidateSenderV07_ExistWithInitCode(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidateSenderV07(op, testutils.MockGetCode); err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidateSenderV07_NotExistNoInitCode(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.InitCode = []byte{}

	if err := ValidateSenderV07(op, testutils.MockGetCodeZero); err == nil {
		t.Fatal("got nil, want err")
	}
}

func TestValidateERC7562Rules(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	if err := ValidateERC7562Rules(op, op.Sender); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}
