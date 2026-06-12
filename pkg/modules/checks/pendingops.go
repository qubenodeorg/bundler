package checks

import (
	"fmt"
	"math/big"

	"github.com/zarbanio/bundler/pkg/userop"
)

var (
	minPriceBump                = int64(10)
	ErrReplacementOpUnderpriced = fmt.Errorf(
		"pending ops: replacement op must increase maxFeePerGas and MaxPriorityFeePerGas by >= %d%%",
		minPriceBump,
	)
)

// calcNewThresholds returns new threshold values where newFee = oldFee  * (100 + minPriceBump) / 100.
func calcNewThresholds(cap *big.Int, tip *big.Int) (newCap *big.Int, newTip *big.Int) {
	a := big.NewInt(100 + minPriceBump)
	aFeeCap := big.NewInt(0).Mul(a, cap)
	aTip := big.NewInt(0).Mul(a, tip)

	b := big.NewInt(100)
	newCap = aFeeCap.Div(aFeeCap, b)
	newTip = aTip.Div(aTip, b)

	return newCap, newTip
}

// ValidatePendingOps checks the pending UserOperations by the same sender and only passes if:
//
//  1. Sender doesn't have another UserOperation already present in the pool.
//  2. It replaces an existing UserOperation with same nonce and higher fee.
func ValidatePendingOps(
	op *userop.UserOperation,
	penOps []*userop.UserOperation,
) error {
	if len(penOps) > 0 {
		var oldOp *userop.UserOperation
		for _, penOp := range penOps {
			// The mempool indexes pending ops by every entity role, so penOps
			// can include ops where our sender is only a factory/paymaster;
			// replacement rules apply to same-sender ops exclusively.
			if penOp.Sender == op.Sender && op.Nonce.Cmp(penOp.Nonce) == 0 {
				oldOp = penOp
			}
		}

		if oldOp != nil {
			newMf, newMpf := calcNewThresholds(oldOp.MaxFeePerGas, oldOp.MaxPriorityFeePerGas)

			if op.MaxFeePerGas.Cmp(newMf) < 0 || op.MaxPriorityFeePerGas.Cmp(newMpf) < 0 {
				return ErrReplacementOpUnderpriced
			}
		}
	}

	return nil
}

// ValidatePendingOpsV07 checks pending ops using the interface. Version-agnostic.
func ValidatePendingOpsV07(
	op userop.UserOperationInterface,
	penOps []*userop.UserOperation,
) error {
	nonce := op.GetNonce()
	if nonce == nil {
		return fmt.Errorf("nonce is nil")
	}

	if len(penOps) > 0 {
		var oldOp *userop.UserOperation
		for _, penOp := range penOps {
			// Same-sender ops only; penOps can include ops where our sender is
			// only a factory/paymaster (see ValidatePendingOps).
			if penOp.Sender == op.GetSender() && nonce.Cmp(penOp.Nonce) == 0 {
				oldOp = penOp
			}
		}

		if oldOp != nil {
			newMf, newMpf := calcNewThresholds(oldOp.MaxFeePerGas, oldOp.MaxPriorityFeePerGas)

			maxFee := op.GetMaxFeePerGas()
			maxPriorityFee := op.GetMaxPriorityFeePerGas()
			if maxFee == nil || maxPriorityFee == nil {
				return fmt.Errorf("maxFeePerGas or maxPriorityFeePerGas is nil")
			}
			if maxFee.Cmp(newMf) < 0 || maxPriorityFee.Cmp(newMpf) < 0 {
				return ErrReplacementOpUnderpriced
			}
		}
	}

	return nil
}
