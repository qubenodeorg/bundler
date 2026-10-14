package filter

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	epv09 "github.com/zarbanio/bundler/pkg/entrypoint/v09"
)

type EIP7702AccountInitializedEvent struct {
	UserOpHash common.Hash
	Sender     common.Address
	Delegate   common.Address
}

// FilterEIP7702AccountInitialized filters the v0.9 EntryPoint for EIP7702AccountInitialized events
// matching the given userOpHash within the specified block range.
func FilterEIP7702AccountInitialized(
	eth *ethclient.Client,
	userOpHash string,
	entryPoint common.Address,
	blkRange uint64,
) (*EIP7702AccountInitializedEvent, error) {
	if eth == nil {
		return nil, errors.New("eth client is nil")
	}
	if len(userOpHash) != 66 || userOpHash[:2] != "0x" {
		return nil, fmt.Errorf("invalid userOpHash: %s", userOpHash)
	}

	ep, err := epv09.NewV09(entryPoint, eth)
	if err != nil {
		return nil, err
	}

	bn, err := eth.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}
	toBlk := big.NewInt(0).SetUint64(bn)
	startBlk := big.NewInt(0)
	subBlkRange := big.NewInt(0).Sub(toBlk, big.NewInt(0).SetUint64(blkRange))
	if subBlkRange.Cmp(startBlk) > 0 {
		startBlk = subBlkRange
	}

	it, err := ep.FilterEIP7702AccountInitialized(
		&bind.FilterOpts{Start: startBlk.Uint64()},
		[][32]byte{common.HexToHash(userOpHash)},
		[]common.Address{},
		[]common.Address{},
	)
	if err != nil {
		return nil, err
	}

	defer func() { _ = it.Close() }()

	for it.Next() {
		return &EIP7702AccountInitializedEvent{
			UserOpHash: it.Event.UserOpHash,
			Sender:     it.Event.Sender,
			Delegate:   it.Event.Delegate,
		}, nil
	}

	if err := it.Error(); err != nil {
		return nil, err
	}

	return nil, nil
}
