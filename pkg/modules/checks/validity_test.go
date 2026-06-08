package checks

import (
	"math/big"
	"testing"
)

func TestIsBlockNumberMode(t *testing.T) {
	tests := []struct {
		name     string
		value    *big.Int
		expected bool
	}{
		{"nil value", nil, false},
		{"zero value", big.NewInt(0), false},
		{"negative value", big.NewInt(-1), false},
		{"timestamp mode", big.NewInt(1000000), false},
		{"block number mode", new(big.Int).SetUint64(1 << 47), true},
		{"block number mode with value", new(big.Int).SetUint64((1<<47)+12345), true},
		{"max uint48 timestamp", new(big.Int).SetUint64((1<<47)-1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBlockNumberMode(tt.value)
			if result != tt.expected {
				t.Fatalf("IsBlockNumberMode(%v) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestStripBlockNumberModeBit(t *testing.T) {
	tests := []struct {
		name     string
		value    *big.Int
		expected uint64
	}{
		{"nil value", nil, 0},
		{"zero value", big.NewInt(0), 0},
		{"timestamp mode", big.NewInt(12345), 12345},
		{"block number mode", new(big.Int).SetUint64((1 << 47) + 12345), 12345},
		{"block number mode zero", new(big.Int).SetUint64(1 << 47), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripBlockNumberModeBit(tt.value)
			if result != tt.expected {
				t.Fatalf("stripBlockNumberModeBit(%v) = %d, want %d", tt.value, result, tt.expected)
			}
		})
	}
}

func TestValidateTimeRange(t *testing.T) {
	tests := []struct {
		name         string
		validAfter   *big.Int
		validUntil   *big.Int
		currentBlock uint64
		currentTime  int64
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "no restrictions",
			validAfter:   nil,
			validUntil:   nil,
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      false,
		},
		{
			name:         "timestamp validAfter satisfied",
			validAfter:   big.NewInt(500),
			validUntil:   nil,
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      false,
		},
		{
			name:         "timestamp validAfter not reached",
			validAfter:   big.NewInt(5000),
			validUntil:   nil,
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      true,
			errMsg:       "validAfter timestamp 5000 not reached",
		},
		{
			name:         "timestamp validUntil satisfied",
			validAfter:   nil,
			validUntil:   big.NewInt(2000),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      false,
		},
		{
			name:         "timestamp validUntil too soon",
			validAfter:   nil,
			validUntil:   big.NewInt(1020),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      true,
			errMsg:       "expires too soon",
		},
		{
			name:         "timestamp validUntil exact buffer",
			validAfter:   nil,
			validUntil:   big.NewInt(1030),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      true,
			errMsg:       "expires too soon",
		},
		{
			name:         "timestamp validUntil just past buffer",
			validAfter:   nil,
			validUntil:   big.NewInt(1031),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      false,
		},
		{
			name:         "block number validAfter satisfied",
			validAfter:   new(big.Int).SetUint64((1 << 47) + 50),
			validUntil:   nil,
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      false,
		},
		{
			name:         "block number validAfter not reached",
			validAfter:   new(big.Int).SetUint64((1 << 47) + 150),
			validUntil:   nil,
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      true,
			errMsg:       "validAfter block 150 not reached (current: 100)",
		},
		{
			name:         "block number validUntil satisfied",
			validAfter:   nil,
			validUntil:   new(big.Int).SetUint64((1 << 47) + 200),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      false,
		},
		{
			name:         "block number validUntil too soon",
			validAfter:   nil,
			validUntil:   new(big.Int).SetUint64((1 << 47) + 101),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      true,
			errMsg:       "validUntil block 101 too soon (current: 100)",
		},
		{
			name:         "block number validUntil exact buffer",
			validAfter:   nil,
			validUntil:   new(big.Int).SetUint64((1 << 47) + 100),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      true,
			errMsg:       "validUntil block 100 too soon (current: 100)",
		},
		{
			name:         "block number validUntil just past buffer",
			validAfter:   nil,
			validUntil:   new(big.Int).SetUint64((1 << 47) + 102),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      false,
		},
		{
			name:         "both validAfter and validUntil satisfied",
			validAfter:   big.NewInt(500),
			validUntil:   big.NewInt(2000),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      false,
		},
		{
			name:         "both validAfter and validUntil failed",
			validAfter:   big.NewInt(5000),
			validUntil:   big.NewInt(6000),
			currentBlock: 100,
			currentTime:  1000,
			wantErr:      true,
			errMsg:       "validAfter timestamp 5000 not reached",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimeRange(tt.validAfter, tt.validUntil, tt.currentBlock, tt.currentTime)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Fatalf("expected error %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateTimeRangeZeroValues(t *testing.T) {
	// Zero values should be treated as no restriction
	err := ValidateTimeRange(big.NewInt(0), big.NewInt(0), 100, 1000)
	if err != nil {
		t.Fatalf("zero values should not error: %v", err)
	}
}
