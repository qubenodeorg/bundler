package v06_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v06"
)

func mockV06Op() *v06.UserOperation {
	return &v06.UserOperation{
		Sender:               common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                big.NewInt(0),
		InitCode:             common.Hex2Bytes("e19e9755942bb0bd0cccce25b1742596b8a8250b3bf2c3e700000000000000000000000078d4f01f56b982a3b03c4e127a5d3afa8ebee6860000000000000000000000008b388a082f370d8ac2e2b3997e9151168bd09ff50000000000000000000000000000000000000000000000000000000000000000"),
		CallData:             common.Hex2Bytes("80c5c7d0000000000000000000000000a13d69573f994bf662c2714560c44dd7266fc547000000000000000000000000000000000000000000000000016345785d8a000000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000"),
		CallGasLimit:         big.NewInt(0x558c),
		VerificationGasLimit: big.NewInt(0x129727),
		PreVerificationGas:   big.NewInt(0xc539),
		MaxFeePerGas:         big.NewInt(0xa862145e),
		MaxPriorityFeePerGas: big.NewInt(0xa8621440),
		PaymasterAndData:     []byte{},
		Signature:            common.Hex2Bytes("a925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b"),
	}
}

func mockV06OpWithPaymaster() *v06.UserOperation {
	op := mockV06Op()
	op.PaymasterAndData = append(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes(), []byte{0x01}...)
	return op
}

func TestV06GetPaymasterEmpty(t *testing.T) {
	op := mockV06Op()
	pm := op.GetPaymaster()
	if pm != common.HexToAddress("0x") {
		t.Fatalf("got %s, want zero address", pm.Hex())
	}
}

func TestV06GetPaymasterWithValue(t *testing.T) {
	op := mockV06OpWithPaymaster()
	want := common.HexToAddress("0x1234567890123456789012345678901234567890")
	if op.GetPaymaster() != want {
		t.Fatalf("got %s, want %s", op.GetPaymaster().Hex(), want.Hex())
	}
}

func TestV06GetFactory(t *testing.T) {
	op := mockV06Op()
	want := common.HexToAddress("0xe19e9755942bb0bd0cccce25b1742596b8a8250b")
	if op.GetFactory() != want {
		t.Fatalf("got %s, want %s", op.GetFactory().Hex(), want.Hex())
	}
}

func TestV06GetFactoryEmpty(t *testing.T) {
	op := mockV06Op()
	op.InitCode = []byte{}
	if op.GetFactory() != common.HexToAddress("0x") {
		t.Fatalf("got %s, want zero address", op.GetFactory().Hex())
	}
}

func TestV06GetFactoryData(t *testing.T) {
	op := mockV06Op()
	fd := op.GetFactoryData()
	if len(fd) == 0 {
		t.Fatal("got empty factory data")
	}
}

func TestV06GetFactoryDataEmpty(t *testing.T) {
	op := mockV06Op()
	op.InitCode = []byte{}
	fd := op.GetFactoryData()
	if len(fd) != 0 {
		t.Fatalf("got %d bytes, want 0", len(fd))
	}
}

func TestV06GetMaxGasAvailable(t *testing.T) {
	op := mockV06Op()
	mag := op.GetMaxGasAvailable()
	if mag == nil || mag.Cmp(big.NewInt(0)) <= 0 {
		t.Fatalf("got invalid max gas: %s", mag.String())
	}
}

func TestV06GetMaxGasAvailableWithPaymaster(t *testing.T) {
	op := mockV06OpWithPaymaster()
	mag := op.GetMaxGasAvailable()
	// v0.6 applies 3x multiplier when paymaster is present
	expectedMul := big.NewInt(3)
	vg := new(big.Int).Mul(op.VerificationGasLimit, expectedMul)
	want := new(big.Int).Add(vg, new(big.Int).Add(op.PreVerificationGas, op.CallGasLimit))
	if mag.Cmp(want) != 0 {
		t.Fatalf("got %s, want %s", mag.String(), want.String())
	}
}

func TestV06GetMaxPrefund(t *testing.T) {
	op := mockV06Op()
	mp := op.GetMaxPrefund()
	want := new(big.Int).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
	if mp.Cmp(want) != 0 {
		t.Fatalf("got %s, want %s", mp.String(), want.String())
	}
}

