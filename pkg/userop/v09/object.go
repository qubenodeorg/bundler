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
	addressType, _ = abi.NewType("address", "", nil)
	uint256Type, _ = abi.NewType("uint256", "", nil)
	bytes32Type, _ = abi.NewType("bytes32", "", nil)
)

type UserOperation struct {
	Sender                       common.Address `json:"sender"                       mapstructure:"sender"                       validate:"required"`
	Nonce                        *big.Int       `json:"nonce"                        mapstructure:"nonce"                        validate:"required"`
	Factory                      common.Address `json:"factory"                      mapstructure:"factory"`
	FactoryData                  []byte         `json:"factoryData"                  mapstructure:"factoryData"`
	CallData                     []byte         `json:"callData"                     mapstructure:"callData"                     validate:"required"`
	CallGasLimit                 *big.Int       `json:"callGasLimit"                 mapstructure:"callGasLimit"                 validate:"required"`
	VerificationGasLimit         *big.Int       `json:"verificationGasLimit"         mapstructure:"verificationGasLimit"         validate:"required"`
	PreVerificationGas           *big.Int       `json:"preVerificationGas"           mapstructure:"preVerificationGas"           validate:"required"`
	MaxFeePerGas                 *big.Int       `json:"maxFeePerGas"                 mapstructure:"maxFeePerGas"                 validate:"required"`
	MaxPriorityFeePerGas         *big.Int       `json:"maxPriorityFeePerGas"         mapstructure:"maxPriorityFeePerGas"         validate:"required"`
	Paymaster                    common.Address `json:"paymaster"                    mapstructure:"paymaster"`
	PaymasterVerificationGasLimit *big.Int      `json:"paymasterVerificationGasLimit" mapstructure:"paymasterVerificationGasLimit"`
	PaymasterPostOpGasLimit      *big.Int       `json:"paymasterPostOpGasLimit"      mapstructure:"paymasterPostOpGasLimit"`
	PaymasterData                []byte         `json:"paymasterData"                mapstructure:"paymasterData"`
	Signature                    []byte         `json:"signature"                    mapstructure:"signature"                    validate:"required"`
	PaymasterSignature           []byte         `json:"paymasterSignature,omitempty" mapstructure:"paymasterSignature"`
	EIP7702Auth                  []byte         `json:"eip7702Auth,omitempty"        mapstructure:"eip7702Auth"`
}

func (op *UserOperation) GetPaymaster() common.Address {
	return op.Paymaster
}

func (op *UserOperation) GetFactory() common.Address {
	return op.Factory
}

func (op *UserOperation) GetFactoryData() []byte {
	return op.FactoryData
}

func (op *UserOperation) assembleInitCode() []byte {
	if op.Factory == common.HexToAddress("0x") && len(op.FactoryData) == 0 {
		return []byte{}
	}
	return append(op.Factory.Bytes(), op.FactoryData...)
}

func (op *UserOperation) assemblePaymasterAndData() []byte {
	if op.Paymaster == (common.Address{}) {
		return []byte{}
	}
	data := make([]byte, 0, 20+16+16+len(op.PaymasterData))
	data = append(data, op.Paymaster.Bytes()...)

	var vGas []byte
	if op.PaymasterVerificationGasLimit != nil {
		vGas = op.PaymasterVerificationGasLimit.Bytes()
	}
	data = append(data, common.LeftPadBytes(vGas, 16)...)

	var pGas []byte
	if op.PaymasterPostOpGasLimit != nil {
		pGas = op.PaymasterPostOpGasLimit.Bytes()
	}
	data = append(data, common.LeftPadBytes(pGas, 16)...)

	data = append(data, op.PaymasterData...)
	return data
}

func (op *UserOperation) GetMaxGasAvailable() *big.Int {
	return big.NewInt(0).Add(
		op.VerificationGasLimit,
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
		Sender                       common.Address
		Nonce                        *big.Int
		Factory                      common.Address
		FactoryData                  []byte
		CallData                     []byte
		CallGasLimit                 *big.Int
		VerificationGasLimit         *big.Int
		PreVerificationGas           *big.Int
		MaxFeePerGas                 *big.Int
		MaxPriorityFeePerGas         *big.Int
		Paymaster                    common.Address
		PaymasterVerificationGasLimit *big.Int
		PaymasterPostOpGasLimit      *big.Int
		PaymasterData                []byte
		Signature                    []byte
		PaymasterSignature           []byte
		EIP7702Auth                  []byte
	}{
		op.Sender,
		op.Nonce,
		op.Factory,
		op.FactoryData,
		op.CallData,
		op.CallGasLimit,
		op.VerificationGasLimit,
		op.PreVerificationGas,
		op.MaxFeePerGas,
		op.MaxPriorityFeePerGas,
		op.Paymaster,
		op.PaymasterVerificationGasLimit,
		op.PaymasterPostOpGasLimit,
		op.PaymasterData,
		op.Signature,
		op.PaymasterSignature,
		op.EIP7702Auth,
	})

	enc := hexutil.Encode(packed)
	enc = "0x" + enc[66:]
	return (hexutil.MustDecode(enc))
}

