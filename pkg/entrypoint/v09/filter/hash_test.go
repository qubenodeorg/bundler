package filter

import "testing"

func TestIsValidUserOpHash(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"0x0000000000000000000000000000000000000000000000000000000000000000", true},
		{"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", true},
		{"0x1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF", true},
		{"", false},
		{"0x", false},
		{"0x1234", false},
		{"0x00000000000000000000000000000000000000000000000000000000000000000", false},
		{"0xgggg00000000000000000000000000000000000000000000000000000000000000", false},
		{"0000000000000000000000000000000000000000000000000000000000000000", false},
		{"0x000000000000000000000000000000000000000000000000000000000000000z", false},
	}

	for _, tt := range tests {
		got := IsValidUserOpHash(tt.input)
		if got != tt.want {
			t.Errorf("IsValidUserOpHash(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
