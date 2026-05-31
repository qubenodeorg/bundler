package v08_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v08"
)

func mockV08Op() *v08.UserOperation {
	return &v08.UserOperation{
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

func mockV08OpWithAuth() *v08.UserOperation {
	op := mockV08Op()
	op.EIP7702Auth = &v08.Authorization{
		ChainID: big.NewInt(1),
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Nonce:   big.NewInt(0),
		YParity: 27,
		R:       big.NewInt(12345),
		S:       big.NewInt(67890),
	}
	return op
}

func TestV08GetPaymaster(t *testing.T) {
	op := mockV08Op()
	if op.GetPaymaster() != common.HexToAddress("0x") {
		t.Fatalf("got %s, want zero address", op.GetPaymaster().Hex())
	}
}

func TestV08GetFactory(t *testing.T) {
	op := mockV08Op()
	want := common.HexToAddress("0xe19e9755942bb0bd0cccce25b1742596b8a8250b")
	if op.GetFactory() != want {
		t.Fatalf("got %s, want %s", op.GetFactory().Hex(), want.Hex())
	}
}

func TestV08GetMaxGasAvailable(t *testing.T) {
	op := mockV08Op()
	mag := op.GetMaxGasAvailable()
	want := new(big.Int).Add(op.VerificationGasLimit, new(big.Int).Add(op.CallGasLimit, op.PreVerificationGas))
	if mag.Cmp(want) != 0 {
		t.Fatalf("got %s, want %s", mag.String(), want.String())
	}
}

func TestV08GetDynamicGasPrice(t *testing.T) {
	bf := big.NewInt(3)
	op := mockV08Op()
	want := big.NewInt(4)
	op.MaxFeePerGas = big.NewInt(4)
	op.MaxPriorityFeePerGas = big.NewInt(3)
	if op.GetDynamicGasPrice(bf).Cmp(want) != 0 {
		t.Fatalf("got %d, want %d", op.GetDynamicGasPrice(bf).Int64(), want.Int64())
	}
}

func TestV08Pack(t *testing.T) {
	op := mockV08Op()
	packed := op.Pack()
	if len(packed) == 0 {
		t.Fatal("Pack returned empty bytes")
	}
}

func TestV08PackWithAuth(t *testing.T) {
	op := mockV08OpWithAuth()
	packed := op.Pack()
	if len(packed) == 0 {
		t.Fatal("Pack returned empty bytes")
	}
}

func TestV08GetUserOpHash(t *testing.T) {
	op := mockV08Op()
	ep := common.HexToAddress("0x4337084d9e255ff0702461cf8895ce9e3b5ff108")
	chainID := big.NewInt(1)
	hash := op.GetUserOpHash(ep, chainID)
	if hash == (common.Hash{}) {
		t.Fatal("GetUserOpHash returned zero hash")
	}
}

func TestV08MarshalJSON(t *testing.T) {
	op := mockV08OpWithAuth()
	data, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalJSON returned empty bytes")
	}
}

func TestV08MarshalJSONNoAuth(t *testing.T) {
	op := mockV08Op()
	data, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalJSON returned empty bytes")
	}
}

func TestV08ToMap(t *testing.T) {
	op := mockV08OpWithAuth()
	m, err := op.ToMap()
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}
	if m == nil {
		t.Fatal("ToMap returned nil")
	}
}

func TestV08NewValid(t *testing.T) {
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
	op, err := v08.New(data)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if op == nil {
		t.Fatal("New returned nil")
	}
}

func TestV08NewWithAuth(t *testing.T) {
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
	op, err := v08.New(data)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if op.EIP7702Auth == nil {
		t.Fatal("EIP7702Auth should not be nil")
	}
}

func TestV08NewInvalid(t *testing.T) {
	_, err := v08.New(map[string]any{
		"sender": "0xinvalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid data")
	}
}
