package simulation

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	epv07 "github.com/zarbanio/bundler/pkg/entrypoint/v07"
	v07reverts "github.com/zarbanio/bundler/pkg/entrypoint/v07/reverts"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/userop"
	uov07 "github.com/zarbanio/bundler/pkg/userop/v07"
)

type SimulateInput struct {
	Rpc        *rpc.Client
	EntryPoint common.Address
	Op         userop.UserOperationInterface
	ChainID    *big.Int
}

type SimulateOutput struct {
	Result *v07reverts.ValidationResultRevert
}

func toPackedOpFromInterface(op userop.UserOperationInterface) (*epv07.PackedUserOperation, error) {
	if v07op, ok := op.(*uov07.UserOperation); ok {
		return &epv07.PackedUserOperation{
			Sender:             v07op.Sender,
			Nonce:              v07op.Nonce,
			InitCode:           v07op.PackInitCode(),
			CallData:           v07op.CallData,
			AccountGasLimits:   v07op.AccountGasLimits(),
			PreVerificationGas: v07op.PreVerificationGas,
			GasFees:            v07op.GasFees(),
			PaymasterAndData:   v07op.PackPaymasterAndData(),
			Signature:          v07op.Signature,
		}, nil
	}
	data, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	v07op, err := uov07.New(data)
	if err != nil {
		return nil, err
	}
	return &epv07.PackedUserOperation{
		Sender:             v07op.Sender,
		Nonce:              v07op.Nonce,
		InitCode:           v07op.PackInitCode(),
		CallData:           v07op.CallData,
		AccountGasLimits:   v07op.AccountGasLimits(),
		PreVerificationGas: v07op.PreVerificationGas,
		GasFees:            v07op.GasFees(),
		PaymasterAndData:   v07op.PackPaymasterAndData(),
		Signature:          v07op.Signature,
	}, nil
}

func SimulateValidation(in *SimulateInput) (*SimulateOutput, error) {
	ep, err := epv07.NewV07(in.EntryPoint, ethclient.NewClient(in.Rpc))
	if err != nil {
		return nil, err
	}

	packedOp, err := toPackedOpFromInterface(in.Op)
	if err != nil {
		return nil, err
	}

	beneficiary := common.HexToAddress("0x")
	var res []interface{}
	rawCaller := &epv07.V07Raw{Contract: ep}
	err = rawCaller.Call(nil, &res, "handleOps", []epv07.PackedUserOperation{*packedOp}, beneficiary)
	if err == nil {
		return nil, fmt.Errorf("unexpected result from handleOps (expected revert)")
	}

	if sim, simErr := v07reverts.NewValidationResult(err); simErr == nil {
		return &SimulateOutput{Result: sim}, nil
	}

	if fo, foErr := v07reverts.NewFailedOpWithRevert(err); foErr == nil {
		return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	if fo, foErr := reverts.NewFailedOp(err); foErr == nil {
		return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	return nil, err
}
