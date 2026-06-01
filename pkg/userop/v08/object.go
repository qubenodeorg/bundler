// Package v08 provides the ERC-4337 v0.8 UserOperation struct with EIP-7702 support.
package v08

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	address, _ = abi.NewType("address", "", nil)
	uint256, _ = abi.NewType("uint256", "", nil)
	bytes32, _ = abi.NewType("bytes32", "", nil)
	maxUint128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

	// PackedUserOpPrimitives defines the ABI types for PackedUserOperation (on-chain format).
	PackedUserOpPrimitives = []abi.ArgumentMarshaling{
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

	PackedUserOpType, _ = abi.NewType("tuple", "PackedUserOperation", PackedUserOpPrimitives)
	PackedUserOpArr, _  = abi.NewType("tuple[]", "ops", PackedUserOpPrimitives)

	// UserOpPrimitives defines the ABI types for the expanded JSON-RPC format.
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
	}

	UserOpType, _ = abi.NewType("tuple", "op", UserOpPrimitives)
	UserOpArr, _  = abi.NewType("tuple[]", "ops", UserOpPrimitives)
)

// Authorization represents an EIP-7702 authorization tuple.
type Authorization struct {
	ChainID *big.Int       `json:"chainId"`
	Address common.Address `json:"address"`
	Nonce   *big.Int       `json:"nonce"`
	YParity uint8          `json:"yParity"`
	R       *big.Int       `json:"r"`
	S       *big.Int       `json:"s"`
}

// UserOperation represents an ERC-4337 v0.8 style transaction with EIP-7702 support.
type UserOperation struct {
	Sender                        common.Address `json:"sender"                        mapstructure:"sender"                        validate:"required"`
	Nonce                         *big.Int       `json:"nonce"                         mapstructure:"nonce"                         validate:"required"`
	Factory                       common.Address `json:"factory"                       mapstructure:"factory"                       validate:"required"`
	FactoryData                   []byte         `json:"factoryData"                   mapstructure:"factoryData"                   validate:"required"`
	CallData                      []byte         `json:"callData"                      mapstructure:"callData"                      validate:"required"`
	CallGasLimit                  *big.Int       `json:"callGasLimit"                  mapstructure:"callGasLimit"                  validate:"required"`
	VerificationGasLimit          *big.Int       `json:"verificationGasLimit"          mapstructure:"verificationGasLimit"          validate:"required"`
	PreVerificationGas            *big.Int       `json:"preVerificationGas"            mapstructure:"preVerificationGas"            validate:"required"`
	MaxFeePerGas                  *big.Int       `json:"maxFeePerGas"                  mapstructure:"maxFeePerGas"                  validate:"required"`
	MaxPriorityFeePerGas          *big.Int       `json:"maxPriorityFeePerGas"          mapstructure:"maxPriorityFeePerGas"          validate:"required"`
	Paymaster                     common.Address `json:"paymaster"                     mapstructure:"paymaster"                     validate:"required"`
	PaymasterVerificationGasLimit *big.Int       `json:"paymasterVerificationGasLimit" mapstructure:"paymasterVerificationGasLimit" validate:"required"`
	PaymasterPostOpGasLimit       *big.Int       `json:"paymasterPostOpGasLimit"       mapstructure:"paymasterPostOpGasLimit"       validate:"required"`
	PaymasterData                 []byte         `json:"paymasterData"                 mapstructure:"paymasterData"                 validate:"required"`
	EIP7702Auth                   *Authorization `json:"eip7702Auth"                   mapstructure:"eip7702Auth"`
	Signature                     []byte         `json:"signature"                     mapstructure:"signature"                     validate:"required"`
}

// GetSender returns the sender address.
func (op *UserOperation) GetSender() common.Address {
	return op.Sender
}

// GetMaxFeePerGas returns the max fee per gas.
func (op *UserOperation) GetMaxFeePerGas() *big.Int {
	return op.MaxFeePerGas
}

// GetPaymaster returns the paymaster address.
func (op *UserOperation) GetPaymaster() common.Address {
	return op.Paymaster
}

// GetFactory returns the factory address.
func (op *UserOperation) GetFactory() common.Address {
	return op.Factory
}

// GetFactoryData returns the factory data.
func (op *UserOperation) GetFactoryData() []byte {
	return op.FactoryData
}

