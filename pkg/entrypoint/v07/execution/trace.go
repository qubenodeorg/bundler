package execution

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	v06exec "github.com/zarbanio/bundler/pkg/entrypoint/execution"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
	epv07 "github.com/zarbanio/bundler/pkg/entrypoint/v07"
	v07reverts "github.com/zarbanio/bundler/pkg/entrypoint/v07/reverts"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/tracer"
	"github.com/zarbanio/bundler/pkg/userop"
)

type TraceInput struct {
	Rpc         *ethRpc.Client
	EntryPoint  common.Address
	Op          userop.UserOperationInterface
	Sos         state.OverrideSet
	ChainID     *big.Int
	Tracer      string
	Target      common.Address
	Data        []byte
	TraceFeeCap *big.Int
}

type TraceOutput struct {
	Trace  *tracer.BundlerExecutionReturn
	Result *v07reverts.ExecutionResultRevert
	Event  *epv07.V07UserOperationEvent
}

func TraceSimulateHandleOp(in *TraceInput) (*TraceOutput, error) {
	ep, err := epv07.NewV07(in.EntryPoint, ethclient.NewClient(in.Rpc))
	if err != nil {
		return nil, err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(utils.DummyPk, in.ChainID)
	if err != nil {
		return nil, err
	}
	auth.GasLimit = math.MaxUint64
	auth.NoSend = true

	packedOp, err := toPackedOp(in.Op)
	if err != nil {
		return nil, err
	}

	beneficiary := common.HexToAddress("0x")
	tx, err := ep.HandleOps(auth, []epv07.PackedUserOperation{*packedOp}, beneficiary)
	if err != nil {
		return nil, err
	}

	mf := in.Op.GetMaxFeePerGas()
	if in.TraceFeeCap != nil {
		mf = in.TraceFeeCap
	}
	if mf == nil {
		mf = big.NewInt(0)
	}

	out := &TraceOutput{}
	req := utils.TraceCallReq{
		From:         common.HexToAddress("0x"),
		To:           in.EntryPoint,
		Data:         tx.Data(),
		MaxFeePerGas: hexutil.Big(*mf),
	}
	opts := utils.TraceCallOpts{
		StateOverrides: state.WithMaxBalanceOverride(common.HexToAddress("0x"), in.Sos),
	}

	var callTrace *tracer.BundlerExecutionReturn
	if tracer.IsNativeExecutor(in.Tracer) {
		opts.Tracer = tracer.NativeExecutor
		opts.TracerConfig = tracer.CallTracerConfig()

		var call v06exec.Call
		if err := in.Rpc.CallContext(context.Background(), &call, "debug_traceCall", &req, "latest", &opts); err != nil {
			return nil, err
		}
		epABI, err := epv07.V07MetaData.GetAbi()
		if err != nil {
			return nil, err
		}
		callTrace, err = v06exec.ProcessBundlerExecution(&call, v06exec.NewTraceConfig(epABI, in.EntryPoint))
		if err != nil {
			return nil, err
		}
	} else {
		exec, err := tracer.Loaded.ForVersion(userop.V0_7, in.EntryPoint.Hex())
		if err != nil {
			return nil, err
		}
		opts.Tracer = exec.Execution

		var ret tracer.BundlerExecutionReturn
		if err := in.Rpc.CallContext(context.Background(), &ret, "debug_traceCall", &req, "latest", &opts); err != nil {
			return nil, err
		}
		callTrace = &ret
	}

	if callTrace.ValidationOOG {
		return nil, errors.NewRPCError(errors.EXECUTION_REVERTED, "validation OOG", nil)
	}

	outErr, err := errors.ParseHexToRpcDataError(callTrace.Output)
	if err != nil {
		return nil, err
	}
	out.Trace = callTrace

	if sim, simErr := v07reverts.NewExecutionResult(outErr); simErr == nil {
		out.Result = sim
	}

	if fo, foErr := reverts.NewFailedOp(outErr); foErr == nil {
		return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	if fo, foErr := v07reverts.NewFailedOpWithRevert(outErr); foErr == nil && callTrace.Error != "" {
		return nil, errors.NewRPCError(errors.EXECUTION_REVERTED, callTrace.Error, nil)
	} else if foErr == nil {
		return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	if len(callTrace.Reverts) != 0 {
		data, err := hexutil.Decode(callTrace.Reverts[len(callTrace.Reverts)-1])
		if err != nil {
			return out, err
		}

		if len(data) == 0 {
			if callTrace.ExecutionOOG {
				return out, errors.NewRPCError(errors.EXECUTION_REVERTED, "execution OOG", nil)
			}
			return out, errors.NewRPCError(errors.EXECUTION_REVERTED, "execution reverted", nil)
		}

		reason, revErr := errors.DecodeRevert(data)
		if revErr != nil {
			code, panErr := errors.DecodePanic(data)
			if panErr != nil {
				return nil, errors.NewRPCError(
					errors.EXECUTION_REVERTED,
					"execution reverted with data",
					hexutil.Encode(data),
				)
			}
			return out, errors.NewRPCError(
				errors.EXECUTION_REVERTED,
				fmt.Sprintf("panic encountered: %s", code),
				code,
			)
		}
		if reason == "" {
			return out, nil
		}
		return out, errors.NewRPCError(errors.EXECUTION_REVERTED, reason, reason)
	}

	return out, nil
}
