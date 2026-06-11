package gas

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	v06exec "github.com/zarbanio/bundler/pkg/entrypoint/execution"
	v07exec "github.com/zarbanio/bundler/pkg/entrypoint/v07/execution"
	v08exec "github.com/zarbanio/bundler/pkg/entrypoint/v08/execution"
	v09exec "github.com/zarbanio/bundler/pkg/entrypoint/v09/execution"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/tracer"
	"github.com/zarbanio/bundler/pkg/userop"
)

var (
	fallBackBinarySearchCutoff = int64(30000)
	maxRetries                 = int64(7)
	baseVGLBuffer              = int64(25)
)

func isPrefundNotPaid(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "AA21 didn't pay prefund") ||
		strings.Contains(err.Error(), "AA31 paymaster deposit too low")
}

func isValidationOOG(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "AA40 over verificationGasLimit") ||
		strings.Contains(err.Error(), "AA26 over verificationGasLimit") ||
		strings.Contains(err.Error(), "AA41 too little verificationGas") ||
		strings.Contains(err.Error(), "AA51 prefund below actualGasCost") ||
		strings.Contains(err.Error(), "AA13 initCode failed or OOG") ||
		strings.Contains(err.Error(), "AA23 reverted") ||
		strings.Contains(err.Error(), "AA33 reverted") ||
		strings.Contains(err.Error(), "return data out of bounds") ||
		strings.Contains(err.Error(), "validation OOG")
}

func isExecutionOOG(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "execution OOG")
}

func isExecutionReverted(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "execution reverted")
}

func readBigStr(data map[string]any, key string) *big.Int {
	v, ok := data[key].(string)
	if !ok {
		return big.NewInt(0)
	}
	n := new(big.Int)
	n.SetString(v, 0)
	return n
}

type EstimateInput struct {
	Rpc         *rpc.Client
	EntryPoint  common.Address
	Op          userop.UserOperationInterface
	Sos         state.OverrideSet
	Ov          *Overhead
	ChainID     *big.Int
	MaxGasLimit *big.Int
	Tracer      string
	Version     userop.Version

	attempts int64
	lastVGL  int64
}

func retryEstimateGas(err error, vgl int64, in *EstimateInput) (uint64, uint64, error) {
	if isValidationOOG(err) && in.attempts < maxRetries {
		return EstimateGas(&EstimateInput{
			Rpc:         in.Rpc,
			EntryPoint:  in.EntryPoint,
			Op:          in.Op,
			Sos:         in.Sos,
			Ov:          in.Ov,
			ChainID:     in.ChainID,
			MaxGasLimit: in.MaxGasLimit,
			Tracer:      in.Tracer,
			Version:     in.Version,
			attempts:    in.attempts + 1,
			lastVGL:     vgl,
		})
	}
	return 0, 0, err
}

func simulateHandleOp(in *EstimateInput, op userop.UserOperationInterface, sos state.OverrideSet) error {
	switch in.Version {
	case userop.V0_6:
		v06op, ok := op.(*userop.UserOperation)
		if !ok {
			var err error
			v06op, err = ConvertToV06(op)
			if err != nil {
				return err
			}
		}
		_, err := v06exec.SimulateHandleOp(&v06exec.SimulateInput{
			Rpc:        in.Rpc,
			EntryPoint: in.EntryPoint,
			Op:         v06op,
			Sos:        sos,
			ChainID:    in.ChainID,
		})
		return err
	case userop.V0_7:
		return v07exec.SimulateHandleOp(&v07exec.SimulateInput{
			Rpc:        in.Rpc,
			EntryPoint: in.EntryPoint,
			Op:         op,
			Sos:        sos,
			ChainID:    in.ChainID,
		})
	case userop.V0_8:
		return v08exec.SimulateHandleOp(&v08exec.SimulateInput{
			Rpc:        in.Rpc,
			EntryPoint: in.EntryPoint,
			Op:         op,
			Sos:        sos,
			ChainID:    in.ChainID,
		})
	case userop.V0_9:
		return v09exec.SimulateHandleOp(&v09exec.SimulateInput{
			Rpc:        in.Rpc,
			EntryPoint: in.EntryPoint,
			Op:         op,
			Sos:        sos,
			ChainID:    in.ChainID,
		})
	default:
		return fmt.Errorf("unsupported version: %s", in.Version)
	}
}

