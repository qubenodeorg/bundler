package transaction

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestToPackedBatch(t *testing.T) {
	op1 := testutils.MockValidInitUserOp()
	op2 := testutils.MockValidInitUserOp()
	op2.Nonce = big.NewInt(1)

	batch := toPackedBatch([]*userop.UserOperation{op1, op2})

	if len(batch) != 2 {
		t.Fatalf("toPackedBatch: got %d ops, want 2", len(batch))
	}
	if batch[0].Sender != op1.Sender {
		t.Fatalf("batch[0].Sender: got %s, want %s", batch[0].Sender.Hex(), op1.Sender.Hex())
	}
	if batch[1].Nonce.Cmp(op2.Nonce) != 0 {
		t.Fatalf("batch[1].Nonce: got %d, want %d", batch[1].Nonce, op2.Nonce)
	}
}

func TestToPackedBatchPackedFields(t *testing.T) {
	op := testutils.MockValidInitUserOp()

	batch := toPackedBatch([]*userop.UserOperation{op})

	if len(batch) != 1 {
		t.Fatalf("toPackedBatch: got %d ops, want 1", len(batch))
	}

	state := batch[0]

	maxUint128 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	extractedVG := new(big.Int).Rsh(new(big.Int).SetBytes(state.AccountGasLimits[:]), 128)
	extractedCG := new(big.Int).And(new(big.Int).SetBytes(state.AccountGasLimits[:]), maxUint128)
	if extractedVG.Cmp(op.VerificationGasLimit) != 0 {
		t.Fatalf("accountGasLimits: extracted VGL %d, want %d", extractedVG, op.VerificationGasLimit)
	}
	if extractedCG.Cmp(op.CallGasLimit) != 0 {
		t.Fatalf("accountGasLimits: extracted CGL %d, want %d", extractedCG, op.CallGasLimit)
	}

	extractedMF := new(big.Int).Rsh(new(big.Int).SetBytes(state.GasFees[:]), 128)
	extractedMP := new(big.Int).And(new(big.Int).SetBytes(state.GasFees[:]), maxUint128)
	if extractedMF.Cmp(op.MaxFeePerGas) != 0 {
		t.Fatalf("gasFees: extracted MF %d, want %d", extractedMF, op.MaxFeePerGas)
	}
	if extractedMP.Cmp(op.MaxPriorityFeePerGas) != 0 {
		t.Fatalf("gasFees: extracted MP %d, want %d", extractedMP, op.MaxPriorityFeePerGas)
	}
}

func TestPackAccountGasLimits(t *testing.T) {
	vgl := big.NewInt(100000)
	cgl := big.NewInt(30000)

	result := packAccountGasLimits(vgl, cgl)

	maxUint128 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	extractedVG := new(big.Int).Rsh(new(big.Int).SetBytes(result[:]), 128)
	extractedCG := new(big.Int).And(new(big.Int).SetBytes(result[:]), maxUint128)
	if extractedVG.Cmp(vgl) != 0 {
		t.Fatalf("packAccountGasLimits: extracted VGL %d, want %d", extractedVG, vgl)
	}
	if extractedCG.Cmp(cgl) != 0 {
		t.Fatalf("packAccountGasLimits: extracted CGL %d, want %d", extractedCG, cgl)
	}
}

func TestPackGasFees(t *testing.T) {
	mf := big.NewInt(2000000000)
	mp := big.NewInt(1000000000)

	result := packGasFees(mf, mp)

	maxUint128 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	extractedMF := new(big.Int).Rsh(new(big.Int).SetBytes(result[:]), 128)
	extractedMP := new(big.Int).And(new(big.Int).SetBytes(result[:]), maxUint128)
	if extractedMF.Cmp(mf) != 0 {
		t.Fatalf("packGasFees: extracted MF %d, want %d", extractedMF, mf)
	}
	if extractedMP.Cmp(mp) != 0 {
		t.Fatalf("packGasFees: extracted MP %d, want %d", extractedMP, mp)
	}
}

func TestToPackedBatchPreservesZeroAddresses(t *testing.T) {
	op := testutils.MockValidInitUserOp()
	op.PaymasterAndData = []byte{}

	batch := toPackedBatch([]*userop.UserOperation{op})

	if len(batch) != 1 {
		t.Fatalf("toPackedBatch: got %d ops, want 1", len(batch))
	}

	state := batch[0]

	if state.Sender == (common.Address{}) {
		t.Fatal("toPackedBatch: sender should not be zero address")
	}
}
