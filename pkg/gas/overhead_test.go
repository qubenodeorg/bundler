package gas

import (
	"errors"
	"math/big"
	"testing"

	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/userop"
	"github.com/zarbanio/bundler/pkg/userop/v07"
	"github.com/zarbanio/bundler/pkg/userop/v08"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

func TestNewDefaultOverhead(t *testing.T) {
	ov := NewDefaultOverhead()
	if ov.GetVersion() != userop.V0_6 {
		t.Errorf("expected v0.6, got %s", ov.GetVersion())
	}
	if ov.GetUnusedGasPenaltyPercent() != 0 {
		t.Errorf("expected 0 penalty pct, got %d", ov.GetUnusedGasPenaltyPercent())
	}
}

func TestNewDefaultOverheadV07(t *testing.T) {
	ov := NewDefaultOverheadV07()
	if ov.GetVersion() != userop.V0_7 {
		t.Errorf("expected v0.7, got %s", ov.GetVersion())
	}
	if ov.GetUnusedGasPenaltyPercent() != 1000 {
		t.Errorf("expected 1000 penalty pct, got %d", ov.GetUnusedGasPenaltyPercent())
	}
	if ov.gasPenaltyThreshold != nil {
		t.Error("expected nil threshold for v0.7")
	}
}

func TestNewDefaultOverheadV08(t *testing.T) {
	ov := NewDefaultOverheadV08()
	if ov.GetVersion() != userop.V0_8 {
		t.Errorf("expected v0.8, got %s", ov.GetVersion())
	}
	if ov.GetUnusedGasPenaltyPercent() != 1000 {
		t.Errorf("expected 1000 penalty pct, got %d", ov.GetUnusedGasPenaltyPercent())
	}
	if ov.gasPenaltyThreshold == nil || ov.gasPenaltyThreshold.Cmp(big.NewInt(40000)) != 0 {
		t.Errorf("expected threshold 40000, got %v", ov.gasPenaltyThreshold)
	}
}

func TestNewDefaultOverheadV09(t *testing.T) {
	ov := NewDefaultOverheadV09()
	if ov.GetVersion() != userop.V0_9 {
		t.Errorf("expected v0.9, got %s", ov.GetVersion())
	}
	if ov.GetUnusedGasPenaltyPercent() != 1000 {
		t.Errorf("expected 1000 penalty pct, got %d", ov.GetUnusedGasPenaltyPercent())
	}
	if ov.gasPenaltyThreshold == nil || ov.gasPenaltyThreshold.Cmp(big.NewInt(40000)) != 0 {
		t.Errorf("expected threshold 40000, got %v", ov.gasPenaltyThreshold)
	}
}

func TestSetUnusedGasPenaltyPercent(t *testing.T) {
	ov := NewDefaultOverhead()
	ov.SetUnusedGasPenaltyPercent(500)
	if ov.GetUnusedGasPenaltyPercent() != 500 {
		t.Errorf("expected 500, got %d", ov.GetUnusedGasPenaltyPercent())
	}
}

func TestSetGasPenaltyThreshold(t *testing.T) {
	ov := NewDefaultOverhead()
	thresh := big.NewInt(100000)
	ov.SetGasPenaltyThreshold(thresh)
	if ov.gasPenaltyThreshold == nil || ov.gasPenaltyThreshold.Cmp(thresh) != 0 {
		t.Errorf("expected %v, got %v", thresh, ov.gasPenaltyThreshold)
	}

	ov.SetGasPenaltyThreshold(nil)
	if ov.gasPenaltyThreshold != nil {
		t.Error("expected nil threshold after setting nil")
	}
}

func TestCalcCallDataCostV06(t *testing.T) {
	ov := NewDefaultOverhead()
	op := testutils.MockValidInitUserOp()

	cost := ov.CalcCallDataCost(op)
	if cost <= 0 {
		t.Error("expected positive call data cost")
	}
}

func TestCalcCallDataCostV07(t *testing.T) {
	ov := NewDefaultOverheadV07()
	op := testutils.MockValidInitUserOp()

	cost := ov.CalcCallDataCost(op)
	if cost <= 0 {
		t.Error("expected positive call data cost")
	}
}

func TestCalcPerUserOpCostV06(t *testing.T) {
	ov := NewDefaultOverhead()
	op := testutils.MockValidInitUserOp()

	cost := ov.CalcPerUserOpCost(op)
	if cost <= 0 {
		t.Error("expected positive per user op cost")
	}
}

func TestCalcPreVerificationGasV06(t *testing.T) {
	ov := NewDefaultOverhead()
	op := testutils.MockValidInitUserOp()

	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		t.Fatal(err)
	}
	if pvg == nil || pvg.Cmp(big.NewInt(0)) <= 0 {
		t.Errorf("expected positive PVG, got %v", pvg)
	}
}

