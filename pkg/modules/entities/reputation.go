// Package entities implements modules for reputation scoring and throttling/banning of entities as specified
// in EIP-4337.
package entities

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zarbanio/bundler/pkg/entrypoint/stake"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/store"
)

// Reputation provides Client and Bundler modules to track the reputation of every entity seen in a
// UserOperation. Counters live in a store.ReputationStore, which may be node-local or shared.
type Reputation struct {
	store    store.ReputationStore
	eth      *ethclient.Client
	repConst *ReputationConstants
}

// New returns an instance of a Reputation object to track and appropriately process userOps by entity status.
func New(repStore store.ReputationStore, eth *ethclient.Client, repConst *ReputationConstants) *Reputation {
	return &Reputation{store: repStore, eth: eth, repConst: repConst}
}

func (r *Reputation) params() store.ReputationParams {
	return store.ReputationParams{
		MinInclusionRateDenominator: r.repConst.MinInclusionRateDenominator,
		ThrottlingSlack:             r.repConst.ThrottlingSlack,
		BanSlack:                    r.repConst.BanSlack,
	}
}

// CheckStatus returns a UserOpHandler that is used by the Client to determine if the userOp is allowed based
// on the entities status.
//  1. ok: entity is allowed
//  2. throttled: No new ops from the entity is allowed if one already exists. And it can only stays in
//     the pool for 10 blocks
//  3. banned: No ops from the entity is allowed
func (r *Reputation) CheckStatus() modules.UserOpHandlerFunc {
	return func(ctx *modules.UserOpHandlerCtx) error {
		bctx := context.Background()
		params := r.params()

		check := func(addr common.Address, pendingCount int) error {
			status, err := r.store.Status(bctx, addr, params)
			if err != nil {
				return err
			}
			switch status {
			case store.StatusBanned:
				return errors.NewRPCError(
					errors.BANNED_OR_THROTTLED_ENTITY,
					fmt.Sprintf("banned entity: %s", addr.Hex()),
					nil,
				)
			case store.StatusThrottled:
				if pendingCount == r.repConst.ThrottledEntityMempoolCount {
					return errors.NewRPCError(
						errors.BANNED_OR_THROTTLED_ENTITY,
						fmt.Sprintf("throttled entity: %s", addr.Hex()),
						nil,
					)
				}
			}
			return nil
		}

		if err := check(ctx.UserOp.GetSender(), len(ctx.GetPendingSenderOps())); err != nil {
			return err
		}
		if factory := ctx.UserOp.GetFactory(); factory != common.HexToAddress("0x") {
			if err := check(factory, len(ctx.GetPendingFactoryOps())); err != nil {
				return err
			}
		}
		if paymaster := ctx.UserOp.GetPaymaster(); paymaster != common.HexToAddress("0x") {
			if err := check(paymaster, len(ctx.GetPendingPaymasterOps())); err != nil {
				return err
			}
		}
		return nil
	}
}

