// Package checks implements modules for running an array of standard validations for both the Client and
// Bundler.
package checks

import (
	"math/big"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/zarbanio/bundler/pkg/altmempools"
	"github.com/zarbanio/bundler/pkg/entrypoint"
	"github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/entrypoint/stake"
	v07sim "github.com/zarbanio/bundler/pkg/entrypoint/v07/simulation"
	v08sim "github.com/zarbanio/bundler/pkg/entrypoint/v08/simulation"
	v09sim "github.com/zarbanio/bundler/pkg/entrypoint/v09/simulation"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/modules/entities"
	"github.com/zarbanio/bundler/pkg/tracer"

	"github.com/zarbanio/bundler/pkg/modules/gasprice"
	"github.com/zarbanio/bundler/pkg/userop"
	"golang.org/x/sync/errgroup"
)

// Standalone exposes modules to perform basic Client and Bundler checks as specified in EIP-4337. It is
// intended for bundlers that are independent of an Ethereum node and hence relies on a given ethClient to
// query blockchain state.
type Standalone struct {
	db                 *badger.DB
	rpc                *rpc.Client
	eth                *ethclient.Client
	ov                 *gas.Overhead
	alt                *altmempools.Directory
	maxVerificationGas *big.Int
	maxBatchGasLimit   *big.Int
	isRIP7212Supported bool
	tracer             string
	repConst           *entities.ReputationConstants
	version            userop.Version
}

// New returns a Standalone instance with methods that can be used in Client and Bundler modules to perform
// standard checks as specified in EIP-4337.
func New(
	db *badger.DB,
	rpc *rpc.Client,
	ov *gas.Overhead,
	alt *altmempools.Directory,
	maxVerificationGas *big.Int,
	maxBatchGasLimit *big.Int,
	isRIP7212Supported bool,
	tracer string,
	repConst *entities.ReputationConstants,
) *Standalone {
	eth := ethclient.NewClient(rpc)
	return &Standalone{
		db,
		rpc,
		eth,
		ov,
		alt,
		maxVerificationGas,
		maxBatchGasLimit,
		isRIP7212Supported,
		tracer,
		repConst,
		ov.GetVersion(),
	}
}

