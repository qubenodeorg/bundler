package v07_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestV07_PrimitiveGetters(t *testing.T) {
	op := mockV07OpWithPaymaster()
	if op.GetSender() != op.Sender {
		t.Fatalf("GetSender mismatch")
	}
	if op.GetNonce().Cmp(op.Nonce) != 0 {
		t.Fatalf("GetNonce mismatch")
	}
	if !bytes.Equal(op.GetCallData(), op.CallData) {
		t.Fatalf("GetCallData mismatch")
	}
	if op.GetCallGasLimit().Cmp(op.CallGasLimit) != 0 {
		t.Fatalf("GetCallGasLimit mismatch")
	}
	if op.GetVerificationGasLimit().Cmp(op.VerificationGasLimit) != 0 {
		t.Fatalf("GetVerificationGasLimit mismatch")
	}
	if op.GetPreVerificationGas().Cmp(op.PreVerificationGas) != 0 {
		t.Fatalf("GetPreVerificationGas mismatch")
	}
	if op.GetMaxFeePerGas().Cmp(op.MaxFeePerGas) != 0 {
		t.Fatalf("GetMaxFeePerGas mismatch")
	}
	if op.GetMaxPriorityFeePerGas().Cmp(op.MaxPriorityFeePerGas) != 0 {
		t.Fatalf("GetMaxPriorityFeePerGas mismatch")
	}
	if !bytes.Equal(op.GetSignature(), op.Signature) {
		t.Fatalf("GetSignature mismatch")
	}
}

func TestV07_PaymasterAndFactoryGetters(t *testing.T) {
	op := mockV07OpWithPaymaster()
	if op.GetPaymasterVerificationGasLimit().Cmp(op.PaymasterVerificationGasLimit) != 0 {
		t.Fatalf("paymaster verification gas limit mismatch")
	}
	if op.GetPaymasterPostOpGasLimit().Cmp(op.PaymasterPostOpGasLimit) != 0 {
		t.Fatalf("paymaster post-op gas limit mismatch")
	}
	if !bytes.Equal(op.GetPaymasterData(), op.PaymasterData) {
		t.Fatalf("paymaster data mismatch")
	}
	if op.GetFactory() != op.Factory {
		t.Fatalf("factory mismatch")
	}
	if !bytes.Equal(op.GetFactoryData(), op.FactoryData) {
		t.Fatalf("factory data mismatch")
	}
}

func TestV07_EIP7702AndPaymasterSignatureDefaults(t *testing.T) {
	op := mockV07Op()
	if op.GetEIP7702Auth() != nil {
		t.Fatalf("GetEIP7702Auth must be nil for v0.7")
	}
	if len(op.GetPaymasterSignature()) != 0 {
		t.Fatalf("GetPaymasterSignature must be empty for v0.7")
	}
}

func TestV07_GetMaxGasAvailableNoMultiplier(t *testing.T) {
	op := mockV07OpWithPaymaster()
	got := op.GetMaxGasAvailable()
	// v0.7 drops the 3x paymaster multiplier.
	want := new(big.Int).Add(op.VerificationGasLimit, op.CallGasLimit)
	want.Add(want, op.PreVerificationGas)
	if got.Cmp(want) != 0 {
		t.Fatalf("v0.7 max gas: got %s want %s", got, want)
	}
}

func TestV07_GetMaxPrefund(t *testing.T) {
	op := mockV07Op()
	got := op.GetMaxPrefund()
	want := new(big.Int).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
	if got.Cmp(want) != 0 {
		t.Fatalf("max prefund: got %s want %s", got, want)
	}
}

func TestV07_GetDynamicGasPrice(t *testing.T) {
	op := mockV07Op()
	got := op.GetDynamicGasPrice(nil)
	want := op.MaxPriorityFeePerGas
	if want.Cmp(op.MaxFeePerGas) > 0 {
		want = op.MaxFeePerGas
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("dynamic gas price (nil basefee): got %s want %s", got, want)
	}
	huge := new(big.Int).Lsh(big.NewInt(1), 200)
	if got := op.GetDynamicGasPrice(huge); got.Cmp(op.MaxFeePerGas) != 0 {
		t.Fatalf("dynamic gas price with huge basefee should equal maxFeePerGas")
	}
}

func TestV07_PackInitCode_Empty(t *testing.T) {
	op := mockV07Op()
	op.Factory = common.Address{}
	if len(op.PackInitCode()) != 0 {
		t.Fatalf("empty factory should pack to empty initCode")
	}
}

func TestV07_PackPaymasterAndData_Empty(t *testing.T) {
	op := mockV07Op()
	if len(op.PackPaymasterAndData()) != 0 {
		t.Fatalf("zero paymaster should pack to empty paymasterAndData")
	}
}

func TestV07_GetUserOpHashEIP712_Deterministic(t *testing.T) {
	op := mockV07Op()
	var ds, th [32]byte
	for i := 0; i < 32; i++ {
		ds[i] = byte(i)
		th[i] = byte(255 - i)
	}
	h1 := op.GetUserOpHashEIP712(ds, th)
	h2 := op.GetUserOpHashEIP712(ds, th)
	if h1 != h2 {
		t.Fatalf("EIP-712 hash should be deterministic")
	}
	if h1 == op.GetUserOpHash(common.Address{}, big.NewInt(1)) {
		t.Fatalf("EIP-712 hash should differ from legacy hash")
	}
}
