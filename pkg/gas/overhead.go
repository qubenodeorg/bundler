package gas

import (
	"bytes"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/zarbanio/bundler/internal/utils"
	"github.com/zarbanio/bundler/pkg/userop"
)

// Overhead provides helper methods for calculating gas limits based on pre-defined parameters.
type Overhead struct {
	intrinsicFixed      float64
	perUserOpFixed      float64
	perUserOpMultiplier float64
	zeroByte            float64
	nonZeroByte         float64
	minBundleSize       float64
	warmStorageRead     float64
	callWithValue       float64
	callOpcode          float64
	nonZeroValueStipend float64
	sanitizedPVG        *big.Int
	sanitizedVGL        *big.Int
	sanitizedCGL        *big.Int
	calcPVGFunc         CalcPreVerificationGasFunc
	pvgBufferFactor     int64
	version             userop.Version
	unusedGasPenaltyPct int64
	gasPenaltyThreshold *big.Int
}

func newOverhead() *Overhead {
	return &Overhead{
		intrinsicFixed:      21000,
		perUserOpFixed:      22874,
		perUserOpMultiplier: 25,
		zeroByte:            4,
		nonZeroByte:         16,
		minBundleSize:       1,
		warmStorageRead:     100,
		callWithValue:       9000,
		callOpcode:          700,
		nonZeroValueStipend: 2300,
		sanitizedPVG:        big.NewInt(100000),
		sanitizedVGL:        big.NewInt(1000000),
		sanitizedCGL:        big.NewInt(1000000),
		calcPVGFunc:         calcPVGFuncNoop(),
		pvgBufferFactor:     0,
		version:             userop.V0_6,
	}
}

// NewDefaultOverhead returns an instance of Overhead using v0.6 parameters.
func NewDefaultOverhead() *Overhead {
	return newOverhead()
}

// NewDefaultOverheadV07 returns an instance of Overhead with v0.7 gas parameters including
// the 10% unused gas penalty.
func NewDefaultOverheadV07() *Overhead {
	return &Overhead{
		intrinsicFixed:      21000,
		perUserOpFixed:      22874,
		perUserOpMultiplier: 25,
		zeroByte:            4,
		nonZeroByte:         16,
		minBundleSize:       1,
		warmStorageRead:     100,
		callWithValue:       9000,
		callOpcode:          700,
		nonZeroValueStipend: 2300,
		sanitizedPVG:        big.NewInt(100000),
		sanitizedVGL:        big.NewInt(1000000),
		sanitizedCGL:        big.NewInt(1000000),
		calcPVGFunc:         calcPVGFuncNoop(),
		pvgBufferFactor:     0,
		version:             userop.V0_7,
		unusedGasPenaltyPct: 1000,
	}
}

// NewDefaultOverheadV08 returns an instance of Overhead with v0.8 gas parameters including
// the 10% unused gas penalty and 40,000 gas penalty threshold.
func NewDefaultOverheadV08() *Overhead {
	return &Overhead{
		intrinsicFixed:      21000,
		perUserOpFixed:      22874,
		perUserOpMultiplier: 25,
		zeroByte:            4,
		nonZeroByte:         16,
		minBundleSize:       1,
		warmStorageRead:     100,
		callWithValue:       9000,
		callOpcode:          700,
		nonZeroValueStipend: 2300,
		sanitizedPVG:        big.NewInt(100000),
		sanitizedVGL:        big.NewInt(1000000),
		sanitizedCGL:        big.NewInt(1000000),
		calcPVGFunc:         calcPVGFuncNoop(),
		pvgBufferFactor:     0,
		version:             userop.V0_8,
		unusedGasPenaltyPct: 1000,
		gasPenaltyThreshold: big.NewInt(40000),
	}
}

// NewDefaultOverheadV09 returns an instance of Overhead with v0.9 gas parameters (same as v0.8).
func NewDefaultOverheadV09() *Overhead {
	return &Overhead{
		intrinsicFixed:      21000,
		perUserOpFixed:      22874,
		perUserOpMultiplier: 25,
		zeroByte:            4,
		nonZeroByte:         16,
		minBundleSize:       1,
		warmStorageRead:     100,
		callWithValue:       9000,
		callOpcode:          700,
		nonZeroValueStipend: 2300,
		sanitizedPVG:        big.NewInt(100000),
		sanitizedVGL:        big.NewInt(1000000),
		sanitizedCGL:        big.NewInt(1000000),
		calcPVGFunc:         calcPVGFuncNoop(),
		pvgBufferFactor:     0,
		version:             userop.V0_9,
		unusedGasPenaltyPct: 1000,
		gasPenaltyThreshold: big.NewInt(40000),
	}
}

// NewDefaultOverheadForVersion returns the Overhead matching the given
// EntryPoint version.
func NewDefaultOverheadForVersion(v userop.Version) *Overhead {
	switch v {
	case userop.V0_7:
		return NewDefaultOverheadV07()
	case userop.V0_8:
		return NewDefaultOverheadV08()
	case userop.V0_9:
		return NewDefaultOverheadV09()
	default:
		return NewDefaultOverhead()
	}
}

