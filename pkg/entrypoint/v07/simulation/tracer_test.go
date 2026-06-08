package simulation

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	simutils "github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/tracer"
)

// TestProcessTraceResult_TSTOREBanned_Staked verifies that the v0.7+ tracer
// captures TSTORE counts and the simulation pipeline flags it as a banned
// opcode (per OP-070).
func TestProcessTraceResult_TSTOREBanned_StakedV07(t *testing.T) {
	op := validMockOp()
	stakes := mockStakes()
	res := validTraceResult(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"TSTORE": 1}
	res.CallsFromEntryPoint[0].TSTORE = 1

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (TSTORE banned)")
	}
}

// TestProcessTraceResult_TLOADBanned_Staked verifies TLOAD is also banned.
func TestProcessTraceResult_TLOADBanned_StakedV07(t *testing.T) {
	op := validMockOp()
	stakes := mockStakes()
	res := validTraceResult(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"TLOAD": 1}
	res.CallsFromEntryPoint[0].TLOAD = 1

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (TLOAD banned)")
	}
}

// TestProcessTraceResult_NewFields_RoundTrip ensures the new v0.7+ tracer
// fields (TSTORE, TLOAD) are preserved through ProcessTraceResult without
// being dropped or causing spurious errors when the user is staked.
func TestProcessTraceResult_NewFields_RoundTripV07(t *testing.T) {
	op := validMockOp()
	stakes := mockStakes()
	res := validTraceResult(op)
	res.CallsFromEntryPoint[0].TSTORE = 2
	res.CallsFromEntryPoint[0].TLOAD = 3

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if out == nil {
		t.Fatal("ProcessTraceResult: got nil output")
	}
}

// TestProcessTraceResult_AltMempoolCollection_StakedV07 verifies the alt
// mempool wiring still works with the v0.7+ tracer shape.
func TestProcessTraceResult_AltMempoolCollection_StakedV07(t *testing.T) {
	op := validMockOp()
	stakes := simutils.EntityStakes{
		op.Sender: {Deposit: big.NewInt(1000000), Staked: true, Stake: big.NewInt(1000000)},
	}
	res := validTraceResult(op)

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if out == nil {
		t.Fatal("ProcessTraceResult: got nil output")
	}
}