// ValidateOpLimit returns a UserOpHandler that is used by the Client to determine if the userOp is allowed
// based on the entities stake and the number of pending ops in the mempool.
func (r *Reputation) ValidateOpLimit() modules.UserOpHandlerFunc {
	return func(ctx *modules.UserOpHandlerCtx) error {
		pso := ctx.GetPendingSenderOps()
		sd := ctx.GetSenderDepositInfo()
		if !sd.Staked && len(pso) == r.repConst.SameSenderMempoolCount {
			return errors.NewRPCError(
				errors.INVALID_ENTITY_STAKE,
				fmt.Sprintf(
					"unstaked entity: %s exceeds pending ops limit of %d",
					ctx.UserOp.GetSender().Hex(),
					r.repConst.SameSenderMempoolCount,
				),
				nil,
			)
		}

		factory := ctx.UserOp.GetFactory()
		if factory != common.HexToAddress("0x") {
			pfo := ctx.GetPendingFactoryOps()
			fd := ctx.GetFactoryDepositInfo()
			if !fd.Staked && len(pfo) == r.repConst.SameUnstakedEntityMempoolCount {
				return errors.NewRPCError(
					errors.INVALID_ENTITY_STAKE,
					fmt.Sprintf(
						"unstaked entity: %s exceeds pending ops limit of %d",
						factory.Hex(),
						r.repConst.SameUnstakedEntityMempoolCount,
					),
					nil,
				)
			}
		}

		paymaster := ctx.UserOp.GetPaymaster()
		if paymaster != common.HexToAddress("0x") {
			ppo := ctx.GetPendingPaymasterOps()
			pd := ctx.GetPaymasterDepositInfo()
			if !pd.Staked && len(ppo) == r.repConst.SameUnstakedEntityMempoolCount {
				return errors.NewRPCError(
					errors.INVALID_ENTITY_STAKE,
					fmt.Sprintf(
						"unstaked entity: %s exceeds pending ops limit of %d",
						paymaster.Hex(),
						r.repConst.SameUnstakedEntityMempoolCount,
					),
					nil,
				)
			}
		}

		return nil
	}
}

// IncOpsSeen returns a UserOpHandler that is used by the Client to increment the opsSeen counter for all
// included entities.
func (r *Reputation) IncOpsSeen() modules.UserOpHandlerFunc {
	return func(ctx *modules.UserOpHandlerCtx) error {
		deltas := []store.ReputationDelta{}

		// When this op replaces a pending one (same sender and nonce), the
		// previous op's increments are reverted so replacement does not inflate
		// opsSeen and a swapped paymaster/factory does not keep the old
		// entity's count.
		for _, pending := range ctx.GetPendingSenderOps() {
			if pending.Nonce.Cmp(ctx.UserOp.GetNonce()) != 0 {
				continue
			}
			if sd := ctx.GetSenderDepositInfo(); sd != nil && sd.Staked {
				deltas = append(deltas, store.ReputationDelta{Address: pending.Sender, OpsSeen: -1})
			}
			if oldFactory := pending.GetFactory(); oldFactory != common.HexToAddress("0x") {
				deltas = append(deltas, store.ReputationDelta{Address: oldFactory, OpsSeen: -1})
			}
			if oldPaymaster := pending.GetPaymaster(); oldPaymaster != common.HexToAddress("0x") {
				deltas = append(deltas, store.ReputationDelta{Address: oldPaymaster, OpsSeen: -1})
			}
			break
		}

		// EREP-030: only a staked account is accountable for failures, so an
		// unstaked sender's opsSeen is not tracked.
		if sd := ctx.GetSenderDepositInfo(); sd != nil && sd.Staked {
			deltas = append(deltas, store.ReputationDelta{Address: ctx.UserOp.GetSender(), OpsSeen: 1})
		}
		if factory := ctx.UserOp.GetFactory(); factory != common.HexToAddress("0x") {
			deltas = append(deltas, store.ReputationDelta{Address: factory, OpsSeen: 1})
		}
		if paymaster := ctx.UserOp.GetPaymaster(); paymaster != common.HexToAddress("0x") {
			deltas = append(deltas, store.ReputationDelta{Address: paymaster, OpsSeen: 1})
		}

		return r.store.Apply(context.Background(), deltas...)
	}
}

// IncOpsIncluded returns a BatchHandler used by the Bundler to increment opsIncluded counters for all
// relevant entities in the batch. This module should be used last once batches have been sent.
func (r *Reputation) IncOpsIncluded() modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		counts := map[common.Address]int{}
		for _, op := range ctx.Batch {
			counts[op.Sender]++
			if factory := op.GetFactory(); factory != common.HexToAddress("0x") {
				counts[factory]++
			}
			if paymaster := op.GetPaymaster(); paymaster != common.HexToAddress("0x") {
				counts[paymaster]++
			}
		}

		deltas := make([]store.ReputationDelta, 0, len(counts))
		for addr, n := range counts {
			deltas = append(deltas, store.ReputationDelta{Address: addr, OpsIncluded: n})
		}
		return r.store.Apply(context.Background(), deltas...)
	}
}

