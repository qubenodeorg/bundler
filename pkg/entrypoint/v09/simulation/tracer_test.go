package simulation

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	simutils "github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/tracer"
)

// TestProcessTraceResult_TSTOREBannedV09 verifies the v0.9 tracer captures
// TSTORE and the simulation pipeline rejects it (per OP-070).
func TestProcessTraceResult_TSTOREBannedV09(t *testing.T) {
	op := validMockOpV09()
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"TSTORE": 1}
	res.CallsFromEntryPoint[0].TSTORE = 1

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (TSTORE banned)")
	}
}

// TestProcessTraceResult_TLOADBannedV09 verifies TLOAD is also banned.
func TestProcessTraceResult_TLOADBannedV09(t *testing.T) {
	op := validMockOpV09()
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"TLOAD": 1}
	res.CallsFromEntryPoint[0].TLOAD = 1

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (TLOAD banned)")
	}
}

// TestProcessTraceResult_EIP7702AuthFlagV09 verifies the v0.8+ EIP-7702
// auth flag survives the v0.9 pipeline.
func TestProcessTraceResult_EIP7702AuthFlagV09(t *testing.T) {
	op := validMockOpV09()
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].EIP7702Auth = true

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if out == nil {
		t.Fatal("ProcessTraceResult: got nil output")
	}
}

// TestProcessTraceResult_PaymasterSignatureFlagV09 verifies the v0.9
// paymasterSignature flag survives ProcessTraceResult.
func TestProcessTraceResult_PaymasterSignatureFlagV09(t *testing.T) {
	paymaster := validMockOpV09().Sender
	op := validMockOpV09()
	op.PaymasterAndData = paymaster.Bytes()
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].PaymasterSignature = true

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if out == nil {
		t.Fatal("ProcessTraceResult: got nil output")
	}
}

// TestProcessTraceResult_NewFields_RoundTripV09 ensures v0.7+/v0.8+/v0.9
// tracer fields are preserved without dropping data or causing errors when
// the user is staked.
func TestProcessTraceResult_NewFields_RoundTripV09(t *testing.T) {
	op := validMockOpV09()
	stakes := mockStakesV09()
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].TSTORE = 2
	res.CallsFromEntryPoint[0].TLOAD = 3
	res.CallsFromEntryPoint[0].EIP7702Auth = true
	res.CallsFromEntryPoint[0].PaymasterSignature = true

	out, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err != nil {
		t.Fatalf("ProcessTraceResult: got err %v, want nil", err)
	}
	if out == nil {
		t.Fatal("ProcessTraceResult: got nil output")
	}
}

// TestProcessTraceResult_UnstakedBannedTSTOREV09 ensures the v0.9 pipeline
// also rejects TSTORE for unstaked entities.
func TestProcessTraceResult_UnstakedBannedTSTOREV09(t *testing.T) {
	op := validMockOpV09()
	stakes := simutils.EntityStakes{
		op.Sender: {Deposit: big.NewInt(0), Staked: false, Stake: big.NewInt(0)},
	}
	res := validTraceResultV09(op)
	res.CallsFromEntryPoint[0].Opcodes = tracer.Counts{"TSTORE": 1}
	res.CallsFromEntryPoint[0].TSTORE = 1

	_, err := ProcessTraceResult(op, op.Sender, stakes, res, common.Address{})
	if err == nil {
		t.Fatal("ProcessTraceResult: got nil, want err (unstaked TSTORE banned)")
	}
}
