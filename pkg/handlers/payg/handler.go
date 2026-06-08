package payg

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/contract"
	"github.com/zarbanio/bundler/pkg/estimator"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/handlers"
	"github.com/zarbanio/bundler/pkg/signer"
	"github.com/zarbanio/bundler/pkg/userop"
	uov09 "github.com/zarbanio/bundler/pkg/userop/v09"
)

type Handler struct {
	signer       *signer.EOA
	rpc          *rpc.Client
	eth          *ethclient.Client
	gasEstimator *estimator.GasEstimator
}

func New(
	signer *signer.EOA,
	rpc *rpc.Client,
	eth *ethclient.Client,
	chain *big.Int,
	ov *gas.Overhead,
) *Handler {
	return &Handler{
		rpc:          rpc,
		eth:          eth,
		signer:       signer,
		gasEstimator: estimator.New(signer, rpc, eth, chain, ov),
	}
}

func (h *Handler) Run(
	op userop.UserOperationInterface,
	ep common.Address,
	pm common.Address,
) (*handlers.SponsorUserOperationResponse, error) {
	data := contract.NewData(pm, common.HexToAddress("0x"), big.NewInt(0))

	// Convert to v0.6 for estimation if needed.
	v06Op, ok := op.(*userop.UserOperation)
	if !ok {
		converted, err := gas.ConvertToV06(op)
		if err != nil {
			return nil, fmt.Errorf("failed to convert userOp to v0.6: %w", err)
		}
		v06Op = converted
	}

	// Estimate gas values to account for paymasterAndData.
	pmOp, err := h.gasEstimator.OverrideOpGasLimitsForPND(v06Op, ep, data)
	if err != nil {
		return nil, fmt.Errorf("failed to override gas limits for paymasterAndData: %w", err)
	}

	// Fetch hash.
	hash, err := contract.GetHash(h.eth, pmOp, data)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract hash: %w", err)
	}

	// Sign hash.
	sig, err := contract.Sign(hash[:], h.signer)
	if err != nil {
		return nil, err
	}

	// For v0.9+, return paymasterSignature separately.
	if _, isV09 := op.(*uov09.UserOperation); isV09 {
		// Encode paymasterAndData without signature for v0.9.
		pnd, err := contract.EncodePaymasterAndData(data, []byte{})
		if err != nil {
			return nil, err
		}
		return &handlers.SponsorUserOperationResponse{
			PaymasterAndData:     hexutil.Encode(pnd),
			PaymasterSignature:   hexutil.Encode(sig),
			PreVerificationGas:   hexutil.EncodeBig(pmOp.PreVerificationGas),
			VerificationGasLimit: hexutil.EncodeBig(pmOp.VerificationGasLimit),
			CallGasLimit:         hexutil.EncodeBig(pmOp.CallGasLimit),
		}, nil
	}

	// Encode final paymasterAndData for v0.6/v0.7/v0.8.
	pnd, err := contract.EncodePaymasterAndData(data, sig)
	if err != nil {
		return nil, err
	}

	return &handlers.SponsorUserOperationResponse{
		PaymasterAndData:     hexutil.Encode(pnd),
		PreVerificationGas:   hexutil.EncodeBig(pmOp.PreVerificationGas),
		VerificationGasLimit: hexutil.EncodeBig(pmOp.VerificationGasLimit),
		CallGasLimit:         hexutil.EncodeBig(pmOp.CallGasLimit),
	}, nil
}
