package entities

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestContractTypeConstants(t *testing.T) {
	if ContractTypeSimpleAccount != "SimpleAccount" {
		t.Error("SimpleAccount constant mismatch")
	}
	if ContractTypeSimple7702Account != "Simple7702Account" {
		t.Error("Simple7702Account constant mismatch")
	}
	if ContractTypeVerifyingPaymaster != "VerifyingPaymaster" {
		t.Error("VerifyingPaymaster constant mismatch")
	}
	if ContractTypeTokenPaymaster != "TokenPaymaster" {
		t.Error("TokenPaymaster constant mismatch")
	}
}

func TestMapContractType_Unknown(t *testing.T) {
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	if result := MapContractType(addr); result != "" {
		t.Errorf("expected empty string for unknown address, got %s", result)
	}
}

func TestMapContractTypeWithClient_NilClient(t *testing.T) {
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	if result := MapContractTypeWithClient(context.Background(), nil, addr); result != "" {
		t.Errorf("expected empty string for nil client, got %s", result)
	}
}

func TestRegisterKnownBytecode(t *testing.T) {
	// Create a fake bytecode and hash
	bytecode := []byte{0x60, 0x80, 0x60, 0x40, 0x52}
	hash := crypto.Keccak256Hash(bytecode).Hex()

	// Snapshot current state
	knownBytecodeHashesMu.RLock()
	prev, hadPrev := knownBytecodeHashes[hash]
	knownBytecodeHashesMu.RUnlock()

	// Register it
	RegisterKnownBytecode(hash, ContractTypeSimple7702Account)

	// Restore state after test
	t.Cleanup(func() {
		knownBytecodeHashesMu.Lock()
		defer knownBytecodeHashesMu.Unlock()
		if hadPrev {
			knownBytecodeHashes[hash] = prev
		} else {
			delete(knownBytecodeHashes, hash)
		}
	})

	// Verify it's in the map (under lock)
	knownBytecodeHashesMu.RLock()
	ct, ok := knownBytecodeHashes[hash]
	knownBytecodeHashesMu.RUnlock()
	if !ok || ct != ContractTypeSimple7702Account {
		t.Error("failed to register known bytecode")
	}
}
