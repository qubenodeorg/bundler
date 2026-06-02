package simulation

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	uov07 "github.com/zarbanio/bundler/pkg/userop/v07"
)

func TestSimulateInputStruct(t *testing.T) {
	in := &SimulateInput{
		Rpc:        nil,
		EntryPoint: common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032"),
		Op:         nil,
		ChainID:    big.NewInt(1),
	}
	if !strings.EqualFold(in.EntryPoint.Hex(), "0x0000000071727de22e5e9d8baf0edac6f37da032") {
		t.Fatalf("SimulateInput.EntryPoint: got %s", in.EntryPoint.Hex())
	}
	if in.ChainID.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("SimulateInput.ChainID: got %d", in.ChainID)
	}
}

func TestSimulateOutputStruct(t *testing.T) {
	out := &SimulateOutput{
		Result: nil,
	}
	if out.Result != nil {
		t.Fatal("SimulateOutput.Result should be nil by default")
	}
}

func TestToPackedOpFromInterfaceV07(t *testing.T) {
	op := &uov07.UserOperation{
		Sender:                        common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                         big.NewInt(0),
		Factory:                       common.HexToAddress("0x0000000000000000000000000000000000000000"),
		FactoryData:                   []byte{},
		CallData:                      common.Hex2Bytes("80c5c7d0"),
		CallGasLimit:                  big.NewInt(30000),
		VerificationGasLimit:          big.NewInt(100000),
		PreVerificationGas:            big.NewInt(50000),
		MaxFeePerGas:                  big.NewInt(2000000000),
		MaxPriorityFeePerGas:          big.NewInt(1000000000),
		Paymaster:                     common.HexToAddress("0x0000000000000000000000000000000000000000"),
		PaymasterVerificationGasLimit: big.NewInt(0),
		PaymasterPostOpGasLimit:       big.NewInt(0),
		PaymasterData:                 []byte{},
		Signature:                     common.Hex2Bytes("a925dcc5"),
	}

	packed, err := toPackedOpFromInterface(op)
	if err != nil {
		t.Fatalf("toPackedOpFromInterface: %v", err)
	}
	if packed.Sender != op.Sender {
		t.Fatalf("packed.Sender: got %s, want %s", packed.Sender.Hex(), op.Sender.Hex())
	}
	if packed.Nonce.Cmp(op.Nonce) != 0 {
		t.Fatalf("packed.Nonce: got %d, want %d", packed.Nonce, op.Nonce)
	}
}

func TestToPackedOpFromInterfaceViaMap(t *testing.T) {
	data := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0x0000000000000000000000000000000000000000",
		"factoryData":                   "0x",
		"callData":                      "0x80c5c7d0",
		"callGasLimit":                  "0x7530",
		"verificationGasLimit":          "0x186a0",
		"preVerificationGas":            "0xc350",
		"maxFeePerGas":                  "0x77359400",
		"maxPriorityFeePerGas":          "0x3b9aca00",
		"paymaster":                     "0x0000000000000000000000000000000000000000",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"signature":                     "0xa925dcc5",
	}

	op, err := uov07.New(data)
	if err != nil {
		t.Fatalf("uov07.New: %v", err)
	}

	packed, err := toPackedOpFromInterface(op)
	if err != nil {
		t.Fatalf("toPackedOpFromInterface: %v", err)
	}
	if packed.Sender == (common.Address{}) {
		t.Fatal("packed.Sender should not be zero")
	}
	if packed.Nonce == nil {
		t.Fatal("packed.Nonce should not be nil")
	}
}
