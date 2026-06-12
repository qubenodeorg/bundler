package contract

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zarbanio/bundler/pkg/userop"
)

func GetHash(
	eth *ethclient.Client,
	op *userop.UserOperation,
	data *Data,
) ([32]byte, error) {
	pm, err := NewContract(data.Paymaster, eth)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to create paymaster contract: %w", err)
	}

	hash, err := pm.GetHash(
		&bind.CallOpts{},
		UserOperation(op.Core()),
		data.ValidUntil,
		data.ValidAfter,
		data.ERC20Token,
		data.ExchangeRate,
	)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to call getHash: %w", err)
	}

	return hash, nil
}