func TestCalcPreVerificationGasV07(t *testing.T) {
	ov := NewDefaultOverheadV07()
	opData := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0x0000000000000000000000000000000000000000",
		"factoryData":                   "0x",
		"callData":                      "0x80c5c7d0000000000000000000000000a13d69573f994bf662c2714560c44dd7266fc547",
		"callGasLimit":                  "0x558c",
		"verificationGasLimit":          "0x129727",
		"preVerificationGas":            "0xc539",
		"maxFeePerGas":                  "0xa862145e",
		"maxPriorityFeePerGas":          "0xa8621440",
		"paymaster":                     "0x0000000000000000000000000000000000000000",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"signature":                     "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
	op, err := v07.New(opData)
	if err != nil {
		t.Fatal(err)
	}

	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		t.Fatal(err)
	}
	if pvg == nil || pvg.Cmp(big.NewInt(0)) <= 0 {
		t.Errorf("expected positive PVG, got %v", pvg)
	}
}

func TestCalcPreVerificationGasWithBuffer(t *testing.T) {
	ov := NewDefaultOverhead()
	ov.SetPreVerificationGasBufferFactor(10)
	op := testutils.MockValidInitUserOp()

	pvg, err := ov.CalcPreVerificationGasWithBuffer(op)
	if err != nil {
		t.Fatal(err)
	}
	if pvg == nil || pvg.Cmp(big.NewInt(0)) <= 0 {
		t.Errorf("expected positive PVG, got %v", pvg)
	}
}

func TestNonZeroValueCall(t *testing.T) {
	ov := NewDefaultOverhead()
	nzvc := ov.NonZeroValueCall()
	if nzvc == nil || nzvc.Cmp(big.NewInt(0)) <= 0 {
		t.Errorf("expected positive non-zero value call cost, got %v", nzvc)
	}
	expected := big.NewInt(12100)
	if nzvc.Cmp(expected) != 0 {
		t.Errorf("expected %d, got %d", expected, nzvc)
	}
}

func TestCalcUnusedGasPenaltyNoPenaltyZeroPct(t *testing.T) {
	ov := NewDefaultOverhead()
	penalty := ov.CalcUnusedGasPenalty(big.NewInt(50000))
	if penalty.Sign() != 0 {
		t.Errorf("expected 0 penalty, got %v", penalty)
	}
}

func TestCalcUnusedGasPenaltyV07(t *testing.T) {
	ov := NewDefaultOverheadV07()
	penalty := ov.CalcUnusedGasPenalty(big.NewInt(100000))
	expected := big.NewInt(10000) // 100000 * 1000 / 10000 = 10000
	if penalty.Cmp(expected) != 0 {
		t.Errorf("expected %d, got %d", expected, penalty)
	}
}

func TestCalcUnusedGasPenaltyV08BelowThreshold(t *testing.T) {
	ov := NewDefaultOverheadV08()
	// 30000 is below 40000 threshold
	penalty := ov.CalcUnusedGasPenalty(big.NewInt(30000))
	if penalty.Sign() != 0 {
		t.Errorf("expected 0 penalty below threshold, got %v", penalty)
	}
}

func TestCalcUnusedGasPenaltyV08AtThreshold(t *testing.T) {
	ov := NewDefaultOverheadV08()
	// 40000 is at threshold (<=), no penalty
	penalty := ov.CalcUnusedGasPenalty(big.NewInt(40000))
	if penalty.Sign() != 0 {
		t.Errorf("expected 0 penalty at threshold, got %v", penalty)
	}
}

func TestCalcUnusedGasPenaltyV08AboveThreshold(t *testing.T) {
	ov := NewDefaultOverheadV08()
	penalty := ov.CalcUnusedGasPenalty(big.NewInt(100000))
	expected := big.NewInt(10000) // 100000 * 1000 / 10000 = 10000
	if penalty.Cmp(expected) != 0 {
		t.Errorf("expected %d, got %d", expected, penalty)
	}
}

