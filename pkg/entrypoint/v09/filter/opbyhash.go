package filter

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

type HashLookupResult struct {
	UserOperation   *v09.UserOperation `json:"userOperation"`
	EntryPoint      string             `json:"entryPoint"`
	BlockNumber     *big.Int           `json:"blockNumber"`
	BlockHash       common.Hash        `json:"blockHash"`
	TransactionHash common.Hash        `json:"transactionHash"`
}

func GetUserOperationByHash(
	eth *ethclient.Client,
	userOpHash string,
	entryPoint common.Address,
	chainID *big.Int,
	blkRange uint64,
) (*HashLookupResult, error) {
	if !IsValidUserOpHash(userOpHash) {
		return nil, errors.New("Missing/invalid userOpHash")
	}

	it, err := filterUserOperationEvent(eth, userOpHash, entryPoint, blkRange)
	if err != nil {
		return nil, err
	}

	if it.Next() {
		receipt, err := eth.TransactionReceipt(context.Background(), it.Event.Raw.TxHash)
		if err != nil {
			return nil, err
		}
		tx, isPending, err := eth.TransactionByHash(context.Background(), it.Event.Raw.TxHash)
		if err != nil {
			return nil, err
		} else if isPending {
			return nil, nil
		}

		hex := hexutil.Encode(tx.Data())
		if strings.HasPrefix(hex, "0x1fad948c") {
			data := common.Hex2Bytes(hex[10:])
			ops, err := unpackPackedOps(data)
			if err != nil {
				return nil, err
			}

			for _, packedOp := range ops {
				op := packedOpToV09(packedOp)
				if op.GetUserOpHash(entryPoint, chainID).String() == userOpHash {
					return &HashLookupResult{
						UserOperation:   op,
						EntryPoint:      entryPoint.String(),
						BlockNumber:     receipt.BlockNumber,
						BlockHash:       receipt.BlockHash,
						TransactionHash: it.Event.Raw.TxHash,
					}, nil
				}
			}
		}
	}

	return nil, nil
}
