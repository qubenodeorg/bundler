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
	op *userop.UserOperation,
	chainID *big.Int,
	ov *gas.Overhead,
	alt *altmempools.Directory,
	isRIP7212Supported bool,
	tracer string,
) error {
	// Try trace-based simulation (deep validation with opcode/storage checks)
	return simulateWithTrace(rpc, entryPoint, op, chainID, ov, alt, isRIP7212Supported, tracer)
}

// simulateWithTrace uses debug_traceCall for validation simulation.
func simulateWithTrace(
	rpc *rpc.Client,
	entryPoint common.Address,
	op *userop.UserOperation,
	chainID *big.Int,
	ov *gas.Overhead,
	alt *altmempools.Directory,
	isRIP7212Supported bool,
	tracer string,
) error {
	// Build stakes map from the operation entities
	stakes := simulation.EntityStakes{
		op.Sender: {Deposit: big.NewInt(0), Stake: big.NewInt(0)},
	}
	if factory := op.GetFactory(); factory != (common.Address{}) {
		stakes[factory] = stakes[op.Sender]
	}
	if paymaster := op.GetPaymaster(); paymaster != (common.Address{}) {
		stakes[paymaster] = stakes[op.Sender]
	}

	out, err := simulation.TraceSimulateValidation(&simulation.TraceInput{
		Rpc:                rpc,
		EntryPoint:         entryPoint,
		Op:                 op,
		ChainID:            chainID,
		IsRIP7212Supported: isRIP7212Supported,
		Tracer:             tracer,
		Stakes:             stakes,
		AltMempools:        alt,
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