// ValidateOpValues returns a UserOpHandler that runs through some first line sanity checks for new UserOps
// received by the Client. This should be one of the first modules executed by the Client.
func (s *Standalone) ValidateOpValues() modules.UserOpHandlerFunc {
	return func(ctx *modules.UserOpHandlerCtx) error {
		gc := getCodeWithEthClient(s.eth)

		g := new(errgroup.Group)

		switch ctx.Version {
		case userop.V0_6:
			op := ctx.UserOp.(*userop.UserOperation)
			g.Go(func() error { return ValidateSender(op, gc) })
			g.Go(func() error { return ValidateInitCode(op) })
			g.Go(func() error { return ValidateVerificationGas(op, s.ov, s.maxVerificationGas) })
			g.Go(func() error { return ValidatePaymasterAndData(op, ctx.GetPaymasterDepositInfo(), gc) })
			g.Go(func() error { return ValidateCallGasLimit(op, s.ov) })
			g.Go(func() error { return ValidateFeePerGas(op, gasprice.GetBaseFeeWithEthClient(s.eth)) })
			g.Go(func() error { return ValidatePendingOps(op, ctx.GetPendingSenderOps()) })
			g.Go(func() error { return ValidateGasAvailable(op, s.maxBatchGasLimit) })
		case userop.V0_9:
			// v0.9 allows initCode with existing accounts
			op := ctx.UserOp
			g.Go(func() error { return ValidateSenderV09(op, gc) })
			g.Go(func() error { return ValidateFactory(op) })
			g.Go(func() error { return ValidateFactoryData(op) })
			g.Go(func() error {
				return ValidateVerificationGasV07(op, s.ov, s.maxVerificationGas)
			})
			g.Go(func() error {
				return ValidatePaymaster(op, ctx.GetPaymasterDepositInfo(), gc, ctx.GetPendingPaymasterOps(), ctx.Version)
			})
			g.Go(func() error {
				return ValidatePaymasterData(op, ctx.Version)
			})
			g.Go(func() error {
				return ValidatePaymasterVerificationGasLimit(op, s.maxVerificationGas)
			})
			g.Go(func() error {
				return ValidatePaymasterPostOpGasLimit(op)
			})
			g.Go(func() error { return ValidateEIP7702Auth(op, ctx.ChainID) })
			g.Go(func() error { return ValidateCallGasLimitV07(op, s.ov) })
			g.Go(func() error { return ValidateFeePerGasV07(op, gasprice.GetBaseFeeWithEthClient(s.eth)) })
			g.Go(func() error { return ValidatePendingOpsV07(op, ctx.GetPendingSenderOps()) })
			g.Go(func() error { return ValidateGasAvailableV07(op, s.ov, s.maxBatchGasLimit) })
		default:
			// v0.7/v0.8 use the interface-based validation functions
			op := ctx.UserOp
			g.Go(func() error { return ValidateSenderV07(op, gc) })
			g.Go(func() error { return ValidateFactory(op) })
			g.Go(func() error { return ValidateFactoryData(op) })
			g.Go(func() error {
				return ValidateVerificationGasV07(op, s.ov, s.maxVerificationGas)
			})
			g.Go(func() error {
				return ValidatePaymaster(op, ctx.GetPaymasterDepositInfo(), gc, ctx.GetPendingPaymasterOps(), ctx.Version)
			})
			g.Go(func() error {
				return ValidatePaymasterData(op, ctx.Version)
			})
			g.Go(func() error {
				return ValidatePaymasterVerificationGasLimit(op, s.maxVerificationGas)
			})
			g.Go(func() error {
				return ValidatePaymasterPostOpGasLimit(op)
			})
			g.Go(func() error { return ValidateEIP7702Auth(op, ctx.ChainID) })
			g.Go(func() error { return ValidateCallGasLimitV07(op, s.ov) })
			g.Go(func() error { return ValidateFeePerGasV07(op, gasprice.GetBaseFeeWithEthClient(s.eth)) })
			g.Go(func() error { return ValidatePendingOpsV07(op, ctx.GetPendingSenderOps()) })
			g.Go(func() error { return ValidateGasAvailableV07(op, s.ov, s.maxBatchGasLimit) })
		}

		if err := g.Wait(); err != nil {
			// Checks that already produced a spec error code (e.g. paymaster
			// deposit too low) keep it; plain errors map to INVALID_FIELDS.
			if rpcErr, ok := err.(*errors.RPCError); ok {
				return rpcErr
			}
			return errors.NewRPCError(errors.INVALID_FIELDS, err.Error(), err.Error())
		}
		return nil
	}
}

// SimulateOp returns a UserOpHandler that runs through simulation of new UserOps with the EntryPoint.
func (s *Standalone) SimulateOp() modules.UserOpHandlerFunc {
	return func(ctx *modules.UserOpHandlerCtx) error {
		if err := ValidateOpEntityRoles(ctx); err != nil {
			return err
		}

		g := new(errgroup.Group)

		switch ctx.Version {
		case userop.V0_6:
			op := ctx.UserOp.(*userop.UserOperation)
			g.Go(func() error {
				sim, err := simulation.SimulateValidation(s.rpc, ctx.EntryPoint, op)
				if err != nil {
					return errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, err.Error(), err.Error())
				}
				if sim.ReturnInfo.SigFailed {
					return errors.NewRPCError(
						errors.INVALID_SIGNATURE,
						"Invalid UserOp signature or paymaster signature",
						nil,
					)
				}
				if sim.ReturnInfo.ValidUntil.Cmp(common.Big0) != 0 &&
					time.Now().Unix() >= sim.ReturnInfo.ValidUntil.Int64()-30 {
					return errors.NewRPCError(
						errors.SHORT_DEADLINE,
						"expires too soon",
						nil,
					)
				}
				return nil
			})
			g.Go(func() error {
				// Deep validation: trace simulateValidation and enforce the
				// ERC-7562 opcode/storage rules.
				_, err := simulation.TraceSimulateValidation(&simulation.TraceInput{
					Rpc:                s.rpc,
					EntryPoint:         ctx.EntryPoint,
					Op:                 op,
					ChainID:            ctx.ChainID,
					IsRIP7212Supported: s.isRIP7212Supported,
					Tracer:             s.tracer,
					AltMempools:        s.alt,
					Stakes: simulation.EntityStakes{
						op.Sender:         ctx.GetSenderDepositInfo(),
						op.GetFactory():   ctx.GetFactoryDepositInfo(),
						op.GetPaymaster(): ctx.GetPaymasterDepositInfo(),
					},
				})
				if err != nil {
					return errors.NewRPCError(errors.BANNED_OPCODE, err.Error(), err.Error())
				}
				return nil
			})
		default:
			// v0.7+: off-chain simulation only (simulateValidation removed from deployed contract)
			g.Go(func() error { return ValidateEIP7702AuthNonce(ctx.UserOp, s.eth) })
			g.Go(func() error {
				return SimulateValidationV07(
					s.rpc,
					ctx.EntryPoint,
					ctx.UserOp,
					ctx.ChainID,
					s.ov,
					s.alt,
					s.isRIP7212Supported,
					s.tracer,
					simulation.EntityStakes{
						ctx.UserOp.GetSender():    ctx.GetSenderDepositInfo(),
						ctx.UserOp.GetFactory():   ctx.GetFactoryDepositInfo(),
						ctx.UserOp.GetPaymaster(): ctx.GetPaymasterDepositInfo(),
					},
					ctx.Version,
				)
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}

		// simulateValidation stops after the validation phase, so
		// handleOps-only reverts (e.g. AA51 prefund below actualGasCost) need
		// a stateless dry run of the real call. Runs after the other checks so
		// their specific error codes (e.g. INVALID_SIGNATURE) take precedence.
		if ctx.Version == userop.V0_6 {
			op := ctx.UserOp.(*userop.UserOperation)
			if err := simulation.CallHandleOps(s.rpc, ctx.EntryPoint, op); err != nil {
				return errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, err.Error(), err.Error())
			}
		}

		return nil
	}
}

