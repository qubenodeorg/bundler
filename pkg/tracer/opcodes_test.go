package tracer_test

import (
	"testing"

	"github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/tracer"
)

func TestOpNameKnownBytes(t *testing.T) {
	cases := map[byte]string{
		0x20: "KECCAK256",
		0x31: "BALANCE",
		0x32: "ORIGIN",
		0x3a: "GASPRICE",
		0x3b: "EXTCODESIZE",
		0x3f: "EXTCODEHASH",
		0x40: "BLOCKHASH",
		0x41: "COINBASE",
		0x42: "TIMESTAMP",
		0x43: "NUMBER",
		0x44: "DIFFICULTY",
		0x45: "GASLIMIT",
		0x47: "SELFBALANCE",
		0x48: "BASEFEE",
		0x54: "SLOAD",
		0x55: "SSTORE",
		0x5a: "GAS",
		0x5c: "TLOAD",
		0x5d: "TSTORE",
		0xf0: "CREATE",
		0xf1: "CALL",
		0xf5: "CREATE2",
		0xfa: "STATICCALL",
		0xff: "SELFDESTRUCT",
	}
	for b, want := range cases {
		if got := tracer.OpName(b); got != want {
			t.Errorf("OpName(0x%02x) = %q, want %q", b, got, want)
		}
	}
}

// TestBannedOpcodesResolvable asserts every name in the banned rule sets is
// producible by the opcode table, so a native trace can never miss a banned
// opcode due to a naming mismatch.
func TestBannedOpcodesResolvable(t *testing.T) {
	producible := map[string]bool{}
	for i := 0; i < 256; i++ {
		producible[tracer.OpName(byte(i))] = true
	}

	for _, name := range simulation.BannedOpCodeSet.ToSlice() {
		if !producible[name] {
			t.Errorf("banned opcode %q is not producible by OpName", name)
		}
	}
	for _, name := range simulation.BannedUnstakedOpCodeSet.ToSlice() {
		if !producible[name] {
			t.Errorf("banned unstaked opcode %q is not producible by OpName", name)
		}
	}
	if !producible[simulation.Create2OpCode] {
		t.Errorf("opcode %q is not producible by OpName", simulation.Create2OpCode)
	}
}

func TestOpNameFromKey(t *testing.T) {
	cases := map[string]string{
		"0x54":  "SLOAD",
		"0X42":  "TIMESTAMP",
		"84":    "SLOAD",
		"66":    "TIMESTAMP",
		"0xf1":  "CALL",
		"SLOAD": "SLOAD",
		"sload": "SLOAD",
	}
	for key, want := range cases {
		if got := tracer.OpNameFromKey(key); got != want {
			t.Errorf("OpNameFromKey(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestOpNameUndefinedByte(t *testing.T) {
	if got := tracer.OpName(0x0c); got != "opcode 0x0c" {
		t.Errorf("OpName(0x0c) = %q, want %q", got, "opcode 0x0c")
	}
}
