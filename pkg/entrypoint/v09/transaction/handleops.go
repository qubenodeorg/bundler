package transaction

import (
	"context"
	"errors"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/v09"
	"github.com/zarbanio/bundler/pkg/signer"
	"github.com/zarbanio/bundler/pkg/userop"
)

type Opts struct {
	EOA         *signer.EOA
	Eth         *ethclient.Client
	ChainID     *big.Int
	EntryPoint  common.Address
	Batch       []*userop.UserOperation
	Beneficiary common.Address
	BaseFee     *big.Int
	Tip         *big.Int
	GasPrice    *big.Int
	GasLimit    uint64
	NoSend      bool
	WaitTimeout time.Duration
}

var maxUint128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

func toPackedBatch(batch []*userop.UserOperation) []v09.PackedUserOperation {
	ops := []v09.PackedUserOperation{}
	for _, op := range batch {
		ops = append(ops, v09.PackedUserOperation{
			Sender:             op.Sender,
			Nonce:              op.Nonce,
			InitCode:           op.InitCode,
			CallData:           op.CallData,
			AccountGasLimits:   packAccountGasLimits(op.VerificationGasLimit, op.CallGasLimit),
			PreVerificationGas: op.PreVerificationGas,
			GasFees:            packGasFees(op.MaxFeePerGas, op.MaxPriorityFeePerGas),
			PaymasterAndData:   op.PaymasterAndData,
			Signature:          op.Signature,
		})
	}
	return ops
}

func packAccountGasLimits(verificationGasLimit, callGasLimit *big.Int) [32]byte {
	vg := new(big.Int).And(verificationGasLimit, maxUint128)
	cg := new(big.Int).And(callGasLimit, maxUint128)
	packed := new(big.Int).Lsh(vg, 128)
	packed.Or(packed, cg)
	b := make([]byte, 32)
	packed.FillBytes(b)
	var arr [32]byte
	copy(arr[:], b)
	return arr
}

func packGasFees(maxFeePerGas, maxPriorityFeePerGas *big.Int) [32]byte {
	mf := new(big.Int).And(maxFeePerGas, maxUint128)
	mp := new(big.Int).And(maxPriorityFeePerGas, maxUint128)
	packed := new(big.Int).Lsh(mf, 128)
	packed.Or(packed, mp)
	b := make([]byte, 32)
	packed.FillBytes(b)
	var arr [32]byte
	copy(arr[:], b)
	return arr
}

func EstimateHandleOpsGas(opts *Opts) (gas uint64, revert *reverts.FailedOpRevert, err error) {
	ep, err := v09.NewV09(opts.EntryPoint, opts.Eth)
	if err != nil {
		return 0, nil, err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(opts.EOA.PrivateKey, opts.ChainID)
	if err != nil {
		return 0, nil, err
	}
	auth.GasLimit = math.MaxUint64
	auth.NoSend = true

	tx, err := ep.HandleOps(auth, toPackedBatch(opts.Batch), opts.Beneficiary)
	if err != nil {
		return 0, nil, err
	}

	est, err := opts.Eth.EstimateGas(context.Background(), ethereum.CallMsg{
		From:       opts.EOA.Address,
		To:         tx.To(),
		Gas:        tx.Gas(),
		GasPrice:   tx.GasPrice(),
		GasFeeCap:  tx.GasFeeCap(),
		GasTipCap:  tx.GasTipCap(),
		Value:      tx.Value(),
		Data:       tx.Data(),
		AccessList: tx.AccessList(),
	})
	if err != nil {
		revert, err := reverts.NewFailedOp(err)
		if err != nil {
			return 0, nil, err
		}
		return 0, revert, nil
	}
	return est, nil, nil
}

func HandleOps(opts *Opts) (txn *types.Transaction, err error) {
	ep, err := v09.NewV09(opts.EntryPoint, opts.Eth)
	if err != nil {
		return nil, err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(opts.EOA.PrivateKey, opts.ChainID)
	if err != nil {
		return nil, err
	}
	auth.GasLimit = opts.GasLimit
	auth.NoSend = opts.NoSend

	nonce, err := opts.Eth.NonceAt(context.Background(), opts.EOA.Address, nil)
	if err != nil {
		return nil, err
	}
	auth.Nonce = big.NewInt(int64(nonce))

	if opts.BaseFee != nil && opts.Tip != nil {
		auth.GasTipCap = SuggestMeanGasTipCap(opts.Tip, opts.Batch)
		auth.GasFeeCap = SuggestMeanGasFeeCap(opts.BaseFee, opts.Tip, opts.Batch)
	} else if opts.GasPrice != nil {
		auth.GasPrice = SuggestMeanGasPrice(opts.GasPrice, opts.Batch)
	} else {
		return nil, errors.New("transaction: either the dynamic or legacy gas fees must be set")
	}

	txn, err = ep.HandleOps(auth, toPackedBatch(opts.Batch), opts.Beneficiary)
	if err != nil {
		return nil, err
	} else if opts.WaitTimeout == 0 || opts.NoSend {
		return txn, nil
	}

	return Wait(txn, opts.Eth, opts.WaitTimeout)
}
