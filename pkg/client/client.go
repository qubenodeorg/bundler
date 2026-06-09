// Package client provides the mediator for processing incoming UserOperations to the bundler.
package client

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-logr/logr"
	"github.com/zarbanio/bundler/internal/logger"
	"github.com/zarbanio/bundler/pkg/entrypoint/filter"
	"github.com/zarbanio/bundler/pkg/entrypoint/stake"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/handlers"
	"github.com/zarbanio/bundler/pkg/handlers/payg"
	"github.com/zarbanio/bundler/pkg/mempool"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/modules/noop"
	"github.com/zarbanio/bundler/pkg/signer"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/userop"
)

// Client controls the end to end process of adding incoming UserOperations to the mempool. It also
// implements the required RPC methods as specified in EIP-4337.
type Client struct {
	mempool              *mempool.Mempool
	ov                   *gas.Overhead
	chainID              *big.Int
	supportedEntryPoints []common.Address
	userOpHandler        modules.UserOpHandlerFunc
	logger               logr.Logger
	getUserOpReceipt     GetUserOpReceiptFunc
	getGasPrices         GetGasPricesFunc
	getGasEstimate       GetGasEstimateFunc
	getUserOpByHash      GetUserOpByHashFunc
	getStakeFunc         stake.GetStakeFunc
	opLookupLimit        uint64

	ep2pms      map[common.Address][]common.Address
	paygHandler *payg.Handler
}

// New initializes a new ERC-4337 client which can be extended with modules for validating UserOperations
// that are allowed to be added to the mempool.
func New(
	mempool *mempool.Mempool,
	ov *gas.Overhead,
	chainID *big.Int,
	supportedEntryPoints []common.Address,
	opLookupLimit uint64,
	signer *signer.EOA,
	eth *ethclient.Client,
	rpc *rpc.Client,
) *Client {
	return &Client{
		mempool:              mempool,
		ov:                   ov,
		chainID:              chainID,
		supportedEntryPoints: supportedEntryPoints,
		userOpHandler:        noop.UserOpHandler,
		logger:               logger.NewZeroLogr().WithName("client"),
		getUserOpReceipt:     getUserOpReceiptNoop(),
		getGasPrices:         getGasPricesNoop(),
		getGasEstimate:       getGasEstimateNoop(),
		getUserOpByHash:      getUserOpByHashNoop(),
		getStakeFunc:         stake.GetStakeFuncNoop(),
		opLookupLimit:        opLookupLimit,
		paygHandler:          payg.New(signer, rpc, eth, chainID, ov),
		ep2pms: map[common.Address][]common.Address{
			supportedEntryPoints[0]: {
				common.HexToAddress("0x2b9755b522757931981ca5b90e821786B75382AB"),
			},
		},
	}
}

func (i *Client) parseEntryPointAddress(ep string) (common.Address, error) {
	for _, addr := range i.supportedEntryPoints {
		if common.HexToAddress(ep) == addr {
			return addr, nil
		}
	}

	return common.Address{}, errors.New("entryPoint: Implementation not supported")
}

// UseLogger defines the logger object used by the Client instance based on the go-logr/logr interface.
func (i *Client) UseLogger(logger logr.Logger) {
	i.logger = logger.WithName("client")
}

// UseModules defines the UserOpHandlers to process a userOp after it has gone through the standard checks.
func (i *Client) UseModules(handlers ...modules.UserOpHandlerFunc) {
	i.userOpHandler = modules.ComposeUserOpHandlerFunc(handlers...)
}

// SetGetUserOpReceiptFunc defines a general function for fetching a UserOpReceipt given a userOpHash and
// EntryPoint address. This function is called in *Client.GetUserOperationReceipt.
func (i *Client) SetGetUserOpReceiptFunc(fn GetUserOpReceiptFunc) {
	i.getUserOpReceipt = fn
}

