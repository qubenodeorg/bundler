// Package v09 provides the ERC-4337 v0.9 UserOperation struct with paymaster signature support.
package v09

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

// EIP7702MarkerFactory is the left-aligned EIP-7702 initCode marker: an op
// whose initCode starts with bytes 0x7702 has a delegated EOA sender instead
// of a factory (Eip7702Support.INITCODE_EIP7702_MARKER).
var EIP7702MarkerFactory = common.HexToAddress("0x7702000000000000000000000000000000000000")

// Authorization represents an EIP-7702 authorization tuple.
type Authorization struct {
	ChainID *big.Int       `json:"chainId" mapstructure:"chainId"`
	Address common.Address `json:"address" mapstructure:"address"`
	Nonce   *big.Int       `json:"nonce"   mapstructure:"nonce"`
	YParity uint8          `json:"yParity" mapstructure:"yParity"`
	R       *big.Int       `json:"r"       mapstructure:"r"`
	S       *big.Int       `json:"s"       mapstructure:"s"`
}

// UserOperation represents an ERC-4337 v0.9 style transaction with paymaster signature support.
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
	PaymasterSignature            []byte         `json:"paymasterSignature"            mapstructure:"paymasterSignature"`
	Signature                     []byte         `json:"signature"                     mapstructure:"signature"                     validate:"required"`
}

// GetSender returns the sender address.
func (op *UserOperation) GetSender() common.Address {
	return op.Sender
}

// GetNonce returns the nonce.
func (op *UserOperation) GetNonce() *big.Int {
	return op.Nonce
}

// GetCallData returns the callData.
func (op *UserOperation) GetCallData() []byte {
	return op.CallData
}

// GetCallGasLimit returns the callGasLimit.
func (op *UserOperation) GetCallGasLimit() *big.Int {
	return op.CallGasLimit
}

// GetVerificationGasLimit returns the verificationGasLimit.
func (op *UserOperation) GetVerificationGasLimit() *big.Int {
	return op.VerificationGasLimit
}

// GetPreVerificationGas returns the preVerificationGas.
func (op *UserOperation) GetPreVerificationGas() *big.Int {
	return op.PreVerificationGas
}

// GetMaxFeePerGas returns the max fee per gas.
func (op *UserOperation) GetMaxFeePerGas() *big.Int {
	return op.MaxFeePerGas
}

// GetMaxPriorityFeePerGas returns the max priority fee per gas.
func (op *UserOperation) GetMaxPriorityFeePerGas() *big.Int {
	return op.MaxPriorityFeePerGas
}

// GetSignature returns the signature.
func (op *UserOperation) GetSignature() []byte {
	return op.Signature
}

// GetEIP7702Auth returns the EIP-7702 authorization.
func (op *UserOperation) GetEIP7702Auth() interface{} {
	if op.EIP7702Auth == nil {
		return nil
	}
	return op.EIP7702Auth
}

// GetPaymasterVerificationGasLimit returns the paymaster verification gas limit.
func (op *UserOperation) GetPaymasterVerificationGasLimit() *big.Int {
	return op.PaymasterVerificationGasLimit
}

// GetPaymasterPostOpGasLimit returns the paymaster post op gas limit.
func (op *UserOperation) GetPaymasterPostOpGasLimit() *big.Int {
	return op.PaymasterPostOpGasLimit
}

