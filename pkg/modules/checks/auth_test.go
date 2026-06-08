package checks

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/zarbanio/bundler/pkg/userop"
	v08 "github.com/zarbanio/bundler/pkg/userop/v08"
	v09 "github.com/zarbanio/bundler/pkg/userop/v09"
)

type mockAuthOp struct {
	userop.UserOperationInterface
	auth   interface{}
	sender common.Address
}

func (m *mockAuthOp) GetEIP7702Auth() interface{} { return m.auth }
func (m *mockAuthOp) GetSender() common.Address   { return m.sender }

func TestValidateEIP7702Auth_NilAuth(t *testing.T) {
	op := &mockAuthOp{auth: nil}
	if err := ValidateEIP7702Auth(op, big.NewInt(1)); err != nil {
		t.Errorf("expected nil error for nil auth, got %v", err)
	}
}

func TestValidateEIP7702Auth_InvalidChainID(t *testing.T) {
	auth := &userop.Authorization{
		ChainID: big.NewInt(999), // wrong chain
		Address: common.HexToAddress("0xABCD"),
		Nonce:   big.NewInt(0),
		YParity: 0,
		R:       big.NewInt(1),
		S:       big.NewInt(1),
	}
	op := &mockAuthOp{auth: auth, sender: common.HexToAddress("0x1234")}
	if err := ValidateEIP7702Auth(op, big.NewInt(1)); err == nil {
		t.Error("expected error for chain ID mismatch, got nil")
	}
}

func TestValidateEIP7702Auth_ZeroAddress(t *testing.T) {
	auth := &userop.Authorization{
		ChainID: big.NewInt(1),
		Address: common.Address{}, // zero address
		Nonce:   big.NewInt(0),
		YParity: 0,
		R:       big.NewInt(1),
		S:       big.NewInt(1),
	}
	op := &mockAuthOp{auth: auth, sender: common.HexToAddress("0x1234")}
	if err := ValidateEIP7702Auth(op, big.NewInt(1)); err == nil {
		t.Error("expected error for zero address, got nil")
	}
}

func TestValidateEIP7702Auth_NilFields(t *testing.T) {
	auth := &userop.Authorization{
		ChainID: nil,
		Address: common.HexToAddress("0xABCD"),
		Nonce:   nil,
		YParity: 0,
	}
	op := &mockAuthOp{auth: auth, sender: common.HexToAddress("0x1234")}
	if err := ValidateEIP7702Auth(op, big.NewInt(1)); err == nil {
		t.Error("expected error for nil fields, got nil")
	}
}

func TestValidateEIP7702Auth_ValidSignature(t *testing.T) {
	// Generate a test keypair
	pk, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	address := crypto.PubkeyToAddress(*pk.Public().(*ecdsa.PublicKey))

	chainID := big.NewInt(1)
	delegateAddr := common.HexToAddress("0xABCDABCDABCDABCDABCDABCDABCDABCDABCDABCD")
	nonce := big.NewInt(0)

	// Compute EIP-7702 auth hash
	chainIDBytes := common.LeftPadBytes(chainID.Bytes(), 32)
	addrBytes := common.LeftPadBytes(delegateAddr.Bytes(), 32)
	nonceBytes := common.LeftPadBytes(nonce.Bytes(), 32)

	msg := make([]byte, 0, 1+1+32+32+32)
	msg = append(msg, 0x05)
	msg = append(msg, 0x00)
	msg = append(msg, chainIDBytes...)
	msg = append(msg, addrBytes...)
	msg = append(msg, nonceBytes...)
	hash := crypto.Keccak256Hash(msg)

	// Sign the hash
	sig, err := crypto.Sign(hash.Bytes(), pk)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])
	yParity := sig[64]

	auth := &userop.Authorization{
		ChainID: chainID,
		Address: delegateAddr,
		Nonce:   nonce,
		YParity: yParity,
		R:       r,
		S:       s,
	}

	op := &mockAuthOp{auth: auth, sender: address}
	if err := ValidateEIP7702Auth(op, chainID); err != nil {
		t.Errorf("expected valid signature, got error: %v", err)
	}
}

