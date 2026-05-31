package userop_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestDetectVersionV06(t *testing.T) {
	data := map[string]any{
		"sender":               "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"initCode":             "0x",
		"paymasterAndData":     "0x",
	}
	v := userop.DetectVersion(data)
	if v != userop.V0_6 {
		t.Fatalf("got %s, want %s", v, userop.V0_6)
	}
}

func TestDetectVersionV07(t *testing.T) {
	data := map[string]any{
		"sender":   "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"factory":  "0xe19e9755942bb0bd0cccce25b1742596b8a8250b",
	}
	v := userop.DetectVersion(data)
	if v != userop.V0_7 {
		t.Fatalf("got %s, want %s", v, userop.V0_7)
	}
}

func TestDetectVersionV08(t *testing.T) {
	data := map[string]any{
		"sender":      "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"eip7702Auth": map[string]any{},
	}
	v := userop.DetectVersion(data)
	if v != userop.V0_8 {
		t.Fatalf("got %s, want %s", v, userop.V0_8)
	}
}

func TestDetectVersionV09(t *testing.T) {
	data := map[string]any{
		"sender":             "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"paymasterSignature": "0xcafe",
	}
	v := userop.DetectVersion(data)
	if v != userop.V0_9 {
		t.Fatalf("got %s, want %s", v, userop.V0_9)
	}
}

func TestDetectVersionFromEntryPoint(t *testing.T) {
	tests := []struct {
		addr common.Address
		want userop.Version
	}{
		{common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"), userop.V0_6},
		{common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032"), userop.V0_7},
		{common.HexToAddress("0x4337084d9e255ff0702461cf8895ce9e3b5ff108"), userop.V0_8},
		{common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009"), userop.V0_9},
		{common.HexToAddress("0x1234567890123456789012345678901234567890"), userop.V0_6},
	}

	for _, tt := range tests {
		got := userop.DetectVersionFromEntryPoint(tt.addr)
		if got != tt.want {
			t.Fatalf("got %s, want %s for %s", got, tt.want, tt.addr.Hex())
		}
	}
}

func TestIsVersionSupported(t *testing.T) {
	if !userop.IsVersionSupported("v0.6") {
		t.Fatal("v0.6 should be supported")
	}
	if !userop.IsVersionSupported("v0.7") {
		t.Fatal("v0.7 should be supported")
	}
	if !userop.IsVersionSupported("v0.8") {
		t.Fatal("v0.8 should be supported")
	}
	if !userop.IsVersionSupported("v0.9") {
		t.Fatal("v0.9 should be supported")
	}
	if userop.IsVersionSupported("v0.5") {
		t.Fatal("v0.5 should not be supported")
	}
	if userop.IsVersionSupported("invalid") {
		t.Fatal("invalid should not be supported")
	}
}

func TestNewUserOperationV06(t *testing.T) {
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
	vop, err := userop.NewUserOperation(userop.V0_6, data)
	if err != nil {
		t.Fatalf("NewUserOperation failed: %v", err)
	}
	if vop == nil {
		t.Fatal("NewUserOperation returned nil")
	}
	if vop.Version != userop.V0_6 {
		t.Fatalf("got version %s, want %s", vop.Version, userop.V0_6)
	}
	if vop.Op == nil {
		t.Fatal("Op should not be nil")
	}
}

func TestNewUserOperationV07(t *testing.T) {
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
	vop, err := userop.NewUserOperation(userop.V0_7, data)
	if err != nil {
		t.Fatalf("NewUserOperation failed: %v", err)
	}
	if vop.Version != userop.V0_7 {
		t.Fatalf("got version %s, want %s", vop.Version, userop.V0_7)
	}
}

func TestNewUserOperationV08(t *testing.T) {
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
	vop, err := userop.NewUserOperation(userop.V0_8, data)
	if err != nil {
		t.Fatalf("NewUserOperation failed: %v", err)
	}
	if vop.Version != userop.V0_8 {
		t.Fatalf("got version %s, want %s", vop.Version, userop.V0_8)
	}
}

func TestNewUserOperationV09(t *testing.T) {
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
	vop, err := userop.NewUserOperation(userop.V0_9, data)
	if err != nil {
		t.Fatalf("NewUserOperation failed: %v", err)
	}
	if vop.Version != userop.V0_9 {
		t.Fatalf("got version %s, want %s", vop.Version, userop.V0_9)
	}
}

func TestNewUserOperationInvalidVersion(t *testing.T) {
	_, err := userop.NewUserOperation("invalid", map[string]any{})
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestUserOperationInterfaceMethods(t *testing.T) {
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
	vop, err := userop.NewUserOperation(userop.V0_7, data)
	if err != nil {
		t.Fatalf("NewUserOperation failed: %v", err)
	}

	// Test all interface methods
	op := vop.Op

	pm := op.GetPaymaster()
	if pm != common.HexToAddress("0x") {
		t.Fatalf("unexpected paymaster: %s", pm.Hex())
	}

	f := op.GetFactory()
	if f != common.HexToAddress("0xe19e9755942bb0bd0cccce25b1742596b8a8250b") {
		t.Fatalf("unexpected factory: %s", f.Hex())
	}

	fd := op.GetFactoryData()
	if len(fd) == 0 {
		t.Fatal("factory data should not be empty")
	}

	mag := op.GetMaxGasAvailable()
	if mag == nil || mag.Cmp(big.NewInt(0)) <= 0 {
		t.Fatalf("invalid max gas available: %s", mag)
	}

	mp := op.GetMaxPrefund()
	if mp == nil || mp.Cmp(big.NewInt(0)) <= 0 {
		t.Fatalf("invalid max prefund: %s", mp)
	}

	gp := op.GetDynamicGasPrice(big.NewInt(1))
	if gp == nil || gp.Cmp(big.NewInt(0)) <= 0 {
		t.Fatalf("invalid dynamic gas price: %s", gp)
	}

	packed := op.Pack()
	if len(packed) == 0 {
		t.Fatal("Pack returned empty bytes")
	}

	sigPacked := op.PackForSignature()
	if len(sigPacked) == 0 {
		t.Fatal("PackForSignature returned empty bytes")
	}

	hash := op.GetUserOpHash(common.HexToAddress("0x4337084d9e255ff0702461cf8895ce9e3b5ff108"), big.NewInt(1))
	if hash == (common.Hash{}) {
		t.Fatal("GetUserOpHash returned zero hash")
	}

	jsonData, err := op.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(jsonData) == 0 {
		t.Fatal("MarshalJSON returned empty bytes")
	}

	m, err := op.ToMap()
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}
	if m == nil {
		t.Fatal("ToMap returned nil")
	}
}