func traceSimulateHandleOp(in *EstimateInput, op userop.UserOperationInterface, sos state.OverrideSet, traceFeeCap *big.Int) (*tracer.BundlerExecutionReturn, error) {
	switch in.Version {
	case userop.V0_6:
		v06op, ok := op.(*userop.UserOperation)
		if !ok {
			var err error
			v06op, err = ConvertToV06(op)
			if err != nil {
				return nil, err
			}
		}
		out, err := v06exec.TraceSimulateHandleOp(&v06exec.TraceInput{
			Rpc:         in.Rpc,
			EntryPoint:  in.EntryPoint,
			Op:          v06op,
			Sos:         sos,
			ChainID:     in.ChainID,
			TraceFeeCap: traceFeeCap,
			Tracer:      in.Tracer,
		})
		if err != nil {
			return nil, err
		}
		return out.Trace, nil
	case userop.V0_7:
		out, err := v07exec.TraceSimulateHandleOp(&v07exec.TraceInput{
			Rpc:         in.Rpc,
			EntryPoint:  in.EntryPoint,
			Op:          op,
			Sos:         sos,
			ChainID:     in.ChainID,
			TraceFeeCap: traceFeeCap,
			Tracer:      in.Tracer,
		})
		if err != nil {
			return nil, err
		}
		return out.Trace, nil
	case userop.V0_8:
		out, err := v08exec.TraceSimulateHandleOp(&v08exec.TraceInput{
			Rpc:         in.Rpc,
			EntryPoint:  in.EntryPoint,
			Op:          op,
			Sos:         sos,
			ChainID:     in.ChainID,
			TraceFeeCap: traceFeeCap,
			Tracer:      in.Tracer,
		})
		if err != nil {
			return nil, err
		}
		return out.Trace, nil
	case userop.V0_9:
		out, err := v09exec.TraceSimulateHandleOp(&v09exec.TraceInput{
			Rpc:         in.Rpc,
			EntryPoint:  in.EntryPoint,
			Op:          op,
			Sos:         sos,
			ChainID:     in.ChainID,
			TraceFeeCap: traceFeeCap,
			Tracer:      in.Tracer,
		})
		if err != nil {
			return nil, err
		}
		return out.Trace, nil
	default:
		return nil, fmt.Errorf("unsupported version: %s", in.Version)
	}
}

// createOp creates a new UserOperation from the data map for the given version.
func createOp(data map[string]any, version userop.Version) (userop.UserOperationInterface, error) {
	if version == userop.V0_6 {
		return userop.New(data)
	}
	vop, err := userop.NewUserOperation(version, data)
	if err != nil {
		return nil, err
	}
	return vop.Op, nil
}

// ConvertToV06 converts a versioned UserOperation (v0.7+) to the v0.6 UserOperation format
// for backward-compatible simulation with v0.6 EntryPoint execution packages.
func ConvertToV06(op userop.UserOperationInterface) (*userop.UserOperation, error) {
	data, err := op.ToMap()
	if err != nil {
		return nil, err
	}

	if _, hasInitCode := data["initCode"]; !hasInitCode {
		if factory, ok := data["factory"].(string); ok && factory != "0x0000000000000000000000000000000000000000" {
			fd := "0x"
			if factoryData, ok := data["factoryData"].(string); ok {
				fd = factoryData
			}
			if len(fd) > 2 {
				data["initCode"] = factory + fd[2:]
			} else {
				data["initCode"] = factory
			}
		} else {
			data["initCode"] = "0x"
		}
	}

	if _, hasPM := data["paymasterAndData"]; !hasPM {
		if pm, ok := data["paymaster"].(string); ok && common.HexToAddress(pm) != (common.Address{}) {
			pmData := "0x"
			if d, ok := data["paymasterData"].(string); ok {
				pmData = d
			}
			// v0.7+ packed paymasterAndData: address(20) + gasLimits(32) + data
			dummyGasLimits := "0000000000000000000000000000000000000000000000000000000000000000"
			if len(pmData) > 2 {
				data["paymasterAndData"] = pm + dummyGasLimits + pmData[2:]
			} else {
				data["paymasterAndData"] = pm + dummyGasLimits
			}
		} else {
			data["paymasterAndData"] = "0x"
		}
	}

	delete(data, "factory")
	delete(data, "factoryData")
	delete(data, "paymaster")
	delete(data, "paymasterVerificationGasLimit")
	delete(data, "paymasterPostOpGasLimit")
	delete(data, "paymasterData")
	delete(data, "eip7702Auth")
	delete(data, "paymasterSignature")

	return userop.New(data)
}

