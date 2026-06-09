package userop_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestV06_RootPackage_PrimitiveGetters(t *testing.T) {
	op := testutils.MockValidInitUserOp()
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
	if op.GetEIP7702Auth() != nil {
		t.Fatalf("eip7702 must be nil for v0.6")
	}
	if op.GetPaymasterVerificationGasLimit().Sign() != 0 {
		t.Fatalf("paymaster verification gas limit should be zero for v0.6")
	}
	if op.GetPaymasterPostOpGasLimit().Sign() != 0 {
		t.Fatalf("paymaster post-op gas limit should be zero for v0.6")
	}
	if len(op.GetPaymasterData()) != 0 {
		t.Fatalf("paymaster data should be empty for v0.6")
	}
	if len(op.GetPaymasterSignature()) != 0 {
		t.Fatalf("paymaster signature should be empty for v0.6")
	}
}

func TestV06_RootPackage_FactoryAndPaymasterPacked(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	if op.GetFactory() == (common.Address{}) {
		t.Fatalf("expected non-zero factory from initCode")
	}
	if len(op.GetFactoryData()) == 0 {
		t.Fatalf("expected non-empty factoryData")
	}

	op.PaymasterAndData = append(
		common.HexToAddress("0xc000000000000000000000000000000000000003").Bytes(),
		[]byte{0xaa, 0xbb}...,
	)
	if op.GetPaymaster() == (common.Address{}) {
		t.Fatalf("expected non-zero paymaster from paymasterAndData")
	}

	// Truncated paymasterAndData -> zero paymaster.
	op.PaymasterAndData = []byte{0x01}
	if op.GetPaymaster() != (common.Address{}) {
		t.Fatalf("truncated paymasterAndData should produce zero paymaster")
	}

	// Truncated InitCode -> empty factoryData.
	op.InitCode = []byte{0x01}
	if len(op.GetFactoryData()) != 0 {
		t.Fatalf("truncated InitCode should produce empty factoryData")
	}
}

func TestV06_RootPackage_GetMaxGasAvailable_NoPaymaster(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = []byte{}
	got := op.GetMaxGasAvailable()
	want := new(big.Int).Add(op.VerificationGasLimit, op.PreVerificationGas)
	want.Add(want, op.CallGasLimit)
	if got.Cmp(want) != 0 {
		t.Fatalf("max gas no paymaster: got %s want %s", got, want)
	}
}

func TestV06_RootPackage_GetMaxGasAvailable_WithPaymaster(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = common.HexToAddress("0xc000000000000000000000000000000000000003").Bytes()
	got := op.GetMaxGasAvailable()
	// v0.6 with paymaster -> 3x verificationGasLimit + preVerificationGas + callGasLimit.
	want := new(big.Int).Mul(op.VerificationGasLimit, big.NewInt(3))
	want.Add(want, op.PreVerificationGas)
	want.Add(want, op.CallGasLimit)
	if got.Cmp(want) != 0 {
		t.Fatalf("max gas with paymaster: got %s want %s", got, want)
	}
}

func TestV06_RootPackage_GetMaxPrefund(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	got := op.GetMaxPrefund()
	want := new(big.Int).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
	if got.Cmp(want) != 0 {
		t.Fatalf("max prefund mismatch")
	}
}

func TestV06_RootPackage_PackAndHash(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	if len(op.Pack()) == 0 {
		t.Fatalf("Pack returned empty")
	}
	if len(op.PackForSignature()) == 0 {
		t.Fatalf("PackForSignature returned empty")
	}
	if op.GetUserOpHash(common.Address{}, big.NewInt(1)) == (common.Hash{}) {
		t.Fatalf("hash should be non-zero")
	}
	var ds, th [32]byte
	for i := 0; i < 32; i++ {
		ds[i] = byte(i)
		th[i] = byte(255 - i)
	}
	if op.GetUserOpHashEIP712(ds, th) == (common.Hash{}) {
		t.Fatalf("EIP-712 hash should be non-zero")
	}
}

func TestVersion_DetectVersion_FromFields(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want userop.Version
	}{
		{"v0.6 default", map[string]any{}, userop.V0_6},
		{"v0.7 by factory", map[string]any{"factory": "0x..."}, userop.V0_7},
		{"v0.8 by eip7702Auth", map[string]any{"eip7702Auth": map[string]any{}}, userop.V0_8},
		{"v0.9 by paymasterSignature", map[string]any{"paymasterSignature": "0x..."}, userop.V0_9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userop.DetectVersion(tt.data); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
}

func TestVersion_DetectVersionFromEntryPoint(t *testing.T) {
	cases := map[string]userop.Version{
		"0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789": userop.V0_6,
		"0x0000000071727De22E5E9d8BAf0edAc6f37da032": userop.V0_7,
		"0x4337084D9E255Ff0702461CF8895CE9E3b5Ff108": userop.V0_8,
		"0x433709009B8330FDa32311DF1C2AFA402eD8D009": userop.V0_9,
		"0x000000000000000000000000000000000000dead": userop.V0_6, // unknown -> defaults to v0.6
	}
	for ep, want := range cases {
		got := userop.DetectVersionFromEntryPoint(common.HexToAddress(ep))
		if got != want {
			t.Fatalf("%s: got %s want %s", ep, got, want)
		}
	}
}

func TestVersion_IsVersionSupported(t *testing.T) {
	for _, v := range []string{"v0.6", "v0.7", "v0.8", "v0.9"} {
		if !userop.IsVersionSupported(v) {
			t.Fatalf("%s should be supported", v)
		}
	}
	for _, v := range []string{"", "v0.5", "v1.0", "junk"} {
		if userop.IsVersionSupported(v) {
			t.Fatalf("%s should not be supported", v)
		}
	}
}

func TestVersion_NewUserOperation_AllVersions(t *testing.T) {
	v06data, _ := testutils.MockValidInitUserOp().ToMap()
	if _, err := userop.NewUserOperation(userop.V0_6, v06data); err != nil {
		t.Fatalf("v0.6 NewUserOperation: %v", err)
	}

	v07data := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0xfac0000000000000000000000000000000000001",
		"factoryData":                   "0xab",
		"callData":                      "0x",
		"callGasLimit":                  "0x1",
		"verificationGasLimit":          "0x1",
		"preVerificationGas":            "0x1",
		"maxFeePerGas":                  "0x1",
		"maxPriorityFeePerGas":          "0x1",
		"paymaster":                     "0x0000000000000000000000000000000000000000",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"signature":                     "0x01",
	}
	if vop, err := userop.NewUserOperation(userop.V0_7, v07data); err != nil {
		t.Fatalf("v0.7 NewUserOperation: %v", err)
	} else if vop.Version != userop.V0_7 {
		t.Fatalf("wrong version")
	}

	// Unsupported version errors out.
	if _, err := userop.NewUserOperation(userop.Version("v1.2"), map[string]any{}); err == nil {
		t.Fatalf("expected error for unsupported version")
	}
}
