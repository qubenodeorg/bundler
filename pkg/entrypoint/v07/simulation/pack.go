package simulation

import (
	"math/big"

	v07 "github.com/zarbanio/bundler/pkg/entrypoint/v07"
	"github.com/zarbanio/bundler/pkg/userop"
)

var maxUint128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

// toPackedOp converts a UserOperation to a v0.7 PackedUserOperation for on-chain use.
// The v0.7 on-chain format uses the same packed encoding as v0.6 (initCode = factory+data,
// paymasterAndData = paymaster+gaslimits+data).
func toPackedOp(op *userop.UserOperation) (*v07.PackedUserOperation, error) {
	accountGasLimits := packAccountGasLimits(op.VerificationGasLimit, op.CallGasLimit)
	gasFees := packGasFees(op.MaxFeePerGas, op.MaxPriorityFeePerGas)

	return &v07.PackedUserOperation{
		Sender:             op.Sender,
		Nonce:              op.Nonce,
		InitCode:           op.InitCode,
		CallData:           op.CallData,
		AccountGasLimits:   accountGasLimits,
		PreVerificationGas: op.PreVerificationGas,
		GasFees:            gasFees,
		PaymasterAndData:   op.PaymasterAndData,
		Signature:          op.Signature,
	}, nil
}

func packAccountGasLimits(verificationGasLimit, callGasLimit *big.Int) [32]byte {
	vg := new(big.Int).And(verificationGasLimit, maxUint128)
	cg := new(big.Int).And(callGasLimit, maxUint128)
	packed := new(big.Int).Lsh(vg, 128)
	packed.Or(packed, cg)
	b := make([]byte, 32)
	packed.FillBytes(b)
	var arr [32]byte
	copy(arr[:], b)
	return arr
}

func packGasFees(maxFeePerGas, maxPriorityFeePerGas *big.Int) [32]byte {
	mf := new(big.Int).And(maxFeePerGas, maxUint128)
	mp := new(big.Int).And(maxPriorityFeePerGas, maxUint128)
	packed := new(big.Int).Lsh(mf, 128)
	packed.Or(packed, mp)
	b := make([]byte, 32)
	packed.FillBytes(b)
	var arr [32]byte
	copy(arr[:], b)
	return arr
}
