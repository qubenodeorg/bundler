package gas

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/pkg/entrypoint/execution"
	"github.com/zarbanio/bundler/pkg/state"
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
		if pm, ok := data["paymaster"].(string); ok && pm != "0x0000000000000000000000000000000000000000" {
			pmData := "0x"
			if d, ok := data["paymasterData"].(string); ok {
				pmData = d
			}
			if len(pmData) > 2 {
				data["paymasterAndData"] = pm + pmData[2:]
			} else {
				data["paymasterAndData"] = pm
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

// retryEstimateGas will recursively call estimateGas if execution has caused VGL to be under estimated. This
// can occur for edge cases where a paymaster's postOp > gas required during verification or if verification
// has a dependency on CGL. Reset the estimate with a higher buffer on VGL.
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
			attempts:    in.attempts + 1,
			lastVGL:     vgl,
		})
	}
	return 0, 0, err
}

// EstimateGas uses the simulateHandleOp method on the EntryPoint to derive an estimate for
// verificationGasLimit and callGasLimit.
//
// Note: For v0.7+ UserOperations, the input Op is converted to v0.6 format for simulation since
// the v0.6 execution packages are used. This is a temporary limitation until v0.7+ execution
// simulation packages are available (see Days 5-6 of the migration roadmap).
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
		simOp, err := userop.New(data)
		if err != nil {
			return 0, 0, err
		}
		_, err = execution.SimulateHandleOp(&execution.SimulateInput{
			Rpc:        in.Rpc,
			EntryPoint: in.EntryPoint,
			Op:         simOp,
			Sos:        sosCpy,
			ChainID:    in.ChainID,
		})
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
	simOp, err := userop.New(data)
	if err != nil {
		return 0, 0, err
	}
	out, err := execution.TraceSimulateHandleOp(&execution.TraceInput{
		Rpc:         in.Rpc,
		EntryPoint:  in.EntryPoint,
		Op:          simOp,
		Sos:         in.Sos,
		ChainID:     in.ChainID,
		TraceFeeCap: in.Op.GetMaxFeePerGas(),
		Tracer:      in.Tracer,
	})
	if err != nil {
		return retryEstimateGas(err, f, in)
	}

	vgl := simOp.VerificationGasLimit
	cgl := big.NewInt(int64(out.Trace.ExecutionGasLimit))
	if cgl.Cmp(in.Ov.NonZeroValueCall()) < 0 {
		cgl = in.Ov.NonZeroValueCall()
	}

	// Run a final simulation using actual gas fees and to confirm one-shot tracing is ok.
	data["maxFeePerGas"] = hexutil.EncodeBig(in.Op.GetMaxFeePerGas())
	data["maxPriorityFeePerGas"] = hexutil.EncodeBig(in.Op.GetMaxFeePerGas())
	data["verificationGasLimit"] = hexutil.EncodeBig(vgl)
	data["callGasLimit"] = hexutil.EncodeBig(cgl)
	simOp, err = userop.New(data)
	if err != nil {
		return 0, 0, err
	}

	_, err = execution.TraceSimulateHandleOp(&execution.TraceInput{
		Rpc:        in.Rpc,
		EntryPoint: in.EntryPoint,
		Op:         simOp,
		Sos:        sosCpy,
		ChainID:    in.ChainID,
		Tracer:     in.Tracer,
	})
	if err != nil {
		if isExecutionOOG(err) || isExecutionReverted(err) {
			l := cgl.Int64()
			r := in.MaxGasLimit.Int64()
			f := int64(0)
			simErr := err
			for r-l >= fallBackBinarySearchCutoff {
				m := (l + r) / 2

				data["callGasLimit"] = hexutil.EncodeBig(big.NewInt(int64(m)))
				simOp, err := userop.New(data)
				if err != nil {
					return 0, 0, err
				}
				_, err = execution.TraceSimulateHandleOp(&execution.TraceInput{
					Rpc:        in.Rpc,
					EntryPoint: in.EntryPoint,
					Op:         simOp,
					Sos:        sosCpy,
					ChainID:    in.ChainID,
					Tracer:     in.Tracer,
				})
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
			return simOp.VerificationGasLimit.Uint64(), big.NewInt(f).Uint64(), nil
		}
		return retryEstimateGas(err, simOp.VerificationGasLimit.Int64(), in)
	}
	return simOp.VerificationGasLimit.Uint64(), simOp.CallGasLimit.Uint64(), nil
}


