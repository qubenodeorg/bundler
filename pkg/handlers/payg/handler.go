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
	ovr *gas.OverheadResolver,
) *Handler {
	return &Handler{
		rpc:    rpc,
		eth:    eth,
		signer: signer,
		// payg normalizes every op to the canonical v0.6 packed form (see
		// convertOp) before estimating, so the v0.6 Overhead is the correct one.
		gasEstimator: estimator.New(signer, rpc, eth, chain, ovr.For(userop.V0_6)),
	}
}

// isV09 reports whether the given op is a v0.9 UserOperation. v0.9 has a special
// response shape where the paymaster signature is returned separately from
// paymasterAndData.
func isV09(op userop.UserOperationInterface) bool {
	_, ok := op.(*uov09.UserOperation)
	return ok
}

// convertOp normalizes a versioned UserOperation to the canonical packed v0.6 format used
// downstream by the gas estimator and contract helpers.
func convertOp(op userop.UserOperationInterface) (*userop.UserOperation, error) {
	v06Op, err := userop.ConvertToV06Packed(op)
	if err != nil {
		return nil, fmt.Errorf("failed to convert userOp to v0.6 packed: %w", err)
	}
	return v06Op, nil
}

// buildResponse assembles the SponsorUserOperationResponse for the given op + post-estimation
// packed op + signature. v0.9 returns the paymaster signature separately; earlier versions
// embed it inside paymasterAndData.
func buildResponse(
	op userop.UserOperationInterface,
	pmOp *userop.UserOperation,
	data *contract.Data,
	sig []byte,
) (*handlers.SponsorUserOperationResponse, error) {
	if isV09(op) {
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

// signOp generates the paymaster signature for the given packed op + contract data.
func (h *Handler) signOp(pmOp *userop.UserOperation, data *contract.Data) ([]byte, error) {
	hash, err := contract.GetHash(h.eth, pmOp, data)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract hash: %w", err)
	}
	sig, err := contract.Sign(hash[:], h.signer)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

func (h *Handler) Run(
	op userop.UserOperationInterface,
	ep common.Address,
	pm common.Address,
) (*handlers.SponsorUserOperationResponse, error) {
	data := contract.NewData(pm, common.HexToAddress("0x"), big.NewInt(0))

	v06Op, err := convertOp(op)
	if err != nil {
		return nil, err
	}

	// Estimate gas values to account for paymasterAndData.
	pmOp, err := h.gasEstimator.OverrideOpGasLimitsForPND(v06Op, ep, data)
	if err != nil {
		return nil, fmt.Errorf("failed to override gas limits for paymasterAndData: %w", err)
	}

	sig, err := h.signOp(pmOp, data)
	if err != nil {
		return nil, err
	}

	return buildResponse(op, pmOp, data, sig)
}
