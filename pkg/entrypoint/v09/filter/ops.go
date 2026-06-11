package filter

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v09"
)

var (
	packedUserOpPrimitives = []abi.ArgumentMarshaling{
		{Name: "sender", InternalType: "Sender", Type: "address"},
		{Name: "nonce", InternalType: "Nonce", Type: "uint256"},
		{Name: "initCode", InternalType: "InitCode", Type: "bytes"},
		{Name: "callData", InternalType: "CallData", Type: "bytes"},
		{Name: "accountGasLimits", InternalType: "AccountGasLimits", Type: "bytes32"},
		{Name: "preVerificationGas", InternalType: "PreVerificationGas", Type: "uint256"},
		{Name: "gasFees", InternalType: "GasFees", Type: "bytes32"},
		{Name: "paymasterAndData", InternalType: "PaymasterAndData", Type: "bytes"},
		{Name: "signature", InternalType: "Signature", Type: "bytes"},
	}

	packedUserOpArr, _ = abi.NewType("tuple[]", "ops", packedUserOpPrimitives)
)

type packedUserOp struct {
	Sender             common.Address `json:"sender"`
	Nonce              *big.Int       `json:"nonce"`
	InitCode           []byte         `json:"initCode"`
	CallData           []byte         `json:"callData"`
	AccountGasLimits   [32]byte       `json:"accountGasLimits"`
	PreVerificationGas *big.Int       `json:"preVerificationGas"`
	GasFees            [32]byte       `json:"gasFees"`
	PaymasterAndData   []byte         `json:"paymasterAndData"`
	Signature          []byte         `json:"signature"`
}

func unpackPackedOps(data []byte) ([]packedUserOp, error) {
	args := abi.Arguments{
		{Name: "ops", Type: packedUserOpArr},
		{Name: "beneficiary", Type: addressType},
	}
	unpacked, err := args.Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("unpackPackedOps: %s", err)
	}
	if len(unpacked) != 2 {
		return nil, fmt.Errorf("unpackPackedOps: invalid args length: expected 2, got %d", len(unpacked))
	}

	// The assertion target must match the struct type geth's abi package
	// builds via reflection, including the json tags it attaches.
	opsRaw, ok := unpacked[0].([]struct {
		Sender             common.Address `json:"sender"`
		Nonce              *big.Int       `json:"nonce"`
		InitCode           []byte         `json:"initCode"`
		CallData           []byte         `json:"callData"`
		AccountGasLimits   [32]byte       `json:"accountGasLimits"`
		PreVerificationGas *big.Int       `json:"preVerificationGas"`
		GasFees            [32]byte       `json:"gasFees"`
		PaymasterAndData   []byte         `json:"paymasterAndData"`
		Signature          []byte         `json:"signature"`
	})
	if !ok {
		return nil, fmt.Errorf("unpackPackedOps: cannot assert type")
	}

	var result []packedUserOp
	for _, op := range opsRaw {
		result = append(result, packedUserOp{
			Sender:             op.Sender,
			Nonce:              op.Nonce,
			InitCode:           op.InitCode,
			CallData:           op.CallData,
			AccountGasLimits:   op.AccountGasLimits,
			PreVerificationGas: op.PreVerificationGas,
			GasFees:            op.GasFees,
			PaymasterAndData:   op.PaymasterAndData,
			Signature:          op.Signature,
		})
	}
	return result, nil
}

var addressType, _ = abi.NewType("address", "", nil)

func packedOpToV09(p packedUserOp) *v09.UserOperation {
	op := &v09.UserOperation{
		Sender:                        p.Sender,
		Nonce:                         p.Nonce,
		CallData:                      p.CallData,
		PreVerificationGas:            p.PreVerificationGas,
		Signature:                     p.Signature,
		Factory:                       common.HexToAddress("0x"),
		Paymaster:                     common.HexToAddress("0x"),
		PaymasterVerificationGasLimit: big.NewInt(0),
		PaymasterPostOpGasLimit:       big.NewInt(0),
		PaymasterData:                 []byte{},
		VerificationGasLimit:          big.NewInt(0).And(big.NewInt(0).Rsh(big.NewInt(0).SetBytes(p.AccountGasLimits[:]), 128), maxUint128),
		CallGasLimit:                  big.NewInt(0).And(big.NewInt(0).SetBytes(p.AccountGasLimits[:]), maxUint128),
		// gasFees: maxPriorityFeePerGas high 128 bits, maxFeePerGas low
		// (UserOperationLib.unpackGasFees).
		MaxPriorityFeePerGas: big.NewInt(0).And(big.NewInt(0).Rsh(big.NewInt(0).SetBytes(p.GasFees[:]), 128), maxUint128),
		MaxFeePerGas:         big.NewInt(0).And(big.NewInt(0).SetBytes(p.GasFees[:]), maxUint128),
	}

	if len(p.InitCode) >= 20 {
		op.Factory = common.BytesToAddress(p.InitCode[:20])
		op.FactoryData = p.InitCode[20:]
	}

	if len(p.PaymasterAndData) >= 20 {
		op.Paymaster = common.BytesToAddress(p.PaymasterAndData[:20])
		if len(p.PaymasterAndData) >= 52 {
			packed := new(big.Int).SetBytes(p.PaymasterAndData[20:52])
			op.PaymasterVerificationGasLimit = big.NewInt(0).And(big.NewInt(0).Rsh(packed, 128), maxUint128)
			op.PaymasterPostOpGasLimit = big.NewInt(0).And(packed, maxUint128)
			op.PaymasterData = p.PaymasterAndData[52:]
		}
	}

	return op
}

var maxUint128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
