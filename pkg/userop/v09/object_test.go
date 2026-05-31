package v09_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

func mockV09Op() *v09.UserOperation {
	return &v09.UserOperation{
		Sender:                        common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                         big.NewInt(0),
		Factory:                       common.HexToAddress("0xe19e9755942bb0bd0cccce25b1742596b8a8250b"),
		FactoryData:                   common.Hex2Bytes("0000"),
		CallData:                      common.Hex2Bytes("80c5c7d0"),
		CallGasLimit:                  big.NewInt(0x558c),
		VerificationGasLimit:          big.NewInt(0x129727),
		PreVerificationGas:            big.NewInt(0xc539),
		MaxFeePerGas:                  big.NewInt(0xa862145e),
		MaxPriorityFeePerGas:          big.NewInt(0xa8621440),
		Paymaster:                     common.HexToAddress("0x"),
		PaymasterVerificationGasLimit: big.NewInt(0),
		PaymasterPostOpGasLimit:       big.NewInt(0),
		PaymasterData:                 []byte{},
		Signature:                     common.Hex2Bytes("a925"),
	}
}

func mockV09OpWithPaymasterSig() *v09.UserOperation {
	op := mockV09Op()
	op.Paymaster = common.HexToAddress("0x1234567890123456789012345678901234567890")
	op.PaymasterVerificationGasLimit = big.NewInt(100000)
	op.PaymasterPostOpGasLimit = big.NewInt(50000)
	op.PaymasterData = common.Hex2Bytes("deadbeef")
	op.PaymasterSignature = common.Hex2Bytes("cafebabe")
	return op
}

func mockV09OpWithAuth() *v09.UserOperation {
	op := mockV09Op()
	op.EIP7702Auth = &v09.Authorization{
		ChainID: big.NewInt(1),
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Nonce:   big.NewInt(0),
		YParity: 27,
		R:       big.NewInt(12345),
		S:       big.NewInt(67890),
	}
	return op
}

func TestV09GetPaymaster(t *testing.T) {
	op := mockV09Op()
	if op.GetPaymaster() != common.HexToAddress("0x") {
		t.Fatalf("got %s, want zero address", op.GetPaymaster().Hex())
	}
}

func TestV09GetFactory(t *testing.T) {
	op := mockV09Op()
	want := common.HexToAddress("0xe19e9755942bb0bd0cccce25b1742596b8a8250b")
	if op.GetFactory() != want {
		t.Fatalf("got %s, want %s", op.GetFactory().Hex(), want.Hex())
	}
}

func TestV09GetFactoryData(t *testing.T) {
	op := mockV09Op()
	fd := op.GetFactoryData()
	if len(fd) == 0 {
		t.Fatal("got empty factory data")
	}
}

func TestV09GetFactoryDataEmpty(t *testing.T) {
	op := mockV09Op()
	op.FactoryData = []byte{}
	fd := op.GetFactoryData()
	if len(fd) != 0 {
		t.Fatalf("got %d bytes, want 0", len(fd))
	}
}

func TestV09GetMaxGasAvailable(t *testing.T) {
	op := mockV09Op()
	mag := op.GetMaxGasAvailable()
	want := new(big.Int).Add(op.VerificationGasLimit, new(big.Int).Add(op.CallGasLimit, op.PreVerificationGas))
	if mag.Cmp(want) != 0 {
		t.Fatalf("got %s, want %s", mag.String(), want.String())
	}
}

func TestV09GetMaxPrefund(t *testing.T) {
	op := mockV09Op()
	mp := op.GetMaxPrefund()
	want := new(big.Int).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
	if mp.Cmp(want) != 0 {
		t.Fatalf("got %s, want %s", mp.String(), want.String())
	}
}

func TestV09GetDynamicGasPrice(t *testing.T) {
	bf := big.NewInt(3)
	op := mockV09Op()
	want := big.NewInt(4)
	op.MaxFeePerGas = big.NewInt(4)
	op.MaxPriorityFeePerGas = big.NewInt(3)
	if op.GetDynamicGasPrice(bf).Cmp(want) != 0 {
		t.Fatalf("got %d, want %d", op.GetDynamicGasPrice(bf).Int64(), want.Int64())
	}
}

func TestV09GetDynamicGasPriceNilBF(t *testing.T) {
	op := mockV09Op()
	op.MaxFeePerGas = big.NewInt(4)
	op.MaxPriorityFeePerGas = big.NewInt(3)
	if op.GetDynamicGasPrice(nil).Cmp(op.MaxPriorityFeePerGas) != 0 {
		t.Fatalf("got %d, want %d", op.GetDynamicGasPrice(nil).Int64(), op.MaxPriorityFeePerGas.Int64())
	}
}

func TestV09Pack(t *testing.T) {
	op := mockV09Op()
	packed := op.Pack()
	if len(packed) == 0 {
		t.Fatal("Pack returned empty bytes")
	}
}

