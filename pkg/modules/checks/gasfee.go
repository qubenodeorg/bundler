package checks

import (
	"fmt"

	"github.com/zarbanio/bundler/pkg/modules/gasprice"
	"github.com/zarbanio/bundler/pkg/userop"
)

// ValidateFeePerGas checks the maxFeePerGas is sufficiently high to be included with the current
// block.basefee. Alternatively, if basefee is not supported, then check that maxPriorityFeePerGas is equal to
// maxFeePerGas as a fallback.
func ValidateFeePerGas(op *userop.UserOperation, gbf gasprice.GetBaseFeeFunc) error {
	bf, err := gbf()
	if err != nil {
		return err
	}

	if bf == nil {
		if op.MaxPriorityFeePerGas.Cmp(op.MaxFeePerGas) != 0 {
			return fmt.Errorf("legacy fee mode: maxPriorityFeePerGas must equal maxFeePerGas")
		}

		return nil
	}

	if op.MaxPriorityFeePerGas.Cmp(op.MaxFeePerGas) == 1 {
		return fmt.Errorf("maxFeePerGas: must be equal to or greater than maxPriorityFeePerGas")
	}

	if op.MaxFeePerGas.Cmp(bf) < 0 {
		return fmt.Errorf("maxFeePerGas: must be equal to or greater than current block.basefee")
	}

	return nil
}

// ValidateFeePerGasV07 checks fee parameters. Uses the interface to be version-agnostic.
func ValidateFeePerGasV07(op userop.UserOperationInterface, gbf gasprice.GetBaseFeeFunc) error {
	bf, err := gbf()
	if err != nil {
		return err
	}

	maxPriorityFee := op.GetMaxPriorityFeePerGas()
	maxFee := op.GetMaxFeePerGas()
	if maxPriorityFee == nil || maxFee == nil {
		return fmt.Errorf("maxPriorityFeePerGas or maxFeePerGas is nil")
	}

	if bf == nil {
		if maxPriorityFee.Cmp(maxFee) != 0 {
			return fmt.Errorf("legacy fee mode: maxPriorityFeePerGas must equal maxFeePerGas")
		}

		return nil
	}

	if maxPriorityFee.Cmp(maxFee) == 1 {
		return fmt.Errorf("maxFeePerGas: must be equal to or greater than maxPriorityFeePerGas")
	}

	if maxFee.Cmp(bf) < 0 {
		return fmt.Errorf("maxFeePerGas: must be equal to or greater than current block.basefee")
	}

	return nil
}
