package userop

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v07"
	"github.com/zarbanio/bundler/pkg/userop/v08"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

func sampleV07() *v07.UserOperation {
	return &v07.UserOperation{
		Sender:                        common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                         big.NewInt(42),
		Factory:                       common.HexToAddress("0xfac0000000000000000000000000000000000001"),
		FactoryData:                   []byte{0xfa, 0xc1},
		CallData:                      []byte{0xca, 0x11},
		CallGasLimit:                  big.NewInt(100000),
		VerificationGasLimit:          big.NewInt(200000),
		PreVerificationGas:            big.NewInt(50000),
		MaxFeePerGas:                  big.NewInt(2_000_000_000),
		MaxPriorityFeePerGas:          big.NewInt(1_000_000_000),
		Paymaster:                     common.HexToAddress("0xb000000000000000000000000000000000000002"),
		PaymasterVerificationGasLimit: big.NewInt(0x111111),
		PaymasterPostOpGasLimit:       big.NewInt(0x222222),
		PaymasterData:                 []byte{0xde, 0xad, 0xbe, 0xef},
		Signature:                     []byte{0x01, 0x02, 0x03},
	}
}

func TestConvertToV06Packed_V06Passthrough(t *testing.T) {
	in := &UserOperation{
		Sender:               common.HexToAddress("0xaa"),
		Nonce:                big.NewInt(1),
		InitCode:             []byte{0x01},
		CallData:             []byte{0x02},
		CallGasLimit:         big.NewInt(1),
		VerificationGasLimit: big.NewInt(1),
		PreVerificationGas:   big.NewInt(1),
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
		PaymasterAndData:     []byte{0x03},
		Signature:            []byte{0x04},
	}
	out, err := ConvertToV06Packed(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != in {
		t.Fatalf("v0.6 input should be returned as-is")
	}
}

func TestConvertToV06Packed_V07RoundTrip(t *testing.T) {
	op := sampleV07()
	out, err := ConvertToV06Packed(op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Sender != op.Sender {
		t.Fatalf("sender mismatch")
	}
	if out.Nonce.Cmp(op.Nonce) != 0 {
		t.Fatalf("nonce mismatch")
	}
	if out.CallGasLimit.Cmp(op.CallGasLimit) != 0 {
		t.Fatalf("callGasLimit mismatch")
	}
	if !bytes.Equal(out.CallData, op.CallData) {
		t.Fatalf("callData mismatch")
	}
	if !bytes.Equal(out.Signature, op.Signature) {
		t.Fatalf("signature mismatch")
	}

	// InitCode must match factory packed format used by v0.7.
	if !bytes.Equal(out.InitCode, op.PackInitCode()) {
		t.Fatalf("initCode mismatch: got %x, want %x", out.InitCode, op.PackInitCode())
	}

	// PaymasterAndData must match the v0.7 packed format including paymaster gas limits.
	if !bytes.Equal(out.PaymasterAndData, op.PackPaymasterAndData()) {
		t.Fatalf("paymasterAndData mismatch: got %x, want %x", out.PaymasterAndData, op.PackPaymasterAndData())
	}

	// Sender derived from packed initCode should equal the factory.
	if out.GetFactory() != op.Factory {
		t.Fatalf("factory recovered from initCode mismatch: got %s, want %s", out.GetFactory(), op.Factory)
	}
	// Paymaster derived from packed paymasterAndData should equal the v0.7 paymaster.
	if out.GetPaymaster() != op.Paymaster {
		t.Fatalf("paymaster recovered from paymasterAndData mismatch: got %s, want %s", out.GetPaymaster(), op.Paymaster)
	}
}

func TestConvertToV06Packed_PaymasterGasLimitsPreserved(t *testing.T) {
	op := sampleV07()
	out, err := ConvertToV06Packed(op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Paymaster gas limits are 32 bytes total starting at offset 20.
	pndGasLimits := out.PaymasterAndData[common.AddressLength : common.AddressLength+32]
	pvg := new(big.Int).SetBytes(pndGasLimits[:16])
	ppg := new(big.Int).SetBytes(pndGasLimits[16:])
	if pvg.Cmp(op.PaymasterVerificationGasLimit) != 0 {
		t.Fatalf("paymasterVerificationGasLimit mismatch: got %s, want %s", pvg, op.PaymasterVerificationGasLimit)
	}
	if ppg.Cmp(op.PaymasterPostOpGasLimit) != 0 {
		t.Fatalf("paymasterPostOpGasLimit mismatch: got %s, want %s", ppg, op.PaymasterPostOpGasLimit)
	}
}

func TestConvertToV06Packed_V08WithEIP7702Auth(t *testing.T) {
	op := &v08.UserOperation{
		Sender:                        common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                         big.NewInt(7),
		Factory:                       common.Address{},
		FactoryData:                   []byte{},
		CallData:                      []byte{0xab, 0xcd},
		CallGasLimit:                  big.NewInt(100000),
		VerificationGasLimit:          big.NewInt(200000),
		PreVerificationGas:            big.NewInt(50000),
		MaxFeePerGas:                  big.NewInt(1_000_000_000),
		MaxPriorityFeePerGas:          big.NewInt(500_000_000),
		Paymaster:                     common.HexToAddress("0xb000000000000000000000000000000000000002"),
		PaymasterVerificationGasLimit: big.NewInt(0xaaaa),
		PaymasterPostOpGasLimit:       big.NewInt(0xbbbb),
		PaymasterData:                 []byte{0xfe, 0xed},
		EIP7702Auth: &v08.Authorization{
			ChainID: big.NewInt(1),
			Address: common.HexToAddress("0xc000000000000000000000000000000000000003"),
			Nonce:   big.NewInt(0),
			YParity: 1,
			R:       big.NewInt(1),
			S:       big.NewInt(2),
		},
		Signature: []byte{0x11, 0x22},
	}

	out, err := ConvertToV06Packed(op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty factory -> empty InitCode.
	if len(out.InitCode) != 0 {
		t.Fatalf("expected empty initCode, got %x", out.InitCode)
	}
	// PaymasterAndData should pack the v0.8 format.
	if !bytes.Equal(out.PaymasterAndData, op.PackPaymasterAndData()) {
		t.Fatalf("paymasterAndData mismatch: got %x, want %x", out.PaymasterAndData, op.PackPaymasterAndData())
	}
}

func TestConvertToV06Packed_V09WithPaymasterSignature(t *testing.T) {
	op := &v09.UserOperation{
		Sender:                        common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                         big.NewInt(0),
		Factory:                       common.HexToAddress("0xfac0000000000000000000000000000000000001"),
		FactoryData:                   []byte{0xfa, 0xc1},
		CallData:                      []byte{},
		CallGasLimit:                  big.NewInt(100000),
		VerificationGasLimit:          big.NewInt(100000),
		PreVerificationGas:            big.NewInt(50000),
		MaxFeePerGas:                  big.NewInt(1_000_000_000),
		MaxPriorityFeePerGas:          big.NewInt(500_000_000),
		Paymaster:                     common.HexToAddress("0xb000000000000000000000000000000000000002"),
		PaymasterVerificationGasLimit: big.NewInt(50000),
		PaymasterPostOpGasLimit:       big.NewInt(50000),
		PaymasterData:                 []byte{0xfe, 0xed},
		PaymasterSignature:            []byte{0xca, 0xfe, 0xba, 0xbe},
		Signature:                     []byte{0xab},
	}

	out, err := ConvertToV06Packed(op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Paymaster gas limits must be preserved in the converted packed form.
	if !bytes.Equal(out.PaymasterAndData, op.PackPaymasterAndData()) {
		t.Fatalf("paymasterAndData mismatch")
	}
	// v0.9-specific paymasterSignature is intentionally not embedded in the converted op.
	// Callers that need it must read it from the original UserOperationInterface.
}

func TestConvertToV06Packed_NoPaymasterNoFactory(t *testing.T) {
	op := &v07.UserOperation{
		Sender:                        common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                         big.NewInt(0),
		Factory:                       common.Address{},
		FactoryData:                   []byte{},
		CallData:                      []byte{0xff},
		CallGasLimit:                  big.NewInt(100000),
		VerificationGasLimit:          big.NewInt(100000),
		PreVerificationGas:            big.NewInt(50000),
		MaxFeePerGas:                  big.NewInt(1),
		MaxPriorityFeePerGas:          big.NewInt(1),
		Paymaster:                     common.Address{},
		PaymasterVerificationGasLimit: big.NewInt(0),
		PaymasterPostOpGasLimit:       big.NewInt(0),
		PaymasterData:                 []byte{},
		Signature:                     []byte{0xab},
	}

	out, err := ConvertToV06Packed(op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.InitCode) != 0 {
		t.Fatalf("expected empty initCode, got %x", out.InitCode)
	}
	if len(out.PaymasterAndData) != 0 {
		t.Fatalf("expected empty paymasterAndData, got %x", out.PaymasterAndData)
	}
}