// GetMaxGasAvailable returns the max amount of gas that can be consumed by this UserOperation.
func (op *UserOperation) GetMaxGasAvailable() *big.Int {
	return big.NewInt(0).Add(
		big.NewInt(0).Add(op.VerificationGasLimit, op.CallGasLimit),
		op.PreVerificationGas,
	)
}

// GetMaxPrefund returns the max amount of wei required to pay for gas fees.
func (op *UserOperation) GetMaxPrefund() *big.Int {
	return big.NewInt(0).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
}

// GetDynamicGasPrice returns the effective gas price given a basefee.
func (op *UserOperation) GetDynamicGasPrice(basefee *big.Int) *big.Int {
	bf := basefee
	if bf == nil {
		bf = big.NewInt(0)
	}
	gp := big.NewInt(0).Add(bf, op.MaxPriorityFeePerGas)
	if gp.Cmp(op.MaxFeePerGas) == 1 {
		return op.MaxFeePerGas
	}
	return gp
}

func (op *UserOperation) AccountGasLimits() [32]byte {
	vg := new(big.Int).And(op.VerificationGasLimit, maxUint128)
	cg := new(big.Int).And(op.CallGasLimit, maxUint128)
	packed := new(big.Int).Lsh(vg, 128)
	packed.Or(packed, cg)
	return common.BytesToHash(common.LeftPadBytes(packed.Bytes(), 32))
}

func (op *UserOperation) GasFees() [32]byte {
	mf := new(big.Int).And(op.MaxFeePerGas, maxUint128)
	mp := new(big.Int).And(op.MaxPriorityFeePerGas, maxUint128)
	packed := new(big.Int).Lsh(mf, 128)
	packed.Or(packed, mp)
	return common.BytesToHash(common.LeftPadBytes(packed.Bytes(), 32))
}

func (op *UserOperation) PackPaymasterAndData() []byte {
	if op.Paymaster == common.HexToAddress("0x") {
		return []byte{}
	}
	data := op.Paymaster.Bytes()
	pvg := new(big.Int).And(op.PaymasterVerificationGasLimit, maxUint128)
	ppg := new(big.Int).And(op.PaymasterPostOpGasLimit, maxUint128)
	packed := new(big.Int).Lsh(pvg, 128)
	packed.Or(packed, ppg)
	data = append(data, common.LeftPadBytes(packed.Bytes(), 32)...)
	data = append(data, op.PaymasterData...)
	return data
}

func (op *UserOperation) PackInitCode() []byte {
	if op.Factory == common.HexToAddress("0x") {
		return []byte{}
	}
	data := op.Factory.Bytes()
	data = append(data, op.FactoryData...)
	return data
}

// Pack returns the PackedUserOperation encoding for on-chain use.
func (op *UserOperation) Pack() []byte {
	args := abi.Arguments{
		{Name: "UserOp", Type: PackedUserOpType},
	}
	packed, _ := args.Pack(&struct {
		Sender             common.Address
		Nonce              *big.Int
		InitCode           []byte
		CallData           []byte
		AccountGasLimits   [32]byte
		PreVerificationGas *big.Int
		GasFees            [32]byte
		PaymasterAndData   []byte
		Signature          []byte
	}{
		op.Sender,
		op.Nonce,
		op.PackInitCode(),
		op.CallData,
		op.AccountGasLimits(),
		op.PreVerificationGas,
		op.GasFees(),
		op.PackPaymasterAndData(),
		op.Signature,
	})

	if len(packed) < 32 {
		return []byte{}
	}
	return packed[32:]
}

// PackForSignature returns a minimal message for userOpHash computation.
func (op *UserOperation) PackForSignature() []byte {
	args := abi.Arguments{
		{Name: "sender", Type: address},
		{Name: "nonce", Type: uint256},
		{Name: "hashInitCode", Type: bytes32},
		{Name: "hashCallData", Type: bytes32},
		{Name: "accountGasLimits", Type: bytes32},
		{Name: "preVerificationGas", Type: uint256},
		{Name: "gasFees", Type: bytes32},
		{Name: "hashPaymasterAndData", Type: bytes32},
	}
	packed, _ := args.Pack(
		op.Sender,
		op.Nonce,
		crypto.Keccak256Hash(op.PackInitCode()),
		crypto.Keccak256Hash(op.CallData),
		op.AccountGasLimits(),
		op.PreVerificationGas,
		op.GasFees(),
		crypto.Keccak256Hash(op.PackPaymasterAndData()),
	)
	return packed
}