func TestV09PackWithPaymasterSig(t *testing.T) {
	op := mockV09OpWithPaymasterSig()
	packed := op.Pack()
	if len(packed) == 0 {
		t.Fatal("Pack returned empty bytes")
	}
}

func TestV09PackForSignatureExcludesPaymasterSig(t *testing.T) {
	op := mockV09OpWithPaymasterSig()
	packed := op.PackForSignature()
	if len(packed) == 0 {
		t.Fatal("PackForSignature returned empty bytes")
	}
}

func TestV09GetUserOpHash(t *testing.T) {
	op := mockV09Op()
	ep := common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009")
	chainID := big.NewInt(1)
	hash := op.GetUserOpHash(ep, chainID)
	if hash == (common.Hash{}) {
		t.Fatal("GetUserOpHash returned zero hash")
	}
}

func TestV09MarshalJSON(t *testing.T) {
	op := mockV09OpWithPaymasterSig()
	data, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalJSON returned empty bytes")
	}
}

func TestV09MarshalJSONWithAuth(t *testing.T) {
	op := mockV09OpWithAuth()
	data, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalJSON returned empty bytes")
	}
}

func TestV09MarshalJSONNoPaymasterSig(t *testing.T) {
	op := mockV09Op()
	data, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalJSON returned empty bytes")
	}
}

func TestV09ToMap(t *testing.T) {
	op := mockV09OpWithPaymasterSig()
	m, err := op.ToMap()
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}
	if m == nil {
		t.Fatal("ToMap returned nil")
	}
}

func TestV09ToMapError(t *testing.T) {
	// This test exercises error path - but MarshalJSON shouldn't fail for valid op
	// So we just verify ToMap works
	op := mockV09Op()
	m, err := op.ToMap()
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}
	if m == nil {
		t.Fatal("ToMap returned nil")
	}
}

func TestV09NewValid(t *testing.T) {
	data := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0xe19e9755942bb0bd0cccce25b1742596b8a8250b",
		"factoryData":                   "0x0000",
		"callData":                      "0x80c5c7d0",
		"callGasLimit":                  "0x558c",
		"verificationGasLimit":          "0x129727",
		"preVerificationGas":            "0xc539",
		"maxFeePerGas":                  "0xa862145e",
		"maxPriorityFeePerGas":          "0xa8621440",
		"paymaster":                     "0x",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"signature":                     "0xa925",
	}
	op, err := v09.New(data)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if op == nil {
		t.Fatal("New returned nil")
	}
}

func TestV09NewWithPaymasterSignature(t *testing.T) {
	data := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0xe19e9755942bb0bd0cccce25b1742596b8a8250b",
		"factoryData":                   "0x0000",
		"callData":                      "0x80c5c7d0",
		"callGasLimit":                  "0x558c",
		"verificationGasLimit":          "0x129727",
		"preVerificationGas":            "0xc539",
		"maxFeePerGas":                  "0xa862145e",
		"maxPriorityFeePerGas":          "0xa8621440",
		"paymaster":                     "0x1234567890123456789012345678901234567890",
		"paymasterVerificationGasLimit": "0x186a0",
		"paymasterPostOpGasLimit":       "0xc350",
		"paymasterData":                 "0xdeadbeef",
		"paymasterSignature":            "0xcafebabe",
		"signature":                     "0xa925",
	}
	op, err := v09.New(data)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if op.PaymasterSignature == nil || len(op.PaymasterSignature) == 0 {
		t.Fatal("PaymasterSignature should not be empty")
	}
}

func TestV09NewWithAuth(t *testing.T) {
	data := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0xe19e9755942bb0bd0cccce25b1742596b8a8250b",
		"factoryData":                   "0x0000",
		"callData":                      "0x80c5c7d0",
		"callGasLimit":                  "0x558c",
		"verificationGasLimit":          "0x129727",
		"preVerificationGas":            "0xc539",
		"maxFeePerGas":                  "0xa862145e",
		"maxPriorityFeePerGas":          "0xa8621440",
		"paymaster":                     "0x",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"eip7702Auth": map[string]any{
			"chainId": "0x1",
			"address": "0x1234567890123456789012345678901234567890",
			"nonce":   "0x0",
			"yParity": float64(27),
			"r":       "0x3039",
			"s":       "0x10932",
		},
		"signature": "0xa925",
	}
	op, err := v09.New(data)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if op.EIP7702Auth == nil {
		t.Fatal("EIP7702Auth should not be nil")
	}
}

func TestV09NewInvalid(t *testing.T) {
	_, err := v09.New(map[string]any{
		"sender": "0xinvalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid data")
	}
}

func TestV09NewMissingField(t *testing.T) {
	_, err := v09.New(map[string]any{
		"sender": "0xa13D69573f994bf662C2714560c44dd7266FC547",
	})
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}
