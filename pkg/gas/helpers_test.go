package gas

import (
	"errors"
	"math/big"
	"testing"

	"github.com/zarbanio/bundler/pkg/userop"
)

func TestReadBigStr_Hex(t *testing.T) {
	data := map[string]any{"x": "0x10"}
	got := readBigStr(data, "x")
	if got.Int64() != 16 {
		t.Fatalf("expected 16, got %s", got)
	}
}

func TestReadBigStr_Missing(t *testing.T) {
	got := readBigStr(map[string]any{}, "x")
	if got.Sign() != 0 {
		t.Fatalf("expected 0 for missing key, got %s", got)
	}
}

func TestReadBigStr_NonString(t *testing.T) {
	got := readBigStr(map[string]any{"x": 42}, "x")
	if got.Sign() != 0 {
		t.Fatalf("expected 0 for non-string value, got %s", got)
	}
}

func TestCreateOp_V06(t *testing.T) {
	op, err := userop.New(map[string]any{
		"sender":               "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                "0x0",
		"initCode":             "0x",
		"callData":             "0x",
		"callGasLimit":         "0x1",
		"verificationGasLimit": "0x1",
		"preVerificationGas":   "0x1",
		"maxFeePerGas":         "0x1",
		"maxPriorityFeePerGas": "0x1",
		"paymasterAndData":     "0x",
		"signature":            "0x01",
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	data, _ := op.ToMap()
	got, err := createOp(data, userop.V0_6)
	if err != nil {
		t.Fatalf("createOp v0.6: %v", err)
	}
	if got.GetSender() != op.GetSender() {
		t.Fatalf("sender mismatch after createOp")
	}
}

func TestCreateOp_V07(t *testing.T) {
	data := map[string]any{
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
	op, err := createOp(data, userop.V0_7)
	if err != nil {
		t.Fatalf("createOp v0.7: %v", err)
	}
	if op == nil {
		t.Fatal("nil op")
	}
}

func TestCreateOp_UnsupportedVersion(t *testing.T) {
	_, err := createOp(map[string]any{}, userop.Version("v1.2"))
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestRetryEstimateGas_NonRetryable(t *testing.T) {
	in := &EstimateInput{
		Version:     userop.V0_6,
		MaxGasLimit: big.NewInt(1_000_000),
	}
	_, _, err := retryEstimateGas(errors.New("unrelated"), 0, in)
	if err == nil {
		t.Fatal("expected non-retryable error to be propagated")
	}
}

func TestRetryEstimateGas_RetryLimitReached(t *testing.T) {
	in := &EstimateInput{
		Version:     userop.V0_6,
		MaxGasLimit: big.NewInt(1_000_000),
		attempts:    maxRetries, // exactly at limit -> should not retry
	}
	_, _, err := retryEstimateGas(errors.New("AA40 over verificationGasLimit"), 0, in)
	if err == nil {
		t.Fatal("expected attempts==maxRetries to halt retry")
	}
}

func TestNewDefaultOverhead_VersionFields(t *testing.T) {
	if NewDefaultOverhead().GetVersion() != userop.V0_6 {
		t.Fatalf("default overhead should be v0.6")
	}
	if NewDefaultOverheadV07().GetVersion() != userop.V0_7 {
		t.Fatalf("v0.7 overhead version mismatch")
	}
	if NewDefaultOverheadV08().GetVersion() != userop.V0_8 {
		t.Fatalf("v0.8 overhead version mismatch")
	}
	if NewDefaultOverheadV09().GetVersion() != userop.V0_9 {
		t.Fatalf("v0.9 overhead version mismatch")
	}
}

func TestNewDefaultOverheadV09_HasPenaltyThreshold(t *testing.T) {
	ov := NewDefaultOverheadV09()
	// Below threshold (default 40,000) -> no penalty regardless of pct.
	if got := ov.CalcUnusedGasPenalty(big.NewInt(1000)); got.Sign() != 0 {
		t.Fatalf("expected zero penalty below threshold, got %s", got)
	}
	// Above threshold -> 10% of value applied.
	if got := ov.CalcUnusedGasPenalty(big.NewInt(100_000)); got.Sign() == 0 {
		t.Fatal("expected non-zero penalty above threshold")
	}
}
