package simulation

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	simutils "github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/tracer"
)

// TestProcessTraceResult_TSTOREBannedV08 verifies v0.8 tracer captures
// TSTORE and the simulation pipeline rejects it (per OP-070).
func TestProcessTraceResult_TSTOREBannedV08(t *testing.T) {
	op := validMockOpV08()
	stakes := mockStakesV08()
	res := validTraceResultV08(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"TSTORE": 1}
	res.CallsFromEntryPoint[0].TSTORE = 1

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (TSTORE banned)")
	}
}

// TestProcessTraceResult_TLOADBannedV08 verifies TLOAD is also banned.
func TestProcessTraceResult_TLOADBannedV08(t *testing.T) {
	op := validMockOpV08()
	stakes := mockStakesV08()
	res := validTraceResultV08(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"TLOAD": 1}
	res.CallsFromEntryPoint[0].TLOAD = 1

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (TLOAD banned)")
	}
}

// TestProcessTraceResult_EIP7702AuthFlagV08 verifies the v0.8 EIP-7702 auth
// flag captured by the tracer survives ProcessTraceResult without causing
// spurious errors.
func TestProcessTraceResult_EIP7702AuthFlagV08(t *testing.T) {
	op := validMockOpV08()
	stakes := mockStakesV08()
	res := validTraceResultV08(op)
	res.CallsFromEntryPoint[0].EIP7702Auth = true

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if out == nil {
		t.Fatal("ProcessTraceResult: got nil output")
	}
}

// TestProcessTraceResult_NewFields_RoundTripV08 ensures v0.7+/v0.8+ tracer
// fields are preserved without dropping data or causing errors when the
// user is staked.
func TestProcessTraceResult_NewFields_RoundTripV08(t *testing.T) {
	op := validMockOpV08()
	stakes := mockStakesV08()
	res := validTraceResultV08(op)
	res.CallsFromEntryPoint[0].TSTORE = 2
	res.CallsFromEntryPoint[0].TLOAD = 3
	res.CallsFromEntryPoint[0].EIP7702Auth = true

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if out == nil {
		t.Fatal("ProcessTraceResult: got nil output")
	}
}

// TestProcessTraceResult_UnstakedBannedTSTOREV08 ensures the v0.8 pipeline
// also rejects TSTORE for unstaked entities (matches v0.6 behavior).
func TestProcessTraceResult_UnstakedBannedTSTOREV08(t *testing.T) {
	op := validMockOpV08()
	stakes := simutils.EntityStakes{
		op.Sender: {Deposit: big.NewInt(0), Staked: false, Stake: big.NewInt(0)},
	}
	res := validTraceResultV08(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"TSTORE": 1}
	res.CallsFromEntryPoint[0].TSTORE = 1

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (unstaked TSTORE banned)")
	}
}
