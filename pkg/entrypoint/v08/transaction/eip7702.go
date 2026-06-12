package transaction

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/zarbanio/bundler/pkg/entrypoint/v08"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/userop"
)

// setCodeTxType is the EIP-7702 transaction type. go-ethereum v1.11 predates
// it, so the transaction is RLP-encoded and signed manually and submitted via
// eth_sendRawTransaction.
const setCodeTxType = byte(0x04)

// perAuthGasCost is PER_EMPTY_ACCOUNT_COST: the gas a type-4 transaction pays
// per authorization tuple on top of execution.
const perAuthGasCost = 25000

// BatchHasAuth reports whether any op in the batch carries an EIP-7702
// authorization, requiring a type-4 bundle transaction.
func BatchHasAuth(batch []*userop.UserOperation) bool {
	for _, op := range batch {
		if op.EIP7702Auth != nil {
			return true
		}
	}
	return false
}

// batchAuthorizations collects the authorization tuples in the batch, one per
// sender.
func batchAuthorizations(batch []*userop.UserOperation) []*userop.Authorization {
	seen := map[common.Address]bool{}
	auths := []*userop.Authorization{}
	for _, op := range batch {
		if op.EIP7702Auth == nil || seen[op.Sender] {
			continue
		}
		seen[op.Sender] = true
		auths = append(auths, op.EIP7702Auth)
	}
	return auths
}

// delegationOverrides applies each pending authorization's delegation
// designation (0xef0100 || delegate) to its sender, so eth_call and
// eth_estimateGas observe the state the type-4 transaction will create.
func delegationOverrides(batch []*userop.UserOperation) state.OverrideSet {
	var os state.OverrideSet
	for _, op := range batch {
		if op.EIP7702Auth == nil {
			continue
		}
		code := append([]byte{0xef, 0x01, 0x00}, op.EIP7702Auth.Address.Bytes()...)
		os = state.WithCodeOverride(op.Sender, code, os)
	}
	return os
}

// estimateHandleOps7702Gas estimates the handleOps calldata with the pending
// delegations applied, plus the per-authorization overhead of the type-4
// transaction itself.
func estimateHandleOps7702Gas(opts *Opts, data []byte) (uint64, error) {
	if opts.Rpc == nil {
		return 0, errors.New("transaction: rpc client required for EIP-7702 bundles")
	}
	msg := map[string]interface{}{
		"from": opts.EOA.Address,
		"to":   opts.EntryPoint,
		"data": hexutil.Encode(data),
	}
	var out hexutil.Uint64
	if err := opts.Rpc.CallContext(
		context.Background(), &out, "eth_estimateGas", msg, "latest", delegationOverrides(opts.Batch),
	); err != nil {
		return 0, err
	}
	return uint64(out) + perAuthGasCost*uint64(len(batchAuthorizations(opts.Batch))), nil
}

// HandleOps7702 submits the batch as an EIP-7702 type-4 transaction carrying
// the senders' authorization tuples, and waits for it to be mined when a
// WaitTimeout is set.
func HandleOps7702(opts *Opts) (common.Hash, error) {
	if opts.Rpc == nil {
		return common.Hash{}, errors.New("transaction: rpc client required for EIP-7702 bundles")
	}

	parsed, err := v08.V08MetaData.GetAbi()
	if err != nil {
		return common.Hash{}, err
	}
	data, err := parsed.Pack("handleOps", toPackedBatch(opts.Batch), opts.Beneficiary)
	if err != nil {
		return common.Hash{}, err
	}

	nonce, err := opts.Eth.NonceAt(context.Background(), opts.EOA.Address, nil)
	if err != nil {
		return common.Hash{}, err
	}

	var tip, feeCap *big.Int
	if opts.BaseFee != nil && opts.Tip != nil {
		tip = SuggestMeanGasTipCap(opts.Tip, opts.Batch)
		feeCap = SuggestMeanGasFeeCap(opts.BaseFee, opts.Tip, opts.Batch)
	} else if opts.GasPrice != nil {
		tip = SuggestMeanGasPrice(opts.GasPrice, opts.Batch)
		feeCap = tip
	} else {
		return common.Hash{}, errors.New("transaction: either the dynamic or legacy gas fees must be set")
	}

	authList := make([]interface{}, 0)
	for _, a := range batchAuthorizations(opts.Batch) {
		yParity := uint64(a.YParity)
		if yParity >= 27 {
			yParity -= 27
		}
		authList = append(authList, []interface{}{a.ChainID, a.Address, a.Nonce, yParity, a.R, a.S})
	}

	payload := []interface{}{
		opts.ChainID,
		nonce,
		tip,
		feeCap,
		opts.GasLimit,
		opts.EntryPoint,
		big.NewInt(0),
		data,
		[]interface{}{}, // access list
		authList,
	}
	unsigned, err := rlp.EncodeToBytes(payload)
	if err != nil {
		return common.Hash{}, err
	}
	sig, err := crypto.Sign(crypto.Keccak256(append([]byte{setCodeTxType}, unsigned...)), opts.EOA.PrivateKey)
	if err != nil {
		return common.Hash{}, err
	}

	signedPayload := append(payload,
		uint64(sig[64]),
		new(big.Int).SetBytes(sig[:32]),
		new(big.Int).SetBytes(sig[32:64]),
	)
	signed, err := rlp.EncodeToBytes(signedPayload)
	if err != nil {
		return common.Hash{}, err
	}
	raw := append([]byte{setCodeTxType}, signed...)
	txHash := common.BytesToHash(crypto.Keccak256(raw))

	if err := opts.Rpc.CallContext(
		context.Background(), nil, "eth_sendRawTransaction", hexutil.Encode(raw),
	); err != nil {
		return common.Hash{}, err
	}

	if opts.WaitTimeout == 0 {
		return txHash, nil
	}
	deadline := time.Now().Add(opts.WaitTimeout)
	for time.Now().Before(deadline) {
		receipt, rErr := opts.Eth.TransactionReceipt(context.Background(), txHash)
		if rErr == nil && receipt != nil {
			if receipt.Status == types.ReceiptStatusFailed {
				return txHash, errors.New("transaction: failed status")
			}
			return txHash, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return txHash, fmt.Errorf("transaction: not mined within %s", opts.WaitTimeout)
}
