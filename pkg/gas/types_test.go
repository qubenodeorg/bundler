package gas

import (
	"encoding/json"
	"math/big"
	"testing"
)

func TestGasEstimatesDefaultValues(t *testing.T) {
	e := &GasEstimates{}
	if e.PreVerificationGas != nil {
		t.Error("expected nil PreVerificationGas")
	}
	if e.PaymasterVerificationGasLimit != nil {
		t.Error("expected nil PaymasterVerificationGasLimit")
	}
	if e.PaymasterPostOpGasLimit != nil {
		t.Error("expected nil PaymasterPostOpGasLimit")
	}
}

func TestGasEstimatesJSONRoundtrip(t *testing.T) {
	e := &GasEstimates{
		PreVerificationGas:            big.NewInt(100000),
		VerificationGasLimit:          big.NewInt(200000),
		CallGasLimit:                  big.NewInt(30000),
		PaymasterVerificationGasLimit: big.NewInt(50000),
		PaymasterPostOpGasLimit:       big.NewInt(20000),
		VerificationGas:               big.NewInt(210000),
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	var decoded GasEstimates
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.PreVerificationGas.Cmp(e.PreVerificationGas) != 0 {
		t.Errorf("PreVerificationGas: got %d, want %d", decoded.PreVerificationGas, e.PreVerificationGas)
	}
	if decoded.VerificationGasLimit.Cmp(e.VerificationGasLimit) != 0 {
		t.Errorf("VerificationGasLimit: got %d, want %d", decoded.VerificationGasLimit, e.VerificationGasLimit)
	}
	if decoded.CallGasLimit.Cmp(e.CallGasLimit) != 0 {
		t.Errorf("CallGasLimit: got %d, want %d", decoded.CallGasLimit, e.CallGasLimit)
	}
	if decoded.PaymasterVerificationGasLimit.Cmp(e.PaymasterVerificationGasLimit) != 0 {
		t.Errorf("PaymasterVerificationGasLimit: got %d, want %d", decoded.PaymasterVerificationGasLimit, e.PaymasterVerificationGasLimit)
	}
	if decoded.PaymasterPostOpGasLimit.Cmp(e.PaymasterPostOpGasLimit) != 0 {
		t.Errorf("PaymasterPostOpGasLimit: got %d, want %d", decoded.PaymasterPostOpGasLimit, e.PaymasterPostOpGasLimit)
	}
	if decoded.VerificationGas.Cmp(e.VerificationGas) != 0 {
		t.Errorf("VerificationGas: got %d, want %d", decoded.VerificationGas, e.VerificationGas)
	}
}

func TestGasEstimatesJSONOmitEmpty(t *testing.T) {
	e := &GasEstimates{
		PreVerificationGas:   big.NewInt(100000),
		VerificationGasLimit: big.NewInt(200000),
		CallGasLimit:         big.NewInt(30000),
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	if _, ok := raw["paymasterVerificationGasLimit"]; ok {
		t.Error("paymasterVerificationGasLimit should be omitted when nil")
	}
	if _, ok := raw["paymasterPostOpGasLimit"]; ok {
		t.Error("paymasterPostOpGasLimit should be omitted when nil")
	}
}

func TestGasEstimatesNilSafety(t *testing.T) {
	e := &GasEstimates{}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	var decoded GasEstimates
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.PreVerificationGas != nil {
		t.Error("expected nil PreVerificationGas")
	}
}
