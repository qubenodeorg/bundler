package execution

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
	epv09 "github.com/zarbanio/bundler/pkg/entrypoint/v09"
	v09reverts "github.com/zarbanio/bundler/pkg/entrypoint/v09/reverts"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/userop"
	uov09 "github.com/zarbanio/bundler/pkg/userop/v09"
)

type SimulateInput struct {
	Rpc        *rpc.Client
	EntryPoint common.Address
	Op         userop.UserOperationInterface
	Sos        state.OverrideSet
	ChainID    *big.Int
	Target     common.Address
	Data       []byte
}

func toPackedOp(op userop.UserOperationInterface) (*epv09.PackedUserOperation, error) {
	if v09op, ok := op.(*uov09.UserOperation); ok {
		return &epv09.PackedUserOperation{
			Sender:             v09op.Sender,
			Nonce:              v09op.Nonce,
			InitCode:           v09op.PackInitCode(),
			CallData:           v09op.CallData,
			AccountGasLimits:   v09op.AccountGasLimits(),
			PreVerificationGas: v09op.PreVerificationGas,
			GasFees:            v09op.GasFees(),
			PaymasterAndData:   v09op.PackPaymasterAndData(),
			Signature:          v09op.Signature,
		}, nil
	}
	data, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	v09op, err := uov09.New(data)
	if err != nil {
		return nil, err
	}
	return &epv09.PackedUserOperation{
		Sender:             v09op.Sender,
		Nonce:              v09op.Nonce,
		InitCode:           v09op.PackInitCode(),
		CallData:           v09op.CallData,
		AccountGasLimits:   v09op.AccountGasLimits(),
		PreVerificationGas: v09op.PreVerificationGas,
		GasFees:            v09op.GasFees(),
		PaymasterAndData:   v09op.PackPaymasterAndData(),
		Signature:          v09op.Signature,
	}, nil
}

// simulateHandleOpCall builds the EntryPointSimulations.simulateHandleOp
// calldata and the state override set that swaps the simulations runtime in
// at the EntryPoint address. simulateValidation/simulateHandleOp were removed
// from the deployed contract in v0.7+, and a plain handleOps eth_call reverts
// with AA24 for the dummy signatures used during gas estimation; the
// simulations variant tolerates signature failure and reports it in the
// returned ExecutionResult instead.
func simulateHandleOpCall(in *SimulateInput) ([]byte, state.OverrideSet, error) {
	packedOp, err := toPackedOp(in.Op)
	if err != nil {
		return nil, nil, err
	}
	calldata, err := epv09.PackSimulateHandleOp(*packedOp, in.Target, in.Data)
	if err != nil {
		return nil, nil, err
	}
	code, err := epv09.SimulationsDeployedBytecode()
	if err != nil {
		return nil, nil, err
	}
	sos := state.WithCodeOverride(in.EntryPoint, code, in.Sos)
	return calldata, sos, nil
}

func SimulateHandleOp(in *SimulateInput) error {
	calldata, sos, err := simulateHandleOpCall(in)
	if err != nil {
		return err
	}

	req := utils.EthCallReq{
		From: common.HexToAddress("0x"),
		To:   in.EntryPoint,
		Data: calldata,
	}
	err = in.Rpc.CallContext(context.Background(), nil, "eth_call", &req, "latest", sos)

	if err == nil {
		return nil
	}

	if fo, foErr := reverts.NewFailedOp(err); foErr == nil {
		return errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	if fo, foErr := v09reverts.NewFailedOpWithRevert(err); foErr == nil {
		reason := fo.Reason
		if inner := reverts.InnerRevertReason(fo.Inner); inner != "" {
			reason = reason + ": " + inner
		}
		return errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, reason, fo)
	}

	return err
}
