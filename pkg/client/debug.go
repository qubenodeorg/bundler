package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zarbanio/bundler/pkg/bundler"
	"github.com/zarbanio/bundler/pkg/entrypoint"
	"github.com/zarbanio/bundler/pkg/modules/entities"
	"github.com/zarbanio/bundler/pkg/signer"
	"github.com/zarbanio/bundler/pkg/store"
	"github.com/zarbanio/bundler/pkg/userop"
)

// Debug exposes methods used for testing the bundler. These should not be made available in production.
type Debug struct {
	eoa         *signer.EOA
	eth         *ethclient.Client
	mempool     store.MempoolStore
	rep         *entities.Reputation
	bundler     *bundler.Bundler
	chainID     *big.Int
	entrypoint  common.Address
	beneficiary common.Address
}

func NewDebug(
	eoa *signer.EOA,
	eth *ethclient.Client,
	mempool store.MempoolStore,
	rep *entities.Reputation,
	bundler *bundler.Bundler,
	chainID *big.Int,
	entrypoint common.Address,
	beneficiary common.Address,
) *Debug {
	return &Debug{eoa, eth, mempool, rep, bundler, chainID, entrypoint, beneficiary}
}

// ClearState clears the bundler mempool and reputation data of paymasters/accounts/factories/aggregators.
func (d *Debug) ClearState() (string, error) {
	if err := d.mempool.Clear(context.Background()); err != nil {
		return "", err
	}
	if err := d.rep.Clear(); err != nil {
		return "", err
	}

	return "ok", nil
}

// DumpMempool dumps the current UserOperations mempool in order of arrival.
// Ops are stored in the packed legacy format, so for v0.7+ EntryPoints they
// are converted back to the versioned wire format (factory/paymaster groups).
func (d *Debug) DumpMempool(ep string) ([]map[string]any, error) {
	epAddr := common.HexToAddress(ep)
	ops, err := d.mempool.All(context.Background(), epAddr)
	if err != nil {
		return []map[string]any{}, err
	}

	version := userop.DetectVersionFromEntryPoint(epAddr)
	res := []map[string]any{}
	for _, op := range ops {
		if version != userop.V0_6 {
			res = append(res, legacyOpToV07Map(op))
			continue
		}

		data, err := op.MarshalJSON()
		if err != nil {
			return []map[string]any{}, err
		}

		item := make(map[string]any)
		if err := json.Unmarshal(data, &item); err != nil {
			return []map[string]any{}, err
		}

		res = append(res, item)
	}

	return res, nil
}

var maxUint128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

// legacyOpToV07Map converts a packed legacy mempool op back to the ERC-4337
// v0.7+ wire format, omitting the factory/paymaster groups when unset.
func legacyOpToV07Map(op *userop.UserOperation) map[string]any {
	item := map[string]any{
		"sender":               op.Sender.String(),
		"nonce":                hexutil.EncodeBig(op.Nonce),
		"callData":             hexutil.Encode(op.CallData),
		"callGasLimit":         hexutil.EncodeBig(op.CallGasLimit),
		"verificationGasLimit": hexutil.EncodeBig(op.VerificationGasLimit),
		"preVerificationGas":   hexutil.EncodeBig(op.PreVerificationGas),
		"maxFeePerGas":         hexutil.EncodeBig(op.MaxFeePerGas),
		"maxPriorityFeePerGas": hexutil.EncodeBig(op.MaxPriorityFeePerGas),
		"signature":            hexutil.Encode(op.Signature),
	}
	if len(op.InitCode) >= common.AddressLength {
		item["factory"] = common.BytesToAddress(op.InitCode[:common.AddressLength]).String()
		item["factoryData"] = hexutil.Encode(op.InitCode[common.AddressLength:])
	}
	if len(op.PaymasterAndData) >= common.AddressLength+32 {
		packed := new(big.Int).SetBytes(op.PaymasterAndData[common.AddressLength : common.AddressLength+32])
		item["paymaster"] = common.BytesToAddress(op.PaymasterAndData[:common.AddressLength]).String()
		item["paymasterVerificationGasLimit"] = hexutil.EncodeBig(new(big.Int).Rsh(packed, 128))
		item["paymasterPostOpGasLimit"] = hexutil.EncodeBig(new(big.Int).And(packed, maxUint128))
		item["paymasterData"] = hexutil.Encode(op.PaymasterAndData[common.AddressLength+32:])
	}
	return item
}

// SendBundleNow forces the bundler to build and execute a bundle from the mempool as handleOps() transaction.
func (d *Debug) SendBundleNow() (string, error) {
	ctx, err := d.bundler.Process(d.entrypoint)
	if err != nil {
		return "", err
	}
	if ctx == nil {
		// Nothing eligible to bundle. Return a truthy marker rather than an
		// empty string: spec-test helpers assert the result is non-empty.
		return "ok", nil
	}

	hash, ok := ctx.Data["txn_hash"].(string)
	if !ok {
		// Every op in the bundle was dropped during processing, so no
		// transaction was sent. Like the empty-mempool case above, return a
		// truthy marker for the spec-test helpers.
		return "ok", nil
	}
	return hash, nil
}

// SetBundlingMode allows the bundler to be stopped so that an explicit call to debug_bundler_sendBundleNow is
// required to send a bundle.
func (d *Debug) SetBundlingMode(mode string) (string, error) {
	switch mode {
	case "manual":
		d.bundler.Stop()
	case "auto":
		if err := d.bundler.Run(); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("debug: unrecognized mode %s", mode)
	}

	return "ok", nil
}

// SetReputation allows the bundler to set the reputation of given addresses.
func (d *Debug) SetReputation(entries []any, ep string) (string, error) {
	roArr := []*entities.ReputationOverride{}
	for _, entry := range entries {
		b, err := json.Marshal(entry)
		if err != nil {
			return "", err
		}

		ro := &entities.ReputationOverride{}
		if err := json.Unmarshal(b, ro); err != nil {
			return "", err
		}

		roArr = append(roArr, ro)
	}
	if err := d.rep.Override(roArr); err != nil {
		return "", err
	}

	return "ok", nil
}

// DumpReputation returns the reputation data of all known addresses.
func (d *Debug) DumpReputation(ep string) ([]map[string]any, error) {
	return d.rep.Dump()
}

// ClearReputation clears the reputation data of all known addresses.
func (d *Debug) ClearReputation() (string, error) {
	if err := d.rep.Clear(); err != nil {
		return "", err
	}

	return "ok", nil
}

// GetStakeStatus returns the EntryPoint stake info of the given address, in
// the shape expected by bundler-spec-tests' debug_bundler_getStakeStatus.
func (d *Debug) GetStakeStatus(addr string, ep string) (map[string]any, error) {
	entryPoint, err := entrypoint.NewEntrypoint(common.HexToAddress(ep), d.eth)
	if err != nil {
		return nil, err
	}

	dep, err := entryPoint.GetDepositInfo(nil, common.HexToAddress(addr))
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"stakeInfo": map[string]any{
			"addr":            addr,
			"stake":           dep.Stake.String(),
			"unstakeDelaySec": big.NewInt(int64(dep.UnstakeDelaySec)).String(),
		},
		"isStaked": dep.Staked,
	}, nil
}