// SetGetGasPricesFunc defines a general function for fetching values for maxFeePerGas and
// maxPriorityFeePerGas. This function is called in *Client.EstimateUserOperationGas if given fee values are
// 0.
func (i *Client) SetGetGasPricesFunc(fn GetGasPricesFunc) {
	i.getGasPrices = fn
}

// SetGetGasEstimateFunc defines a general function for fetching an estimate for verificationGasLimit and
// callGasLimit given a userOp and EntryPoint address. This function is called in
// *Client.EstimateUserOperationGas.
func (i *Client) SetGetGasEstimateFunc(fn GetGasEstimateFunc) {
	i.getGasEstimate = fn
}

// SetGetUserOpByHashFunc defines a general function for fetching a userOp given a userOpHash, EntryPoint
// address, and chain ID. This function is called in *Client.GetUserOperationByHash.
func (i *Client) SetGetUserOpByHashFunc(fn GetUserOpByHashFunc) {
	i.getUserOpByHash = fn
}

// SetGetStakeFunc defines a general function for retrieving the EntryPoint stake for a given address. This
// function is called in *Client.SendUserOperation to create a context.
func (i *Client) SetGetStakeFunc(fn stake.GetStakeFunc) {
	i.getStakeFunc = fn
}

// parseVersionedUserOp parses the given map into the version-appropriate UserOperation,
// using the EntryPoint address as the authoritative version source and falling back to
// field-based detection on the request map.
func parseVersionedUserOp(op map[string]any, ep common.Address) (userop.UserOperationInterface, userop.Version, error) {
	version := userop.DetectVersionFromEntryPoint(ep)
	if version == userop.V0_6 {
		// Override v0.6 default if the request body clearly indicates a newer version.
		if fieldVersion := userop.DetectVersion(op); fieldVersion != userop.V0_6 {
			version = fieldVersion
		}
	}

	if version == userop.V0_6 {
		v06op, err := userop.New(op)
		if err != nil {
			return nil, version, err
		}
		return v06op, version, nil
	}
	vop, err := userop.NewUserOperation(version, op)
	if err != nil {
		return nil, version, err
	}
	return vop.Op, version, nil
}

// SendUserOperation implements the method call for eth_sendUserOperation.
// It returns true if userOp was accepted otherwise returns an error.
func (i *Client) SendUserOperation(op map[string]any, ep string) (string, error) {
	// Init logger
	l := i.logger.WithName("eth_sendUserOperation")

	// Check EntryPoint and userOp is valid.
	epAddr, err := i.parseEntryPointAddress(ep)
	if err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}
	l = l.
		WithValues("entrypoint", epAddr.String()).
		WithValues("chain_id", i.chainID.String())

	userOp, version, err := parseVersionedUserOp(op, epAddr)
	if err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}
	l = l.WithValues("version", string(version))

	hash := userOp.GetUserOpHash(epAddr, i.chainID)
	l = l.WithValues("userop_hash", hash)

	// Run through client module stack.
	ctx, err := modules.NewUserOpHandlerContext(
		userOp,
		epAddr,
		i.chainID,
		i.mempool,
		i.getStakeFunc,
	)
	if err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}
	if err := i.userOpHandler(ctx); err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}

	// Convert to canonical packed v0.6 format before adding to the mempool. The mempool, batch
	// pipeline, and existing v07/v08/v09 transaction packages all operate on this single shape.
	packedOp, err := userop.ConvertToV06Packed(ctx.UserOp)
	if err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}

	if err := i.mempool.AddOp(epAddr, packedOp); err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}

	l.Info("eth_sendUserOperation ok")
	return hash.String(), nil
}