// EstimateGas derives an estimate for verificationGasLimit and callGasLimit by simulating
// the user operation against the EntryPoint contract. It dispatches to version-specific
// execution simulation packages (v0.6, v0.7, v0.8, v0.9).
func EstimateGas(in *EstimateInput) (verificationGas uint64, callGas uint64, err error) {
	data, err := in.Op.ToMap()
	if err != nil {
		return 0, 0, err
	}
	data["maxPriorityFeePerGas"] = hexutil.EncodeBig(in.Op.GetMaxFeePerGas())
	data["verificationGasLimit"] = hexutil.EncodeBig(big.NewInt(0))
	data["callGasLimit"] = hexutil.EncodeBig(big.NewInt(0))

	sosCpy, err := state.Copy(in.Sos)
	if err != nil {
		return 0, 0, err
	}
	if in.Op.GetPaymaster() == common.HexToAddress("0x") {
		sosCpy = state.WithMaxBalanceOverride(in.Op.GetSender(), sosCpy)
	}

	// Find the optimal verificationGasLimit with binary search.
	l := int64(0)
	r := in.MaxGasLimit.Int64()
	f := in.lastVGL
	var simErr error
	for in.lastVGL == 0 && r-l >= fallBackBinarySearchCutoff {
		m := (l + r) / 2

		data["verificationGasLimit"] = hexutil.EncodeBig(big.NewInt(int64(m)))
		simOp, err := createOp(data, in.Version)
		if err != nil {
			return 0, 0, err
		}
		err = simulateHandleOp(in, simOp, sosCpy)
		simErr = err
		if err == nil {
			r = m - 1
			f = m
			continue
		} else if isPrefundNotPaid(err) {
			r = m - 1
			continue
		} else if isValidationOOG(err) {
			l = m + 1
			continue
		} else {
			return 0, 0, err
		}
	}
	if f == 0 {
		return 0, 0, simErr
	}
	f = (f * (100 + baseVGLBuffer)) / 100
	data["verificationGasLimit"] = hexutil.EncodeBig(big.NewInt(int64(f)))

	// Find the optimal callGasLimit by setting the gas price to 0 and maxing out the gas limit.
	data["maxFeePerGas"] = hexutil.EncodeBig(big.NewInt(0))
	data["maxPriorityFeePerGas"] = hexutil.EncodeBig(big.NewInt(0))
	data["callGasLimit"] = hexutil.EncodeBig(in.MaxGasLimit)
	simOp, err := createOp(data, in.Version)
	if err != nil {
		return 0, 0, err
	}
	out, err := traceSimulateHandleOp(in, simOp, in.Sos, in.Op.GetMaxFeePerGas())
	if err != nil {
		return retryEstimateGas(err, f, in)
	}

	vgl := readBigStr(data, "verificationGasLimit")
	cgl := big.NewInt(int64(out.ExecutionGasLimit))
	if cgl.Cmp(in.Ov.NonZeroValueCall()) < 0 {
		cgl = in.Ov.NonZeroValueCall()
	}

	// Run a final simulation using actual gas fees and to confirm one-shot tracing is ok.
	data["maxFeePerGas"] = hexutil.EncodeBig(in.Op.GetMaxFeePerGas())
	data["maxPriorityFeePerGas"] = hexutil.EncodeBig(in.Op.GetMaxFeePerGas())
	data["verificationGasLimit"] = hexutil.EncodeBig(vgl)
	data["callGasLimit"] = hexutil.EncodeBig(cgl)
	simOp, err = createOp(data, in.Version)
	if err != nil {
		return 0, 0, err
	}

	_, err = traceSimulateHandleOp(in, simOp, sosCpy, nil)
	if err != nil {
		if isExecutionOOG(err) || isExecutionReverted(err) {
			l := cgl.Int64()
			r := in.MaxGasLimit.Int64()
			f := int64(0)
			simErr := err
			for r-l >= fallBackBinarySearchCutoff {
				m := (l + r) / 2

				data["callGasLimit"] = hexutil.EncodeBig(big.NewInt(int64(m)))
				simOp, err := createOp(data, in.Version)
				if err != nil {
					return 0, 0, err
				}
				_, err = traceSimulateHandleOp(in, simOp, sosCpy, nil)
				simErr = err
				if err == nil {
					r = m - 1
					f = m
					continue
				} else if isPrefundNotPaid(err) {
					r = m - 1
				} else if isExecutionOOG(err) || isExecutionReverted(err) {
					l = m + 1
					continue
				} else {
					return 0, 0, err
				}
			}
			if f == 0 {
				return 0, 0, simErr
			}
			return readBigStr(data, "verificationGasLimit").Uint64(), big.NewInt(f).Uint64(), nil
		}
		return retryEstimateGas(err, readBigStr(data, "verificationGasLimit").Int64(), in)
	}
	return readBigStr(data, "verificationGasLimit").Uint64(), readBigStr(data, "callGasLimit").Uint64(), nil
}
