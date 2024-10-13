package payg

import (
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
	op *userop.UserOperation,
	ep common.Address,
	pm common.Address,
) (*handlers.SponsorUserOperationResponse, error) {
	// Get paymaster data.
	data := contract.NewData(pm, common.HexToAddress("0x"), big.NewInt(0))

	// Estimate gas values to account for paymasterAndData.
	pmOp, err := h.gasEstimator.OverrideOpGasLimitsForPND(op, ep, data)
	if err != nil {
		return nil, err
	}

	// Fetch hash.
	hash, err := contract.GetHash(h.eth, pmOp, data)
	if err != nil {
		return nil, err
	}

	// Sign hash.
	sig, err := contract.Sign(hash[:], h.signer)
	if err != nil {
		return nil, err
	}

	// Encode final paymasterAndData.
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
