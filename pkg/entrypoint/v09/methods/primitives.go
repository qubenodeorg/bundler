package methods

import "github.com/ethereum/go-ethereum/accounts/abi"

var (
	address     = func() abi.Type { t, _ := abi.NewType("address", "", nil); return t }()
	uint256     = func() abi.Type { t, _ := abi.NewType("uint256", "", nil); return t }()
	bytes       = func() abi.Type { t, _ := abi.NewType("bytes", "", nil); return t }()
	packedUserOp = func() abi.Type {
		t, _ := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
			{Name: "sender", Type: "address"},
			{Name: "nonce", Type: "uint256"},
			{Name: "initCode", Type: "bytes"},
			{Name: "callData", Type: "bytes"},
			{Name: "accountGasLimits", Type: "bytes32"},
			{Name: "preVerificationGas", Type: "uint256"},
			{Name: "gasFees", Type: "bytes32"},
			{Name: "paymasterAndData", Type: "bytes"},
			{Name: "signature", Type: "bytes"},
		})
		return t
	}()
	packedUserOpArr = func() abi.Type {
		t, _ := abi.NewType("tuple[]", "", []abi.ArgumentMarshaling{
			{Name: "sender", Type: "address"},
			{Name: "nonce", Type: "uint256"},
			{Name: "initCode", Type: "bytes"},
			{Name: "callData", Type: "bytes"},
			{Name: "accountGasLimits", Type: "bytes32"},
			{Name: "preVerificationGas", Type: "uint256"},
			{Name: "gasFees", Type: "bytes32"},
			{Name: "paymasterAndData", Type: "bytes"},
			{Name: "signature", Type: "bytes"},
		})
		return t
	}()
)
