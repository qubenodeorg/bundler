package v07_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v07"
)

func mockV07Op() *v07.UserOperation {
	return &v07.UserOperation{
		Sender:                        common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                         big.NewInt(0),
		Factory:                       common.HexToAddress("0xe19e9755942bb0bd0cccce25b1742596b8a8250b"),
		FactoryData:                   common.Hex2Bytes("00000000000000000000000078d4f01f56b982a3b03c4e127a5d3afa8ebee6860000000000000000000000008b388a082f370d8ac2e2b3997e9151168bd09ff50000000000000000000000000000000000000000000000000000000000000000"),
		CallData:                      common.Hex2Bytes("80c5c7d0000000000000000000000000a13d69573f994bf662c2714560c44dd7266fc547000000000000000000000000000000000000000000000000016345785d8a000000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000"),
		CallGasLimit:                  big.NewInt(0x558c),
		VerificationGasLimit:          big.NewInt(0x129727),
		PreVerificationGas:            big.NewInt(0xc539),
		MaxFeePerGas:                  big.NewInt(0xa862145e),
		MaxPriorityFeePerGas:          big.NewInt(0xa8621440),
		Paymaster:                     common.HexToAddress("0x"),
		PaymasterVerificationGasLimit: big.NewInt(0),
		PaymasterPostOpGasLimit:       big.NewInt(0),
		PaymasterData:                 []byte{},
		Signature:                     common.Hex2Bytes("a925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b"),
	}
}

func mockV07OpWithPaymaster() *v07.UserOperation {
	op := mockV07Op()
	op.Paymaster = common.HexToAddress("0x1234567890123456789012345678901234567890")
	op.PaymasterVerificationGasLimit = big.NewInt(100000)
	op.PaymasterPostOpGasLimit = big.NewInt(50000)
	op.PaymasterData = common.Hex2Bytes("deadbeef")
	return op
}

func TestV07GetPaymaster(t *testing.T) {
	op := mockV07Op()
	if op.GetPaymaster() != common.HexToAddress("0x") {
		t.Fatalf("got %s, want zero address", op.GetPaymaster().Hex())
	}
}

func TestV07GetPaymasterWithValue(t *testing.T) {
	op := mockV07OpWithPaymaster()
	want := common.HexToAddress("0x1234567890123456789012345678901234567890")
	if op.GetPaymaster() != want {
		t.Fatalf("got %s, want %s", op.GetPaymaster().Hex(), want.Hex())
	}
}

func TestV07GetFactory(t *testing.T) {
	op := mockV07Op()
	want := common.HexToAddress("0xe19e9755942bb0bd0cccce25b1742596b8a8250b")
	if op.GetFactory() != want {
		t.Fatalf("got %s, want %s", op.GetFactory().Hex(), want.Hex())
	}
}

func TestV07GetFactoryData(t *testing.T) {
	op := mockV07Op()
	fd := op.GetFactoryData()
	if len(fd) == 0 {
		t.Fatal("got empty factory data")
	}
}

func TestV07GetMaxGasAvailable(t *testing.T) {
	op := mockV07Op()
	mag := op.GetMaxGasAvailable()
	// v0.7: NO 3x multiplier
	want := new(big.Int).Add(op.VerificationGasLimit, new(big.Int).Add(op.CallGasLimit, op.PreVerificationGas))
	if mag.Cmp(want) != 0 {
		t.Fatalf("got %s, want %s", mag.String(), want.String())
	}
}

func TestV07GetMaxGasAvailableWithPaymaster(t *testing.T) {
	op := mockV07OpWithPaymaster()
	mag := op.GetMaxGasAvailable()
	// v0.7: no 3x multiplier; the explicit paymaster gas limits are added.
	want := new(big.Int).Add(op.VerificationGasLimit, new(big.Int).Add(op.CallGasLimit, op.PreVerificationGas))
	want.Add(want, op.PaymasterVerificationGasLimit)
	want.Add(want, op.PaymasterPostOpGasLimit)
	if mag.Cmp(want) != 0 {
		t.Fatalf("got %s, want %s", mag.String(), want.String())
	}
}

func TestV07GetMaxPrefund(t *testing.T) {
	op := mockV07Op()
	mp := op.GetMaxPrefund()
	want := new(big.Int).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
	if mp.Cmp(want) != 0 {
		t.Fatalf("got %s, want %s", mp.String(), want.String())
	}
}

func TestV07GetDynamicGasPrice(t *testing.T) {
	bf := big.NewInt(3)
	op := mockV07Op()

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
}

func TestV07GetDynamicGasPriceNilBF(t *testing.T) {
	op := mockV07Op()
	op.MaxFeePerGas = big.NewInt(4)
	op.MaxPriorityFeePerGas = big.NewInt(3)
	if op.GetDynamicGasPrice(nil).Cmp(op.MaxPriorityFeePerGas) != 0 {
		t.Fatalf("got %d, want %d", op.GetDynamicGasPrice(nil).Int64(), op.MaxPriorityFeePerGas.Int64())
	}
}

func TestV07Pack(t *testing.T) {
	op := mockV07Op()
	packed := op.Pack()
	if len(packed) == 0 {
		t.Fatal("Pack returned empty bytes")
	}
}

func TestV07PackWithPaymaster(t *testing.T) {
	op := mockV07OpWithPaymaster()
	packed := op.Pack()
	if len(packed) == 0 {
		t.Fatal("Pack returned empty bytes")
	}
}

func TestV07PackForSignature(t *testing.T) {
	op := mockV07Op()
	packed := op.PackForSignature()
	if len(packed) == 0 {
		t.Fatal("PackForSignature returned empty bytes")
	}
}

func TestV07GetUserOpHash(t *testing.T) {
	op := mockV07Op()
	ep := common.HexToAddress("0x4337084d9e255ff0702461cf8895ce9e3b5ff108")
	chainID := big.NewInt(1)
	hash := op.GetUserOpHash(ep, chainID)
	if hash == (common.Hash{}) {
		t.Fatal("GetUserOpHash returned zero hash")
	}
}

func TestV07MarshalJSON(t *testing.T) {
	op := mockV07Op()
	data, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalJSON returned empty bytes")
	}
}

func TestV07ToMap(t *testing.T) {
	op := mockV07Op()
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
	if _, ok := m["factory"]; !ok {
		t.Fatal("ToMap missing factory field")
	}
}

func TestV07NewValid(t *testing.T) {
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
	op, err := v07.New(data)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if op == nil {
		t.Fatal("New returned nil")
	}
}

func TestV07NewInvalid(t *testing.T) {
	_, err := v07.New(map[string]any{
		"sender": "0xinvalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid data")
	}
}

func TestV07NewMissingField(t *testing.T) {
	_, err := v07.New(map[string]any{
		"sender": "0xa13D69573f994bf662C2714560c44dd7266FC547",
	})
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}
