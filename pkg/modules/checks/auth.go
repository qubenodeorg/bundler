package checks

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
	rpcerrors "github.com/zarbanio/bundler/pkg/errors"
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

	authStruct, err := NormalizeEIP7702Auth(auth)
	if err != nil {
		return err
	}

	return validateEIP7702AuthFields(authStruct, op.GetSender(), chainID)
}

// NormalizeEIP7702Auth converts any supported Authorization type into the
// version-agnostic userop.Authorization.
func NormalizeEIP7702Auth(auth interface{}) (*userop.Authorization, error) {
	switch a := auth.(type) {
	case *userop.Authorization:
		return a, nil
	case *v08.Authorization:
		return &userop.Authorization{
			ChainID: a.ChainID,
			Address: a.Address,
			Nonce:   a.Nonce,
			YParity: a.YParity,
			R:       a.R,
			S:       a.S,
		}, nil
	case *v09.Authorization:
		return &userop.Authorization{
			ChainID: a.ChainID,
			Address: a.Address,
			Nonce:   a.Nonce,
			YParity: a.YParity,
			R:       a.R,
			S:       a.S,
		}, nil
	default:
		return nil, fmt.Errorf("unknown auth type: %T", auth)
	}
}

func validateEIP7702AuthFields(auth *userop.Authorization, sender common.Address, chainID *big.Int) error {
	if auth.Address == (common.Address{}) {
		return ErrEIP7702AuthInvalidAddr
	}
	if auth.ChainID == nil || auth.Nonce == nil || auth.R == nil || auth.S == nil {
		return ErrEIP7702AuthNil
	}
	// EIP-7702 allows chainid 0 (valid on any chain) besides the current one.
	if chainID != nil && auth.ChainID.Sign() != 0 && auth.ChainID.Cmp(chainID) != 0 {
		return fmt.Errorf("eip7702Auth chainid mismatch: got %s, want 0 or %s", auth.ChainID, chainID)
	}

	recovered, err := recoverEIP7702Signer(auth)
	if err != nil {
		return ErrEIP7702AuthInvalidSig
	}
	if recovered != sender {
		return fmt.Errorf("eip7702Auth signer does not match sender: recovered %s, want %s", recovered.Hex(), sender.Hex())
	}

	return nil
}

// recoverEIP7702Signer recovers the signer address from an EIP-7702 authorization.
// The authorization hash is: keccak256(0x05 || rlp([chain_id, address, nonce]))
func recoverEIP7702Signer(auth *userop.Authorization) (common.Address, error) {
	// Validate R and S bit lengths to prevent DoS from oversized values
	if auth.R == nil || auth.S == nil {
		return common.Address{}, ErrEIP7702AuthInvalidSig
	}
	if auth.R.BitLen() > 256 || auth.S.BitLen() > 256 {
		return common.Address{}, ErrEIP7702AuthInvalidSig
	}

	payload, err := rlp.EncodeToBytes([]interface{}{auth.ChainID, auth.Address, auth.Nonce})
	if err != nil {
		return common.Address{}, err
	}
	hash := crypto.Keccak256Hash(append([]byte{0x05}, payload...))

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

// ValidateEIP7702AuthNonce checks that the authorization nonce matches the
// sender's current account nonce. A stale or future nonce means the chain
// would skip the delegation and handleOps would revert on-chain.
func ValidateEIP7702AuthNonce(op userop.UserOperationInterface, eth *ethclient.Client) error {
	auth := op.GetEIP7702Auth()
	if auth == nil {
		return nil
	}
	authStruct, err := NormalizeEIP7702Auth(auth)
	if err != nil {
		return err
	}
	nonce, err := eth.NonceAt(context.Background(), op.GetSender(), nil)
	if err != nil {
		return err
	}
	if authStruct.Nonce == nil || authStruct.Nonce.Cmp(new(big.Int).SetUint64(nonce)) != 0 {
		return rpcerrors.NewRPCError(
			rpcerrors.REJECTED_BY_EP_OR_ACCOUNT,
			fmt.Sprintf("eip7702Auth nonce mismatch: got %s, want %d", authStruct.Nonce, nonce),
			nil,
		)
	}
	return nil
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
