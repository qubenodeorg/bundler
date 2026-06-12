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
	"github.com/zarbanio/bundler/pkg/entrypoint"
	"github.com/zarbanio/bundler/pkg/entrypoint/methods"
	"github.com/zarbanio/bundler/pkg/entrypoint/utils"
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
	Stakes             EntityStakes
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

// TraceSimulateValidation makes a debug_traceCall to Entrypoint.simulateValidation(userop) and returns
// information related to the validation phase of a UserOperation.
func TraceSimulateValidation(in *TraceInput) (*TraceOutput, error) {
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
	tx, err := ep.SimulateValidation(auth, entrypoint.UserOperation(in.Op.Core()))
	if err != nil {
		return nil, err
	}

	req := utils.TraceCallReq{
		From:         common.HexToAddress("0x"),
		To:           in.EntryPoint,
		Data:         tx.Data(),
		MaxFeePerGas: hexutil.Big(*in.Op.MaxFeePerGas),
	}
	opts := utils.TraceCallOpts{
		StateOverrides: state.WithMaxBalanceOverride(common.HexToAddress("0x"), nil),
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
		opts.Tracer = tracer.Loaded.V06.Collector

		var ret tracer.BundlerCollectorReturn
		if err := in.Rpc.CallContext(context.Background(), &ret, "debug_traceCall", &req, "latest", &opts); err != nil {
			return nil, err
		}
		res = &ret
	}

	knownEntity, err := newKnownEntity(in.Op, res, in.Stakes, in.DelegateAddress)
	altMempoolIds := []string{}
	if err != nil {
		return nil, err
	}

	ic := mapset.NewSet[common.Address]()
	for title, entity := range knownEntity {
		if entity.Info.OOG {
			return nil, fmt.Errorf("%s OOG", title)
		}
		if _, ok := entity.Info.ExtCodeAccessInfo[in.EntryPoint]; ok {
			return nil, fmt.Errorf("%s has forbidden EXTCODE* access to the EntryPoint", title)
		}
		for opcode := range entity.Info.Opcodes {
			if bannedOpCodes.Contains(opcode) {
				return nil, fmt.Errorf("%s uses banned opcode: %s", title, opcode)
			}

			if !entity.IsStaked && bannedUnstakedOpCodes.Contains(opcode) {
				return nil, fmt.Errorf("unstaked %s uses banned opcode: %s", title, opcode)
			}
		}

		ic.Add(entity.Address)
		for addr := range entity.Info.ContractSize {
			ic.Add(addr)
		}
	}

	ids, err := ValidateEntityRules(
		userop.V0_6, in.Op, in.EntryPoint, in.IsRIP7212Supported, in.AltMempools, in.Stakes, res, knownEntity,
	)
	if err != nil {
		return nil, err
	}
	altMempoolIds = append(altMempoolIds, ids...)

	callStack := newCallStack(res.Calls)
	for _, call := range callStack {
		if call.Method == methods.ValidatePaymasterUserOpSelector {
			out, err := methods.DecodeValidatePaymasterUserOpOutput(call.Return)
			if err != nil {
				return nil, fmt.Errorf(
					"unexpected tracing result for op: %s, %s",
					in.Op.GetUserOpHash(in.EntryPoint, in.ChainID),
					err,
				)
			}

			if len(out.Context) != 0 && !knownEntity["paymaster"].IsStaked {
				return nil, errors.New("unstaked paymaster must not return context")
			}
		} else if call.To == in.EntryPoint && call.Method == methods.BalanceOfSelector {
			return nil, fmt.Errorf(
				"%s cannot call balanceOf on EntryPoint",
				addr2KnownEntity(in.Op, call.From),
			)
		} else if call.To != in.EntryPoint && call.Value.Cmp(common.Big0) == 1 {
			return nil, fmt.Errorf(
				"%s has a forbidden value transfer to %s",
				addr2KnownEntity(in.Op, call.From),
				addr2KnownEntity(in.Op, call.To),
			)
		}
	}

	return &TraceOutput{
		TouchedContracts: ic.ToSlice(),
		AltMempoolIds:    altMempoolIds,
		Trace:            res,
	}, nil
}