// SetCalcPreVerificationGasFunc allows a custom function to be defined that can control how it calculates
// PVG. This is useful for networks that have different models for gas.
func (ov *Overhead) SetCalcPreVerificationGasFunc(fn CalcPreVerificationGasFunc) {
	ov.calcPVGFunc = fn
}

// SetPreVerificationGasBufferFactor defines the percentage to increase the preVerificationGas by during an
// estimation. This is useful for rollups that use 2D gas values where the L1 gas component is
// non-deterministic. This buffer accounts for any variability in-between eth_estimateUserOperationGas and
// eth_sendUserOperation. Defaults to 0.
func (ov *Overhead) SetPreVerificationGasBufferFactor(factor int64) {
	ov.pvgBufferFactor = factor
}

// SetUnusedGasPenaltyPercent sets the unused gas penalty percentage in basis points (e.g., 1000 = 10%).
func (ov *Overhead) SetUnusedGasPenaltyPercent(pct int64) {
	ov.unusedGasPenaltyPct = pct
}

// SetGasPenaltyThreshold sets the gas threshold below which no penalty is applied.
// Set to nil for no threshold (penalty always applies).
func (ov *Overhead) SetGasPenaltyThreshold(threshold *big.Int) {
	ov.gasPenaltyThreshold = threshold
}

// GetVersion returns the UserOperation version configured for this Overhead.
func (ov *Overhead) GetVersion() userop.Version {
	return ov.version
}

// GetUnusedGasPenaltyPercent returns the unused gas penalty percentage in basis points.
func (ov *Overhead) GetUnusedGasPenaltyPercent() int64 {
	return ov.unusedGasPenaltyPct
}

// CalcCallDataCost calculates the additional gas cost required to serialize the userOp when making the
// transaction to submit the entire batch.
func (ov *Overhead) CalcCallDataCost(op userop.UserOperationInterface) float64 {
	cost := float64(0)
	for _, b := range op.Pack() {
		if b == byte(0) {
			cost += ov.zeroByte
		} else {
			cost += ov.nonZeroByte
		}
	}

	return cost
}

// CalcPerUserOpCost calculates the gas overhead from processing a UserOperation's validation and execution
// phase. This overhead is not constant and is correlated to the number of 32 byte words in the UserOperation.
// It can be summarized in the equation perUserOpMultiplier * lenInWord + perUserOpFixed.
//
// Note: The constant values have been derived empirically by plotting the relationship between per userOp
// overhead vs length in words with a sample size of 30.
func (ov *Overhead) CalcPerUserOpCost(op userop.UserOperationInterface) float64 {
	opLen := math.Floor(float64(len(op.Pack())+31) / 32)
	cost := (ov.perUserOpMultiplier * opLen) + ov.perUserOpFixed

	return cost
}

func sanitizeOpData(data map[string]any, op userop.UserOperationInterface) {
	data["preVerificationGas"] = hexutil.EncodeBig(big.NewInt(100000))
	data["verificationGasLimit"] = hexutil.EncodeBig(big.NewInt(1000000))
	data["callGasLimit"] = hexutil.EncodeBig(big.NewInt(1000000))

	if sigStr, ok := data["signature"].(string); ok {
		sigBytes, err := hexutil.Decode(sigStr)
		if err == nil {
			data["signature"] = hexutil.Encode(bytes.Repeat([]byte{1}, len(sigBytes)))
		}
	}
}

// CalcPreVerificationGas returns an expected gas cost for processing a UserOperation from a batch.
func (ov *Overhead) CalcPreVerificationGas(op userop.UserOperationInterface) (*big.Int, error) {
	data, err := op.ToMap()
	if err != nil {
		return nil, err
	}

	sanitizeOpData(data, op)

	var tmpPacked []byte

	switch ov.version {
	case userop.V0_6:
		tmp, err := userop.New(data)
		if err != nil {
			return nil, err
		}
		tmpPacked = tmp.Pack()
	default:
		vop, err := userop.NewUserOperation(ov.version, data)
		if err != nil {
			return nil, err
		}
		tmpPacked = vop.Op.Pack()
	}

	var pvg float64
	if ov.version == userop.V0_6 {
		batchOv := (ov.intrinsicFixed / ov.minBundleSize) + ov.calcCallDataCost(tmpPacked)
		pvg = batchOv + ov.calcPerUserOpCost(tmpPacked)
	} else {
		pvg = ov.calcPreVerificationGasV07(op, tmpPacked)
	}
	static := big.NewInt(int64(math.Round(pvg)))

	g, err := ov.calcPVGFunc(op, static)
	if err != nil {
		return nil, err
	}
	if g != nil {
		return g, nil
	}
	return static, nil
}