func TestV06GetDynamicGasPrice(t *testing.T) {
	bf := big.NewInt(3)
	op := mockV06Op()

	want := big.NewInt(4)
	op.MaxFeePerGas = big.NewInt(4)
	op.MaxPriorityFeePerGas = big.NewInt(3)
	if op.GetDynamicGasPrice(bf).Cmp(want) != 0 {
		t.Fatalf("got %d, want %d", op.GetDynamicGasPrice(bf).Int64(), want.Int64())
	}

	want = big.NewInt(5)
	op.MaxFeePerGas = big.NewInt(5)
	op.MaxPriorityFeePerGas = big.NewInt(2)
	if op.GetDynamicGasPrice(bf).Cmp(want) != 0 {
		t.Fatalf("got %d, want %d", op.GetDynamicGasPrice(bf).Int64(), want.Int64())
	}

	want = big.NewInt(4)
	op.MaxFeePerGas = big.NewInt(6)
	op.MaxPriorityFeePerGas = big.NewInt(1)
	if op.GetDynamicGasPrice(bf).Cmp(want) != 0 {
		t.Fatalf("got %d, want %d", op.GetDynamicGasPrice(bf).Int64(), want.Int64())
	}
}

func TestV06GetDynamicGasPriceNilBF(t *testing.T) {
	op := mockV06Op()
	op.MaxFeePerGas = big.NewInt(4)
	op.MaxPriorityFeePerGas = big.NewInt(3)
	if op.GetDynamicGasPrice(nil).Cmp(op.MaxPriorityFeePerGas) != 0 {
		t.Fatalf("got %d, want %d", op.GetDynamicGasPrice(nil).Int64(), op.MaxPriorityFeePerGas.Int64())
	}
}

func TestV06Pack(t *testing.T) {
	op := mockV06Op()
	packed := op.Pack()
	if len(packed) == 0 {
		t.Fatal("Pack returned empty bytes")
	}
}

func TestV06PackForSignature(t *testing.T) {
	op := mockV06Op()
	packed := op.PackForSignature()
	if len(packed) == 0 {
		t.Fatal("PackForSignature returned empty bytes")
	}
}

func TestV06GetUserOpHash(t *testing.T) {
	op := mockV06Op()
	ep := common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789")
	chainID := big.NewInt(1)
	hash := op.GetUserOpHash(ep, chainID)
	if hash == (common.Hash{}) {
		t.Fatal("GetUserOpHash returned zero hash")
	}
}

func TestV06MarshalJSON(t *testing.T) {
	op := mockV06Op()
	data, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalJSON returned empty bytes")
	}
}

func TestV06MarshalJSONEmptyInitCode(t *testing.T) {
	op := mockV06Op()
	op.InitCode = []byte{}
	data, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalJSON returned empty bytes")
	}
}

func TestV06ToMap(t *testing.T) {
	op := mockV06Op()
	m, err := op.ToMap()
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}
	if m == nil {
		t.Fatal("ToMap returned nil")
	}
	if _, ok := m["sender"]; !ok {
		t.Fatal("ToMap missing sender field")
	}
}

func TestV06NewValidUserOperation(t *testing.T) {
	data := map[string]any{
		"sender":               "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                "0x0",
		"initCode":             "0xe19e9755942bb0bd0cccce25b1742596b8a8250b3bf2c3e700000000000000000000000078d4f01f56b982a3b03c4e127a5d3afa8ebee6860000000000000000000000008b388a082f370d8ac2e2b3997e9151168bd09ff50000000000000000000000000000000000000000000000000000000000000000",
		"callData":             "0x80c5c7d0000000000000000000000000a13d69573f994bf662c2714560c44dd7266fc547000000000000000000000000000000000000000000000000016345785d8a000000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000",
		"callGasLimit":         "0x558c",
		"verificationGasLimit": "0x129727",
		"maxFeePerGas":         "0xa862145e",
		"maxPriorityFeePerGas": "0xa8621440",
		"paymasterAndData":     "0x",
		"preVerificationGas":   "0xc539",
		"signature":            "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
	op, err := v06.New(data)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if op == nil {
		t.Fatal("New returned nil")
	}
}

func TestV06NewInvalidUserOperation(t *testing.T) {
	_, err := v06.New(map[string]any{
		"sender": "0xinvalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid data")
	}
}

func TestV06NewMissingField(t *testing.T) {
	data := map[string]any{
		"sender": "0xa13D69573f994bf662C2714560c44dd7266FC547",
	}
	_, err := v06.New(data)
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestV06NewFloatNonce(t *testing.T) {
	data := map[string]any{
		"sender":               "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                float64(42),
		"initCode":             "0x",
		"callData":             "0x",
		"callGasLimit":         "0x558c",
		"verificationGasLimit": "0x129727",
		"preVerificationGas":   "0xc539",
		"maxFeePerGas":         "0xa862145e",
		"maxPriorityFeePerGas": "0xa8621440",
		"paymasterAndData":     "0x",
		"signature":            "0xa925",
	}
	op, err := v06.New(data)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if op.Nonce.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("got nonce %s, want 42", op.Nonce.String())
	}
}
