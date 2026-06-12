// Package entities implements modules for reputation scoring and throttling/banning of entities as specified
// in EIP-4337.
package entities

import (
	stdErr "errors"
	"fmt"
	"strings"

	"github.com/dgraph-io/badger/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zarbanio/bundler/internal/dbutils"
	"github.com/zarbanio/bundler/pkg/entrypoint/stake"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/modules"
)

// Reputation provides Client and Bundler modules to track the reputation of every entity seen in a
// UserOperation.
type Reputation struct {
	db       *badger.DB
	eth      *ethclient.Client
	repConst *ReputationConstants
}

// New returns an instance of a Reputation object to track and appropriately process userOps by entity status.
func New(db *badger.DB, eth *ethclient.Client, repConst *ReputationConstants) *Reputation {
	return &Reputation{db, eth, repConst}
}

// CheckStatus returns a UserOpHandler that is used by the Client to determine if the userOp is allowed based
// on the entities status.
//  1. ok: entity is allowed
//  2. throttled: No new ops from the entity is allowed if one already exists. And it can only stays in
//     the pool for 10 blocks
//  3. banned: No ops from the entity is allowed
func (r *Reputation) CheckStatus() modules.UserOpHandlerFunc {
	return func(ctx *modules.UserOpHandlerCtx) error {
		return r.db.Update(func(txn *badger.Txn) error {
			if status, err := getStatus(txn, ctx.UserOp.GetSender(), r.repConst); err != nil {
				return err
			} else if status == banned {
				return errors.NewRPCError(
					errors.BANNED_OR_THROTTLED_ENTITY,
					fmt.Sprintf("banned entity: %s", ctx.UserOp.GetSender().Hex()),
					nil,
				)
			} else if status == throttled && len(ctx.GetPendingSenderOps()) == r.repConst.ThrottledEntityMempoolCount {
				return errors.NewRPCError(
					errors.BANNED_OR_THROTTLED_ENTITY,
					fmt.Sprintf("throttled entity: %s", ctx.UserOp.GetSender().Hex()),
					nil,
				)
			}

			factory := ctx.UserOp.GetFactory()
			if factory != common.HexToAddress("0x") {
				if status, err := getStatus(txn, factory, r.repConst); err != nil {
					return err
				} else if status == banned {
					return errors.NewRPCError(
						errors.BANNED_OR_THROTTLED_ENTITY,
						fmt.Sprintf("banned entity: %s", factory.Hex()),
						nil,
					)
				} else if status == throttled && len(ctx.GetPendingFactoryOps()) == r.repConst.ThrottledEntityMempoolCount {
					return errors.NewRPCError(
						errors.BANNED_OR_THROTTLED_ENTITY,
						fmt.Sprintf("throttled entity: %s", factory.Hex()),
						nil,
					)
				}
			}

			paymaster := ctx.UserOp.GetPaymaster()
			if paymaster != common.HexToAddress("0x") {
				if status, err := getStatus(txn, paymaster, r.repConst); err != nil {
					return err
				} else if status == banned {
					return errors.NewRPCError(
						errors.BANNED_OR_THROTTLED_ENTITY,
						fmt.Sprintf("banned entity: %s", paymaster.Hex()),
						nil,
					)
				} else if status == throttled && len(ctx.GetPendingPaymasterOps()) == r.repConst.ThrottledEntityMempoolCount {
					return errors.NewRPCError(
						errors.BANNED_OR_THROTTLED_ENTITY,
						fmt.Sprintf("throttled entity: %s", paymaster.Hex()),
						nil,
					)
				}
			}

			return nil
		})
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
		return r.db.Update(func(txn *badger.Txn) error {
			var err error
			// When this op replaces a pending one (same sender and nonce),
			// the previous op's increments are reverted so replacement does
			// not inflate opsSeen and a swapped paymaster/factory does not
			// keep the old entity's count.
			for _, pending := range ctx.GetPendingSenderOps() {
				if pending.Nonce.Cmp(ctx.UserOp.GetNonce()) != 0 {
					continue
				}
				if sd := ctx.GetSenderDepositInfo(); sd != nil && sd.Staked {
					err = stdErr.Join(err, decrementOpsSeenByEntity(txn, pending.Sender))
				}
				if oldFactory := pending.GetFactory(); oldFactory != common.HexToAddress("0x") {
					err = stdErr.Join(err, decrementOpsSeenByEntity(txn, oldFactory))
				}
				if oldPaymaster := pending.GetPaymaster(); oldPaymaster != common.HexToAddress("0x") {
					err = stdErr.Join(err, decrementOpsSeenByEntity(txn, oldPaymaster))
				}
				break
			}

			// EREP-030: only a staked account is accountable for failures, so
			// an unstaked sender's opsSeen is not tracked.
			if sd := ctx.GetSenderDepositInfo(); sd != nil && sd.Staked {
				err = stdErr.Join(err, incrementOpsSeenByEntity(txn, ctx.UserOp.GetSender()))
			}

			factory := ctx.UserOp.GetFactory()
			if factory != common.HexToAddress("0x") {
				err = stdErr.Join(err, incrementOpsSeenByEntity(txn, factory))
			}

			paymaster := ctx.UserOp.GetPaymaster()
			if paymaster != common.HexToAddress("0x") {
				err = stdErr.Join(err, incrementOpsSeenByEntity(txn, paymaster))
			}

			return err
		})
	}
}

