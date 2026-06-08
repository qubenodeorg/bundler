package checks

import (
	"fmt"
	"math/big"
)

// IsBlockNumberMode checks if the high bit (bit 47) of a uint48 value is set,
// indicating that the value represents a block number rather than a timestamp.
func IsBlockNumberMode(value *big.Int) bool {
	if value == nil || value.Sign() <= 0 {
		return false
	}
	return value.Bit(47) == 1
}

// stripBlockNumberModeBit clears the block number mode bit to get the actual block number.
func stripBlockNumberModeBit(value *big.Int) uint64 {
	if value == nil {
		return 0
	}
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 48), big.NewInt(1)) // 48 bits of 1s
	mask.Xor(mask, new(big.Int).Lsh(big.NewInt(1), 47))                          // Clear bit 47
	v := new(big.Int).And(value, mask)
	return v.Uint64()
}

// ValidateTimeRange validates validAfter and validUntil against current block number or timestamp.
//
// When IsBlockNumberMode is true for a value, it is compared against currentBlock.
// Otherwise, it is compared against currentTime (Unix timestamp).
//
// For validUntil, a buffer is applied:
//   - Block number mode: 1 block buffer
//   - Timestamp mode: 30 second buffer
func ValidateTimeRange(validAfter, validUntil *big.Int, currentBlock uint64, currentTime int64) error {
	// Check validAfter
	if validAfter != nil && validAfter.Sign() > 0 {
		if IsBlockNumberMode(validAfter) {
			blockNum := stripBlockNumberModeBit(validAfter)
			if currentBlock < blockNum {
				return fmt.Errorf("validAfter block %d not reached (current: %d)", blockNum, currentBlock)
			}
		} else {
			if currentTime < validAfter.Int64() {
				return fmt.Errorf("validAfter timestamp %d not reached", validAfter.Int64())
			}
		}
	}

	// Check validUntil with buffer
	if validUntil != nil && validUntil.Sign() > 0 {
		if IsBlockNumberMode(validUntil) {
			blockNum := stripBlockNumberModeBit(validUntil)
			buffer := uint64(1) // 1 block buffer
			if currentBlock >= blockNum-buffer {
				return fmt.Errorf("validUntil block %d too soon (current: %d)", blockNum, currentBlock)
			}
		} else {
			buffer := int64(30) // 30 second buffer
			if currentTime >= validUntil.Int64()-buffer {
				return fmt.Errorf("expires too soon")
			}
		}
	}

	return nil
}
