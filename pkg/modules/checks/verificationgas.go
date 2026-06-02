package checks

import (
	"fmt"
	"math/big"

	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/userop"
)

// ValidateVerificationGas checks that the verificationGasLimit is sufficiently low (<= MAX_VERIFICATION_GAS)
// and the preVerificationGas is sufficiently high (enough to pay for the calldata gas cost of serializing
// the UserOperation plus PRE_VERIFICATION_OVERHEAD_GAS).
func ValidateVerificationGas(op *userop.UserOperation, ov *gas.Overhead, maxVerificationGas *big.Int) error {
	if op.VerificationGasLimit.Cmp(maxVerificationGas) > 0 {
		return fmt.Errorf(
			"verificationGasLimit: exceeds maxVerificationGas of %s",
			maxVerificationGas.String(),
		)
	}

	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		return err
	}
	if op.PreVerificationGas.Cmp(pvg) < 0 {
		return fmt.Errorf("preVerificationGas: below expected gas of %s", pvg.String())
	}

	return nil
}

// ValidateVerificationGasV07 checks verification gas limits for v0.7+.
// For v0.7+, the total verification gas is verificationGasLimit + paymasterVerificationGasLimit.
func ValidateVerificationGasV07(
	op userop.UserOperationInterface,
	ov *gas.Overhead,
	maxVerificationGas *big.Int,
) error {
	totalVG := new(big.Int).Add(op.GetVerificationGasLimit(), op.GetPaymasterVerificationGasLimit())
	if totalVG.Cmp(maxVerificationGas) > 0 {
		return fmt.Errorf(
			"verificationGasLimit: total (verificationGasLimit + paymasterVerificationGasLimit) "+
				"exceeds maxVerificationGas of %s",
			maxVerificationGas.String(),
		)
	}

	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		return err
	}
	if op.GetPreVerificationGas().Cmp(pvg) < 0 {
		return fmt.Errorf("preVerificationGas: below expected gas of %s", pvg.String())
	}

	return nil
}
