package checks

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/zarbanio/bundler/pkg/userop"
	v08 "github.com/zarbanio/bundler/pkg/userop/v08"
	v09 "github.com/zarbanio/bundler/pkg/userop/v09"
)

var (
	ErrEIP7702AuthNil          = errors.New("EIP-7702 authorization is nil")
	ErrEIP7702AuthInvalidChain = errors.New("EIP-7702 authorization chain ID mismatch")
	ErrEIP7702AuthInvalidSig   = errors.New("EIP-7702 authorization signature invalid")
	ErrEIP7702AuthInvalidAddr  = errors.New("EIP-7702 authorization address is zero")
)

func ValidateEIP7702Auth(op userop.UserOperationInterface, chainID *big.Int) error {
	auth := op.GetEIP7702Auth()
	if auth == nil {
		return nil
	}

	var authStruct *userop.Authorization
	switch a := auth.(type) {
	case *userop.Authorization:
		authStruct = a
	case *v08.Authorization:
		authStruct = &userop.Authorization{
			ChainID: a.ChainID,
			Address: a.Address,
			Nonce:   a.Nonce,
			YParity: a.YParity,
			R:       a.R,
			S:       a.S,
		}
	case *v09.Authorization:
		authStruct = &userop.Authorization{
			ChainID: a.ChainID,
			Address: a.Address,
			Nonce:   a.Nonce,
			YParity: a.YParity,
			R:       a.R,
			S:       a.S,
		}
	default:
		return fmt.Errorf("unknown auth type: %T", auth)
	}

	return validateEIP7702AuthFields(authStruct, op.GetSender(), chainID)
}

func validateEIP7702AuthFields(auth *userop.Authorization, sender common.Address, chainID *big.Int) error {
	if auth.Address == (common.Address{}) {
		return ErrEIP7702AuthInvalidAddr
	}
	if auth.ChainID == nil || auth.Nonce == nil || auth.R == nil || auth.S == nil {
		return ErrEIP7702AuthNil
	}
	if chainID != nil && auth.ChainID.Cmp(chainID) != 0 {
		return fmt.Errorf("%w: got %s, want %s", ErrEIP7702AuthInvalidChain, auth.ChainID, chainID)
	}

	recovered, err := recoverEIP7702Signer(auth)
	if err != nil {
		return ErrEIP7702AuthInvalidSig
	}
	if recovered != sender {
		return fmt.Errorf("%w: recovered %s, want %s", ErrEIP7702AuthInvalidSig, recovered.Hex(), sender.Hex())
	}

	return nil
}

// recoverEIP7702Signer recovers the signer address from an EIP-7702 authorization.
// The authorization hash is: keccak256(0x05 || 0x00 || chain_id || address || nonce)
func recoverEIP7702Signer(auth *userop.Authorization) (common.Address, error) {
	// Validate R and S bit lengths to prevent DoS from oversized values
	if auth.R == nil || auth.S == nil {
		return common.Address{}, ErrEIP7702AuthInvalidSig
	}
	if auth.R.BitLen() > 256 || auth.S.BitLen() > 256 {
		return common.Address{}, ErrEIP7702AuthInvalidSig
	}

	// EIP-7702 authorization message:
	// 0x05 (magic) || 0x00 (version) || chain_id (32 bytes) || address (32 bytes) || nonce (32 bytes)
	chainIDBytes := common.LeftPadBytes(auth.ChainID.Bytes(), 32)
	addrBytes := common.LeftPadBytes(auth.Address.Bytes(), 32)
	nonceBytes := common.LeftPadBytes(auth.Nonce.Bytes(), 32)

	msg := make([]byte, 0, 1+1+32+32+32)
	msg = append(msg, 0x05)                         // EIP-7702 magic byte
	msg = append(msg, 0x00)                         // Version
	msg = append(msg, chainIDBytes...)               // Chain ID (32 bytes)
	msg = append(msg, addrBytes...)                  // Address (32 bytes)
	msg = append(msg, nonceBytes...)                 // Nonce (32 bytes)

	hash := crypto.Keccak256Hash(msg)

	// Recover the signer from (v, r, s)
	sig := make([]byte, 65)
	auth.R.FillBytes(sig[:32])
	auth.S.FillBytes(sig[32:64])
	// EIP-7702 yParity is 0 or 1; crypto.SigToPub expects recovery id 0 or 1
	v := auth.YParity
	if v >= 27 {
		v -= 27
	}
	sig[64] = v

	pubKey, err := crypto.SigToPub(hash.Bytes(), sig)
	if err != nil {
		return common.Address{}, err
	}

	return crypto.PubkeyToAddress(*pubKey), nil
}

// ValidateEIP7702AuthStandalone is a standalone validation for the v0.8+ path in ValidateOpValues.
func ValidateEIP7702AuthStandalone(op userop.UserOperationInterface, chainID *big.Int) error {
	return ValidateEIP7702Auth(op, chainID)
}

// GetEIP7702Address extracts the delegate address from any supported Authorization type.
func GetEIP7702Address(auth interface{}) common.Address {
	if auth == nil {
		return common.Address{}
	}
	switch a := auth.(type) {
	case *userop.Authorization:
		return a.Address
	case *v08.Authorization:
		return a.Address
	case *v09.Authorization:
		return a.Address
	}
	return common.Address{}
}