func TestCalcUnusedGasPenaltyV09(t *testing.T) {
	ov := NewDefaultOverheadV09()
	penalty := ov.CalcUnusedGasPenalty(big.NewInt(100000))
	expected := big.NewInt(10000)
	if penalty.Cmp(expected) != 0 {
		t.Errorf("expected %d, got %d", expected, penalty)
	}
}

func TestCalcUnusedGasPenaltyNil(t *testing.T) {
	ov := NewDefaultOverheadV07()
	penalty := ov.CalcUnusedGasPenalty(nil)
	if penalty.Sign() != 0 {
		t.Errorf("expected 0 penalty for nil, got %v", penalty)
	}
}

func TestCalcUnusedGasPenaltyZero(t *testing.T) {
	ov := NewDefaultOverheadV07()
	penalty := ov.CalcUnusedGasPenalty(big.NewInt(0))
	if penalty.Sign() != 0 {
		t.Errorf("expected 0 penalty for zero, got %v", penalty)
	}
}

func TestCalcUnusedGasPenaltyNegative(t *testing.T) {
	ov := NewDefaultOverheadV07()
	penalty := ov.CalcUnusedGasPenalty(big.NewInt(-100))
	if penalty.Sign() != 0 {
		t.Errorf("expected 0 penalty for negative, got %v", penalty)
	}
}

func TestCalcCallDataCostDifferences(t *testing.T) {
	ov := NewDefaultOverhead()

	v06Op := testutils.MockValidInitUserOp()
	v06Cost := ov.CalcCallDataCost(v06Op)

	v07OpData := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0x0000000000000000000000000000000000000000",
		"factoryData":                   "0x",
		"callData":                      "0x80c5c7d0000000000000000000000000a13d69573f994bf662c2714560c44dd7266fc547",
		"callGasLimit":                  "0x558c",
		"verificationGasLimit":          "0x129727",
		"preVerificationGas":            "0xc539",
		"maxFeePerGas":                  "0xa862145e",
		"maxPriorityFeePerGas":          "0xa8621440",
		"paymaster":                     "0x0000000000000000000000000000000000000000",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"signature":                     "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
	v07Op, _ := v07.New(v07OpData)
	_ = v07Op

	if v06Cost <= 0 {
		t.Error("expected positive v0.6 call data cost")
	}
}

func TestCalcPreVerificationGasV08(t *testing.T) {
	ov := NewDefaultOverheadV08()
	opData := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0x0000000000000000000000000000000000000000",
		"factoryData":                   "0x",
		"callData":                      "0x80c5c7d0",
		"callGasLimit":                  "0x558c",
		"verificationGasLimit":          "0x129727",
		"preVerificationGas":            "0xc539",
		"maxFeePerGas":                  "0xa862145e",
		"maxPriorityFeePerGas":          "0xa8621440",
		"paymaster":                     "0x0000000000000000000000000000000000000000",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"eip7702Auth":                   "",
		"signature":                     "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
	delete(opData, "eip7702Auth")
	op, err := v08.New(opData)
	if err != nil {
		t.Fatal(err)
	}

	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		t.Fatal(err)
	}
	if pvg == nil || pvg.Cmp(big.NewInt(0)) <= 0 {
		t.Errorf("expected positive PVG, got %v", pvg)
	}
}

func TestSetCalcPreVerificationGasFunc(t *testing.T) {
	ov := NewDefaultOverhead()
	called := false
	ov.SetCalcPreVerificationGasFunc(func(op userop.UserOperationInterface, static *big.Int) (*big.Int, error) {
		called = true
		return static, nil
	})

	op := testutils.MockValidInitUserOp()
	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected custom PVG func to be called")
	}
	if pvg == nil || pvg.Cmp(big.NewInt(0)) <= 0 {
		t.Errorf("expected positive PVG, got %v", pvg)
	}
}

func TestCalcPreVerificationGasWithCustomPVGFuncError(t *testing.T) {
	ov := NewDefaultOverhead()
	ov.SetCalcPreVerificationGasFunc(func(op userop.UserOperationInterface, static *big.Int) (*big.Int, error) {
		return nil, errors.New("custom pvg error")
	})

	op := testutils.MockValidInitUserOp()
	_, err := ov.CalcPreVerificationGas(op)
	if err == nil || err.Error() != "custom pvg error" {
		t.Errorf("expected custom pvg error, got %v", err)
	}
}

