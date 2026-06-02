package checks

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/entrypoint"
	"github.com/zarbanio/bundler/pkg/userop"
)

var (
	ErrPaymasterInvalidLength     = errors.New("PaymasterAndData: invalid length")
	ErrPaymasterNotDeployed       = errors.New("paymaster: code not deployed")
	ErrPaymasterInsufficientDep   = errors.New("paymaster: not enough deposit to cover max prefund")
	ErrPaymasterVGLExceedsMax     = errors.New("paymasterVerificationGasLimit: exceeds max verification gas")
	ErrPaymasterPOGLTooLow        = errors.New("paymasterPostOpGasLimit: below minimum expected gas")
	ErrPaymasterZeroAddrWithData  = errors.New("paymaster: zero address with non-empty paymasterData")
	ErrPaymasterExceedsVerificationGas = errors.New("paymasterVerificationGasLimit + verificationGasLimit exceeds max verification gas")
)

// ValidatePaymasterAndData checks the paymasterAndData is either zero bytes or the first 20 bytes contain an
// address that
//
//  1. currently has nonempty code on chain
//  2. has a sufficient deposit to pay for the UserOperation
func ValidatePaymasterAndData(
	op *userop.UserOperation,
	dep *entrypoint.IStakeManagerDepositInfo,
	gc GetCodeFunc,
) error {
	if len(op.PaymasterAndData) == 0 {
		return nil
	}
	if len(op.PaymasterAndData) < common.AddressLength {
		return ErrPaymasterInvalidLength
	}

	pm := op.GetPaymaster()
	bytecode, err := gc(pm)
	if err != nil {
		return err
	}
	if len(bytecode) == 0 {
		return ErrPaymasterNotDeployed
	}

	if dep.Deposit.Cmp(op.GetMaxPrefund()) < 0 {
		return ErrPaymasterInsufficientDep
	}

	return nil
}

// ValidatePaymaster checks that the paymaster is deployed and has sufficient deposit.
func ValidatePaymaster(
	op userop.UserOperationInterface,
	dep *entrypoint.IStakeManagerDepositInfo,
	gc GetCodeFunc,
	version userop.Version,
) error {
	pm := op.GetPaymaster()

	// v0.8+: reject zero-address paymaster if paymasterData is non-empty
	if version == userop.V0_8 || version == userop.V0_9 {
		if pm == (common.Address{}) && len(op.GetPaymasterData()) > 0 {
			return ErrPaymasterZeroAddrWithData
		}
	}

	if pm == (common.Address{}) {
		return nil
	}

	bytecode, err := gc(pm)
	if err != nil {
		return err
	}
	if len(bytecode) == 0 {
		return ErrPaymasterNotDeployed
	}

	if dep.Deposit.Cmp(op.GetMaxPrefund()) < 0 {
		return ErrPaymasterInsufficientDep
	}

	return nil
}

// ValidatePaymasterData validates the paymaster data format.
func ValidatePaymasterData(op userop.UserOperationInterface, version userop.Version) error {
	pm := op.GetPaymaster()
	if pm == (common.Address{}) {
		if len(op.GetPaymasterData()) > 0 && version == userop.V0_7 {
			return ErrPaymasterZeroAddrWithData
		}
		return nil
	}
	return nil
}

// ValidatePaymasterVerificationGasLimit checks the paymasterVerificationGasLimit is valid.
func ValidatePaymasterVerificationGasLimit(op userop.UserOperationInterface, maxVerificationGas *big.Int) error {
	pm := op.GetPaymaster()
	if pm == (common.Address{}) {
		return nil
	}
	pvgl := op.GetPaymasterVerificationGasLimit()
	if pvgl == nil {
		return fmt.Errorf("paymasterVerificationGasLimit is nil")
	}
	if pvgl.Cmp(maxVerificationGas) > 0 {
		return ErrPaymasterVGLExceedsMax
	}
	return nil
}

// ValidatePaymasterPostOpGasLimit checks the paymasterPostOpGasLimit is sufficiently high.
func ValidatePaymasterPostOpGasLimit(op userop.UserOperationInterface) error {
	pm := op.GetPaymaster()
	if pm == (common.Address{}) {
		return nil
	}
	pogl := op.GetPaymasterPostOpGasLimit()
	if pogl == nil {
		return fmt.Errorf("paymasterPostOpGasLimit is nil")
	}
	// Minimum postOp gas should be at least the cost of a basic CALL with non-zero value
	if pogl.Cmp(big.NewInt(12000)) < 0 {
		return ErrPaymasterPOGLTooLow
	}
	return nil
}
