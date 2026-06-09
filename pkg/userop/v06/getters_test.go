package v06_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v06"
)

// fullOp returns a v0.6 op populated for exhaustive getter tests.
func fullOp() *v06.UserOperation {
	op := mockV06Op()
	op.PaymasterAndData = append(
		common.HexToAddress("0xb000000000000000000000000000000000000002").Bytes(),
		[]byte{0xfe, 0xed}...,
	)
	return op
}

func TestV06_PrimitiveGetters(t *testing.T) {
	op := fullOp()
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

func TestV06_V07PlusGettersReturnDefaults(t *testing.T) {
	op := fullOp()
	if op.GetEIP7702Auth() != nil {
		t.Fatalf("GetEIP7702Auth should be nil for v0.6")
	}
	if op.GetPaymasterVerificationGasLimit().Sign() != 0 {
		t.Fatalf("v0.6 paymasterVerificationGasLimit should be 0")
	}
	if op.GetPaymasterPostOpGasLimit().Sign() != 0 {
		t.Fatalf("v0.6 paymasterPostOpGasLimit should be 0")
	}
	if len(op.GetPaymasterData()) != 0 {
		t.Fatalf("v0.6 paymasterData should be empty")
	}
	if len(op.GetPaymasterSignature()) != 0 {
		t.Fatalf("v0.6 paymasterSignature should be empty")
	}
}

func TestV06_GetFactoryData_WithoutFactory(t *testing.T) {
	op := mockV06Op()
	op.InitCode = []byte{0x01}
	if len(op.GetFactoryData()) != 0 {
		t.Fatalf("GetFactoryData should be empty when InitCode < 20 bytes")
	}
}

func TestV06_GetFactoryData_WithFactory(t *testing.T) {
	op := mockV06Op()
	if len(op.GetFactoryData()) == 0 {
		t.Fatalf("expected non-empty factory data from prefixed InitCode")
	}
}

func TestV06_GetMaxGasAvailable_WithPaymaster(t *testing.T) {
	op := fullOp()
	mg := op.GetMaxGasAvailable()
	// With paymaster, multiplier is 3 on verificationGasLimit.
	want := new(big.Int).Mul(op.VerificationGasLimit, big.NewInt(3))
	want.Add(want, op.PreVerificationGas)
	want.Add(want, op.CallGasLimit)
	if mg.Cmp(want) != 0 {
		t.Fatalf("GetMaxGasAvailable with paymaster: got %s, want %s", mg, want)
	}
}

func TestV06_GetMaxGasAvailable_NoPaymaster(t *testing.T) {
	op := mockV06Op()
	mg := op.GetMaxGasAvailable()
	want := new(big.Int).Add(op.VerificationGasLimit, op.PreVerificationGas)
	want.Add(want, op.CallGasLimit)
	if mg.Cmp(want) != 0 {
		t.Fatalf("GetMaxGasAvailable no paymaster: got %s, want %s", mg, want)
	}
}

func TestV06_GetMaxPrefund(t *testing.T) {
	op := mockV06Op()
	got := op.GetMaxPrefund()
	want := new(big.Int).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
	if got.Cmp(want) != 0 {
		t.Fatalf("GetMaxPrefund: got %s, want %s", got, want)
	}
}

func TestV06_GetDynamicGasPrice(t *testing.T) {
	op := mockV06Op()
	// nil basefee -> just maxPriorityFeePerGas (capped at maxFeePerGas).
	got := op.GetDynamicGasPrice(nil)
	want := op.MaxPriorityFeePerGas
	if want.Cmp(op.MaxFeePerGas) == 1 {
		want = op.MaxFeePerGas
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("GetDynamicGasPrice(nil): got %s want %s", got, want)
	}
	// Large basefee -> capped at maxFeePerGas.
	huge := new(big.Int).Lsh(big.NewInt(1), 200)
	if got := op.GetDynamicGasPrice(huge); got.Cmp(op.MaxFeePerGas) != 0 {
		t.Fatalf("GetDynamicGasPrice(huge): expected cap at maxFeePerGas")
	}
}

func TestV06_PackForSignatureAndHash(t *testing.T) {
	op := mockV06Op()
	if got := op.PackForSignature(); len(got) == 0 {
		t.Fatalf("PackForSignature returned empty")
	}
	ep := common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789")
	h := op.GetUserOpHash(ep, big.NewInt(1))
	if h == (common.Hash{}) {
		t.Fatalf("GetUserOpHash returned zero hash")
	}
}

func TestV06_GetUserOpHashEIP712_Deterministic(t *testing.T) {
	op := mockV06Op()
	var ds [32]byte
	var th [32]byte
	for i := 0; i < 32; i++ {
		ds[i] = byte(i)
		th[i] = byte(31 - i)
	}
	h1 := op.GetUserOpHashEIP712(ds, th)
	h2 := op.GetUserOpHashEIP712(ds, th)
	if h1 != h2 {
		t.Fatalf("EIP-712 hash must be deterministic")
	}
	// Different from legacy hash.
	if h1 == op.GetUserOpHash(common.Address{}, big.NewInt(1)) {
		t.Fatalf("EIP-712 hash should differ from legacy hash")
	}
}

func TestV06_Pack_NotEmpty(t *testing.T) {
	op := mockV06Op()
	if len(op.Pack()) == 0 {
		t.Fatalf("Pack returned empty bytes")
	}
}
