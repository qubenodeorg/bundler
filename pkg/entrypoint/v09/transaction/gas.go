package transaction

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop"
)

func SuggestMeanGasTipCap(tip *big.Int, batch []*userop.UserOperation) *big.Int {
	sum := big.NewInt(0)
	for _, op := range batch {
		sum = big.NewInt(0).Add(sum, op.MaxPriorityFeePerGas)
	}
	avg := big.NewInt(0).Div(sum, big.NewInt(int64(len(batch))))
	if avg.Cmp(tip) == 1 {
		return avg
	}
	return tip
}

func SuggestMeanGasFeeCap(basefee *big.Int, tip *big.Int, batch []*userop.UserOperation) *big.Int {
	mf := big.NewInt(0).Add(tip, big.NewInt(0).Mul(basefee, common.Big2))
	sum := big.NewInt(0)
	for _, op := range batch {
		sum = big.NewInt(0).Add(sum, op.MaxFeePerGas)
	}
	avg := big.NewInt(0).Div(sum, big.NewInt(int64(len(batch))))
	if avg.Cmp(mf) == 1 {
		return avg
	}
	return mf
}

func SuggestMeanGasPrice(gasPrice *big.Int, batch []*userop.UserOperation) *big.Int {
	sum := big.NewInt(0)
	for _, op := range batch {
		sum = big.NewInt(0).Add(sum, op.MaxFeePerGas)
	}
	avg := big.NewInt(0).Div(sum, big.NewInt(int64(len(batch))))
	if avg.Cmp(gasPrice) == 1 {
		return avg
	}
	return gasPrice
}
