package simulation

import (
	stdError "errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	epv08 "github.com/zarbanio/bundler/pkg/entrypoint/v08"
	v08reverts "github.com/zarbanio/bundler/pkg/entrypoint/v08/reverts"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/userop"
	uov08 "github.com/zarbanio/bundler/pkg/userop/v08"
)

type SimulateInput struct {
	Rpc        *rpc.Client
	EntryPoint common.Address
	Op         userop.UserOperationInterface
	ChainID    *big.Int
}

type SimulateOutput struct {
	Result *v08reverts.ValidationResultRevert
}

func toPackedOpFromInterface(op userop.UserOperationInterface) (*epv08.PackedUserOperation, error) {
	if op == nil {
		return nil, stdError.New("user operation is nil")
	}
	if v08op, ok := op.(*uov08.UserOperation); ok {
		return &epv08.PackedUserOperation{
			Sender:             v08op.Sender,
			Nonce:              v08op.Nonce,
			InitCode:           v08op.PackInitCode(),
			CallData:           v08op.CallData,
			AccountGasLimits:   v08op.AccountGasLimits(),
			PreVerificationGas: v08op.PreVerificationGas,
			GasFees:            v08op.GasFees(),
			PaymasterAndData:   v08op.PackPaymasterAndData(),
			Signature:          v08op.Signature,
		}, nil
	}
	data, err := op.ToMap()
	if err != nil {
		return nil, err
	}
	v08op, err := uov08.New(data)
	if err != nil {
		return nil, err
	}
	return &epv08.PackedUserOperation{
		Sender:             v08op.Sender,
		Nonce:              v08op.Nonce,
		InitCode:           v08op.PackInitCode(),
		CallData:           v08op.CallData,
		AccountGasLimits:   v08op.AccountGasLimits(),
		PreVerificationGas: v08op.PreVerificationGas,
		GasFees:            v08op.GasFees(),
		PaymasterAndData:   v08op.PackPaymasterAndData(),
		Signature:          v08op.Signature,
	}, nil
}

func SimulateValidation(in *SimulateInput) (*SimulateOutput, error) {
	if in == nil {
		return nil, stdError.New("simulate input is nil")
	}
	if in.Rpc == nil {
		return nil, stdError.New("rpc client is nil")
	}
	if in.Op == nil {
		return nil, stdError.New("user operation is nil")
	}
	if in.ChainID == nil {
		return nil, stdError.New("chain id is nil")
	}

	ep, err := epv08.NewV08(in.EntryPoint, ethclient.NewClient(in.Rpc))
	if err != nil {
		return nil, err
	}

	packedOp, err := toPackedOpFromInterface(in.Op)
	if err != nil {
		return nil, err
	}

	beneficiary := common.Address{}
	var res []interface{}
	rawCaller := &epv08.V08Raw{Contract: ep}
	err = rawCaller.Call(nil, &res, "handleOps", []epv08.PackedUserOperation{*packedOp}, beneficiary)
	if err == nil {
		return nil, fmt.Errorf("unexpected result from handleOps (expected revert)")
	}

	if sim, simErr := v08reverts.NewValidationResult(err); simErr == nil {
		return &SimulateOutput{Result: sim}, nil
	}

	if fo, foErr := v08reverts.NewFailedOpWithRevert(err); foErr == nil {
		return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	if fo, foErr := reverts.NewFailedOp(err); foErr == nil {
		return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	return nil, err
}