// CodeHashes returns a BatchHandler that verifies the code for any interacted contracts has not changed since
// the first simulation.
func (s *Standalone) CodeHashes() modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		gc := getCodeWithEthClient(s.eth)

		end := len(ctx.Batch) - 1
		for i := end; i >= 0; i-- {
			op := ctx.Batch[i]
			chs, err := getSavedCodeHashes(s.db, op.GetUserOpHash(ctx.EntryPoint, ctx.ChainID))
			if err != nil {
				return err
			}

			changed, err := hasCodeHashChanges(chs, gc)
			if err != nil {
				return err
			}
			if changed {
				ctx.MarkOpIndexForRemoval(i, "code hash changed")
			}
		}
		return nil
	}
}

// PaymasterDeposit returns a BatchHandler that tracks each paymaster in the batch and ensures it has enough
// deposit to pay for all the UserOps that use it.
func (s *Standalone) PaymasterDeposit() modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		ep, err := entrypoint.NewEntrypoint(ctx.EntryPoint, s.eth)
		if err != nil {
			return err
		}

		deps := make(map[common.Address]*big.Int)
		for i, op := range ctx.Batch {
			pm := op.GetPaymaster()
			if pm == common.HexToAddress("0x") {
				continue
			}

			if _, ok := deps[pm]; !ok {
				dep, err := ep.GetDepositInfo(nil, pm)
				if err != nil {
					return err
				}

				deps[pm] = dep.Deposit
			}

			deps[pm] = big.NewInt(0).Sub(deps[pm], op.GetMaxPrefund())
			if deps[pm].Cmp(common.Big0) < 0 {
				ctx.MarkOpIndexForRemoval(i, "insufficient paymaster deposit")
			}
		}

		return nil
	}
}

