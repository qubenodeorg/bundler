package gas

import (
	"math/big"
	"testing"
)

func TestCalcPVGFuncNoop(t *testing.T) {
	fn := calcPVGFuncNoop()
	result, err := fn(nil, big.NewInt(1000))
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil result from noop, got %v", result)
	}
}

func TestFieldToHexStr(t *testing.T) {
	data := map[string]any{
		"present": "0x1234",
		"empty":   "0x",
	}

	if got := fieldToHexStr(data, "present"); got != "0x1234" {
		t.Errorf("expected 0x1234, got %s", got)
	}

	if got := fieldToHexStr(data, "empty"); got != "0x" {
		t.Errorf("expected 0x, got %s", got)
	}

	if got := fieldToHexStr(data, "missing"); got != "0x" {
		t.Errorf("expected 0x for missing key, got %s", got)
	}
}

func TestFieldToHexStrNonString(t *testing.T) {
	data := map[string]any{
		"number": 42,
	}

	if got := fieldToHexStr(data, "number"); got != "0x" {
		t.Errorf("expected 0x for non-string value, got %s", got)
	}
}