// EstimateUserOperationGas returns estimates for PreVerificationGas, VerificationGasLimit, and CallGasLimit
// given a UserOperation, EntryPoint address, and state OverrideSet. The signature field and current gas
// values will not be validated although there should be dummy values in place for the most reliable results
// (e.g. a signature with the correct length).
func (i *Client) EstimateUserOperationGas(
	op map[string]any,
	ep string,
	os map[string]any,
) (*gas.GasEstimates, error) {
	// Init logger
	l := i.logger.WithName("eth_estimateUserOperationGas")

	// Check EntryPoint and userOp is valid.
	epAddr, err := i.parseEntryPointAddress(ep)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}
	l = l.
		WithValues("entrypoint", epAddr.String()).
		WithValues("chain_id", i.chainID.String())

	userOp, version, err := parseVersionedUserOp(op, epAddr)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}
	l = l.WithValues("version", string(version))

	hash := userOp.GetUserOpHash(epAddr, i.chainID)
	l = l.WithValues("userop_hash", hash)

	// Parse state override set.
	sos, err := state.ParseOverrideData(os)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}

	// Override op with suggested gas prices if maxFeePerGas is 0. This allows for more reliable gas
	// estimations upstream. The default balance override also ensures simulations won't revert on
	// insufficient funds.
	if userOp.GetMaxFeePerGas().Cmp(common.Big0) != 1 {
		gp, err := i.getGasPrices()
		if err != nil {
			l.Error(err, "eth_estimateUserOperationGas error")
			return nil, err
		}
		userOp, err = setGasPrices(userOp, version, gp.MaxFeePerGas, gp.MaxPriorityFeePerGas)
		if err != nil {
			l.Error(err, "eth_estimateUserOperationGas error")
			return nil, err
		}
	}

	// Estimate gas limits
	vg, cg, err := i.getGasEstimate(epAddr, userOp, version, sos)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}

	// Calculate PreVerificationGas
	pvg, err := i.ov.CalcPreVerificationGasWithBuffer(userOp)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}

	res := &gas.GasEstimates{
		PreVerificationGas:   pvg,
		VerificationGasLimit: big.NewInt(int64(vg)),
		CallGasLimit:         big.NewInt(int64(cg)),

		// TODO: Deprecate in v0.7
		VerificationGas: big.NewInt(int64(vg)),
	}
	if version != userop.V0_6 {
		res.PaymasterVerificationGasLimit = userOp.GetPaymasterVerificationGasLimit()
		res.PaymasterPostOpGasLimit = userOp.GetPaymasterPostOpGasLimit()
	}

	l.Info("eth_estimateUserOperationGas ok")
	return res, nil
}

// setGasPrices returns a new versioned userOp with the given maxFeePerGas/maxPriorityFeePerGas applied.
// This is used during gas estimation to overlay suggested gas prices onto the user-provided op.
func setGasPrices(op userop.UserOperationInterface, version userop.Version, maxFee, maxPrio *big.Int) (userop.UserOperationInterface, error) {
	data, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	data["maxFeePerGas"] = hexutil.EncodeBig(maxFee)
	data["maxPriorityFeePerGas"] = hexutil.EncodeBig(maxPrio)

	if version == userop.V0_6 {
		return userop.New(data)
	}
	vop, err := userop.NewUserOperation(version, data)
	if err != nil {
		return nil, err
	}
	return vop.Op, nil
}

// GetUserOperationReceipt fetches a UserOperation receipt based on a userOpHash returned by
// *Client.SendUserOperation. Routing to the correct EntryPoint version is delegated to the
// installed GetUserOpReceiptFunc.
func (i *Client) GetUserOperationReceipt(
	hash string,
) (*filter.UserOperationReceipt, error) {
	// Init logger
	l := i.logger.WithName("eth_getUserOperationReceipt").WithValues("userop_hash", hash)

	// Try each supported EntryPoint until a receipt is found.
	for _, ep := range i.supportedEntryPoints {
		ev, err := i.getUserOpReceipt(hash, ep, i.opLookupLimit)
		if err != nil {
			l.Error(err, "eth_getUserOperationReceipt error")
			return nil, err
		}
		if ev != nil {
			l.Info("eth_getUserOperationReceipt ok")
			return ev, nil
		}
	}

	return nil, nil
}

