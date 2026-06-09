// Package relay implements a module for private bundlers to send batches to the EntryPoint through regular
// EOA transactions.
package relay

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-logr/logr"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/transaction"
	v07txn "github.com/zarbanio/bundler/pkg/entrypoint/v07/transaction"
	v08txn "github.com/zarbanio/bundler/pkg/entrypoint/v08/transaction"
	v09txn "github.com/zarbanio/bundler/pkg/entrypoint/v09/transaction"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/signer"
	"github.com/zarbanio/bundler/pkg/userop"
)

// Relayer provides a module that can relay batches with a regular EOA. Relaying batches to the EntryPoint
// through a regular transaction comes with several important notes:
//
//   - The bundler will NOT be operating as a block builder.
//   - This opens the bundler up to frontrunning.
//
// This module only works in the case of a private mempool and will not work in the P2P case where ops are
// propagated through the network and it is impossible to prevent collisions from multiple bundlers trying to
// relay the same ops.
type Relayer struct {
	eoa         *signer.EOA
	eth         *ethclient.Client
	chainID     *big.Int
	beneficiary common.Address
	logger      logr.Logger
	waitTimeout time.Duration
}

// New initializes a new EOA relayer for sending batches to the EntryPoint.
func New(
	eoa *signer.EOA,
	eth *ethclient.Client,
	chainID *big.Int,
	beneficiary common.Address,
	l logr.Logger,
) *Relayer {
	return &Relayer{
		eoa:         eoa,
		eth:         eth,
		chainID:     chainID,
		beneficiary: beneficiary,
		logger:      l.WithName("relayer"),
		waitTimeout: DefaultWaitTimeout,
	}
}

// SetWaitTimeout sets the total time to wait for a transaction to be included. When a timeout is reached, the
// BatchHandler will throw an error if the transaction has not been included or has been included but with a
// failed status.
//
// The default value is 30 seconds. Setting the value to 0 will skip waiting for a transaction to be included.
func (r *Relayer) SetWaitTimeout(timeout time.Duration) {
	r.waitTimeout = timeout
}

// estimateHandleOpsGas dispatches to the version-appropriate EstimateHandleOpsGas implementation.
func estimateHandleOpsGas(
	version userop.Version,
	opts *transaction.Opts,
) (uint64, *reverts.FailedOpRevert, error) {
	switch version {
	case userop.V0_7:
		return v07txn.EstimateHandleOpsGas(toV07Opts(opts))
	case userop.V0_8:
		return v08txn.EstimateHandleOpsGas(toV08Opts(opts))
	case userop.V0_9:
		return v09txn.EstimateHandleOpsGas(toV09Opts(opts))
	default:
		return transaction.EstimateHandleOpsGas(opts)
	}
}

// handleOps dispatches to the version-appropriate HandleOps implementation. Returns an
// empty hash with no error if the underlying call returned a nil transaction without
// an error (which can happen in NoSend mode).
func handleOps(version userop.Version, opts *transaction.Opts) (txnHash string, err error) {
	var txn *types.Transaction
	switch version {
	case userop.V0_7:
		txn, err = v07txn.HandleOps(toV07Opts(opts))
	case userop.V0_8:
		txn, err = v08txn.HandleOps(toV08Opts(opts))
	case userop.V0_9:
		txn, err = v09txn.HandleOps(toV09Opts(opts))
	default:
		txn, err = transaction.HandleOps(opts)
	}
	if err != nil {
		return "", err
	}
	if txn == nil {
		return "", nil
	}
	return txn.Hash().String(), nil
}

func toV07Opts(o *transaction.Opts) *v07txn.Opts {
	return &v07txn.Opts{
		EOA:         o.EOA,
		Eth:         o.Eth,
		ChainID:     o.ChainID,
		EntryPoint:  o.EntryPoint,
		Batch:       o.Batch,
		Beneficiary: o.Beneficiary,
		BaseFee:     o.BaseFee,
		Tip:         o.Tip,
		GasPrice:    o.GasPrice,
		GasLimit:    o.GasLimit,
		NoSend:      o.NoSend,
		WaitTimeout: o.WaitTimeout,
	}
}

func toV08Opts(o *transaction.Opts) *v08txn.Opts {
	return &v08txn.Opts{
		EOA:         o.EOA,
		Eth:         o.Eth,
		ChainID:     o.ChainID,
		EntryPoint:  o.EntryPoint,
		Batch:       o.Batch,
		Beneficiary: o.Beneficiary,
		BaseFee:     o.BaseFee,
		Tip:         o.Tip,
		GasPrice:    o.GasPrice,
		GasLimit:    o.GasLimit,
		NoSend:      o.NoSend,
		WaitTimeout: o.WaitTimeout,
	}
}

func toV09Opts(o *transaction.Opts) *v09txn.Opts {
	return &v09txn.Opts{
		EOA:         o.EOA,
		Eth:         o.Eth,
		ChainID:     o.ChainID,
		EntryPoint:  o.EntryPoint,
		Batch:       o.Batch,
		Beneficiary: o.Beneficiary,
		BaseFee:     o.BaseFee,
		Tip:         o.Tip,
		GasPrice:    o.GasPrice,
		GasLimit:    o.GasLimit,
		NoSend:      o.NoSend,
		WaitTimeout: o.WaitTimeout,
	}
}

// SendUserOperation returns a BatchHandler that is used by the Bundler to send batches in a regular EOA
// transaction.
func (r *Relayer) SendUserOperation() modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		version := userop.DetectVersionFromEntryPoint(ctx.EntryPoint)
		opts := &transaction.Opts{
			EOA:         r.eoa,
			Eth:         r.eth,
			ChainID:     ctx.ChainID,
			EntryPoint:  ctx.EntryPoint,
			Batch:       ctx.Batch,
			Beneficiary: r.beneficiary,
			BaseFee:     ctx.BaseFee,
			Tip:         ctx.Tip,
			GasPrice:    ctx.GasPrice,
			GasLimit:    0,
			WaitTimeout: r.waitTimeout,
		}
		// Estimate gas for handleOps() and drop all userOps that cause unexpected reverts.
		for len(ctx.Batch) > 0 {
			prevLen := len(ctx.Batch)
			est, revert, err := estimateHandleOpsGas(version, opts)

			if err != nil {
				return err
			} else if revert != nil {
				ctx.MarkOpIndexForRemoval(revert.OpIndex, revert.Reason)
				// MarkOpIndexForRemoval is a no-op for out-of-range indexes. Without this
				// guard the loop would spin forever on repeated reverts.
				if len(ctx.Batch) == prevLen {
					return fmt.Errorf("relay: failed to remove reverted op at index %d (%s)", revert.OpIndex, revert.Reason)
				}
				opts.Batch = ctx.Batch
			} else {
				opts.GasLimit = est
				break
			}
		}

		// Call handleOps() with gas estimate. Any userOps that cause a revert at this stage will be
		// caught and dropped in the next iteration.
		if len(ctx.Batch) > 0 {
			opts.Batch = ctx.Batch
			txnHash, err := handleOps(version, opts)
			if err != nil {
				return err
			}
			ctx.Data["txn_hash"] = txnHash
		}

		return nil
	}
}