// Constants mirroring eth-infinitism's PreVerificationGasCalculator MainnetConfig,
// used for the v0.7+ preVerificationGas model including the EIP-7623 calldata
// floor pricing.
const (
	pvgFixedGasOverhead             = 9830
	pvgPerUserOpGasOverhead         = 7260
	pvgPerUserOpWordGasOverhead     = 9.5
	pvgExecuteUserOpGasOverhead     = 1610
	pvgExecuteUserOpPerWordOverhead = 8.2
	pvgStandardTokenGasCost         = 4
	pvgFloorPerTokenGasCost         = 10
	pvgTokensPerNonzeroByte         = 4
	pvgEip7702AuthGas               = 25000
)

// executeUserOpMethodSig is the method selector that triggers the EntryPoint's
// special executeUserOp calldata handling.
var executeUserOpMethodSig = []byte{0x8d, 0xd7, 0x71, 0x2f}

// calcPreVerificationGasV07 mirrors eth-infinitism's PreVerificationGasCalculator:
// the userOp's share of the bundle transaction cost plus the EIP-7623 floor
// price of its calldata tokens. Since the actual validation gas usage is not
// known at this point, the op's own gas limits stand in for it: this slightly
// relaxes the floor branch, which only dominates for large calldata with small
// gas limits.
func (ov *Overhead) calcPreVerificationGasV07(op userop.UserOperationInterface, packed []byte) float64 {
	tokenCount := float64(0)
	for _, b := range packed {
		if b == 0 {
			tokenCount++
		} else {
			tokenCount += pvgTokensPerNonzeroByte
		}
	}
	words := math.Floor(float64(len(packed)+31) / 32)

	perUserOpOverhead := float64(pvgPerUserOpGasOverhead)
	if op.GetEIP7702Auth() != nil {
		perUserOpOverhead += pvgEip7702AuthGas
	}

	callDataOverhead := float64(0)
	callData := op.GetCallData()
	if len(callData) >= 4 && bytes.Equal(callData[:4], executeUserOpMethodSig) {
		perUserOpOverhead += pvgExecuteUserOpGasOverhead + pvgExecuteUserOpPerWordOverhead*words
	} else {
		callDataOverhead = math.Ceil(float64(len(callData))/32) * pvgPerUserOpWordGasOverhead
	}

	overhead := (pvgFixedGasOverhead / ov.minBundleSize) + perUserOpOverhead + callDataOverhead
	stipend := ov.intrinsicFixed / ov.minBundleSize

	gasUsed := float64(0)
	if cgl := op.GetCallGasLimit(); cgl != nil {
		gasUsed += float64(cgl.Int64())
	}
	if pogl := op.GetPaymasterPostOpGasLimit(); pogl != nil {
		gasUsed += float64(pogl.Int64())
	}
	gasUsed /= 10
	if vgl := op.GetVerificationGasLimit(); vgl != nil {
		gasUsed += float64(vgl.Int64())
	}

	return stipend + math.Max(
		pvgStandardTokenGasCost*tokenCount+overhead+gasUsed,
		pvgFloorPerTokenGasCost*tokenCount,
	) - gasUsed
}

func (ov *Overhead) calcCallDataCost(packed []byte) float64 {
	cost := float64(0)
	for _, b := range packed {
		if b == byte(0) {
			cost += ov.zeroByte
		} else {
			cost += ov.nonZeroByte
		}
	}
	return cost
}

func (ov *Overhead) calcPerUserOpCost(packed []byte) float64 {
	opLen := math.Floor(float64(len(packed)+31) / 32)
	return (ov.perUserOpMultiplier * opLen) + ov.perUserOpFixed
}

// CalcPreVerificationGasWithBuffer returns CalcPreVerificationGas increased by the set PVG buffer factor.
func (ov *Overhead) CalcPreVerificationGasWithBuffer(op userop.UserOperationInterface) (*big.Int, error) {
	pvg, err := ov.CalcPreVerificationGas(op)
	if err != nil {
		return nil, err
	}
	return utils.AddBuffer(pvg, ov.pvgBufferFactor), nil
}

// NonZeroValueCall returns an expected gas cost of using the CALL opcode with non-zero value.
// See https://github.com/wolflo/evm-opcodes/blob/main/gas.md#aa-1-call.
func (ov *Overhead) NonZeroValueCall() *big.Int {
	return big.NewInt(
		int64(
			ov.callOpcode + ov.callWithValue + ov.warmStorageRead + ov.nonZeroValueStipend,
		),
	)
}

// CalcUnusedGasPenalty calculates the 10% penalty on unused verification gas (v0.7+).
// In v0.8+, the penalty only applies if unused gas exceeds the threshold (40,000).
func (ov *Overhead) CalcUnusedGasPenalty(unusedGas *big.Int) *big.Int {
	if unusedGas == nil || unusedGas.Sign() <= 0 {
		return big.NewInt(0)
	}

	if ov.gasPenaltyThreshold != nil && unusedGas.Cmp(ov.gasPenaltyThreshold) <= 0 {
		return big.NewInt(0)
	}

	if ov.unusedGasPenaltyPct <= 0 {
		return big.NewInt(0)
	}

	penalty := new(big.Int).Mul(unusedGas, big.NewInt(ov.unusedGasPenaltyPct))
	penalty.Div(penalty, big.NewInt(10000))
	return penalty
}