// IncOpsIncluded returns a BatchHandler used by the Bundler to increment opsIncluded counters for all
// relevant entities in the batch. This module should be used last once batches have been sent.
func (r *Reputation) IncOpsIncluded() modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		return r.db.Update(func(txn *badger.Txn) error {
			c := make(addressCounter)
			for _, op := range ctx.Batch {
				if _, ok := c[op.Sender]; !ok {
					c[op.Sender] = 0
				}
				c[op.Sender]++

				factory := op.GetFactory()
				if factory != common.HexToAddress("0x") {
					if _, ok := c[factory]; !ok {
						c[factory] = 0
					}

					c[factory]++
				}

				paymaster := op.GetPaymaster()
				if paymaster != common.HexToAddress("0x") {
					if _, ok := c[paymaster]; !ok {
						c[paymaster] = 0
					}

					c[paymaster]++
				}
			}

			return incrementOpsIncludedByEntity(txn, c)
		})
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
		return r.db.Update(func(txn *badger.Txn) error {
			for _, item := range ctx.PendingRemoval {
				accountOrFactoryFault := strings.Contains(item.Reason, "AA1") ||
					strings.Contains(item.Reason, "AA2")
				if !accountOrFactoryFault {
					// EREP-030: a staked account is accountable for failures
					// in other entities, so a paymaster fault (AA3*) is
					// blamed on a staked sender and the paymaster's opsSeen
					// is rolled back.
					paymasterFault := strings.Contains(item.Reason, "AA3") ||
						strings.Contains(item.Reason, "insufficient paymaster deposit")
					if paymasterFault {
						paymaster := item.Op.GetPaymaster()
						if paymaster != common.HexToAddress("0x") {
							dep, err := gs(ctx.EntryPoint, item.Op.Sender)
							if err == nil && dep != nil && dep.Staked {
								if err := decrementOpsSeenByEntity(txn, paymaster); err != nil {
									return err
								}
							}
						}
					}
					continue
				}

				paymaster := item.Op.GetPaymaster()
				if paymaster != common.HexToAddress("0x") {
					if err := decrementOpsSeenByEntity(txn, paymaster); err != nil {
						return err
					}
				}

				factory := item.Op.GetFactory()
				if factory != common.HexToAddress("0x") && strings.Contains(item.Reason, "AA2") {
					dep, err := gs(ctx.EntryPoint, factory)
					if err == nil && dep != nil && dep.Staked {
						if err := addOpsSeenByEntity(txn, factory, crashedHandleOpsPenalty); err != nil {
							return err
						}
					}
				}
			}
			return nil
		})
	}
}

// Dump returns the reputation entry of every known entity in the shape
// expected by debug_bundler_dumpReputation.
func (r *Reputation) Dump() ([]map[string]any, error) {
	res := []map[string]any{}
	err := r.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		prefix := []byte(opsCountPrefix)
		addrs := []common.Address{}
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			vals := dbutils.SplitValues(string(it.Item().Key()))
			addrs = append(addrs, common.HexToAddress(vals[len(vals)-1]))
		}
		it.Close()

		for _, addr := range addrs {
			opsSeen, opsIncluded, err := getOpsCountByEntity(txn, addr)
			if err != nil {
				return err
			}
			st, err := getStatus(txn, addr, r.repConst)
			if err != nil {
				return err
			}
			res = append(res, map[string]any{
				"address":     addr.String(),
				"opsSeen":     hexutil.EncodeUint64(uint64(opsSeen)),
				"opsIncluded": hexutil.EncodeUint64(uint64(opsIncluded)),
				"status":      int(st),
			})
		}
		return nil
	})
	return res, err
}

// Clear removes the reputation entries of all known entities.
func (r *Reputation) Clear() error {
	return r.db.DropPrefix([]byte(opsCountPrefix))
}

func (r *Reputation) Override(entries []*ReputationOverride) error {
	return r.db.Update(func(txn *badger.Txn) error {
		var err error
		for _, entry := range entries {
			stdErr.Join(err, overrideEntity(txn, entry))
		}
		return err
	})
}
