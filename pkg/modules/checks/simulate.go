package checks

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/altmempools"
	"github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/userop"
)

// SimulateValidationV07 performs off-chain simulation for v0.7+.
// v0.7+ removed simulateValidation from the deployed contract, so validation is entirely off-chain.
// Falls back to debug_traceCall for deep validation.
func SimulateValidationV07(
	rpc *rpc.Client,
	entryPoint common.Address,
	op userop.UserOperationInterface,
	chainID *big.Int,
	ov *gas.Overhead,
	alt *altmempools.Directory,
	isRIP7212Supported bool,
	tracer string,
	stakes simulation.EntityStakes,
) error {
	// Try trace-based simulation (deep validation with opcode/storage checks)
	return simulateWithTrace(rpc, entryPoint, op, chainID, ov, alt, isRIP7212Supported, tracer, stakes)
}

// simulateWithTrace uses debug_traceCall for validation simulation.
func simulateWithTrace(
	rpc *rpc.Client,
	entryPoint common.Address,
	op userop.UserOperationInterface,
	chainID *big.Int,
	ov *gas.Overhead,
	alt *altmempools.Directory,
	isRIP7212Supported bool,
	tracer string,
	stakes simulation.EntityStakes,
) error {
	// Extract delegate address from EIP-7702 auth BEFORE any type conversion.
	delegateAddr := GetEIP7702Address(op.GetEIP7702Auth())

	v06Op, ok := op.(*userop.UserOperation)
	if !ok {
		converted, convErr := gas.ConvertToV06(op)
		if convErr != nil {
			return errors.NewRPCError(
				errors.BANNED_OPCODE,
				convErr.Error(),
				convErr.Error(),
			)
		}
		v06Op = converted
	}

	out, err := simulation.TraceSimulateValidation(&simulation.TraceInput{
		Rpc:                rpc,
		EntryPoint:         entryPoint,
		Op:                 v06Op,
		ChainID:            chainID,
		IsRIP7212Supported: isRIP7212Supported,
		Tracer:             tracer,
		Stakes:             stakes,
		AltMempools:        alt,
		DelegateAddress:    delegateAddr,
	})
	if err != nil {
		return errors.NewRPCError(errors.BANNED_OPCODE, err.Error(), err.Error())
	}

	// Validate additional ERC-7562 rules
	if err := ValidateERC7562Rules(op, entryPoint); err != nil {
		return errors.NewRPCError(errors.BANNED_OPCODE, err.Error(), err.Error())
	}

	_ = out
	return nil
}
