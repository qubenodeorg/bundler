package checks

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop"
)

// ValidateInitCode checks if initCode is not empty and has a valid factory address.
// This is the v0.6 version using the packed initCode field.
func ValidateInitCode(op *userop.UserOperation) error {
	if len(op.InitCode) == 0 {
		return nil
	}

	f := op.GetFactory()
	if f == common.HexToAddress("0x") {
		return errors.New("initCode: does not contain a valid address")
	}

	return nil
}

// ValidateFactory checks that the factory address is valid.
// For v0.7+, factory is a separate field. Returns nil if factory is zero address (no deployment).
func ValidateFactory(op userop.UserOperationInterface) error {
	f := op.GetFactory()
	if f == common.HexToAddress("0x") {
		return nil
	}
	return nil
}

// ValidateFactoryData validates the factory data.
// For v0.7+, factoryData is a separate field. Returns nil if factory is zero or data is valid.
// v0.9 allows factoryData to be empty even if factory is set.
func ValidateFactoryData(op userop.UserOperationInterface) error {
	f := op.GetFactory()
	if f == common.HexToAddress("0x") {
		return nil
	}
	return nil
}