// GetPaymasterData returns the paymaster data.
func (op *UserOperation) GetPaymasterData() []byte {
	return op.PaymasterData
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
// v0.7 removes the 3x verificationGasLimit multiplier for paymasters and adds
// the paymaster gas limits when a paymaster is set.
func (op *UserOperation) GetMaxGasAvailable() *big.Int {
	sum := big.NewInt(0).Add(
		big.NewInt(0).Add(op.VerificationGasLimit, op.CallGasLimit),
		op.PreVerificationGas,
	)
	if op.Paymaster != (common.Address{}) {
		if op.PaymasterVerificationGasLimit != nil {
			sum.Add(sum, op.PaymasterVerificationGasLimit)
		}
		if op.PaymasterPostOpGasLimit != nil {
			sum.Add(sum, op.PaymasterPostOpGasLimit)
		}
	}
	return sum
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

// GasFees packs maxPriorityFeePerGas (high 128 bits) and maxFeePerGas
// (low 128 bits) per UserOperationLib.unpackGasFees.
func (op *UserOperation) GasFees() [32]byte {
	mf := new(big.Int).And(op.MaxFeePerGas, maxUint128)
	mp := new(big.Int).And(op.MaxPriorityFeePerGas, maxUint128)
	packed := new(big.Int).Lsh(mp, 128)
	packed.Or(packed, mf)
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
// Note: paymasterSignature is excluded from the userOpHash per v0.9 spec.
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
		op.hashInitCode(),
		crypto.Keccak256Hash(op.CallData),
		op.AccountGasLimits(),
		op.PreVerificationGas,
		op.GasFees(),
		crypto.Keccak256Hash(op.PackPaymasterAndData()),
	)
	return packed
}

// hashInitCode mirrors Eip7702Support._getEip7702InitCodeHashOverride: for a
// delegated EOA sender (marker factory), the EntryPoint hashes the delegate
// address plus any init calldata instead of the literal initCode.
func (op *UserOperation) hashInitCode() common.Hash {
	if op.Factory == EIP7702MarkerFactory && op.EIP7702Auth != nil {
		return crypto.Keccak256Hash(append(op.EIP7702Auth.Address.Bytes(), op.FactoryData...))
	}
	return crypto.Keccak256Hash(op.PackInitCode())
}

var (
	// PackedUserOpTypeHash is UserOperationLib.PACKED_USEROP_TYPEHASH.
	PackedUserOpTypeHash = crypto.Keccak256Hash([]byte(
		"PackedUserOperation(address sender,uint256 nonce,bytes initCode,bytes callData,bytes32 accountGasLimits,uint256 preVerificationGas,bytes32 gasFees,bytes paymasterAndData)",
	))

	eip712DomainTypeHash = crypto.Keccak256Hash([]byte(
		"EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)",
	))
	domainNameHash    = crypto.Keccak256Hash([]byte("ERC4337"))
	domainVersionHash = crypto.Keccak256Hash([]byte("1"))
)

// DomainSeparator computes the v0.9 EntryPoint's EIP-712 domain separator
// (name "ERC4337", version "1", verifyingContract = the EntryPoint).
func DomainSeparator(entryPoint common.Address, chainID *big.Int) [32]byte {
	enc := make([]byte, 0, 160)
	enc = append(enc, eip712DomainTypeHash.Bytes()...)
	enc = append(enc, domainNameHash.Bytes()...)
	enc = append(enc, domainVersionHash.Bytes()...)
	enc = append(enc, common.LeftPadBytes(chainID.Bytes(), 32)...)
	enc = append(enc, common.LeftPadBytes(entryPoint.Bytes(), 32)...)
	var ds [32]byte
	copy(ds[:], crypto.Keccak256(enc))
	return ds
}

// GetUserOpHash returns the hash the v0.9 EntryPoint signs over: the ERC-712
// typed-data hash of the packed operation (EntryPoint.getUserOpHash). The
// legacy keccak(hash || entryPoint || chainID) scheme was replaced in v0.8.
func (op *UserOperation) GetUserOpHash(entryPoint common.Address, chainID *big.Int) common.Hash {
	return op.GetUserOpHashEIP712(DomainSeparator(entryPoint, chainID), PackedUserOpTypeHash)
}

// GetUserOpHashEIP712 computes the ERC-712 typed data hash.
func (op *UserOperation) GetUserOpHashEIP712(domainSeparator [32]byte, typeHash [32]byte) common.Hash {
	structHash := crypto.Keccak256(append(typeHash[:], op.PackForSignature()...))
	buf := make([]byte, 2+32+len(structHash))
	copy(buf[0:2], "\x19\x01")
	copy(buf[2:34], domainSeparator[:])
	copy(buf[34:], structHash)
	return crypto.Keccak256Hash(buf)
}

// GetPaymasterSignature returns the paymaster signature for v0.9.
func (op *UserOperation) GetPaymasterSignature() []byte {
	return op.PaymasterSignature
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

	// encodeBigOrZero tolerates nil big.Ints: partially-populated auth
	// tuples can reach marshalling before validation rejects them.
	encodeBigOrZero := func(b *big.Int) string {
		if b == nil {
			return "0x0"
		}
		return hexutil.EncodeBig(b)
	}

	var auth *authJSON
	if op.EIP7702Auth != nil {
		auth = &authJSON{
			ChainID: encodeBigOrZero(op.EIP7702Auth.ChainID),
			Address: op.EIP7702Auth.Address.String(),
			Nonce:   encodeBigOrZero(op.EIP7702Auth.Nonce),
			YParity: op.EIP7702Auth.YParity,
			R:       encodeBigOrZero(op.EIP7702Auth.R),
			S:       encodeBigOrZero(op.EIP7702Auth.S),
		}
	}

	// The factory and paymaster field groups are optional per the ERC-4337 RPC
	// spec and are omitted when their address is unset, so the encoded op
	// round-trips to the same userOpHash.
	out := &struct {
		Sender                        string    `json:"sender"`
		Nonce                         string    `json:"nonce"`
		Factory                       string    `json:"factory,omitempty"`
		FactoryData                   string    `json:"factoryData,omitempty"`
		CallData                      string    `json:"callData"`
		CallGasLimit                  string    `json:"callGasLimit"`
		VerificationGasLimit          string    `json:"verificationGasLimit"`
		PreVerificationGas            string    `json:"preVerificationGas"`
		MaxFeePerGas                  string    `json:"maxFeePerGas"`
		MaxPriorityFeePerGas          string    `json:"maxPriorityFeePerGas"`
		Paymaster                     string    `json:"paymaster,omitempty"`
		PaymasterVerificationGasLimit string    `json:"paymasterVerificationGasLimit,omitempty"`
		PaymasterPostOpGasLimit       string    `json:"paymasterPostOpGasLimit,omitempty"`
		PaymasterData                 string    `json:"paymasterData,omitempty"`
		EIP7702Auth                   *authJSON `json:"eip7702Auth,omitempty"`
		PaymasterSignature            string    `json:"paymasterSignature,omitempty"`
		Signature                     string    `json:"signature"`
	}{
		Sender:               op.Sender.String(),
		Nonce:                hexutil.EncodeBig(op.Nonce),
		CallData:             hexutil.Encode(op.CallData),
		CallGasLimit:         hexutil.EncodeBig(op.CallGasLimit),
		VerificationGasLimit: hexutil.EncodeBig(op.VerificationGasLimit),
		PreVerificationGas:   hexutil.EncodeBig(op.PreVerificationGas),
		MaxFeePerGas:         hexutil.EncodeBig(op.MaxFeePerGas),
		MaxPriorityFeePerGas: hexutil.EncodeBig(op.MaxPriorityFeePerGas),
		EIP7702Auth:          auth,
		Signature:            hexutil.Encode(op.Signature),
	}
	if op.Factory != (common.Address{}) {
		out.Factory = op.Factory.String()
		out.FactoryData = hexutil.Encode(op.FactoryData)
	}
	if op.Paymaster != (common.Address{}) {
		out.Paymaster = op.Paymaster.String()
		out.PaymasterVerificationGasLimit = hexutil.EncodeBig(op.PaymasterVerificationGasLimit)
		out.PaymasterPostOpGasLimit = hexutil.EncodeBig(op.PaymasterPostOpGasLimit)
		out.PaymasterData = hexutil.Encode(op.PaymasterData)
	}
	if len(op.PaymasterSignature) > 0 {
		out.PaymasterSignature = hexutil.Encode(op.PaymasterSignature)
	}
	return json.Marshal(out)
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
