package client

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint/filter"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/handlers"
)

// Named UserOperation type for jsonrpc package.
type userOperation map[string]any

// Named StateOverride type for jsonrpc package.
type optional_stateOverride map[string]any

// RpcAdapter is an adapter for routing JSON-RPC method calls to the correct client functions.
type RpcAdapter struct {
	client  *Client
	debug   *Debug
	backend *rpc.Client
}

// NewRpcAdapter initializes a new RpcAdapter which can be used with a JSON-RPC server.
func NewRpcAdapter(client *Client, debug *Debug, backend *rpc.Client) *RpcAdapter {
	return &RpcAdapter{client: client, debug: debug, backend: backend}
}

// Eth_sendUserOperation routes method calls to *Client.SendUserOperation.
func (r *RpcAdapter) Eth_sendUserOperation(op userOperation, ep string) (string, error) {
	return r.client.SendUserOperation(op, ep)
}

// Eth_estimateUserOperationGas routes method calls to *Client.EstimateUserOperationGas.
func (r *RpcAdapter) Eth_estimateUserOperationGas(
	op userOperation,
	ep string,
	os optional_stateOverride,
) (*gas.GasEstimates, error) {
	return r.client.EstimateUserOperationGas(op, ep, os)
}

// Eth_getUserOperationReceipt routes method calls to *Client.GetUserOperationReceipt.
func (r *RpcAdapter) Eth_getUserOperationReceipt(
	userOpHash string,
) (*filter.UserOperationReceipt, error) {
	return r.client.GetUserOperationReceipt(userOpHash)
}

// Eth_getUserOperationByHash routes method calls to *Client.GetUserOperationByHash.
func (r *RpcAdapter) Eth_getUserOperationByHash(
	userOpHash string,
) (*filter.HashLookupResult, error) {
	return r.client.GetUserOperationByHash(userOpHash)
}

// Eth_supportedEntryPoints routes method calls to *Client.SupportedEntryPoints.
func (r *RpcAdapter) Eth_supportedEntryPoints() ([]string, error) {
	return r.client.SupportedEntryPoints()
}

// Eth_chainId routes method calls to *Client.ChainID.
func (r *RpcAdapter) Eth_chainId() (string, error) {
	return r.client.ChainID()
}

// Debug_bundler_clearState routes method calls to *Debug.ClearState.
func (r *RpcAdapter) Debug_bundler_clearState() (string, error) {
	if r.debug == nil {
		return "", errors.New("rpc: debug mode is not enabled")
	}

	return r.debug.ClearState()
}

// Debug_bundler_dumpMempool routes method calls to *Debug.DumpMempool.
func (r *RpcAdapter) Debug_bundler_dumpMempool(ep string) ([]map[string]any, error) {
	if r.debug == nil {
		return []map[string]any{}, errors.New("rpc: debug mode is not enabled")
	}

	return r.debug.DumpMempool(ep)
}

// Debug_bundler_sendBundleNow routes method calls to *Debug.SendBundleNow.
func (r *RpcAdapter) Debug_bundler_sendBundleNow() (string, error) {
	if r.debug == nil {
		return "", errors.New("rpc: debug mode is not enabled")
	}

	return r.debug.SendBundleNow()
}

// Debug_bundler_setBundlingMode routes method calls to *Debug.SetBundlingMode.
func (r *RpcAdapter) Debug_bundler_setBundlingMode(mode string) (string, error) {
	if r.debug == nil {
		return "", errors.New("rpc: debug mode is not enabled")
	}

	return r.debug.SetBundlingMode(mode)
}

// Debug_bundler_setReputation routes method calls to *Debug.SetReputation.
func (r *RpcAdapter) Debug_bundler_setReputation(entries []any, ep string) (string, error) {
	if r.debug == nil {
		return "", errors.New("rpc: debug mode is not enabled")
	}

	return r.debug.SetReputation(entries, ep)
}

// Debug_bundler_dumpReputation routes method calls to *Debug.DumpReputation.
func (r *RpcAdapter) Debug_bundler_dumpReputation(ep string) ([]map[string]any, error) {
	if r.debug == nil {
		return []map[string]any{}, errors.New("rpc: debug mode is not enabled")
	}

	return r.debug.DumpReputation(ep)
}

func (r *RpcAdapter) Eth_call(args interface{}, blockNumber string) ([]byte, error) {
	var hex hexutil.Bytes
	err := r.backend.CallContext(context.Background(), &hex, "eth_call", args, blockNumber)
	if err != nil {
		return nil, err
	}
	return hex, nil
}

func (r *RpcAdapter) Eth_getBlockByNumber(args ...interface{}) ([]byte, error) {
	var raw json.RawMessage
	err := r.backend.CallContext(context.Background(), &raw, "eth_getBlockByNumber", args...)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (r *RpcAdapter) Pm_accounts(ep string) ([]string, error) {
	return r.client.Accounts(ep)
}

func (r *RpcAdapter) Pm_sponsorUserOperation(op map[string]any,
	ep string,
	ctx map[string]any) (*handlers.SponsorUserOperationResponse, error) {
	return r.client.SponsorUserOperation(op, ep, ctx)
}
