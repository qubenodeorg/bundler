package simulation

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop"
	uov08 "github.com/zarbanio/bundler/pkg/userop/v08"
)

func TestSimulateInputStructV08(t *testing.T) {
	in := &SimulateInput{
		Rpc:        nil,
		EntryPoint: common.HexToAddress("0x4337084d9e255ff0702461cf8895ce9e3b5ff108"),
		Op:         nil,
		ChainID:    big.NewInt(1),
	}
	if !strings.EqualFold(in.EntryPoint.Hex(), "0x4337084d9e255ff0702461cf8895ce9e3b5ff108") {
		t.Fatalf("SimulateInput.EntryPoint: got %s", in.EntryPoint.Hex())
	}
	if in.ChainID.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("SimulateInput.ChainID: got %d", in.ChainID)
	}
}

func TestSimulateOutputStructV08(t *testing.T) {
	out := &SimulateOutput{
		Result: nil,
	}
	if out.Result != nil {
		t.Fatal("SimulateOutput.Result should be nil by default")
	}
}

func TestToPackedOpFromInterfaceNilV08(t *testing.T) {
	_, err := toPackedOpFromInterface(nil)
	if err == nil {
		t.Fatal("toPackedOpFromInterface(nil): got nil, want error")
	}
}

func TestToPackedOpFromInterfaceTypedNilV08(t *testing.T) {
	var op *uov08.UserOperation
	var iface userop.UserOperationInterface = op
	_, err := toPackedOpFromInterface(iface)
	if err == nil {
		t.Fatal("toPackedOpFromInterface(typed nil): got nil, want error")
	}
}

func TestSimulateValidationNilInputV08(t *testing.T) {
	_, err := SimulateValidation(nil)
	if err == nil {
		t.Fatal("SimulateValidation(nil): got nil, want error")
	}
}

func TestSimulateValidationNilRpcV08(t *testing.T) {
	_, err := SimulateValidation(&SimulateInput{
		Rpc:        nil,
		EntryPoint: common.HexToAddress("0x4337084d9e255ff0702461cf8895ce9e3b5ff108"),
		Op:         &uov08.UserOperation{},
		ChainID:    big.NewInt(1),
	})
	if err == nil {
		t.Fatal("SimulateValidation(nil Rpc): got nil, want error")
	}
}

func TestToPackedOpFromInterfaceV08(t *testing.T) {
	op := &uov08.UserOperation{
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

func TestToPackedOpFromInterfaceViaMapV08(t *testing.T) {
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

	op, err := uov08.New(data)
	if err != nil {
		t.Fatalf("uov08.New: %v", err)
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
