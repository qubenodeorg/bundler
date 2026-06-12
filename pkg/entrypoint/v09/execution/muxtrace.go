package execution

import (
	"context"
	"encoding/json"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/altmempools"
	v06exec "github.com/zarbanio/bundler/pkg/entrypoint/execution"
	simutils "github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
	epv09 "github.com/zarbanio/bundler/pkg/entrypoint/v09"
	v09sim "github.com/zarbanio/bundler/pkg/entrypoint/v09/simulation"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/tracer"
	"github.com/zarbanio/bundler/pkg/userop"
)

// CombinedTraceInput drives a single muxTracer debug_traceCall of
// EntryPoint.handleOps that yields both the ERC-7562 validation result and
// the callTracer execution result in one round-trip (one EVM run).
type CombinedTraceInput struct {
	Rpc                *ethRpc.Client
	EntryPoint         common.Address
	Op                 *userop.UserOperation
	ChainID            *big.Int
	IsRIP7212Supported bool
	Stakes             simutils.EntityStakes
	AltMempools        *altmempools.Directory
	DelegateAddress    common.Address
	Sos                state.OverrideSet
	TraceFeeCap        *big.Int
}

// CombinedTraceOutput carries both halves of the muxed trace.
type CombinedTraceOutput struct {
	Validation *v09sim.TraceOutput
	Execution  *tracer.BundlerExecutionReturn
}

// TraceCombined replaces the separate collector and executor debug_traceCalls
// with one muxTracer call. Both sub-tracers observe the exact same handleOps
// execution, so this halves the trace round-trips for the v0.9
// validation+execution pass. Note the request uses the execution-path fee and
// state overrides; use the dedicated single-tracer paths when the two passes
// must run under different conditions.
func TraceCombined(in *CombinedTraceInput) (*CombinedTraceOutput, error) {
	ep, err := epv09.NewV09(in.EntryPoint, ethclient.NewClient(in.Rpc))
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

	// handleOps reverts AA90 for a zero beneficiary even when the op is valid,
	// so trace with a dummy non-zero beneficiary.
	beneficiary := common.BigToAddress(common.Big1)
	tx, err := ep.HandleOps(auth, []epv09.PackedUserOperation{*packedOp}, beneficiary)
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

	req := utils.TraceCallReq{
		From:         common.HexToAddress("0x"),
		To:           in.EntryPoint,
		Data:         tx.Data(),
		MaxFeePerGas: hexutil.Big(*mf),
	}
	opts := utils.TraceCallOpts{
		Tracer:         tracer.MuxTracer,
		TracerConfig:   tracer.MuxTracerConfig(),
		StateOverrides: state.WithMaxBalanceOverride(common.HexToAddress("0x"), in.Sos),
	}

	var muxed tracer.MuxedTrace
	if err := in.Rpc.CallContext(context.Background(), &muxed, "debug_traceCall", &req, "latest", &opts); err != nil {
		return nil, err
	}

	collected, err := tracer.FromErc7562(muxed.Erc7562, in.EntryPoint, v09sim.PaymasterSignatureHook)
	if err != nil {
		return nil, err
	}
	validation, err := v09sim.ProcessTraceResultWithRules(
		in.Op, in.EntryPoint, in.Stakes, collected, in.DelegateAddress, in.IsRIP7212Supported, in.AltMempools,
	)
	if err != nil {
		return nil, err
	}

	var call v06exec.Call
	if err := json.Unmarshal(muxed.CallTracer, &call); err != nil {
		return nil, err
	}
	epABI, err := epv09.V09MetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	execTrace, err := v06exec.ProcessBundlerExecution(&call, v06exec.NewTraceConfig(epABI, in.EntryPoint))
	if err != nil {
		return nil, err
	}

	return &CombinedTraceOutput{
		Validation: validation,
		Execution:  execTrace,
	}, nil
}
