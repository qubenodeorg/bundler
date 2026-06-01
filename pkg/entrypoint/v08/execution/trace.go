package execution

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	v06exec "github.com/zarbanio/bundler/pkg/entrypoint/execution"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
	epv08 "github.com/zarbanio/bundler/pkg/entrypoint/v08"
	v08reverts "github.com/zarbanio/bundler/pkg/entrypoint/v08/reverts"
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
	Result *v08reverts.ExecutionResultRevert
	Event  *epv08.V08UserOperationEvent
}

func TraceSimulateHandleOp(in *TraceInput) (*TraceOutput, error) {
	ep, err := epv08.NewV08(in.EntryPoint, ethclient.NewClient(in.Rpc))
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
	tx, err := ep.HandleOps(auth, []epv08.PackedUserOperation{*packedOp}, beneficiary)
	if err != nil {
		return nil, err
	}

	mf := in.Op.GetMaxFeePerGas()
	if in.TraceFeeCap != nil {
		mf = in.TraceFeeCap
	}

	out := &TraceOutput{}
	var call v06exec.Call
	req := utils.TraceCallReq{
		From:         common.HexToAddress("0x"),
		To:           in.EntryPoint,
		Data:         tx.Data(),
		MaxFeePerGas: hexutil.Big(*mf),
	}
	opts := utils.TraceCallOpts{
		Tracer:         in.Tracer,
		StateOverrides: state.WithMaxBalanceOverride(common.HexToAddress("0x"), in.Sos),
	}
	if err := in.Rpc.CallContext(context.Background(), &call, "debug_traceCall", &req, "latest", &opts); err != nil {
		return nil, err
	}

	entryPointAddr := strings.ToLower(in.EntryPoint.Hex())
	callTrace, err := v06exec.ProcessBundlerExecution(&call, entryPointAddr)
	if err != nil {
		return nil, err
	}

	if callTrace.ValidationOOG {
		return nil, errors.NewRPCError(errors.EXECUTION_REVERTED, "validation OOG", nil)
	}

	outErr, err := errors.ParseHexToRpcDataError(callTrace.Output)
	if err != nil {
		return nil, err
	}
	out.Trace = callTrace

	if sim, simErr := v08reverts.NewExecutionResult(outErr); simErr == nil {
		out.Result = sim
	}

	if fo, foErr := reverts.NewFailedOp(outErr); foErr == nil {
		return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	}

	if fo, foErr := v08reverts.NewFailedOpWithRevert(outErr); foErr == nil && callTrace.Error != "" {
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
