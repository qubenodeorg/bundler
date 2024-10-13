package estimator

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/contract"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/signer"
	"github.com/zarbanio/bundler/pkg/userop"
)

func updateOpVerificationGasLimit(op *userop.UserOperation, val *big.Int) (*userop.UserOperation, error) {
	opData, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	opData["verificationGasLimit"] = common.BigToHash(val).String()
	return userop.New(opData)
}

func updateOpCallGasLimit(op *userop.UserOperation, val *big.Int) (*userop.UserOperation, error) {
	opData, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	opData["callGasLimit"] = common.BigToHash(val).String()
	return userop.New(opData)
}

func updateOpPreVerificationGas(
	op *userop.UserOperation,
	ov *gas.Overhead,
) (*userop.UserOperation, error) {
	opData, err := op.ToMap()
	if err != nil {
		return nil, err
	}

	pvg, err := ov.CalcPreVerificationGasWithBuffer(op)
	if err != nil {
		return nil, err
	}

	opData["preVerificationGas"] = common.BigToHash(pvg).String()
	return userop.New(opData)
}

func updateOpPaymasterAndData(
	op *userop.UserOperation,
	paymasterAndDataHex string,
) (*userop.UserOperation, error) {
	opData, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	opData["paymasterAndData"] = paymasterAndDataHex
	return userop.New(opData)
}

type GasEstimator struct {
	signer  *signer.EOA
	rpc     *rpc.Client
	eth     *ethclient.Client
	chainID *big.Int
	ov      *gas.Overhead
}

func New(
	signer *signer.EOA,
	rpc *rpc.Client,
	eth *ethclient.Client,
	chain *big.Int,
	ov *gas.Overhead,
) *GasEstimator {
	return &GasEstimator{
		signer:  signer,
		rpc:     rpc,
		eth:     eth,
		chainID: chain,
		ov:      ov,
	}
}

func (g *GasEstimator) OverrideOpGasLimitsForPND(
	op *userop.UserOperation,
	ep common.Address,
	data *contract.Data,
) (*userop.UserOperation, error) {
	// Generate a PND for EstimateGas.
	hash, err := contract.GetHash(g.eth, op, data)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract hash: %w", err)
	}

	sig, err := contract.Sign(hash[:], g.signer)
	if err != nil {
		return nil, fmt.Errorf("failed to sign contract hash: %w", err)
	}
	pnd, err := contract.EncodePaymasterAndData(data, sig)
	if err != nil {
		return nil, fmt.Errorf("failed to encode paymasterAndData: %w", err)
	}
	pmOp, err := updateOpPaymasterAndData(op, hexutil.Encode(pnd))
	if err != nil {
		return nil, fmt.Errorf("failed to update paymasterAndData: %w", err)
	}

	j, _ := json.MarshalIndent(pmOp, "", "  ")
	fmt.Println("UserOperation:", string(j))

	// Run EstimateGas.
	vgl, cgl, err := gas.EstimateGas(&gas.EstimateInput{
		Rpc:         g.rpc,
		EntryPoint:  ep,
		Op:          pmOp,
		Ov:          g.ov,
		ChainID:     g.chainID,
		MaxGasLimit: maxGasLimit,
		Tracer:      "callTracer",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to estimate gas: %w", err)
	}

	// Update gas fields.
	pmOp, err = updateOpPaymasterAndData(pmOp, DummyPaymasterAndDataHex)
	if err != nil {
		return nil, fmt.Errorf("failed to update paymasterAndData: %w", err)
	}
	pmOp, err = updateOpVerificationGasLimit(pmOp, big.NewInt(int64(vgl)))
	if err != nil {
		return nil, fmt.Errorf("failed to update verificationGasLimit: %w", err)
	}
	pmOp, err = updateOpCallGasLimit(pmOp, big.NewInt(int64(cgl)))
	if err != nil {
		return nil, fmt.Errorf("failed to update callGasLimit: %w", err)
	}
	pmOp, err = updateOpPreVerificationGas(pmOp, g.ov)
	if err != nil {
		return nil, fmt.Errorf("failed to update preVerificationGas: %w", err)
	}

	return pmOp, nil
}
