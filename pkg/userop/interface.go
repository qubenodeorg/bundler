package userop

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type UserOperationInterface interface {
	GetPaymaster() common.Address
	GetFactory() common.Address
	GetFactoryData() []byte
	GetMaxGasAvailable() *big.Int
	GetMaxPrefund() *big.Int
	GetDynamicGasPrice(basefee *big.Int) *big.Int
	Pack() []byte
	PackForSignature() []byte
	GetUserOpHash(entryPoint common.Address, chainID *big.Int) common.Hash
	MarshalJSON() ([]byte, error)
	ToMap() (map[string]any, error)
}
