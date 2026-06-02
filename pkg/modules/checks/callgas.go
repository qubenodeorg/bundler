package checks

import (
	"fmt"

	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/userop"
)

// ValidateCallGasLimit checks the callGasLimit is at least the cost of a CALL with non-zero value.
func ValidateCallGasLimit(op *userop.UserOperation, ov *gas.Overhead) error {
	cg := ov.NonZeroValueCall()
	if op.CallGasLimit.Cmp(cg) < 0 {
		return fmt.Errorf("callGasLimit: below expected gas of %s", cg.String())
	}

	return nil
}

// ValidateCallGasLimitV07 checks the callGasLimit is at least the cost of a CALL with non-zero value.
// Uses the interface to be version-agnostic.
func ValidateCallGasLimitV07(op userop.UserOperationInterface, ov *gas.Overhead) error {
	cg := ov.NonZeroValueCall()
	if op.GetCallGasLimit().Cmp(cg) < 0 {
		return fmt.Errorf("callGasLimit: below expected gas of %s", cg.String())
	}

	return nil
}
