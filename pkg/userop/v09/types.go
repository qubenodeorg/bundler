package v09

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
)

var (
	UserOpPrimitives = []abi.ArgumentMarshaling{
		{Name: "sender", InternalType: "Sender", Type: "address"},
		{Name: "nonce", InternalType: "Nonce", Type: "uint256"},
		{Name: "factory", InternalType: "Factory", Type: "address"},
		{Name: "factoryData", InternalType: "FactoryData", Type: "bytes"},
		{Name: "callData", InternalType: "CallData", Type: "bytes"},
		{Name: "callGasLimit", InternalType: "CallGasLimit", Type: "uint256"},
		{Name: "verificationGasLimit", InternalType: "VerificationGasLimit", Type: "uint256"},
		{Name: "preVerificationGas", InternalType: "PreVerificationGas", Type: "uint256"},
		{Name: "maxFeePerGas", InternalType: "MaxFeePerGas", Type: "uint256"},
		{Name: "maxPriorityFeePerGas", InternalType: "MaxPriorityFeePerGas", Type: "uint256"},
		{Name: "paymaster", InternalType: "Paymaster", Type: "address"},
		{Name: "paymasterVerificationGasLimit", InternalType: "PaymasterVerificationGasLimit", Type: "uint256"},
		{Name: "paymasterPostOpGasLimit", InternalType: "PaymasterPostOpGasLimit", Type: "uint256"},
		{Name: "paymasterData", InternalType: "PaymasterData", Type: "bytes"},
		{Name: "signature", InternalType: "Signature", Type: "bytes"},
		{Name: "paymasterSignature", InternalType: "PaymasterSignature", Type: "bytes"},
		{Name: "eip7702Auth", InternalType: "Eip7702Auth", Type: "bytes"},
	}

	UserOpType, _ = abi.NewType("tuple", "op", UserOpPrimitives)

	UserOpArr, _ = abi.NewType("tuple[]", "ops", UserOpPrimitives)
)