// crashedHandleOpsPenalty is the opsSeen penalty applied to an entity blamed
// for a userOp that crashed bundle creation, effectively banning it.
const crashedHandleOpsPenalty = 10000

// ProcessDrops returns a BatchHandler that applies the extended reputation
// rules to ops dropped during bundling (ctx.PendingRemoval):
//   - EREP-015: a paymaster's opsSeen is rolled back when the drop was caused
//     by a factory or account failure (AA1*/AA2* reasons), so it is not
//     penalized for failures that are not its own.
//   - EREP-020: a staked factory is accountable for its account's failure and
//     takes the crashed-handleOps penalty.
func (r *Reputation) ProcessDrops(gs stake.GetStakeFunc) modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		deltas := []store.ReputationDelta{}
		applyDelta := func(addr common.Address, d store.ReputationDelta) {
			d.Address = addr
			deltas = append(deltas, d)
		}

		for _, item := range ctx.PendingRemoval {
			accountOrFactoryFault := strings.Contains(item.Reason, "AA1") ||
				strings.Contains(item.Reason, "AA2")
			if !accountOrFactoryFault {
				// EREP-030: a staked account is accountable for failures in
				// other entities, so a paymaster fault (AA3*) is blamed on a
				// staked sender and the paymaster's opsSeen is rolled back.
				paymasterFault := strings.Contains(item.Reason, "AA3") ||
					strings.Contains(item.Reason, "insufficient paymaster deposit")
				if paymasterFault {
					paymaster := item.Op.GetPaymaster()
					if paymaster != common.HexToAddress("0x") {
						dep, err := gs(ctx.EntryPoint, item.Op.Sender)
						if err == nil && dep != nil && dep.Staked {
							applyDelta(paymaster, store.ReputationDelta{OpsSeen: -1})
						}
					}
				}
				continue
			}

			if paymaster := item.Op.GetPaymaster(); paymaster != common.HexToAddress("0x") {
				applyDelta(paymaster, store.ReputationDelta{OpsSeen: -1})
			}

			factory := item.Op.GetFactory()
			if factory != common.HexToAddress("0x") && strings.Contains(item.Reason, "AA2") {
				dep, err := gs(ctx.EntryPoint, factory)
				if err == nil && dep != nil && dep.Staked {
					applyDelta(factory, store.ReputationDelta{OpsSeen: crashedHandleOpsPenalty})
				}
			}
		}
		return r.store.Apply(context.Background(), deltas...)
	}
}

// Dump returns the reputation entry of every known entity in the shape
// expected by debug_bundler_dumpReputation.
func (r *Reputation) Dump() ([]map[string]any, error) {
	entries, err := r.store.Dump(context.Background())
	if err != nil {
		return nil, err
	}
	res := []map[string]any{}
	for _, e := range entries {
		status, err := r.store.Status(context.Background(), e.Address, r.params())
		if err != nil {
			return nil, err
		}
		res = append(res, map[string]any{
			"address":     e.Address.String(),
			"opsSeen":     hexutil.EncodeUint64(uint64(e.OpsSeen)),
			"opsIncluded": hexutil.EncodeUint64(uint64(e.OpsIncluded)),
			"status":      int(status),
		})
	}
	return res, nil
}

// Clear removes the reputation entries of all known entities.
func (r *Reputation) Clear() error {
	return r.store.Clear(context.Background())
}

// Override replaces the reputation counters of the given entities.
func (r *Reputation) Override(entries []*ReputationOverride) error {
	converted := make([]store.ReputationEntry, 0, len(entries))
	for _, e := range entries {
		converted = append(converted, store.ReputationEntry{
			Address:     e.Address,
			OpsSeen:     int(e.OpsSeen),
			OpsIncluded: int(e.OpsIncluded),
		})
	}
	return r.store.Override(context.Background(), converted)
}
