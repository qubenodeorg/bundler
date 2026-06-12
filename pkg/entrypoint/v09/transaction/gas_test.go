package transaction

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestSuggestMeanGasTipCapNormalLoad(t *testing.T) {
	op1 := testutils.MockValidInitUserOp()
	op1.MaxPriorityFeePerGas = big.NewInt(1)
	op2 := testutils.MockValidInitUserOp()
	op2.MaxPriorityFeePerGas = big.NewInt(1)
	batch := []*userop.UserOperation{op1, op2}

	expected := big.NewInt(2)
	if tip := SuggestMeanGasTipCap(expected, batch); tip.Cmp(expected) != 0 {
		t.Fatalf("got %d, want %d", tip.Int64(), expected.Int64())
	}
}

func TestSuggestMeanGasTipCapHighLoad(t *testing.T) {
	op1 := testutils.MockValidInitUserOp()
	op1.MaxPriorityFeePerGas = big.NewInt(5)
	op2 := testutils.MockValidInitUserOp()
	op2.MaxPriorityFeePerGas = big.NewInt(10)
	batch := []*userop.UserOperation{op1, op2}

	expected := big.NewInt(7)
	if tip := SuggestMeanGasTipCap(big.NewInt(2), batch); tip.Cmp(expected) != 0 {
		t.Fatalf("got %d, want %d", tip.Int64(), expected.Int64())
	}
}

func TestSuggestMeanGasFeeCapNormalLoad(t *testing.T) {
	op1 := testutils.MockValidInitUserOp()
	op1.MaxFeePerGas = big.NewInt(1)
	op2 := testutils.MockValidInitUserOp()
	op2.MaxFeePerGas = big.NewInt(1)
	batch := []*userop.UserOperation{op1, op2}

	bf := big.NewInt(1)
	tip := big.NewInt(0)
	expected := big.NewInt(0).Mul(bf, common.Big2)
	mf := SuggestMeanGasFeeCap(bf, tip, batch)
	if mf.Cmp(expected) != 0 {
		t.Fatalf("got %d, want %d", mf.Int64(), expected.Int64())
	}
}

func TestSuggestMeanGasFeeCapHighLoad(t *testing.T) {
	op1 := testutils.MockValidInitUserOp()
	op1.MaxFeePerGas = big.NewInt(5)
	op2 := testutils.MockValidInitUserOp()
	op2.MaxFeePerGas = big.NewInt(10)
	batch := []*userop.UserOperation{op1, op2}

	bf := big.NewInt(1)
	tip := big.NewInt(0)
	expected := big.NewInt(7)
	mf := SuggestMeanGasFeeCap(bf, tip, batch)
	if mf.Cmp(expected) != 0 {
		t.Fatalf("got %d, want %d", mf.Int64(), expected.Int64())
	}
}

func TestSuggestMeanGasPriceNormalLoad(t *testing.T) {
	op1 := testutils.MockValidInitUserOp()
	op1.MaxFeePerGas = big.NewInt(1)
	op2 := testutils.MockValidInitUserOp()
	op2.MaxFeePerGas = big.NewInt(1)

	expected := big.NewInt(2)
	batch := []*userop.UserOperation{op1, op2}
	gp := SuggestMeanGasPrice(expected, batch)
	if gp.Cmp(expected) != 0 {
		t.Fatalf("got %d, want %d", gp.Int64(), expected.Int64())
	}
}

func TestSuggestMeanGasPriceHighLoad(t *testing.T) {
	op1 := testutils.MockValidInitUserOp()
	op1.MaxFeePerGas = big.NewInt(5)
	op2 := testutils.MockValidInitUserOp()
	op2.MaxFeePerGas = big.NewInt(10)

	expected := big.NewInt(7)
	batch := []*userop.UserOperation{op1, op2}
	gp := SuggestMeanGasPrice(big.NewInt(2), batch)
	if gp.Cmp(expected) != 0 {
		t.Fatalf("got %d, want %d", gp.Int64(), expected.Int64())
	}
}

// TestSuggestMeanGasTipCapEmptyBatch verifies SuggestMeanGasTipCap does not
// panic on an empty batch and returns the supplied tip.
func TestSuggestMeanGasTipCapEmptyBatch(t *testing.T) {
	tip := big.NewInt(5)
	got := SuggestMeanGasTipCap(tip, nil)
	if got.Cmp(tip) != 0 {
		t.Fatalf("got %d, want %d", got.Int64(), tip.Int64())
	}
}

// TestSuggestMeanGasFeeCapEmptyBatch verifies SuggestMeanGasFeeCap does not
// panic on an empty batch and returns the recommended max fee.
func TestSuggestMeanGasFeeCapEmptyBatch(t *testing.T) {
	bf := big.NewInt(1)
	tip := big.NewInt(3)
	got := SuggestMeanGasFeeCap(bf, tip, nil)
	want := big.NewInt(0).Add(tip, big.NewInt(0).Mul(bf, common.Big2))
	if got.Cmp(want) != 0 {
		t.Fatalf("got %d, want %d", got.Int64(), want.Int64())
	}
}

// TestSuggestMeanGasPriceEmptyBatch verifies SuggestMeanGasPrice does not
// panic on an empty batch and returns the supplied gasPrice.
func TestSuggestMeanGasPriceEmptyBatch(t *testing.T) {
	gasPrice := big.NewInt(7)
	got := SuggestMeanGasPrice(gasPrice, nil)
	if got.Cmp(gasPrice) != 0 {
		t.Fatalf("got %d, want %d", got.Int64(), gasPrice.Int64())
	}
}
