package client

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint/filter"
	v07filter "github.com/zarbanio/bundler/pkg/entrypoint/v07/filter"
	v08filter "github.com/zarbanio/bundler/pkg/entrypoint/v08/filter"
	v09filter "github.com/zarbanio/bundler/pkg/entrypoint/v09/filter"
	"github.com/zarbanio/bundler/pkg/fees"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/userop"
)

// HashLookupResult is the version-agnostic JSON-RPC response shape for eth_getUserOperationByHash.
// The UserOperation field is a map[string]any so per-version field naming
// (initCode for v0.6, factory/factoryData for v0.7+, eip7702Auth/paymasterSignature for v0.8/v0.9)
// is preserved in the JSON response.
type HashLookupResult struct {
	UserOperation   map[string]any `json:"userOperation"`
	EntryPoint      string         `json:"entryPoint"`
	BlockNumber     *big.Int       `json:"blockNumber"`
	BlockHash       common.Hash    `json:"blockHash"`
	TransactionHash common.Hash    `json:"transactionHash"`
}

// GetUserOpReceiptFunc is a general interface for fetching a UserOperationReceipt given a userOpHash,
// EntryPoint address, and block range. The receipt JSON shape is identical across versions, so
// the v0.6 filter type is used as the canonical return type.
type GetUserOpReceiptFunc = func(hash string, ep common.Address, blkRange uint64) (*filter.UserOperationReceipt, error)

func getUserOpReceiptNoop() GetUserOpReceiptFunc {
	return func(hash string, ep common.Address, blkRange uint64) (*filter.UserOperationReceipt, error) {
		return nil, nil
	}
}

// GetUserOpReceiptWithEthClient returns an implementation of GetUserOpReceiptFunc that routes
// to the correct version-specific filter package based on the EntryPoint address.
func GetUserOpReceiptWithEthClient(eth *ethclient.Client) GetUserOpReceiptFunc {
	return func(hash string, ep common.Address, blkRange uint64) (*filter.UserOperationReceipt, error) {
		switch userop.DetectVersionFromEntryPoint(ep) {
		case userop.V0_7:
			r, err := v07filter.GetUserOperationReceipt(eth, hash, ep, blkRange)
			return convertV07Receipt(r), err
		case userop.V0_8:
			r, err := v08filter.GetUserOperationReceipt(eth, hash, ep, blkRange)
			return convertV08Receipt(r), err
		case userop.V0_9:
			r, err := v09filter.GetUserOperationReceipt(eth, hash, ep, blkRange)
			return convertV09Receipt(r), err
		default:
			return filter.GetUserOperationReceipt(eth, hash, ep, blkRange)
		}
	}
}

// convertVxxReceipt converts version-specific receipts to the canonical v0.6 type by JSON round-trip.
// All version receipt structs share the same JSON layout, so this is lossless.
func convertV07Receipt(r *v07filter.UserOperationReceipt) *filter.UserOperationReceipt {
	if r == nil {
		return nil
	}
	return reEncodeReceipt(r)
}

func convertV08Receipt(r *v08filter.UserOperationReceipt) *filter.UserOperationReceipt {
	if r == nil {
		return nil
	}
	return reEncodeReceipt(r)
}

func convertV09Receipt(r *v09filter.UserOperationReceipt) *filter.UserOperationReceipt {
	if r == nil {
		return nil
	}
	return reEncodeReceipt(r)
}

func reEncodeReceipt(in any) *filter.UserOperationReceipt {
	b, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	var out filter.UserOperationReceipt
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return &out
}

// GetGasPricesFunc is a general interface for fetching values for maxFeePerGas and maxPriorityFeePerGas.
type GetGasPricesFunc = func() (*fees.GasPrices, error)

func getGasPricesNoop() GetGasPricesFunc {
	return func() (*fees.GasPrices, error) {
		return &fees.GasPrices{
			MaxFeePerGas:         big.NewInt(0),
			MaxPriorityFeePerGas: big.NewInt(0),
		}, nil
	}
}

// GetGasPricesWithEthClient returns an implementation of GetGasPricesFunc that relies on an eth client to
// fetch values for maxFeePerGas and maxPriorityFeePerGas.
func GetGasPricesWithEthClient(eth *ethclient.Client) GetGasPricesFunc {
	return func() (*fees.GasPrices, error) {
		return fees.NewGasPrices(eth)
	}
}

// GetGasEstimateFunc is a general interface for fetching an estimate for verificationGasLimit and
// callGasLimit given a versioned userOp and EntryPoint address.
type GetGasEstimateFunc = func(
	ep common.Address,
	op userop.UserOperationInterface,
	version userop.Version,
	sos state.OverrideSet,
) (verificationGas uint64, callGas uint64, err error)

