package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint"
	"github.com/zarbanio/bundler/pkg/entrypoint/reverts"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
	"github.com/zarbanio/bundler/pkg/errors"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/tracer"
	"github.com/zarbanio/bundler/pkg/userop"
)

type TraceInput struct {
	Rpc        *ethRpc.Client
	EntryPoint common.Address
	Op         *userop.UserOperation
	Sos        state.OverrideSet
	ChainID    *big.Int
	Tracer     string

	// Optional params for simulateHandleOps
	Target      common.Address
	Data        []byte
	TraceFeeCap *big.Int
}

type TraceOutput struct {
	Trace  *tracer.BundlerExecutionReturn
	Result *reverts.ExecutionResultRevert
	Event  *entrypoint.EntrypointUserOperationEvent
}

func TraceSimulateHandleOp(in *TraceInput) (*TraceOutput, error) {
	ep, err := entrypoint.NewEntrypoint(in.EntryPoint, ethclient.NewClient(in.Rpc))
	if err != nil {
		return nil, err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(utils.DummyPk, in.ChainID)
	if err != nil {
		return nil, err
	}
	auth.GasLimit = math.MaxUint64
	auth.NoSend = true
	mf := in.Op.MaxFeePerGas
	if in.TraceFeeCap != nil {
		mf = in.TraceFeeCap
	}
	tx, err := ep.SimulateHandleOp(auth, entrypoint.UserOperation(*in.Op), in.Target, in.Data)
	if err != nil {
		return nil, err
	}
	out := &TraceOutput{}

	var call Call
	req := utils.TraceCallReq{
		From:         common.HexToAddress("0x"),
		To:           in.EntryPoint,
		Data:         tx.Data(),
		MaxFeePerGas: hexutil.Big(*mf),
	}
	opts := utils.TraceCallOpts{
		Tracer:         "callTracer",
		StateOverrides: state.WithMaxBalanceOverride(common.HexToAddress("0x"), in.Sos),
	}
	j, _ := json.MarshalIndent(opts, "", "  ")
	jj, _ := json.MarshalIndent(req, "", "  ")
	fmt.Println(string(jj))
	fmt.Println(string(j))
	if err := in.Rpc.CallContext(context.Background(), &call, "debug_traceCall", &req, "latest", &opts); err != nil {
		return nil, err
	}
	res, err := ProcessBundlerExecution(&call)
	if err != nil {
		return nil, err
	}
	jj, _ = json.MarshalIndent(res, "", "  ")
	fmt.Println(string(jj))

	out.Trace = res
	if res.ValidationOOG {
		return nil, errors.NewRPCError(errors.EXECUTION_REVERTED, "validation OOG", nil)
	}

	// sim, simErr := reverts.NewExecutionResult(fmt.Errorf(res.Error))
	// if simErr != nil {
	// 	fo, foErr := reverts.NewFailedOp(fmt.Errorf(res.Error))
	// 	if foErr != nil && res.Error != "" {
	// 		return nil, errors.NewRPCError(errors.EXECUTION_REVERTED, res.Error, nil)
	// 	} else if foErr != nil {
	// 		return nil, fmt.Errorf("%s, %s", simErr, foErr)
	// 	}
	// 	return nil, errors.NewRPCError(errors.REJECTED_BY_EP_OR_ACCOUNT, fo.Reason, fo)
	// }
	// out.Result = sim

	if len(res.Reverts) != 0 {
		data, err := hexutil.Decode(res.Reverts[len(res.Reverts)-1])
		if err != nil {
			return out, err
		}

		if len(data) == 0 {
			if res.ExecutionOOG {
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
		return out, errors.NewRPCError(errors.EXECUTION_REVERTED, reason, reason)
	}

	return out, nil
}
