package gas

import "math/big"

// GasEstimates provides estimate values for all gas fields in a UserOperation.
type GasEstimates struct {
	PreVerificationGas            *big.Int `json:"preVerificationGas"`
	VerificationGasLimit          *big.Int `json:"verificationGasLimit"`
	CallGasLimit                  *big.Int `json:"callGasLimit"`
	PaymasterVerificationGasLimit *big.Int `json:"paymasterVerificationGasLimit,omitempty"`
	PaymasterPostOpGasLimit       *big.Int `json:"paymasterPostOpGasLimit,omitempty"`

	// Deprecated: v0.6 legacy field, use VerificationGasLimit instead.
	VerificationGas *big.Int `json:"verificationGas"`
}
