package userop

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// ErrPaymasterGasOutOfRange indicates a paymaster gas limit value cannot be encoded as
// uint128 (either negative or exceeds 2^128 - 1).
var ErrPaymasterGasOutOfRange = errors.New("paymaster gas limit out of range for uint128")

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
// For v0.6 inputs, the op is returned as-is. A nil input returns ErrBadUserOperationData.
func ConvertToV06Packed(op UserOperationInterface) (*UserOperation, error) {
	if op == nil {
		return nil, ErrBadUserOperationData
	}
	if v06op, ok := op.(*UserOperation); ok {
		return v06op, nil
	}

	initCode := packFactory(op.GetFactory(), op.GetFactoryData())
	paymasterAndData, err := packPaymaster(
		op.GetPaymaster(),
		op.GetPaymasterVerificationGasLimit(),
		op.GetPaymasterPostOpGasLimit(),
		op.GetPaymasterData(),
	)
	if err != nil {
		return nil, err
	}

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
		EIP7702Auth:          normalizeAuth(op.GetEIP7702Auth()),
	}, nil
}

// normalizeAuth converts a version-specific Authorization value into the base
// *Authorization. The versioned packages cannot be imported here (they are
// imported by this package), so the conversion goes through JSON, which all
// Authorization shapes share.
func normalizeAuth(a interface{}) *Authorization {
	if a == nil {
		return nil
	}
	if v, ok := a.(*Authorization); ok {
		return v
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil
	}
	out := &Authorization{}
	if err := json.Unmarshal(b, out); err != nil {
		return nil
	}
	return out
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
// Returns an empty slice when the paymaster address is the zero address. Returns an error if
// either gas limit is negative or does not fit in uint128 — silently masking would change
// the meaning of the value and produce a packed op that no longer matches the input.
func packPaymaster(paymaster common.Address, pvg *big.Int, ppg *big.Int, paymasterData []byte) ([]byte, error) {
	if paymaster == (common.Address{}) {
		return []byte{}, nil
	}
	if pvg == nil {
		pvg = big.NewInt(0)
	}
	if ppg == nil {
		ppg = big.NewInt(0)
	}
	if pvg.Sign() < 0 || ppg.Sign() < 0 || pvg.BitLen() > 128 || ppg.BitLen() > 128 {
		return nil, ErrPaymasterGasOutOfRange
	}

	packed := new(big.Int).Lsh(new(big.Int).Set(pvg), 128)
	packed.Or(packed, ppg)

	out := make([]byte, 0, common.AddressLength+32+len(paymasterData))
	out = append(out, paymaster.Bytes()...)
	out = append(out, common.LeftPadBytes(packed.Bytes(), 32)...)
	out = append(out, paymasterData...)
	return out, nil
}
