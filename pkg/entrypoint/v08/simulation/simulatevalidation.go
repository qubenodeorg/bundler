package simulation

import (
	"context"
	stdError "errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	epv08 "github.com/zarbanio/bundler/pkg/entrypoint/v08"
	v08reverts "github.com/zarbanio/bundler/pkg/entrypoint/v08/reverts"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/userop"
	uov08 "github.com/zarbanio/bundler/pkg/userop/v08"
)

type SimulateInput struct {
	Rpc        *rpc.Client
	EntryPoint common.Address
	Op         userop.UserOperationInterface
	ChainID    *big.Int

	// DelegateAddress, when set, is the EIP-7702 delegate of the sender. The
	// call then runs with the sender's code overridden to the delegation
	// designation, mirroring the state after the authorization is applied.
	DelegateAddress common.Address
}

type SimulateOutput struct {
	Result *v08reverts.ValidationResultRevert
}

func toPackedOpFromInterface(op userop.UserOperationInterface) (*epv08.PackedUserOperation, error) {
	if op == nil {
		return nil, stdError.New("user operation is nil")
	}
	if v08op, ok := op.(*uov08.UserOperation); ok {
		if v08op == nil {
			return nil, stdError.New("user operation is nil")
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

// SimulateValidation is the off-chain validation-failure check for v0.8 EntryPoints.
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
// TraceSimulateValidation from the same package, which leverages the v0.8
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

	ep, err := epv08.NewV08(in.EntryPoint, ethclient.NewClient(in.Rpc))
	if err != nil {
		return nil, err
	}

	packedOp, err := toPackedOpFromInterface(in.Op)
	if err != nil {
		return nil, err
	}

	// A zero beneficiary makes handleOps revert AA90 even for valid ops, so a
	// dummy non-zero beneficiary lets a clean return signal that validation
	// passed (v0.7+ has no on-chain ValidationResult to decode).
	beneficiary := common.BigToAddress(common.Big1)
	if in.DelegateAddress != (common.Address{}) {
		parsed, abiErr := epv08.V08MetaData.GetAbi()
		if abiErr != nil {
			return nil, abiErr
		}
		data, packErr := parsed.Pack("handleOps", []epv08.PackedUserOperation{*packedOp}, beneficiary)
		if packErr != nil {
			return nil, packErr
		}
		code := append([]byte{0xef, 0x01, 0x00}, in.DelegateAddress.Bytes()...)
		overrides := state.WithCodeOverride(in.Op.GetSender(), code, nil)
		var out hexutil.Bytes
		err = in.Rpc.CallContext(
			context.Background(),
			&out,
			"eth_call",
			map[string]interface{}{
				"to":   in.EntryPoint,
				"data": hexutil.Encode(data),
			},
			"latest",
			overrides,
		)
	} else {
		var res []interface{}
		rawCaller := &epv08.V08Raw{Contract: ep}
		err = rawCaller.Call(nil, &res, "handleOps", []epv08.PackedUserOperation{*packedOp}, beneficiary)
	}
	if err == nil {
		return &SimulateOutput{}, nil
	}

	if sim, simErr := v08reverts.NewValidationResult(err); simErr == nil {
		return &SimulateOutput{Result: sim}, nil
	}

	if fo, foErr := v08reverts.NewFailedOpWithRevert(err); foErr == nil {
		reason := fo.Reason
		if inner := reverts.InnerRevertReason(fo.Inner); inner != "" {
			reason = fmt.Sprintf("%s: %s", reason, inner)
		}
		return nil, errors.NewRPCError(reverts.RPCErrorCodeForFailedOp(fo.Reason), reason, fo)
	}

	if fo, foErr := reverts.NewFailedOp(err); foErr == nil {
		return nil, errors.NewRPCError(reverts.RPCErrorCodeForFailedOp(fo.Reason), fo.Reason, fo)
	}

	return nil, err
}