// GetUserOpHash returns the hash of the userOp + entryPoint address + chainID.
func (op *UserOperation) GetUserOpHash(entryPoint common.Address, chainID *big.Int) common.Hash {
	return crypto.Keccak256Hash(
		crypto.Keccak256(op.PackForSignature()),
		common.LeftPadBytes(entryPoint.Bytes(), 32),
		common.LeftPadBytes(chainID.Bytes(), 32),
	)
}

// MarshalJSON returns a JSON encoding of the UserOperation.
func (op *UserOperation) MarshalJSON() ([]byte, error) {
	type authJSON struct {
		ChainID string `json:"chainId"`
		Address string `json:"address"`
		Nonce   string `json:"nonce"`
		YParity uint8  `json:"yParity"`
		R       string `json:"r"`
		S       string `json:"s"`
	}

	var auth *authJSON
	if op.EIP7702Auth != nil {
		auth = &authJSON{
			ChainID: hexutil.EncodeBig(op.EIP7702Auth.ChainID),
			Address: op.EIP7702Auth.Address.String(),
			Nonce:   hexutil.EncodeBig(op.EIP7702Auth.Nonce),
			YParity: op.EIP7702Auth.YParity,
			R:       hexutil.EncodeBig(op.EIP7702Auth.R),
			S:       hexutil.EncodeBig(op.EIP7702Auth.S),
		}
	}

	return json.Marshal(&struct {
		Sender                        string      `json:"sender"`
		Nonce                         string      `json:"nonce"`
		Factory                       string      `json:"factory"`
		FactoryData                   string      `json:"factoryData"`
		CallData                      string      `json:"callData"`
		CallGasLimit                  string      `json:"callGasLimit"`
		VerificationGasLimit          string      `json:"verificationGasLimit"`
		PreVerificationGas            string      `json:"preVerificationGas"`
		MaxFeePerGas                  string      `json:"maxFeePerGas"`
		MaxPriorityFeePerGas          string      `json:"maxPriorityFeePerGas"`
		Paymaster                     string      `json:"paymaster"`
		PaymasterVerificationGasLimit string      `json:"paymasterVerificationGasLimit"`
		PaymasterPostOpGasLimit       string      `json:"paymasterPostOpGasLimit"`
		PaymasterData                 string      `json:"paymasterData"`
		EIP7702Auth                   *authJSON   `json:"eip7702Auth,omitempty"`
		Signature                     string      `json:"signature"`
	}{
		Sender:                        op.Sender.String(),
		Nonce:                         hexutil.EncodeBig(op.Nonce),
		Factory:                       op.Factory.String(),
		FactoryData:                   hexutil.Encode(op.FactoryData),
		CallData:                      hexutil.Encode(op.CallData),
		CallGasLimit:                  hexutil.EncodeBig(op.CallGasLimit),
		VerificationGasLimit:          hexutil.EncodeBig(op.VerificationGasLimit),
		PreVerificationGas:            hexutil.EncodeBig(op.PreVerificationGas),
		MaxFeePerGas:                  hexutil.EncodeBig(op.MaxFeePerGas),
		MaxPriorityFeePerGas:          hexutil.EncodeBig(op.MaxPriorityFeePerGas),
		Paymaster:                     op.Paymaster.String(),
		PaymasterVerificationGasLimit: hexutil.EncodeBig(op.PaymasterVerificationGasLimit),
		PaymasterPostOpGasLimit:       hexutil.EncodeBig(op.PaymasterPostOpGasLimit),
		PaymasterData:                 hexutil.Encode(op.PaymasterData),
		EIP7702Auth:                   auth,
		Signature:                     hexutil.Encode(op.Signature),
	})
}

// ToMap returns the current UserOp struct as a map type.
func (op *UserOperation) ToMap() (map[string]any, error) {
	data, err := op.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var opData map[string]any
	if err := json.Unmarshal(data, &opData); err != nil {
		return nil, err
	}
	return opData, nil
}
