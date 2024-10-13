package checks

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop"
)

// ValidateInitCode checks if initCode is not empty and has a valid factory address.
func ValidateInitCode(op *userop.UserOperation) error {
	if len(op.InitCode) == 0 {
		return nil
	}

	f := op.GetFactory()
	if f == common.HexToAddress("0x") {
		return errors.New("initCode: does not contain a valid address")
	}

	fmt.Println("initCode: ok")

	return nil
}
