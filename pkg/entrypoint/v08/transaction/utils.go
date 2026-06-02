package transaction

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func ToRawTxHex(txn *types.Transaction) string {
	rawTxn := new(bytes.Buffer)
	types.Transactions{txn}.EncodeIndex(0, rawTxn)
	return hexutil.Encode(rawTxn.Bytes())
}

func Wait(txn *types.Transaction, eth *ethclient.Client, timeout time.Duration) (*types.Transaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if receipt, err := bind.WaitMined(ctx, eth, txn); err != nil {
		return nil, err
	} else if receipt.Status == types.ReceiptStatusFailed {
		return nil, errors.New("transaction: failed status")
	}
	return txn, nil
}