func getGasEstimateNoop() GetGasEstimateFunc {
	return func(
		ep common.Address,
		op userop.UserOperationInterface,
		version userop.Version,
		sos state.OverrideSet,
	) (verificationGas uint64, callGas uint64, err error) {
		return 0, 0, nil
	}
}

// GetGasEstimateWithEthClient returns an implementation of GetGasEstimateFunc that relies on an eth client to
// fetch an estimate for verificationGasLimit and callGasLimit.
func GetGasEstimateWithEthClient(
	rpc *rpc.Client,
	ov *gas.Overhead,
	chain *big.Int,
	maxGasLimit *big.Int,
	tracer string,
) GetGasEstimateFunc {
	return func(
		ep common.Address,
		op userop.UserOperationInterface,
		version userop.Version,
		sos state.OverrideSet,
	) (verificationGas uint64, callGas uint64, err error) {
		return gas.EstimateGas(&gas.EstimateInput{
			Rpc:         rpc,
			EntryPoint:  ep,
			Op:          op,
			Sos:         sos,
			Ov:          ov,
			ChainID:     chain,
			MaxGasLimit: maxGasLimit,
			Tracer:      tracer,
			Version:     version,
		})
	}
}

// GetUserOpByHashFunc is a general interface for fetching a UserOperation given a userOpHash, EntryPoint
// address, chain ID, and block range. The UserOperation field is a map[string]any so each version's
// field naming is preserved in the JSON response.
type GetUserOpByHashFunc func(hash string, ep common.Address, chain *big.Int, blkRange uint64) (*HashLookupResult, error)

func getUserOpByHashNoop() GetUserOpByHashFunc {
	return func(hash string, ep common.Address, chain *big.Int, blkRange uint64) (*HashLookupResult, error) {
		return nil, nil
	}
}

// GetUserOpByHashWithEthClient returns an implementation of GetUserOpByHashFunc that routes
// to the correct version-specific filter package based on the EntryPoint address.
func GetUserOpByHashWithEthClient(eth *ethclient.Client) GetUserOpByHashFunc {
	return func(hash string, ep common.Address, chain *big.Int, blkRange uint64) (*HashLookupResult, error) {
		switch userop.DetectVersionFromEntryPoint(ep) {
		case userop.V0_7:
			res, err := v07filter.GetUserOperationByHash(eth, hash, ep, chain, blkRange)
			if err != nil || res == nil {
				return nil, err
			}
			opMap, err := res.UserOperation.ToMap()
			if err != nil {
				return nil, err
			}
			return &HashLookupResult{
				UserOperation:   opMap,
				EntryPoint:      res.EntryPoint,
				BlockNumber:     res.BlockNumber,
				BlockHash:       res.BlockHash,
				TransactionHash: res.TransactionHash,
			}, nil
		case userop.V0_8:
			res, err := v08filter.GetUserOperationByHash(eth, hash, ep, chain, blkRange)
			if err != nil || res == nil {
				return nil, err
			}
			opMap, err := res.UserOperation.ToMap()
			if err != nil {
				return nil, err
			}
			return &HashLookupResult{
				UserOperation:   opMap,
				EntryPoint:      res.EntryPoint,
				BlockNumber:     res.BlockNumber,
				BlockHash:       res.BlockHash,
				TransactionHash: res.TransactionHash,
			}, nil
		case userop.V0_9:
			res, err := v09filter.GetUserOperationByHash(eth, hash, ep, chain, blkRange)
			if err != nil || res == nil {
				return nil, err
			}
			opMap, err := res.UserOperation.ToMap()
			if err != nil {
				return nil, err
			}
			return &HashLookupResult{
				UserOperation:   opMap,
				EntryPoint:      res.EntryPoint,
				BlockNumber:     res.BlockNumber,
				BlockHash:       res.BlockHash,
				TransactionHash: res.TransactionHash,
			}, nil
		default:
			res, err := filter.GetUserOperationByHash(eth, hash, ep, chain, blkRange)
			if err != nil || res == nil {
				return nil, err
			}
			opMap, err := res.UserOperation.ToMap()
			if err != nil {
				return nil, err
			}
			return &HashLookupResult{
				UserOperation:   opMap,
				EntryPoint:      res.EntryPoint,
				BlockNumber:     res.BlockNumber,
				BlockHash:       res.BlockHash,
				TransactionHash: res.TransactionHash,
			}, nil
		}
	}
}