func TestValidateEIP7702Auth_WrongSigner(t *testing.T) {
	// Generate two different keypairs
	pk1, _ := crypto.GenerateKey()

	// Use a different sender than the signer
	wrongSender := common.HexToAddress("0x1234567890123456789012345678901234567890")

	chainID := big.NewInt(1)
	delegateAddr := common.HexToAddress("0xABCDABCDABCDABCDABCDABCDABCDABCDABCDABCD")
	nonce := big.NewInt(0)

	chainIDBytes := common.LeftPadBytes(chainID.Bytes(), 32)
	addrBytes := common.LeftPadBytes(delegateAddr.Bytes(), 32)
	nonceBytes := common.LeftPadBytes(nonce.Bytes(), 32)

	msg := make([]byte, 0, 1+1+32+32+32)
	msg = append(msg, 0x05)
	msg = append(msg, 0x00)
	msg = append(msg, chainIDBytes...)
	msg = append(msg, addrBytes...)
	msg = append(msg, nonceBytes...)
	hash := crypto.Keccak256Hash(msg)

	sig, _ := crypto.Sign(hash.Bytes(), pk1)
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])
	yParity := sig[64]

	auth := &userop.Authorization{
		ChainID: chainID,
		Address: delegateAddr,
		Nonce:   nonce,
		YParity: yParity,
		R:       r,
		S:       s,
	}

	op := &mockAuthOp{auth: auth, sender: wrongSender}
	if err := ValidateEIP7702Auth(op, chainID); err == nil {
		t.Error("expected error for wrong signer, got nil")
	}
}

func TestValidateEIP7702Auth_V08InvalidSignature(t *testing.T) {
	auth := &v08.Authorization{
		ChainID: big.NewInt(1),
		Address: common.HexToAddress("0xABCD"),
		Nonce:   big.NewInt(0),
		YParity: 0,
		R:       big.NewInt(1),
		S:       big.NewInt(1),
	}
	op := &mockAuthOp{auth: auth, sender: common.HexToAddress("0x1234")}
	if err := ValidateEIP7702Auth(op, big.NewInt(1)); err == nil {
		t.Error("expected error for invalid v08 signature, got nil")
	}
}

func TestValidateEIP7702Auth_V09InvalidSignature(t *testing.T) {
	auth := &v09.Authorization{
		ChainID: big.NewInt(1),
		Address: common.HexToAddress("0xABCD"),
		Nonce:   big.NewInt(0),
		YParity: 0,
		R:       big.NewInt(1),
		S:       big.NewInt(1),
	}
	op := &mockAuthOp{auth: auth, sender: common.HexToAddress("0x1234")}
	if err := ValidateEIP7702Auth(op, big.NewInt(1)); err == nil {
		t.Error("expected error for invalid v09 signature, got nil")
	}
}