// GetUserOperationByHash returns a UserOperation based on a given userOpHash returned by
// *Client.SendUserOperation. Routing to the correct EntryPoint version is delegated to the
// installed GetUserOpByHashFunc.
func (i *Client) GetUserOperationByHash(hash string) (*HashLookupResult, error) {
	// Init logger
	l := i.logger.WithName("eth_getUserOperationByHash").WithValues("userop_hash", hash)

	for _, ep := range i.supportedEntryPoints {
		res, err := i.getUserOpByHash(hash, ep, i.chainID, i.opLookupLimit)
		if err != nil {
			l.Error(err, "eth_getUserOperationByHash error")
			return nil, err
		}
		if res != nil {
			return res, nil
		}
	}

	return nil, nil
}

// SupportedEntryPoints implements the method call for eth_supportedEntryPoints. It returns the array of
// EntryPoint addresses that is supported by the client. The first address in the array is the preferred
// EntryPoint.
func (i *Client) SupportedEntryPoints() ([]string, error) {
	slc := []string{}
	for _, ep := range i.supportedEntryPoints {
		slc = append(slc, ep.String())
	}

	return slc, nil
}

// ChainID implements the method call for eth_chainId. It returns the current chainID used by the client.
// This method is used to validate that the client's chainID is in sync with the caller.
func (i *Client) ChainID() (string, error) {
	return hexutil.EncodeBig(i.chainID), nil
}

func (c *Client) Accounts(ep string) ([]string, error) {
	l := c.logger.WithName("pm_accounts")

	epAddr := common.HexToAddress(ep)
	pmAddr, ok := c.ep2pms[epAddr]
	if !ok {
		err := errors.New("entryPoint: Implementation not supported")
		l.Error(err, "pm_accounts error")
		return nil, err
	}
	l = l.WithValues("entrypoint", epAddr.String()).
		WithValues("paymasters", pmAddr).
		WithValues("chain_id", c.chainID.String())

	res := []string{}
	for _, pm := range pmAddr {
		res = append(res, pm.Hex())
	}
	l.Info("pm_accounts ok")

	return res, nil
}

func (c *Client) SponsorUserOperation(
	op map[string]any,
	ep string,
	ctx map[string]any,
) (*handlers.SponsorUserOperationResponse, error) {
	l := c.logger.WithName("pm_sponsorUserOperation")

	epAddr := common.HexToAddress(ep)
	pmAddrs, ok := c.ep2pms[epAddr]
	if !ok {
		err := errors.New("entryPoint: Implementation not supported")
		l.Error(err, "pm_sponsorUserOperation error")
		return nil, err
	}
	l = l.WithValues("entrypoint", epAddr.String()).
		WithValues("paymasters", pmAddrs).
		WithValues("chain_id", c.chainID.String())

	userOp, version, err := parseVersionedUserOp(op, epAddr)
	if err != nil {
		err = fmt.Errorf("bad userOp: %s", err)
		l.Error(err, "pm_sponsorUserOperation error")
		return nil, err
	}
	l = l.WithValues("version", string(version))

	ct, err := handlers.NewContextType(ctx)
	if err != nil {
		err = fmt.Errorf("bad context: %s", err)
		l.Error(err, "pm_sponsorUserOperation error")
		return nil, err
	}
	l = l.WithValues("type", ct.Type)

	switch ct.Type {
	case "payg":
		res, err := c.paygHandler.Run(userOp, epAddr, pmAddrs[0])
		if err != nil {
			l.Error(err, "pm_sponsorUserOperation error")
			return nil, err
		}

		l.Info("pm_sponsorUserOperation ok")
		return res, nil
	default:
		err := fmt.Errorf("type: %s not recognized", ct.Type)
		l.Error(err, "pm_sponsorUserOperation error")
		return nil, err
	}
}
