package v06

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
)

var (
	address, _ = abi.NewType("address", "", nil)
	uint256, _ = abi.NewType("uint256", "", nil)
	bytes32, _ = abi.NewType("bytes32", "", nil)

	UserOpPrimitives = []abi.ArgumentMarshaling{
		{Name: "sender", InternalType: "Sender", Type: "address"},
		{Name: "nonce", InternalType: "Nonce", Type: "uint256"},
		{Name: "initCode", InternalType: "InitCode", Type: "bytes"},
		{Name: "callData", InternalType: "CallData", Type: "bytes"},
		{Name: "callGasLimit", InternalType: "CallGasLimit", Type: "uint256"},
		{Name: "verificationGasLimit", InternalType: "VerificationGasLimit", Type: "uint256"},
		{Name: "preVerificationGas", InternalType: "PreVerificationGas", Type: "uint256"},
		{Name: "maxFeePerGas", InternalType: "MaxFeePerGas", Type: "uint256"},
		{Name: "maxPriorityFeePerGas", InternalType: "MaxPriorityFeePerGas", Type: "uint256"},
		{Name: "paymasterAndData", InternalType: "PaymasterAndData", Type: "bytes"},
		{Name: "signature", InternalType: "Signature", Type: "bytes"},
	}

	UserOpType, _ = abi.NewType("tuple", "op", UserOpPrimitives)

	UserOpArr, _ = abi.NewType("tuple[]", "ops", UserOpPrimitives)
)
