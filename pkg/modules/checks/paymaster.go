package checks

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/entrypoint"
	rpcerrors "github.com/zarbanio/bundler/pkg/errors"
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

// ValidatePaymaster checks that the paymaster is deployed and has a deposit
// covering the max prefund of this op plus all its pending ops in the mempool.
func ValidatePaymaster(
	op userop.UserOperationInterface,
	dep *entrypoint.IStakeManagerDepositInfo,
	gc GetCodeFunc,
	pendingOps []*userop.UserOperation,
	version userop.Version,
) error {
	// Zero-address paymaster validation is handled in ValidatePaymasterData.
	if op.GetPaymaster() == (common.Address{}) {
		return nil
	}

	pm := op.GetPaymaster()
	bytecode, err := gc(pm)
	if err != nil {
		return err
	}
	if len(bytecode) == 0 {
		return ErrPaymasterNotDeployed
	}

	required := new(big.Int).Set(op.GetMaxPrefund())
	for _, pending := range pendingOps {
		if version == userop.V0_6 {
			required.Add(required, pending.GetMaxPrefund())
		} else {
			required.Add(required, packedMaxPrefund(pending))
		}
	}
	if dep.Deposit.Cmp(required) < 0 {
		return rpcerrors.NewRPCError(
			rpcerrors.PAYMASTER_DEPOSIT_TOO_LOW,
			"paymaster deposit too low to cover max prefund of all pending ops",
			nil,
		)
	}

	return nil
}

var maxUint128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

// packedMaxPrefund mirrors the v0.7+ max-cost formula (gas limits incl. the
// paymaster limits, times maxFeePerGas) for a mempool op stored in the packed
// legacy format.
func packedMaxPrefund(op *userop.UserOperation) *big.Int {
	sum := new(big.Int).Add(op.VerificationGasLimit, op.CallGasLimit)
	sum.Add(sum, op.PreVerificationGas)
	if len(op.PaymasterAndData) >= common.AddressLength+32 {
		packed := new(big.Int).SetBytes(op.PaymasterAndData[common.AddressLength : common.AddressLength+32])
		sum.Add(sum, new(big.Int).Rsh(packed, 128))
		sum.Add(sum, new(big.Int).And(packed, maxUint128))
	}
	return sum.Mul(sum, op.MaxFeePerGas)
}

// ValidatePaymasterData validates the paymaster data format. A zero-address
// paymaster with non-empty paymasterData is rejected for all versions.
func ValidatePaymasterData(op userop.UserOperationInterface, _ userop.Version) error {
	if op.GetPaymaster() == (common.Address{}) {
		if len(op.GetPaymasterData()) > 0 {
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

