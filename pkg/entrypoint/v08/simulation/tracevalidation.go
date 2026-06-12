package simulation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/altmempools"
	"github.com/zarbanio/bundler/pkg/entrypoint/methods"
	simutils "github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
	v08 "github.com/zarbanio/bundler/pkg/entrypoint/v08"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/tracer"
	"github.com/zarbanio/bundler/pkg/userop"
)

type TraceInput struct {
	Rpc                *rpc.Client
	EntryPoint         common.Address
	Op                 *userop.UserOperation
	ChainID            *big.Int
	IsRIP7212Supported bool
	Tracer             string
	Stakes             simutils.EntityStakes
	AltMempools        *altmempools.Directory
	DelegateAddress    common.Address
}

type TraceOutput struct {
	TouchedContracts []common.Address
	AltMempoolIds    []string
	// Trace is the raw collector output the validation rules were evaluated
	// against; batch-level cross-op rules (e.g. STO-041) consume it.
	Trace *tracer.BundlerCollectorReturn
}

func TraceSimulateValidation(in *TraceInput) (*TraceOutput, error) {
	ep, err := v08.NewV08(in.EntryPoint, ethclient.NewClient(in.Rpc))
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
	tx, err := ep.HandleOps(auth, []v08.PackedUserOperation{*packedOp}, beneficiary)
	if err != nil {
		return nil, err
	}

	req := utils.TraceCallReq{
		From:         common.HexToAddress("0x"),
		To:           in.EntryPoint,
		Data:         tx.Data(),
		MaxFeePerGas: hexutil.Big(*big.NewInt(0)),
	}
	opts := utils.TraceCallOpts{
		StateOverrides: state.WithMaxBalanceOverride(common.HexToAddress("0x"), nil),
	}
	if in.DelegateAddress != (common.Address{}) {
		// Run the trace as if the sender's EIP-7702 delegation were applied.
		code := append([]byte{0xef, 0x01, 0x00}, in.DelegateAddress.Bytes()...)
		opts.StateOverrides = state.WithCodeOverride(in.Op.Sender, code, opts.StateOverrides)
	}

	var res *tracer.BundlerCollectorReturn
	if tracer.IsNativeCollector(in.Tracer) {
		opts.Tracer = tracer.NativeCollector
		opts.TracerConfig = tracer.Erc7562TracerConfig()

		var frame tracer.Erc7562Frame
		if err := in.Rpc.CallContext(context.Background(), &frame, "debug_traceCall", &req, "latest", &opts); err != nil {
			return nil, err
		}
		res, err = tracer.FromErc7562(&frame, in.EntryPoint, nil)
		if err != nil {
			return nil, err
		}
	} else {
		collector, err := tracer.Loaded.ForVersion(userop.V0_8, in.EntryPoint.Hex())
		if err != nil {
			return nil, err
		}
		opts.Tracer = collector.Collector

		var ret tracer.BundlerCollectorReturn
		if err := in.Rpc.CallContext(context.Background(), &ret, "debug_traceCall", &req, "latest", &opts); err != nil {
			return nil, err
		}
		res = &ret
	}

	return ProcessTraceResultWithRules(
		in.Op, in.EntryPoint, in.Stakes, res, in.DelegateAddress, in.IsRIP7212Supported, in.AltMempools,
	)
}

// ProcessTraceResultWithRules runs ProcessTraceResult and then the shared
// per-entity ERC-7562 rules (CREATE2 restrictions and storage slot access).
func ProcessTraceResultWithRules(
	op *userop.UserOperation,
	entryPoint common.Address,
	stakes simutils.EntityStakes,
	res *tracer.BundlerCollectorReturn,
	delegateAddr common.Address,
	isRIP7212Supported bool,
	alt *altmempools.Directory,
) (*TraceOutput, error) {
	out, err := ProcessTraceResult(op, entryPoint, stakes, res, delegateAddr)
	if err != nil {
		return nil, err
	}
	knownEntity, err := simutils.NewKnownEntity(op, res, stakes, delegateAddr)
	if err != nil {
		return nil, err
	}
	ids, err := simutils.ValidateEntityRules(userop.V0_8, op, entryPoint, isRIP7212Supported, alt, stakes, res, knownEntity)
	if err != nil {
		return nil, err
	}
	out.AltMempoolIds = append(out.AltMempoolIds, ids...)
	return out, nil
}

func ProcessTraceResult(
	op *userop.UserOperation,
	entryPoint common.Address,
	stakes simutils.EntityStakes,
	res *tracer.BundlerCollectorReturn,
	delegateAddr common.Address,
) (*TraceOutput, error) {
	knownEntity, err := simutils.NewKnownEntity(op, res, stakes, delegateAddr)
	altMempoolIds := []string{}
	if err != nil {
		return nil, err
	}

	ic := mapset.NewSet[common.Address]()
	for title, entity := range knownEntity {
		if entity.Info.OOG {
			return nil, fmt.Errorf("%s OOG", title)
		}
		if _, ok := entity.Info.ExtCodeAccessInfo[entryPoint]; ok {
			return nil, fmt.Errorf("%s has forbidden EXTCODE* access to the EntryPoint", title)
		}
		for opcode := range entity.Info.Opcodes {
			if simutils.BannedOpCodeSet.Contains(opcode) {
				return nil, fmt.Errorf("%s uses banned opcode: %s", title, opcode)
			}
			if !entity.IsStaked && simutils.BannedUnstakedOpCodeSet.Contains(opcode) {
				return nil, fmt.Errorf("unstaked %s uses banned opcode: %s", title, opcode)
			}
		}
		ic.Add(entity.Address)
		for addr := range entity.Info.ContractSize {
			ic.Add(addr)
		}
	}

	callStack := simutils.NewCallStack(res.Calls)
	for _, call := range callStack {
		if call.Method == methods.PackedValidatePaymasterUserOpSelector {
			out, err := methods.DecodeValidatePaymasterUserOpOutput(call.Return)
			if err != nil {
				return nil, fmt.Errorf("unexpected tracing result: %s", err)
			}
			if len(out.Context) != 0 && !knownEntity["paymaster"].IsStaked {
				return nil, errors.New("unstaked paymaster must not return context")
			}
		} else if call.To == entryPoint && call.Method == methods.BalanceOfSelector {
			return nil, fmt.Errorf(
				"%s cannot call balanceOf on EntryPoint",
				simutils.Addr2KnownEntity(op, call.From),
			)
		} else if call.To != entryPoint && call.Value.Cmp(common.Big0) == 1 {
			return nil, fmt.Errorf(
				"%s has a forbidden value transfer to %s",
				simutils.Addr2KnownEntity(op, call.From),
				simutils.Addr2KnownEntity(op, call.To),
			)
		}
	}

	return &TraceOutput{
		TouchedContracts: ic.ToSlice(),
		AltMempoolIds:    altMempoolIds,
		Trace:            res,
	}, nil
}
