package v09_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

func TestV09_PrimitiveGetters(t *testing.T) {
	op := mockV09Op()
	op.Paymaster = common.HexToAddress("0xb000000000000000000000000000000000000002")
	op.PaymasterVerificationGasLimit = big.NewInt(0x1111)
	op.PaymasterPostOpGasLimit = big.NewInt(0x2222)
	op.PaymasterData = []byte{0xde, 0xad}

	if op.GetSender() != op.Sender {
		t.Fatalf("sender")
	}
	if op.GetNonce().Cmp(op.Nonce) != 0 {
		t.Fatalf("nonce")
	}
	if !bytes.Equal(op.GetCallData(), op.CallData) {
		t.Fatalf("callData")
	}
	if op.GetCallGasLimit().Cmp(op.CallGasLimit) != 0 {
		t.Fatalf("callGasLimit")
	}
	if op.GetVerificationGasLimit().Cmp(op.VerificationGasLimit) != 0 {
		t.Fatalf("verificationGasLimit")
	}
	if op.GetPreVerificationGas().Cmp(op.PreVerificationGas) != 0 {
		t.Fatalf("preVerificationGas")
	}
	if op.GetMaxFeePerGas().Cmp(op.MaxFeePerGas) != 0 {
		t.Fatalf("maxFeePerGas")
	}
	if op.GetMaxPriorityFeePerGas().Cmp(op.MaxPriorityFeePerGas) != 0 {
		t.Fatalf("maxPriorityFeePerGas")
	}
	if !bytes.Equal(op.GetSignature(), op.Signature) {
		t.Fatalf("signature")
	}
	if op.GetPaymasterVerificationGasLimit().Cmp(op.PaymasterVerificationGasLimit) != 0 {
		t.Fatalf("paymasterVerificationGasLimit")
	}
	if op.GetPaymasterPostOpGasLimit().Cmp(op.PaymasterPostOpGasLimit) != 0 {
		t.Fatalf("paymasterPostOpGasLimit")
	}
	if !bytes.Equal(op.GetPaymasterData(), op.PaymasterData) {
		t.Fatalf("paymasterData")
	}
}

func TestV09_EIP7702AuthGetter(t *testing.T) {
	none := mockV09Op()
	if none.GetEIP7702Auth() != nil {
		t.Fatalf("expected nil EIP7702Auth for unset field")
	}
	auth := mockV09OpWithAuth()
	got := auth.GetEIP7702Auth()
	if got == nil {
		t.Fatalf("expected non-nil EIP7702Auth")
	}
	if _, ok := got.(*v09.Authorization); !ok {
		t.Fatalf("expected *v09.Authorization, got %T", got)
	}
}

func TestV09_PaymasterSignatureRoundTrip(t *testing.T) {
	op := mockV09OpWithPaymasterSig()
	if len(op.GetPaymasterSignature()) == 0 {
		t.Fatalf("expected non-empty paymasterSignature")
	}
}

func TestV09_PackInitCode_Empty(t *testing.T) {
	op := mockV09Op()
	op.Factory = common.Address{}
	if len(op.PackInitCode()) != 0 {
		t.Fatalf("zero factory should pack empty initCode")
	}
}
