package transaction

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestToRawTxHex(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %s", err)
	}
	signer := types.LatestSignerForChainID(big.NewInt(1))
	tx, err := types.SignNewTx(key, signer, &types.LegacyTx{
		Nonce:    0,
		To:       nil,
		Value:    big.NewInt(100),
		Gas:      21000,
		GasPrice: big.NewInt(1000000000),
		Data:     nil,
	})
	if err != nil {
		t.Fatalf("SignNewTx: %s", err)
	}

	hex := ToRawTxHex(tx)
	if len(hex) == 0 {
		t.Fatal("ToRawTxHex: got empty string, want non-empty")
	}
	if hex[:2] != "0x" {
		t.Fatalf("ToRawTxHex: got %s, want 0x prefix", hex[:2])
	}
}
