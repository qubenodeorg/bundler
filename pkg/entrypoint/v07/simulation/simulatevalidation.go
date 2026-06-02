package simulation

import (
	stdError "errors"
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
	if op == nil {
		return nil, stdError.New("user operation is nil")
	}
	if v07op, ok := op.(*uov07.UserOperation); ok {
		if v07op == nil {
			return nil, stdError.New("user operation is nil")
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

// SimulateValidation is the off-chain validation-failure check for v0.7+ EntryPoints.
//
// In v0.7+, the on-chain simulateValidation() method was removed. This function
// performs a quick off-chain check by invoking handleOps() via eth_call and
// decoding any revert into a FailedOp or FailedOpWithRevert. If the call does
// not revert, validation passed; if it reverts with a validation error, the
// reason is returned.
//
// Note: this is a simplified check intended to detect validation failures
// without the overhead of a full debug_traceCall. For operations that pass
// validation it does not return a structured ValidationResultRevert - it
// merely reports success. Callers that need the full structured result
// (storage access, referenced contracts, etc.) should use
// TraceSimulateValidation from the same package, which leverages the v0.7+
// bundler collector tracer.
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

	ep, err := epv07.NewV07(in.EntryPoint, ethclient.NewClient(in.Rpc))
	if err != nil {
		return nil, err
	}

	packedOp, err := toPackedOpFromInterface(in.Op)
	if err != nil {
		return nil, err
	}

	beneficiary := common.Address{}
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
