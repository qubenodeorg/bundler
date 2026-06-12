package checks

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/altmempools"
	"github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	v07sim "github.com/zarbanio/bundler/pkg/entrypoint/v07/simulation"
	v08sim "github.com/zarbanio/bundler/pkg/entrypoint/v08/simulation"
	v09sim "github.com/zarbanio/bundler/pkg/entrypoint/v09/simulation"
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
	version userop.Version,
) error {
	// eth_call handleOps first: a validation-phase revert (FailedOp) carries
	// the spec error code and the entity's revert reason, which the
	// trace-based rules below cannot provide.
	if err := callSimulateValidation(rpc, entryPoint, op, chainID, version); err != nil {
		return err
	}

	// Try trace-based simulation (deep validation with opcode/storage checks)
	return simulateWithTrace(rpc, entryPoint, op, chainID, ov, alt, isRIP7212Supported, tracer, stakes, version)
}

// callSimulateValidation surfaces validation-phase reverts of handleOps via a
// plain eth_call. AA25 (invalid account nonce) is not an error here: ops with
// future nonces are valid in the mempool while earlier ops are still pending.
func callSimulateValidation(
	rpc *rpc.Client,
	entryPoint common.Address,
	op userop.UserOperationInterface,
	chainID *big.Int,
	version userop.Version,
) error {
	delegateAddr := GetEIP7702Address(op.GetEIP7702Auth())

	var err error
	switch version {
	case userop.V0_7:
		_, err = v07sim.SimulateValidation(&v07sim.SimulateInput{
			Rpc: rpc, EntryPoint: entryPoint, Op: op, ChainID: chainID,
		})
	case userop.V0_8:
		_, err = v08sim.SimulateValidation(&v08sim.SimulateInput{
			Rpc: rpc, EntryPoint: entryPoint, Op: op, ChainID: chainID,
			DelegateAddress: delegateAddr,
		})
	case userop.V0_9:
		_, err = v09sim.SimulateValidation(&v09sim.SimulateInput{
			Rpc: rpc, EntryPoint: entryPoint, Op: op, ChainID: chainID,
			DelegateAddress: delegateAddr,
		})
	default:
		return nil
	}
	if err == nil || strings.Contains(err.Error(), "AA25") {
		return nil
	}
	if rpcErr, ok := err.(*errors.RPCError); ok {
		return rpcErr
	}
	return errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, err.Error(), err.Error())
}

// simulateWithTrace uses debug_traceCall for validation simulation. It
// dispatches to the version-specific simulation package so the trace targets
// the right EntryPoint method (v0.7+ trace handleOps; only v0.6 still has an
// on-chain simulateValidation).
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
	version userop.Version,
) error {
	// Extract delegate address from EIP-7702 auth BEFORE any type conversion.
	delegateAddr := GetEIP7702Address(op.GetEIP7702Auth())

	v06Op, ok := op.(*userop.UserOperation)
	if !ok {
		// ConvertToV06Packed preserves the real paymaster gas limits; the trace
		// runs handleOps, so a dummy zero paymasterVerificationGasLimit would
		// make validatePaymasterUserOp run out of gas instead of validating.
		converted, convErr := userop.ConvertToV06Packed(op)
		if convErr != nil {
			return errors.NewRPCError(
				errors.BANNED_OPCODE,
				convErr.Error(),
				convErr.Error(),
			)
		}
		v06Op = converted
	}

	var err error
	switch version {
	case userop.V0_7:
		_, err = v07sim.TraceSimulateValidation(&v07sim.TraceInput{
			Rpc:                rpc,
			EntryPoint:         entryPoint,
			Op:                 v06Op,
			ChainID:            chainID,
			IsRIP7212Supported: isRIP7212Supported,
			Tracer:             tracer,
			Stakes:             stakes,
			AltMempools:        alt,
		})
	case userop.V0_8:
		_, err = v08sim.TraceSimulateValidation(&v08sim.TraceInput{
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
	case userop.V0_9:
		_, err = v09sim.TraceSimulateValidation(&v09sim.TraceInput{
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
	default:
		_, err = simulation.TraceSimulateValidation(&simulation.TraceInput{
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
	}
	if err != nil {
		return errors.NewRPCError(errors.BANNED_OPCODE, err.Error(), err.Error())
	}

	// Validate additional ERC-7562 rules
	if err := ValidateERC7562Rules(op, entryPoint); err != nil {
		return errors.NewRPCError(errors.BANNED_OPCODE, err.Error(), err.Error())
	}

	// Validate time range (validAfter/validUntil) with block number support
	if err := validateTimeRangeFromTrace(rpc, entryPoint, op, chainID); err != nil {
		return err
	}

	return nil
}

// validateTimeRangeFromTrace fetches current block/time and validates validAfter/validUntil.
func validateTimeRangeFromTrace(
	rpc *rpc.Client,
	entryPoint common.Address,
	op userop.UserOperationInterface,
	chainID *big.Int,
) error {
	// Only validate for v0.7+ where validAfter/validUntil is supported.
	version := userop.DetectVersionFromEntryPoint(entryPoint)
	if version < userop.V0_7 {
		return nil
	}

	// Call SimulateValidation to get ReturnInfo with validAfter/validUntil.
	// This is a best-effort check; if simulation fails, we skip time-range
	// validation since the main validation path will catch errors.
	simOut, err := v09sim.SimulateValidation(&v09sim.SimulateInput{
		Rpc:        rpc,
		EntryPoint: entryPoint,
		Op:         op,
		ChainID:    chainID,
	})
	if err != nil {
		// Time-range validation skipped: SimulateValidation failed.
		return nil
	}
	if simOut == nil || simOut.Result == nil || simOut.Result.ReturnInfo == nil {
		// Time-range validation skipped: no ReturnInfo available.
		return nil
	}

	ri := simOut.Result.ReturnInfo
	if ri.ValidAfter == nil && ri.ValidUntil == nil {
		// No time restrictions set.
		return nil
	}

	// Fetch current block number and time.
	eth := ethclient.NewClient(rpc)
	currentBlock, err := eth.BlockNumber(context.Background())
	if err != nil {
		// Time-range validation skipped: unable to fetch current block number.
		return nil
	}
	currentTime := time.Now().Unix()

	if err := ValidateTimeRange(ri.ValidAfter, ri.ValidUntil, currentBlock, currentTime); err != nil {
		return errors.NewRPCError(errors.SHORT_DEADLINE, err.Error(), nil)
	}

	return nil
}