func (op *UserOperation) packAccountGasLimits() [32]byte {
	out := [32]byte{}
	if op.VerificationGasLimit != nil {
		copy(out[0:16], common.LeftPadBytes(op.VerificationGasLimit.Bytes(), 16))
	}
	if op.CallGasLimit != nil {
		copy(out[16:32], common.LeftPadBytes(op.CallGasLimit.Bytes(), 16))
	}
	return out
}

func (op *UserOperation) packGasFees() [32]byte {
	out := [32]byte{}
	if op.MaxPriorityFeePerGas != nil {
		copy(out[0:16], common.LeftPadBytes(op.MaxPriorityFeePerGas.Bytes(), 16))
	}
	if op.MaxFeePerGas != nil {
		copy(out[16:32], common.LeftPadBytes(op.MaxFeePerGas.Bytes(), 16))
	}
	return out
}

func (op *UserOperation) PackForSignature() []byte {
	accountGasLimits := op.packAccountGasLimits()
	gasFees := op.packGasFees()

	if len(op.EIP7702Auth) > 0 {
		args := abi.Arguments{
			{Name: "sender", Type: addressType},
			{Name: "nonce", Type: uint256Type},
			{Name: "hashInitCode", Type: bytes32Type},
			{Name: "hashCallData", Type: bytes32Type},
			{Name: "accountGasLimits", Type: bytes32Type},
			{Name: "preVerificationGas", Type: uint256Type},
			{Name: "gasFees", Type: bytes32Type},
			{Name: "hashPaymasterAndData", Type: bytes32Type},
			{Name: "hashEip7702Auth", Type: bytes32Type},
		}
		packed, _ := args.Pack(
			op.Sender,
			op.Nonce,
			crypto.Keccak256Hash(op.assembleInitCode()),
			crypto.Keccak256Hash(op.CallData),
			accountGasLimits,
			op.PreVerificationGas,
			gasFees,
			crypto.Keccak256Hash(op.assemblePaymasterAndData()),
			crypto.Keccak256Hash(op.EIP7702Auth),
		)
		return packed
	}

	args := abi.Arguments{
		{Name: "sender", Type: addressType},
		{Name: "nonce", Type: uint256Type},
		{Name: "hashInitCode", Type: bytes32Type},
		{Name: "hashCallData", Type: bytes32Type},
		{Name: "accountGasLimits", Type: bytes32Type},
		{Name: "preVerificationGas", Type: uint256Type},
		{Name: "gasFees", Type: bytes32Type},
		{Name: "hashPaymasterAndData", Type: bytes32Type},
	}
	packed, _ := args.Pack(
		op.Sender,
		op.Nonce,
		crypto.Keccak256Hash(op.assembleInitCode()),
		crypto.Keccak256Hash(op.CallData),
		accountGasLimits,
		op.PreVerificationGas,
		gasFees,
		crypto.Keccak256Hash(op.assemblePaymasterAndData()),
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
	return json.Marshal(&struct {
		Sender                       string `json:"sender"`
		Nonce                        string `json:"nonce"`
		Factory                      string `json:"factory"`
		FactoryData                  string `json:"factoryData"`
		CallData                     string `json:"callData"`
		CallGasLimit                 string `json:"callGasLimit"`
		VerificationGasLimit         string `json:"verificationGasLimit"`
		PreVerificationGas           string `json:"preVerificationGas"`
		MaxFeePerGas                 string `json:"maxFeePerGas"`
		MaxPriorityFeePerGas         string `json:"maxPriorityFeePerGas"`
		Paymaster                    string `json:"paymaster"`
		PaymasterVerificationGasLimit string `json:"paymasterVerificationGasLimit"`
		PaymasterPostOpGasLimit      string `json:"paymasterPostOpGasLimit"`
		PaymasterData                string `json:"paymasterData"`
		Signature                    string `json:"signature"`
		PaymasterSignature           string `json:"paymasterSignature,omitempty"`
		EIP7702Auth                  string `json:"eip7702Auth,omitempty"`
	}{
		Sender:                       op.Sender.String(),
		Nonce:                        hexutil.EncodeBig(op.Nonce),
		Factory:                      op.Factory.String(),
		FactoryData:                  hexutil.Encode(op.FactoryData),
		CallData:                     hexutil.Encode(op.CallData),
		CallGasLimit:                 hexutil.EncodeBig(op.CallGasLimit),
		VerificationGasLimit:         hexutil.EncodeBig(op.VerificationGasLimit),
		PreVerificationGas:           hexutil.EncodeBig(op.PreVerificationGas),
		MaxFeePerGas:                 hexutil.EncodeBig(op.MaxFeePerGas),
		MaxPriorityFeePerGas:         hexutil.EncodeBig(op.MaxPriorityFeePerGas),
		Paymaster:                    op.Paymaster.String(),
		PaymasterVerificationGasLimit: hexutil.EncodeBig(op.PaymasterVerificationGasLimit),
		PaymasterPostOpGasLimit:      hexutil.EncodeBig(op.PaymasterPostOpGasLimit),
		PaymasterData:                hexutil.Encode(op.PaymasterData),
		Signature:                    hexutil.Encode(op.Signature),
		PaymasterSignature:           hexutil.Encode(op.PaymasterSignature),
		EIP7702Auth:                  hexutil.Encode(op.EIP7702Auth),
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
