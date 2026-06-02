package simulation

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	epv09 "github.com/zarbanio/bundler/pkg/entrypoint/v09"
	v09reverts "github.com/zarbanio/bundler/pkg/entrypoint/v09/reverts"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/userop"
	uov09 "github.com/zarbanio/bundler/pkg/userop/v09"
)

type SimulateInput struct {
	Rpc        *rpc.Client
	EntryPoint common.Address
	Op         userop.UserOperationInterface
	ChainID    *big.Int
}

type SimulateOutput struct {
	Result *v09reverts.ValidationResultRevert
}

func toPackedOpFromInterface(op userop.UserOperationInterface) (*epv09.PackedUserOperation, error) {
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

func SimulateValidation(in *SimulateInput) (*SimulateOutput, error) {
	ep, err := epv09.NewV09(in.EntryPoint, ethclient.NewClient(in.Rpc))
	if err != nil {
		return nil, err
	}

	packedOp, err := toPackedOpFromInterface(in.Op)
	if err != nil {
		return nil, err
	}

	beneficiary := common.HexToAddress("0x")
	var res []interface{}
	rawCaller := &epv09.V09Raw{Contract: ep}
	err = rawCaller.Call(nil, &res, "handleOps", []epv09.PackedUserOperation{*packedOp}, beneficiary)
	if err == nil {
		return nil, fmt.Errorf("unexpected result from handleOps (expected revert)")
	}

	if sim, simErr := v09reverts.NewValidationResult(err); simErr == nil {
		return &SimulateOutput{Result: sim}, nil
	}

	if fo, foErr := v09reverts.NewFailedOpWithRevert(err); foErr == nil {
		return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	if fo, foErr := reverts.NewFailedOp(err); foErr == nil {
		return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	return nil, err
}
