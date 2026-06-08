package userop

import (
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/userop/v06"
	"github.com/zarbanio/bundler/pkg/userop/v07"
	"github.com/zarbanio/bundler/pkg/userop/v08"
	"github.com/zarbanio/bundler/pkg/userop/v09"
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

// ErrBadUserOperationData is returned when user operation data cannot be decoded.
var ErrBadUserOperationData = errors.New("cannot decode UserOperation")

// Version represents an ERC-4337 version.
type Version string

const (
	V0_6 Version = "v0.6"
	V0_7 Version = "v0.7"
	V0_8 Version = "v0.8"
	V0_9 Version = "v0.9"
)

// UserOperationInterface defines the common methods across all UserOperation versions.
type UserOperationInterface interface {
	GetSender() common.Address
	GetNonce() *big.Int
	GetPaymaster() common.Address
	GetFactory() common.Address
	GetFactoryData() []byte
	GetCallData() []byte
	GetCallGasLimit() *big.Int
	GetVerificationGasLimit() *big.Int
	GetPreVerificationGas() *big.Int
	GetMaxFeePerGas() *big.Int
	GetMaxPriorityFeePerGas() *big.Int
	GetSignature() []byte
	GetEIP7702Auth() interface{}
	GetPaymasterVerificationGasLimit() *big.Int
	GetPaymasterPostOpGasLimit() *big.Int
	GetPaymasterData() []byte
	GetMaxGasAvailable() *big.Int
	GetMaxPrefund() *big.Int
	GetDynamicGasPrice(basefee *big.Int) *big.Int
	Pack() []byte
	PackForSignature() []byte
	GetUserOpHash(entryPoint common.Address, chainID *big.Int) common.Hash
	MarshalJSON() ([]byte, error)
	ToMap() (map[string]any, error)
}

// VersionedUserOperation wraps a UserOperation with its version.
type VersionedUserOperation struct {
	Version Version
	Op      UserOperationInterface
}

// NewUserOperation creates a versioned UserOperation from map data.
func NewUserOperation(version Version, data map[string]any) (*VersionedUserOperation, error) {
	switch version {
	case V0_6:
		op, err := v06.New(data)
		if err != nil {
			return nil, err
		}
		return &VersionedUserOperation{Version: V0_6, Op: op}, nil
	case V0_7:
		op, err := v07.New(data)
		if err != nil {
			return nil, err
		}
		return &VersionedUserOperation{Version: V0_7, Op: op}, nil
	case V0_8:
		op, err := v08.New(data)
		if err != nil {
			return nil, err
		}
		return &VersionedUserOperation{Version: V0_8, Op: op}, nil
	case V0_9:
		op, err := v09.New(data)
		if err != nil {
			return nil, err
		}
		return &VersionedUserOperation{Version: V0_9, Op: op}, nil
	default:
		return nil, ErrBadUserOperationData
	}
}

// DetectVersion attempts to auto-detect the version from the data fields.
func DetectVersion(data map[string]any) Version {
	// v0.9 has paymasterSignature field
	if _, ok := data["paymasterSignature"]; ok {
		return V0_9
	}
	// v0.8 has eip7702Auth field (but not paymasterSignature)
	if _, ok := data["eip7702Auth"]; ok {
		return V0_8
	}
	// v0.7+ has factory field (expanded format)
	if _, ok := data["factory"]; ok {
		return V0_7
	}
	// Default to v0.6
	return V0_6
}

// DetectVersionFromEntryPoint returns the version based on the EntryPoint address.
func DetectVersionFromEntryPoint(entryPoint common.Address) Version {
	switch strings.ToLower(entryPoint.Hex()) {
	case "0x5ff137d4b0fdcd49dca30c7cf57e578a026d2789":
		return V0_6
	case "0x0000000071727de22e5e9d8baf0edac6f37da032":
		return V0_7
	case "0x4337084d9e255ff0702461cf8895ce9e3b5ff108":
		return V0_8
	case "0x433709009b8330fda32311df1c2afa402ed8d009":
		return V0_9
	default:
		return V0_6
	}
}

// IsVersionSupported checks if the given version string is a supported version.
func IsVersionSupported(v string) bool {
	switch Version(v) {
	case V0_6, V0_7, V0_8, V0_9:
		return true
	default:
		return false
	}
}
