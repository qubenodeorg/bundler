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
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/modules/entities"

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
				return ValidatePaymaster(op, ctx.GetPaymasterDepositInfo(), gc, ctx.Version)
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
			g.Go(func() error {
				return ValidatePaymasterSignature(op, ctx.Version)
			})
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
				return ValidatePaymaster(op, ctx.GetPaymasterDepositInfo(), gc, ctx.Version)
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
			g.Go(func() error { return ValidateCallGasLimitV07(op, s.ov) })
			g.Go(func() error { return ValidateFeePerGasV07(op, gasprice.GetBaseFeeWithEthClient(s.eth)) })
			g.Go(func() error { return ValidatePendingOpsV07(op, ctx.GetPendingSenderOps()) })
			g.Go(func() error { return ValidateGasAvailableV07(op, s.ov, s.maxBatchGasLimit) })
		}

		if err := g.Wait(); err != nil {
			return errors.NewRPCError(errors.INVALID_FIELDS, err.Error(), err.Error())
		}
		return nil
	}
}

// SimulateOp returns a UserOpHandler that runs through simulation of new UserOps with the EntryPoint.
func (s *Standalone) SimulateOp() modules.UserOpHandlerFunc {
	return func(ctx *modules.UserOpHandlerCtx) error {
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
		default:
			// v0.7+: off-chain simulation only (simulateValidation removed from deployed contract)
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
						ctx.UserOp.GetSender():   ctx.GetSenderDepositInfo(),
						ctx.UserOp.GetFactory():  ctx.GetFactoryDepositInfo(),
						ctx.UserOp.GetPaymaster(): ctx.GetPaymasterDepositInfo(),
					},
					ctx.Version,
				)
			})
		}

		return g.Wait()
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

// TODO: Implement
func (s *Standalone) SimulateBatch() modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		return nil
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
