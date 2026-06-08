package entities

import (
	"context"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Contract type constants for known account implementations.
const (
	ContractTypeSimpleAccount      = "SimpleAccount"
	ContractTypeSimple7702Account  = "Simple7702Account"
	ContractTypeVerifyingPaymaster = "VerifyingPaymaster"
	ContractTypeTokenPaymaster     = "TokenPaymaster"
)

var (
	knownBytecodeHashesMu sync.RWMutex
	knownBytecodeHashes   = map[string]string{
		// Placeholder hashes - in production these would be populated with
		// actual SimpleAccount, Simple7702Account, etc. bytecode hashes.
	}
	contractTypeByAddressMu sync.RWMutex
	contractTypeByAddress   = map[string]string{}
)

// RegisterKnownBytecode registers a bytecode hash -> contract type mapping.
// This allows extending the detection registry at runtime or in tests.
// The hash key is normalized to lowercase hex.
func RegisterKnownBytecode(bytecodeHash string, contractType string) {
	knownBytecodeHashesMu.Lock()
	defer knownBytecodeHashesMu.Unlock()
	knownBytecodeHashes[strings.ToLower(bytecodeHash)] = contractType
}

// RegisterContractAddress registers a known contract address -> type mapping.
// This allows address-based contract type resolution without an eth client.
func RegisterContractAddress(addr common.Address, contractType string) {
	contractTypeByAddressMu.Lock()
	defer contractTypeByAddressMu.Unlock()
	contractTypeByAddress[strings.ToLower(addr.Hex())] = contractType
}

// MapContractType attempts to determine the contract type for a given address
// using a registry of known addresses.
// Returns an empty string if the type cannot be determined.
func MapContractType(addr common.Address) string {
	contractTypeByAddressMu.RLock()
	defer contractTypeByAddressMu.RUnlock()
	if t, ok := contractTypeByAddress[strings.ToLower(addr.Hex())]; ok {
		return t
	}
	return ""
}

// MapContractTypeWithClient attempts to determine the contract type for a given
// address by fetching the on-chain bytecode and matching against known hashes.
// Returns an empty string if the type cannot be determined or if the client call fails.
func MapContractTypeWithClient(ctx context.Context, eth *ethclient.Client, addr common.Address) string {
	if eth == nil {
		return ""
	}

	code, err := eth.CodeAt(ctx, addr, nil)
	if err != nil {
		return ""
	}

	if len(code) == 0 {
		return ""
	}

	hash := strings.ToLower(crypto.Keccak256Hash(code).Hex())
	knownBytecodeHashesMu.RLock()
	t, ok := knownBytecodeHashes[hash]
	knownBytecodeHashesMu.RUnlock()
	if ok {
		return t
	}

	return ""
}