func TestRecoverEIP7702Signer_YParityNormalization(t *testing.T) {
	pk, _ := crypto.GenerateKey()
	address := crypto.PubkeyToAddress(*pk.Public().(*ecdsa.PublicKey))

	chainID := big.NewInt(1)
	delegateAddr := common.HexToAddress("0xABCDABCDABCDABCDABCDABCDABCDABCDABCDABCD")
	nonce := big.NewInt(0)

	chainIDBytes := common.LeftPadBytes(chainID.Bytes(), 32)
	addrBytes := common.LeftPadBytes(delegateAddr.Bytes(), 32)
	nonceBytes := common.LeftPadBytes(nonce.Bytes(), 32)

	msg := make([]byte, 0, 1+1+32+32+32)
	msg = append(msg, 0x05)
	msg = append(msg, 0x00)
	msg = append(msg, chainIDBytes...)
	msg = append(msg, addrBytes...)
	msg = append(msg, nonceBytes...)
	hash := crypto.Keccak256Hash(msg)

	sig, _ := crypto.Sign(hash.Bytes(), pk)
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])

	// Test with yParity = 0 or 1 (modern format)
	auth01 := &userop.Authorization{
		ChainID: chainID,
		Address: delegateAddr,
		Nonce:   nonce,
		YParity: sig[64], // 0 or 1
		R:       r,
		S:       s,
	}
	recovered01, err := recoverEIP7702Signer(auth01)
	if err != nil {
		t.Fatalf("recovery with yParity 0/1 failed: %v", err)
	}
	if recovered01 != address {
		t.Errorf("recovery with yParity 0/1: got %s, want %s", recovered01.Hex(), address.Hex())
	}

	// Test with yParity = 27 or 28 (legacy format)
	auth2728 := &userop.Authorization{
		ChainID: chainID,
		Address: delegateAddr,
		Nonce:   nonce,
		YParity: sig[64] + 27, // 27 or 28
		R:       r,
		S:       s,
	}
	recovered2728, err := recoverEIP7702Signer(auth2728)
	if err != nil {
		t.Fatalf("recovery with yParity 27/28 failed: %v", err)
	}
	if recovered2728 != address {
		t.Errorf("recovery with yParity 27/28: got %s, want %s", recovered2728.Hex(), address.Hex())
	}
}

func TestRecoverEIP7702Signer_OversizedRS(t *testing.T) {
	// R value with 257 bits should be rejected
	oversizedR := new(big.Int).Lsh(big.NewInt(1), 256)
	oversizedS := new(big.Int).Lsh(big.NewInt(1), 256)

	auth := &userop.Authorization{
		ChainID: big.NewInt(1),
		Address: common.HexToAddress("0xABCD"),
		Nonce:   big.NewInt(0),
		YParity: 0,
		R:       oversizedR,
		S:       oversizedS,
	}

	_, err := recoverEIP7702Signer(auth)
	if err == nil {
		t.Error("expected error for oversized R/S, got nil")
	}
}

func TestRecoverEIP7702Signer_NilRS(t *testing.T) {
	auth := &userop.Authorization{
		ChainID: big.NewInt(1),
		Address: common.HexToAddress("0xABCD"),
		Nonce:   big.NewInt(0),
		YParity: 0,
		R:       nil,
		S:       nil,
	}

	_, err := recoverEIP7702Signer(auth)
	if err == nil {
		t.Error("expected error for nil R/S, got nil")
	}
}

func TestGetEIP7702Address_UserOp(t *testing.T) {
	auth := &userop.Authorization{
		Address: common.HexToAddress("0xABCD"),
	}
	addr := GetEIP7702Address(auth)
	if addr != common.HexToAddress("0xABCD") {
		t.Errorf("got %s, want 0xABCD", addr.Hex())
	}
}

func TestGetEIP7702Address_V08(t *testing.T) {
	auth := &v08.Authorization{
		Address: common.HexToAddress("0xABCD"),
	}
	addr := GetEIP7702Address(auth)
	if addr != common.HexToAddress("0xABCD") {
		t.Errorf("got %s, want 0xABCD", addr.Hex())
	}
}

func TestGetEIP7702Address_V09(t *testing.T) {
	auth := &v09.Authorization{
		Address: common.HexToAddress("0xABCD"),
	}
	addr := GetEIP7702Address(auth)
	if addr != common.HexToAddress("0xABCD") {
		t.Errorf("got %s, want 0xABCD", addr.Hex())
	}
}

func TestGetEIP7702Address_Nil(t *testing.T) {
	addr := GetEIP7702Address(nil)
	if addr != (common.Address{}) {
		t.Errorf("got %s, want zero address", addr.Hex())
	}
}

func TestGetEIP7702Address_UnknownType(t *testing.T) {
	addr := GetEIP7702Address("not-an-auth")
	if addr != (common.Address{}) {
		t.Errorf("got %s, want zero address", addr.Hex())
	}
}
