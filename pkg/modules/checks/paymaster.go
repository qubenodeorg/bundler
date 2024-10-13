package checks

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/entrypoint"
	"github.com/zarbanio/bundler/pkg/userop"
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
		fmt.Println("paymaster: ok")
		return nil
	}

	fmt.Println("paymaster: PaymasterAndData", op.PaymasterAndData)

	if len(op.PaymasterAndData) < common.AddressLength {
		return errors.New("PaymasterAndData: invalid length")
	}

	pm := op.GetPaymaster()
	bytecode, err := gc(pm)
	if err != nil {
		return err
	}
	fmt.Println("paymaster: bytecode", bytecode)
	if len(bytecode) == 0 {
		return errors.New("paymaster: code not deployed")
	}

	if dep.Deposit.Cmp(op.GetMaxPrefund()) < 0 {
		return errors.New("paymaster: not enough deposit to cover max prefund")
	}

	fmt.Println("paymaster: ok")

	return nil
}
