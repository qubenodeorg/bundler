package checks

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop"
)

var (
	zeroAddress             = common.HexToAddress("0x")
	ErrSenderNotDeployed   = errors.New("sender: not deployed, initCode must be set")
	ErrSenderAlreadyDeploy = errors.New("sender: already deployed, initCode must be empty")
)

// ValidateSender accepts a userOp and a generic function that can retrieve the bytecode of the sender.
// Either the sender is deployed (non-zero length bytecode) or the initCode is not empty (but not both).
func ValidateSender(op *userop.UserOperation, gc GetCodeFunc) error {
	bytecode, err := gc(op.Sender)
	if err != nil {
		return err
	}

	if len(bytecode) == 0 && len(op.InitCode) == 0 {
		return ErrSenderNotDeployed
	}
	if len(bytecode) > 0 && len(op.InitCode) > 0 {
		return ErrSenderAlreadyDeploy
	}

	return nil
}

// ValidateSenderV07 validates the sender for v0.7+ UserOperations.
// Uses GetFactory() to determine if initCode is present. An undeployed sender
// is also fine with an EIP-7702 authorization, which delegates code to it.
func ValidateSenderV07(op userop.UserOperationInterface, gc GetCodeFunc) error {
	bytecode, err := gc(op.GetSender())
	if err != nil {
		return err
	}

	hasFactory := op.GetFactory() != zeroAddress
	hasAuth := op.GetEIP7702Auth() != nil
	if len(bytecode) == 0 && !hasFactory && !hasAuth {
		return ErrSenderNotDeployed
	}
	if len(bytecode) > 0 && hasFactory && !hasAuth {
		return ErrSenderAlreadyDeploy
	}

	return nil
}

// ValidateSenderV09 validates the sender for v0.9 UserOperations.
// v0.9 allows initCode even when the account already exists (silently ignored).
func ValidateSenderV09(op userop.UserOperationInterface, gc GetCodeFunc) error {
	bytecode, err := gc(op.GetSender())
	if err != nil {
		return err
	}

	hasFactory := op.GetFactory() != zeroAddress
	hasAuth := op.GetEIP7702Auth() != nil
	if len(bytecode) == 0 && !hasFactory && !hasAuth {
		return ErrSenderNotDeployed
	}
	// v0.9: ALLOW initCode with existing accounts (no ErrSenderAlreadyDeploy)
	return nil
}
