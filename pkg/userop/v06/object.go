package v06

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	addressType, _ = abi.NewType("address", "", nil)
	uint256Type, _ = abi.NewType("uint256", "", nil)
	bytes32Type, _ = abi.NewType("bytes32", "", nil)
)

type UserOperation struct {
	Sender               common.Address `json:"sender"               mapstructure:"sender"               validate:"required"`
	Nonce                *big.Int       `json:"nonce"                mapstructure:"nonce"                validate:"required"`
	InitCode             []byte         `json:"initCode"             mapstructure:"initCode"             validate:"required"`
	CallData             []byte         `json:"callData"             mapstructure:"callData"             validate:"required"`
	CallGasLimit         *big.Int       `json:"callGasLimit"         mapstructure:"callGasLimit"         validate:"required"`
	VerificationGasLimit *big.Int       `json:"verificationGasLimit" mapstructure:"verificationGasLimit" validate:"required"`
	PreVerificationGas   *big.Int       `json:"preVerificationGas"   mapstructure:"preVerificationGas"   validate:"required"`
	MaxFeePerGas         *big.Int       `json:"maxFeePerGas"         mapstructure:"maxFeePerGas"         validate:"required"`
	MaxPriorityFeePerGas *big.Int       `json:"maxPriorityFeePerGas" mapstructure:"maxPriorityFeePerGas" validate:"required"`
	PaymasterAndData     []byte         `json:"paymasterAndData"     mapstructure:"paymasterAndData"     validate:"required"`
	Signature            []byte         `json:"signature"            mapstructure:"signature"            validate:"required"`
}

func (op *UserOperation) GetPaymaster() common.Address {
	if len(op.PaymasterAndData) < common.AddressLength {
		return common.HexToAddress("0x")
	}

	return common.BytesToAddress(op.PaymasterAndData[:common.AddressLength])
}

func (op *UserOperation) GetFactory() common.Address {
	if len(op.InitCode) < common.AddressLength {
		return common.HexToAddress("0x")
	}

	return common.BytesToAddress(op.InitCode[:common.AddressLength])
}

func (op *UserOperation) GetFactoryData() []byte {
	if len(op.InitCode) < common.AddressLength {
		return []byte{}
	}

	return op.InitCode[common.AddressLength:]
}

func (op *UserOperation) GetMaxGasAvailable() *big.Int {
	mul := big.NewInt(1)
	paymaster := op.GetPaymaster()
	if paymaster != common.HexToAddress("0x") {
		mul = big.NewInt(3)
	}

	return big.NewInt(0).Add(
		big.NewInt(0).Mul(op.VerificationGasLimit, mul),
		big.NewInt(0).Add(op.PreVerificationGas, op.CallGasLimit),
	)
}

func (op *UserOperation) GetMaxPrefund() *big.Int {
	return big.NewInt(0).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
}

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

func (op *UserOperation) Pack() []byte {
	args := abi.Arguments{
		{Name: "UserOp", Type: UserOpType},
	}
	packed, _ := args.Pack(&struct {
		Sender               common.Address
		Nonce                *big.Int
		InitCode             []byte
		CallData             []byte
		CallGasLimit         *big.Int
		VerificationGasLimit *big.Int
		PreVerificationGas   *big.Int
		MaxFeePerGas         *big.Int
		MaxPriorityFeePerGas *big.Int
		PaymasterAndData     []byte
		Signature            []byte
	}{
		op.Sender,
		op.Nonce,
		op.InitCode,
		op.CallData,
		op.CallGasLimit,
		op.VerificationGasLimit,
		op.PreVerificationGas,
		op.MaxFeePerGas,
		op.MaxPriorityFeePerGas,
		op.PaymasterAndData,
		op.Signature,
	})

	enc := hexutil.Encode(packed)
	enc = "0x" + enc[66:]
	return (hexutil.MustDecode(enc))
}

func (op *UserOperation) PackForSignature() []byte {
	args := abi.Arguments{
		{Name: "sender", Type: addressType},
		{Name: "nonce", Type: uint256Type},
		{Name: "hashInitCode", Type: bytes32Type},
		{Name: "hashCallData", Type: bytes32Type},
		{Name: "callGasLimit", Type: uint256Type},
		{Name: "verificationGasLimit", Type: uint256Type},
		{Name: "preVerificationGas", Type: uint256Type},
		{Name: "maxFeePerGas", Type: uint256Type},
		{Name: "maxPriorityFeePerGas", Type: uint256Type},
		{Name: "hashPaymasterAndData", Type: bytes32Type},
	}
	packed, _ := args.Pack(
		op.Sender,
		op.Nonce,
		crypto.Keccak256Hash(op.InitCode),
		crypto.Keccak256Hash(op.CallData),
		op.CallGasLimit,
		op.VerificationGasLimit,
		op.PreVerificationGas,
		op.MaxFeePerGas,
		op.MaxPriorityFeePerGas,
		crypto.Keccak256Hash(op.PaymasterAndData),
	)

	return packed
}

func (op *UserOperation) GetUserOpHash(entryPoint common.Address, chainID *big.Int) common.Hash {
	return crypto.Keccak256Hash(
		crypto.Keccak256(op.PackForSignature()),
		common.LeftPadBytes(entryPoint.Bytes(), 32),
		common.LeftPadBytes(chainID.Bytes(), 32),
	)
}

func (op *UserOperation) MarshalJSON() ([]byte, error) {
	ic := "0x"
	if fa := op.GetFactory(); fa != common.HexToAddress("0x") {
		ic = fmt.Sprintf("%s%s", fa, common.Bytes2Hex(op.GetFactoryData()))
	}

	return json.Marshal(&struct {
		Sender               string `json:"sender"`
		Nonce                string `json:"nonce"`
		InitCode             string `json:"initCode"`
		CallData             string `json:"callData"`
		CallGasLimit         string `json:"callGasLimit"`
		VerificationGasLimit string `json:"verificationGasLimit"`
		PreVerificationGas   string `json:"preVerificationGas"`
		MaxFeePerGas         string `json:"maxFeePerGas"`
		MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
		PaymasterAndData     string `json:"paymasterAndData"`
		Signature            string `json:"signature"`
	}{
		Sender:               op.Sender.String(),
		Nonce:                hexutil.EncodeBig(op.Nonce),
		InitCode:             ic,
		CallData:             hexutil.Encode(op.CallData),
		CallGasLimit:         hexutil.EncodeBig(op.CallGasLimit),
		VerificationGasLimit: hexutil.EncodeBig(op.VerificationGasLimit),
		PreVerificationGas:   hexutil.EncodeBig(op.PreVerificationGas),
		MaxFeePerGas:         hexutil.EncodeBig(op.MaxFeePerGas),
		MaxPriorityFeePerGas: hexutil.EncodeBig(op.MaxPriorityFeePerGas),
		PaymasterAndData:     hexutil.Encode(op.PaymasterAndData),
		Signature:            hexutil.Encode(op.Signature),
	})
}

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
