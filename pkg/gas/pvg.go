package gas

import (
	"bytes"
	"context"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/arbitrum/nodeinterface"
	"github.com/zarbanio/bundler/pkg/entrypoint"
	"github.com/zarbanio/bundler/pkg/entrypoint/methods"
	"github.com/zarbanio/bundler/pkg/entrypoint/transaction"
	"github.com/zarbanio/bundler/pkg/optimism/gaspriceoracle"
	"github.com/zarbanio/bundler/pkg/signer"
	"github.com/zarbanio/bundler/pkg/userop"
)

// CalcPreVerificationGasFunc defines an interface for a function to calculate PVG given a userOp and a static
// value. The static input is the value derived from the default overheads.
type CalcPreVerificationGasFunc = func(op userop.UserOperationInterface, static *big.Int) (*big.Int, error)

func calcPVGFuncNoop() CalcPreVerificationGasFunc {
	return func(op userop.UserOperationInterface, static *big.Int) (*big.Int, error) {
		return nil, nil
	}
}

func fieldToHexStr(data map[string]any, key string) string {
	if v, ok := data[key].(string); ok {
		return v
	}
	return "0x"
}

// CalcArbitrumPVGWithEthClient uses Arbitrum's NodeInterface precompile to get an estimate for
// preVerificationGas that takes into account the L1 gas component. see
// https://medium.com/offchainlabs/understanding-arbitrum-2-dimensional-fees-fd1d582596c9.
func CalcArbitrumPVGWithEthClient(
	rpc *rpc.Client,
	entryPoint common.Address,
) CalcPreVerificationGasFunc {
	pk, _ := crypto.GenerateKey()
	dummy, _ := signer.New(hexutil.Encode(crypto.FromECDSA(pk))[2:])
	return func(op userop.UserOperationInterface, static *big.Int) (*big.Int, error) {
		// Convert v0.7+ ops to the legacy v0.6 struct purely for calldata
		// sizing; the packed v0.6 encoding is a close (slightly conservative)
		// proxy for the on-chain handleOps calldata fed to the L1 estimator.
		tmp, err := ConvertToV06(op)
		if err != nil {
			return nil, err
		}

		// Replace paymasterAndData with same-length nonzero bytes so the L1
		// size estimate is not skewed by zero-byte calldata discounts.
		pmLen := len(tmp.PaymasterAndData)
		if pmLen < 1 {
			pmLen = 1
		}
		tmp.PaymasterAndData = bytes.Repeat([]byte{1}, pmLen)

		// Pack handleOps method inputs
		ho, err := methods.HandleOpsMethod.Inputs.Pack(
			[]entrypoint.UserOperation{entrypoint.UserOperation(tmp.Core())},
			dummy.Address,
		)
		if err != nil {
			return nil, err
		}

		create := tmp.Nonce != nil && tmp.Nonce.Cmp(common.Big0) == 0
		ge, err := nodeinterface.GasEstimateL1ComponentMethod.Inputs.Pack(
			entryPoint,
			create,
			append(methods.HandleOpsMethod.ID, ho...),
		)
		if err != nil {
			return nil, err
		}

		// Use eth_call to call the NodeInterface precompile
		req := map[string]any{
			"from": common.HexToAddress("0x"),
			"to":   nodeinterface.PrecompileAddress,
			"data": hexutil.Encode(append(nodeinterface.GasEstimateL1ComponentMethod.ID, ge...)),
		}

		var out any
		if err := rpc.Call(&out, "eth_call", &req, "latest"); err != nil {
			return nil, err
		}

		// Return static + GasEstimateForL1 as PVG
		gas, err := nodeinterface.DecodeGasEstimateL1ComponentOutput(out)
		if err != nil {
			return nil, err
		}
		return big.NewInt(0).Add(static, big.NewInt(int64(gas.GasEstimateForL1))), nil
	}
}

// CalcOptimismPVGWithEthClient uses Optimism's Gas Price Oracle precompile to get an estimate for
// preVerificationGas that takes into account the L1 gas component.
func CalcOptimismPVGWithEthClient(
	rpc *rpc.Client,
	chainID *big.Int,
	entryPoint common.Address,
) CalcPreVerificationGasFunc {
	pk, _ := crypto.GenerateKey()
	dummy, _ := signer.New(hexutil.Encode(crypto.FromECDSA(pk))[2:])
	return func(op userop.UserOperationInterface, static *big.Int) (*big.Int, error) {
		v06Op, err := ConvertToV06(op)
		if err != nil {
			return nil, err
		}

		// Create Raw HandleOps Transaction
		eth := ethclient.NewClient(rpc)
		head, err := eth.HeaderByNumber(context.Background(), nil)
		if err != nil {
			return nil, err
		}
		tip, err := eth.SuggestGasTipCap(context.Background())
		if err != nil {
			return nil, err
		}
		tx, err := transaction.HandleOps(&transaction.Opts{
			EOA:         dummy,
			Eth:         eth,
			ChainID:     chainID,
			EntryPoint:  entryPoint,
			Batch:       []*userop.UserOperation{v06Op},
			Beneficiary: dummy.Address,
			BaseFee:     head.BaseFee,
			Tip:         tip,
			GasLimit:    math.MaxUint64,
			NoSend:      true,
		})
		if err != nil {
			return nil, err
		}

		// Encode function data for GetL1Fee
		rawHex := transaction.ToRawTxHex(tx)
		txData, err := hexutil.Decode(rawHex)
		if err != nil {
			return nil, err
		}
		ge, err := gaspriceoracle.GetL1FeeMethod.Inputs.Pack(txData)
		if err != nil {
			return nil, err
		}

		// Use eth_call to call the Gas Price Oracle precompile
		req := map[string]any{
			"from": common.HexToAddress("0x"),
			"to":   gaspriceoracle.PrecompileAddress,
			"data": hexutil.Encode(append(gaspriceoracle.GetL1FeeMethod.ID, ge...)),
		}
		var out any
		if err := rpc.Call(&out, "eth_call", &req, "latest"); err != nil {
			return nil, err
		}

		// Get L1Fee and L2Price
		l1fee, err := gaspriceoracle.DecodeGetL1FeeMethodOutput(out)
		if err != nil {
			return nil, err
		}
		l2price := op.GetMaxFeePerGas()
		dataMap, _ := op.ToMap()
		maxPriStr := fieldToHexStr(dataMap, "maxPriorityFeePerGas")
		maxPri, _ := hexutil.DecodeBig(maxPriStr)
		l2priority := big.NewInt(0).Add(maxPri, head.BaseFee)
		if l2priority.Cmp(l2price) == -1 {
			l2price = l2priority
		}

		// Return static + L1 buffer as PVG. L1 buffer is equal to L1Fee/L2Price.
		return big.NewInt(0).Add(static, big.NewInt(0).Div(l1fee, l2price)), nil
	}
}