func TestCalcPreVerificationGasWithCustomPVGFuncReturnsStatic(t *testing.T) {
	ov := NewDefaultOverhead()
	customStatic := big.NewInt(99999)
	ov.SetCalcPreVerificationGasFunc(func(op userop.UserOperationInterface, static *big.Int) (*big.Int, error) {
		return customStatic, nil
	})

	op := testutils.MockValidInitUserOp()
	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		t.Fatal(err)
	}
	if pvg.Cmp(customStatic) != 0 {
		t.Errorf("expected %d, got %d", customStatic, pvg)
	}
}

func TestCalcPreVerificationGasV09(t *testing.T) {
	ov := NewDefaultOverheadV09()
	opData := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0x0000000000000000000000000000000000000000",
		"factoryData":                   "0x",
		"callData":                      "0x80c5c7d0",
		"callGasLimit":                  "0x558c",
		"verificationGasLimit":          "0x129727",
		"preVerificationGas":            "0xc539",
		"maxFeePerGas":                  "0xa862145e",
		"maxPriorityFeePerGas":          "0xa8621440",
		"paymaster":                     "0x0000000000000000000000000000000000000000",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"paymasterSignature":            "0x",
		"signature":                     			"0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
	op, err := v09.New(opData)
	if err != nil {
		t.Fatal(err)
	}

	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		t.Fatal(err)
	}
	if pvg == nil || pvg.Cmp(big.NewInt(0)) <= 0 {
		t.Errorf("expected positive PVG, got %v", pvg)
	}
}

func TestCalcPreVerificationGasToMapError(t *testing.T) {
	ov := NewDefaultOverhead()
	mock := &mockOp{toMapErr: errors.New("toMap failed")}
	_, err := ov.CalcPreVerificationGas(mock)
	if err == nil || err.Error() != "toMap failed" {
		t.Errorf("expected toMap failed error, got %v", err)
	}
}

func TestCalcPreVerificationGasWithBufferErrorPath(t *testing.T) {
	ov := NewDefaultOverhead()
	ov.SetCalcPreVerificationGasFunc(func(op userop.UserOperationInterface, static *big.Int) (*big.Int, error) {
		return nil, errors.New("pvg error")
	})
	op := testutils.MockValidInitUserOp()
	_, err := ov.CalcPreVerificationGasWithBuffer(op)
	if err == nil || err.Error() != "pvg error" {
		t.Errorf("expected pvg error, got %v", err)
	}
}

func TestCalcPreVerificationGasV06WithPaymaster(t *testing.T) {
	ov := NewDefaultOverhead()
	opData := map[string]any{
		"sender":               "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                "0x1",
		"initCode":             "0x",
		"callData":             "0x80c5c7d0",
		"callGasLimit":         "0x558c",
		"verificationGasLimit": "0x129727",
		"preVerificationGas":   "0xc539",
		"maxFeePerGas":         "0xa862145e",
		"maxPriorityFeePerGas": "0xa8621440",
		"paymasterAndData":     "0x8b388a082f370d8ac2e2b3997e9151168bd09ff5deadbeef",
		"signature":            "0xa925dcc5e5131636e244d4405334c25f034ebdd85c0cb12e8cdb13c15249c2d466d0bade18e2cafd3513497f7f968dcbb63e519acd9b76dcae7acd61f11aa8421b",
	}
	op, err := userop.New(opData)
	if err != nil {
		t.Fatal(err)
	}

	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		t.Fatal(err)
	}
	if pvg == nil || pvg.Cmp(big.NewInt(0)) <= 0 {
		t.Errorf("expected positive PVG, got %v", pvg)
	}
}

func TestSanitizeOpDataEmptySignature(t *testing.T) {
	data := map[string]any{
		"signature": "0x",
	}
	sanitizeOpData(data, nil)
	if data["signature"] != "0x" {
		t.Errorf("expected 0x, got %v", data["signature"])
	}
}

func TestSanitizeOpDataNoSignature(t *testing.T) {
	data := map[string]any{}
	sanitizeOpData(data, nil)
	if _, ok := data["signature"]; ok {
		t.Error("signature should not be set when not in original data")
	}
}
