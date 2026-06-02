package simulation

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestToPackedOpV08(t *testing.T) {
	op := &userop.UserOperation{
		Sender:               common.HexToAddress("0xa13D69573f994bf662C2714560c44dd7266FC547"),
		Nonce:                big.NewInt(0),
		InitCode:             common.Hex2Bytes("e19e9755"),
		CallData:             common.Hex2Bytes("80c5c7d0"),
		CallGasLimit:         big.NewInt(30000),
		VerificationGasLimit: big.NewInt(100000),
		PreVerificationGas:   big.NewInt(50000),
		MaxFeePerGas:         big.NewInt(2000000000),
		MaxPriorityFeePerGas: big.NewInt(1000000000),
		PaymasterAndData:     []byte{},
		Signature:            common.Hex2Bytes("a925dcc5"),
	}

	packed, err := toPackedOp(op)
	if err != nil {
		t.Fatalf("toPackedOp: got err %v, want nil", err)
	}

	if packed.Sender != op.Sender {
		t.Fatalf("packed.Sender: got %s, want %s", packed.Sender.Hex(), op.Sender.Hex())
	}
	if packed.Nonce.Cmp(op.Nonce) != 0 {
		t.Fatalf("packed.Nonce: got %d, want %d", packed.Nonce, op.Nonce)
	}
	if packed.PreVerificationGas.Cmp(op.PreVerificationGas) != 0 {
		t.Fatalf("packed.PreVerificationGas: got %d, want %d", packed.PreVerificationGas, op.PreVerificationGas)
	}
}

func TestPackAccountGasLimitsV08(t *testing.T) {
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

func TestPackAccountGasLimitsTruncationV08(t *testing.T) {
	vgl := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 129), big.NewInt(1))
	cgl := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 129), big.NewInt(1))

	result := packAccountGasLimits(vgl, cgl)

	maxUint128 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	extractedVG := new(big.Int).Rsh(new(big.Int).SetBytes(result[:]), 128)
	extractedCG := new(big.Int).And(new(big.Int).SetBytes(result[:]), maxUint128)
	if extractedVG.Cmp(maxUint128) != 0 {
		t.Fatalf("packAccountGasLimits: extracted VGL %d, want %d", extractedVG, maxUint128)
	}
	if extractedCG.Cmp(maxUint128) != 0 {
		t.Fatalf("packAccountGasLimits: extracted CGL %d, want %d", extractedCG, maxUint128)
	}
}

func TestPackGasFeesTruncationV08(t *testing.T) {
	mf := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 129), big.NewInt(1))
	mp := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 129), big.NewInt(1))

	result := packGasFees(mf, mp)

	maxUint128 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	extractedMF := new(big.Int).Rsh(new(big.Int).SetBytes(result[:]), 128)
	extractedMP := new(big.Int).And(new(big.Int).SetBytes(result[:]), maxUint128)
	if extractedMF.Cmp(maxUint128) != 0 {
		t.Fatalf("packGasFees: extracted MF %d, want %d", extractedMF, maxUint128)
	}
 	if extractedMP.Cmp(maxUint128) != 0 {
 		t.Fatalf("packGasFees: extracted MP %d, want %d", extractedMP, maxUint128)
 	}
 }

func TestPackGasFeesV08(t *testing.T) {
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


