package gas

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

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

// MarshalJSON encodes all gas values as hex QUANTITY strings, per the
// ERC-4337 JSON-RPC spec (raw big.Ints would serialize as JSON numbers).
func (g *GasEstimates) MarshalJSON() ([]byte, error) {
	enc := func(b *big.Int) any {
		if b == nil {
			return nil
		}
		return hexutil.EncodeBig(b)
	}
	out := map[string]any{
		"preVerificationGas":   enc(g.PreVerificationGas),
		"verificationGasLimit": enc(g.VerificationGasLimit),
		"callGasLimit":         enc(g.CallGasLimit),
		"verificationGas":      enc(g.VerificationGas),
	}
	if g.PaymasterVerificationGasLimit != nil {
		out["paymasterVerificationGasLimit"] = enc(g.PaymasterVerificationGasLimit)
	}
	if g.PaymasterPostOpGasLimit != nil {
		out["paymasterPostOpGasLimit"] = enc(g.PaymasterPostOpGasLimit)
	}
	return json.Marshal(out)
}

// UnmarshalJSON accepts gas values either as hex QUANTITY strings (the wire
// format produced by MarshalJSON) or as plain JSON numbers (legacy format).
func (g *GasEstimates) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	dec := func(key string) (*big.Int, error) {
		v, ok := raw[key]
		if !ok || string(v) == "null" {
			return nil, nil
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			return hexutil.DecodeBig(s)
		}
		n := new(big.Int)
		if err := json.Unmarshal(v, n); err != nil {
			return nil, err
		}
		return n, nil
	}

	var err error
	if g.PreVerificationGas, err = dec("preVerificationGas"); err != nil {
		return err
	}
	if g.VerificationGasLimit, err = dec("verificationGasLimit"); err != nil {
		return err
	}
	if g.CallGasLimit, err = dec("callGasLimit"); err != nil {
		return err
	}
	if g.PaymasterVerificationGasLimit, err = dec("paymasterVerificationGasLimit"); err != nil {
		return err
	}
	if g.PaymasterPostOpGasLimit, err = dec("paymasterPostOpGasLimit"); err != nil {
		return err
	}
	if g.VerificationGas, err = dec("verificationGas"); err != nil {
		return err
	}
	return nil
}
