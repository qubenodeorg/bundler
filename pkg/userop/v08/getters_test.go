package v08_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v08"
)

func TestV08_PrimitiveGetters(t *testing.T) {
	op := mockV08Op()
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
	if !bytes.Equal(op.GetFactoryData(), op.FactoryData) {
		t.Fatalf("factoryData")
	}
	if op.GetFactory() != op.Factory {
		t.Fatalf("factory")
	}
}

func TestV08_EIP7702AuthGetter(t *testing.T) {
	noAuth := mockV08Op()
	if noAuth.GetEIP7702Auth() != nil {
		t.Fatalf("nil EIP7702Auth field should return nil from getter")
	}
	withAuth := mockV08OpWithAuth()
	got := withAuth.GetEIP7702Auth()
	if got == nil {
		t.Fatalf("EIP7702Auth set should return non-nil")
	}
	auth, ok := got.(*v08.Authorization)
	if !ok {
		t.Fatalf("expected *v08.Authorization, got %T", got)
	}
	if auth.Address != withAuth.EIP7702Auth.Address {
		t.Fatalf("authorization address mismatch")
	}
}

func TestV08_PaymasterSignatureIsEmpty(t *testing.T) {
	op := mockV08Op()
	if len(op.GetPaymasterSignature()) != 0 {
		t.Fatalf("v0.8 paymasterSignature must be empty")
	}
}

func TestV08_GetMaxPrefund(t *testing.T) {
	op := mockV08Op()
	got := op.GetMaxPrefund()
	want := new(big.Int).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
	if got.Cmp(want) != 0 {
		t.Fatalf("max prefund: got %s want %s", got, want)
	}
}

func TestV08_PackInitCodeAndPaymasterAndData(t *testing.T) {
	op := mockV08Op()
	op.Paymaster = common.HexToAddress("0xb000000000000000000000000000000000000002")
	op.PaymasterVerificationGasLimit = big.NewInt(0xaaaa)
	op.PaymasterPostOpGasLimit = big.NewInt(0xbbbb)
	op.PaymasterData = []byte{0x01, 0x02}

	pic := op.PackInitCode()
	if len(pic) == 0 {
		t.Fatalf("PackInitCode returned empty for set factory")
	}
	ppd := op.PackPaymasterAndData()
	if len(ppd) < common.AddressLength+32 {
		t.Fatalf("PackPaymasterAndData missing gas-limits or paymaster")
	}

	// Zero paymaster → empty paymasterAndData.
	op.Paymaster = common.Address{}
	if len(op.PackPaymasterAndData()) != 0 {
		t.Fatalf("zero paymaster should pack empty")
	}
	// Zero factory → empty initCode.
	op.Factory = common.Address{}
	if len(op.PackInitCode()) != 0 {
		t.Fatalf("zero factory should pack empty")
	}
}

func TestV08_GetUserOpHashEIP712(t *testing.T) {
	op := mockV08Op()
	var ds, th [32]byte
	for i := 0; i < 32; i++ {
		ds[i] = byte(i)
		th[i] = byte(31 - i)
	}
	h1 := op.GetUserOpHashEIP712(ds, th)
	h2 := op.GetUserOpHashEIP712(ds, th)
	if h1 != h2 {
		t.Fatalf("EIP-712 hash must be deterministic")
	}
}

func TestV08_MarshalJSONWithAndWithoutAuth(t *testing.T) {
	op := mockV08OpWithAuth()
	withAuth, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON with auth: %v", err)
	}
	if !bytes.Contains(withAuth, []byte("eip7702Auth")) {
		t.Fatalf("expected JSON to contain eip7702Auth, got %s", withAuth)
	}

	op2 := mockV08Op()
	noAuth, err := op2.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON no auth: %v", err)
	}
	if bytes.Contains(noAuth, []byte("eip7702Auth")) {
		t.Fatalf("expected JSON to omit eip7702Auth when unset, got %s", noAuth)
	}
}

func TestV08_ToMap(t *testing.T) {
	op := mockV08OpWithAuth()
	m, err := op.ToMap()
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}
	if _, ok := m["sender"]; !ok {
		t.Fatalf("ToMap missing sender")
	}
}
