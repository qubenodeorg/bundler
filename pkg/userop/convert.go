package userop

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

var maxUint128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

// ConvertToV06Packed converts any versioned UserOperation to a packed v0.6 *UserOperation
// using the on-chain PackedUserOperation layout for InitCode and PaymasterAndData.
//
//   - factory + factoryData -> InitCode (20 bytes address + factoryData)
//   - paymaster + paymasterVerificationGasLimit(16) + paymasterPostOpGasLimit(16) + paymasterData -> PaymasterAndData
//
// Unlike gas.ConvertToV06 which packs dummy zero paymaster gas limits, this preserves the
// real paymasterVerificationGasLimit and paymasterPostOpGasLimit so the canonical packed
// op can be safely used downstream (mempool, batch, on-chain transaction submission).
//
// For v0.6 inputs, the op is returned as-is.
func ConvertToV06Packed(op UserOperationInterface) (*UserOperation, error) {
	if v06op, ok := op.(*UserOperation); ok {
		return v06op, nil
	}

	initCode := packFactory(op.GetFactory(), op.GetFactoryData())
	paymasterAndData := packPaymaster(
		op.GetPaymaster(),
		op.GetPaymasterVerificationGasLimit(),
		op.GetPaymasterPostOpGasLimit(),
		op.GetPaymasterData(),
	)

	return &UserOperation{
		Sender:               op.GetSender(),
		Nonce:                op.GetNonce(),
		InitCode:             initCode,
		CallData:             op.GetCallData(),
		CallGasLimit:         op.GetCallGasLimit(),
		VerificationGasLimit: op.GetVerificationGasLimit(),
		PreVerificationGas:   op.GetPreVerificationGas(),
		MaxFeePerGas:         op.GetMaxFeePerGas(),
		MaxPriorityFeePerGas: op.GetMaxPriorityFeePerGas(),
		PaymasterAndData:     paymasterAndData,
		Signature:            op.GetSignature(),
	}, nil
}

// packFactory packs factory + factoryData into v0.6-style InitCode bytes.
// Returns an empty slice when the factory address is the zero address.
func packFactory(factory common.Address, factoryData []byte) []byte {
	if factory == (common.Address{}) {
		return []byte{}
	}
	out := make([]byte, 0, common.AddressLength+len(factoryData))
	out = append(out, factory.Bytes()...)
	out = append(out, factoryData...)
	return out
}

// packPaymaster packs the v0.7+ paymaster fields into the on-chain PaymasterAndData layout:
// address(20) || paymasterVerificationGasLimit(16) || paymasterPostOpGasLimit(16) || paymasterData.
// Returns an empty slice when the paymaster address is the zero address.
func packPaymaster(paymaster common.Address, pvg *big.Int, ppg *big.Int, paymasterData []byte) []byte {
	if paymaster == (common.Address{}) {
		return []byte{}
	}
	if pvg == nil {
		pvg = big.NewInt(0)
	}
	if ppg == nil {
		ppg = big.NewInt(0)
	}

	pvgMasked := new(big.Int).And(pvg, maxUint128)
	ppgMasked := new(big.Int).And(ppg, maxUint128)
	packed := new(big.Int).Lsh(pvgMasked, 128)
	packed.Or(packed, ppgMasked)

	out := make([]byte, 0, common.AddressLength+32+len(paymasterData))
	out = append(out, paymaster.Bytes()...)
	out = append(out, common.LeftPadBytes(packed.Bytes(), 32)...)
	out = append(out, paymasterData...)
	return out
}
