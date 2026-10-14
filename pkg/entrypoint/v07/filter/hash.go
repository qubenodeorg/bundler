package filter

import "strings"

func IsValidUserOpHash(userOpHash string) bool {
	if !strings.HasPrefix(userOpHash, "0x") {
		return false
	}
	hex := strings.TrimPrefix(userOpHash, "0x")
	if len(hex) != 64 {
		return false
	}
	for _, c := range hex {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