// SimulateBatch returns a BatchHandler that re-validates each op at bundle
// time and enforces cross-op bundle rules:
//
//  1. Only one op per sender is included per bundle; extras stay in the
//     mempool for a later bundle.
//  2. Ops that no longer pass trace validation (state changed since they were
//     accepted) are dropped from the mempool.
//  3. STO-041: an op whose validation accessed the sender (or its associated
//     storage) of another op in the bundle cannot share that bundle; the
//     later op stays in the mempool.
func (s *Standalone) SimulateBatch() modules.BatchHandlerFunc {
	gs := stake.GetStakeWithEthClient(s.eth)
	return func(ctx *modules.BatchHandlerCtx) error {
		keep := []*userop.UserOperation{}
		traces := []*tracer.BundlerCollectorReturn{}
		seen := map[common.Address]bool{}

		for _, op := range ctx.Batch {
			if seen[op.Sender] {
				continue
			}

			trace, err := s.traceForBatch(ctx, op, gs)
			if err != nil {
				ctx.PendingRemoval = append(ctx.PendingRemoval, &modules.PendingRemovalItem{
					Op:     op,
					Reason: err.Error(),
				})
				continue
			}

			conflict := false
			for i, prev := range keep {
				if simulation.TraceAccessesAddress(traces[i], op.Sender) ||
					simulation.TraceAccessesAddress(trace, prev.Sender) {
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}

			seen[op.Sender] = true
			keep = append(keep, op)
			traces = append(traces, trace)
		}

		ctx.Batch = keep
		return nil
	}
}

// traceForBatch runs the version-appropriate trace validation for a batched op
// and returns the raw collector trace.
func (s *Standalone) traceForBatch(
	ctx *modules.BatchHandlerCtx,
	op *userop.UserOperation,
	gs stake.GetStakeFunc,
) (*tracer.BundlerCollectorReturn, error) {
	stakes := simulation.EntityStakes{}
	for _, addr := range []common.Address{op.Sender, op.GetFactory(), op.GetPaymaster()} {
		if addr == common.HexToAddress("0x") {
			continue
		}
		dep, err := gs(ctx.EntryPoint, addr)
		if err != nil {
			return nil, err
		}
		stakes[addr] = dep
	}

	switch s.version {
	case userop.V0_7:
		out, err := v07sim.TraceSimulateValidation(&v07sim.TraceInput{
			Rpc:                s.rpc,
			EntryPoint:         ctx.EntryPoint,
			Op:                 op,
			ChainID:            ctx.ChainID,
			IsRIP7212Supported: s.isRIP7212Supported,
			Tracer:             s.tracer,
			Stakes:             stakes,
			AltMempools:        s.alt,
		})
		if err != nil {
			return nil, err
		}
		return out.Trace, nil
	case userop.V0_8:
		out, err := v08sim.TraceSimulateValidation(&v08sim.TraceInput{
			Rpc:                s.rpc,
			EntryPoint:         ctx.EntryPoint,
			Op:                 op,
			ChainID:            ctx.ChainID,
			IsRIP7212Supported: s.isRIP7212Supported,
			Tracer:             s.tracer,
			Stakes:             stakes,
			AltMempools:        s.alt,
			DelegateAddress:    GetEIP7702Address(op.GetEIP7702Auth()),
		})
		if err != nil {
			return nil, err
		}
		return out.Trace, nil
	case userop.V0_9:
		out, err := v09sim.TraceSimulateValidation(&v09sim.TraceInput{
			Rpc:                s.rpc,
			EntryPoint:         ctx.EntryPoint,
			Op:                 op,
			ChainID:            ctx.ChainID,
			IsRIP7212Supported: s.isRIP7212Supported,
			Tracer:             s.tracer,
			Stakes:             stakes,
			AltMempools:        s.alt,
			DelegateAddress:    GetEIP7702Address(op.GetEIP7702Auth()),
		})
		if err != nil {
			return nil, err
		}
		return out.Trace, nil
	default:
		out, err := simulation.TraceSimulateValidation(&simulation.TraceInput{
			Rpc:                s.rpc,
			EntryPoint:         ctx.EntryPoint,
			Op:                 op,
			ChainID:            ctx.ChainID,
			IsRIP7212Supported: s.isRIP7212Supported,
			Tracer:             s.tracer,
			Stakes:             stakes,
			AltMempools:        s.alt,
		})
		if err != nil {
			return nil, err
		}
		return out.Trace, nil
	}
}

// Clean returns a BatchHandler that clears the DB of data that is no longer required. This should be one of
// the last modules executed by the Bundler.
func (s *Standalone) Clean() modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		all := append([]*userop.UserOperation{}, ctx.Batch...)
		for _, item := range ctx.PendingRemoval {
			all = append(all, item.Op)
		}
		hashes := []common.Hash{}
		for _, op := range all {
			hashes = append(hashes, op.GetUserOpHash(ctx.EntryPoint, ctx.ChainID))
		}

		return removeSavedCodeHashes(s.db, hashes...)
	}
}
