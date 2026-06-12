package tracer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zarbanio/bundler/pkg/entrypoint/simulation"
	"github.com/zarbanio/bundler/pkg/tracer"
)

var (
	testEntryPoint = common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009")
	testSender     = common.HexToAddress("0x1111111111111111111111111111111111111111")
	testPaymaster  = common.HexToAddress("0x2222222222222222222222222222222222222222")
	testExternal   = common.HexToAddress("0x3333333333333333333333333333333333333333")
)

func loadErc7562Fixture(t *testing.T, name string) *tracer.Erc7562Frame {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	frame := &tracer.Erc7562Frame{}
	if err := json.Unmarshal(data, frame); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return frame
}

func TestFromErc7562HandleOps(t *testing.T) {
	frame := loadErc7562Fixture(t, "erc7562_handleops_v09.json")

	res, err := tracer.FromErc7562(frame, testEntryPoint, nil)
	if err != nil {
		t.Fatalf("FromErc7562: %v", err)
	}

	if len(res.CallsFromEntryPoint) != 2 {
		t.Fatalf("CallsFromEntryPoint len = %d, want 2", len(res.CallsFromEntryPoint))
	}

	sender := res.CallsFromEntryPoint[0]
	if sender.TopLevelTargetAddress != testSender {
		t.Errorf("sender target = %s, want %s", sender.TopLevelTargetAddress, testSender)
	}
	if sender.TopLevelMethodSig.String() != "0x19822f7c" {
		t.Errorf("sender method sig = %s, want 0x19822f7c", sender.TopLevelMethodSig)
	}

	// Subtree opcode aggregation, keys mapped to names: SLOAD x2 + KECCAK256
	// from the sender frame, SSTORE from the nested DELEGATECALL frame.
	wantOps := map[string]float64{"SLOAD": 2, "KECCAK256": 1, "SSTORE": 1}
	for name, count := range wantOps {
		if sender.Opcodes[name] != count {
			t.Errorf("sender opcode %s = %v, want %v", name, sender.Opcodes[name], count)
		}
	}
	if len(sender.Opcodes) != len(wantOps) {
		t.Errorf("sender opcodes = %v, want %v", sender.Opcodes, wantOps)
	}

	// Storage access keyed by storage context: the DELEGATECALL frame's write
	// belongs to the sender (frame.From), merging into one entry.
	access, ok := sender.Access[testSender]
	if !ok {
		t.Fatalf("sender access map missing %s: %v", testSender, sender.Access)
	}
	slot1 := "0x0000000000000000000000000000000000000000000000000000000000000001"
	slot3 := "0x0000000000000000000000000000000000000000000000000000000000000003"
	if access.Reads[slot1] != "0x0000000000000000000000000000000000000000000000000000000000000005" {
		t.Errorf("read slot1 = %q", access.Reads[slot1])
	}
	if access.Writes[slot1] != 1 || access.Writes[slot3] != 1 {
		t.Errorf("writes = %v, want slot1:1 and slot3:1", access.Writes)
	}

	if sender.TSTORE != 2 || sender.TLOAD != 1 {
		t.Errorf("TSTORE/TLOAD = %v/%v, want 2/1", sender.TSTORE, sender.TLOAD)
	}

	cs, ok := sender.ContractSize[testExternal]
	if !ok || cs.ContractSize != 120 || cs.Opcode != "CALL" {
		t.Errorf("contractSize[external] = %+v, want {120 CALL}", cs)
	}
	if sender.ExtCodeAccessInfo[testExternal] != "EXTCODESIZE" {
		t.Errorf("extCodeAccessInfo[external] = %q, want EXTCODESIZE", sender.ExtCodeAccessInfo[testExternal])
	}

	pm := res.CallsFromEntryPoint[1]
	if pm.TopLevelTargetAddress != testPaymaster || pm.TopLevelMethodSig.String() != "0xc04a6310" {
		t.Errorf("paymaster entry = %s %s", pm.TopLevelTargetAddress, pm.TopLevelMethodSig)
	}

	// Keccak: the native tracer stores the whole transaction's preimages on
	// the root frame; both the root's and any per-frame captures must land in
	// the aggregate, or associated-storage rules cannot match.
	wantKeccak := "0x00000000000000000000000011111111111111111111111111111111111111110000000000000000000000000000000000000000000000000000000000000000"
	found := false
	for _, k := range res.Keccak {
		if k == wantKeccak {
			found = true
		}
	}
	if !found {
		t.Errorf("keccak = %v, want it to contain the sender preimage %q", res.Keccak, wantKeccak)
	}
}

// TestFromErc7562FlatCalls verifies the synthesized flat calls list drives
// NewCallStack identically to the JS tracer's enter/exit stream.
func TestFromErc7562FlatCalls(t *testing.T) {
	frame := loadErc7562Fixture(t, "erc7562_handleops_v09.json")

	res, err := tracer.FromErc7562(frame, testEntryPoint, nil)
	if err != nil {
		t.Fatalf("FromErc7562: %v", err)
	}

	// enter(sender), enter(delegate), exit, exit, enter(paymaster), exit
	if len(res.Calls) != 6 {
		t.Fatalf("flat calls len = %d, want 6: %+v", len(res.Calls), res.Calls)
	}
	if res.Calls[0].Type != "CALL" || res.Calls[0].To != testSender || res.Calls[0].Method != "0x19822f7c" {
		t.Errorf("calls[0] = %+v", res.Calls[0])
	}
	if res.Calls[1].Type != "DELEGATECALL" {
		t.Errorf("calls[1].Type = %s, want DELEGATECALL", res.Calls[1].Type)
	}
	if res.Calls[2].Type != "RETURN" || res.Calls[3].Type != "RETURN" {
		t.Errorf("calls[2,3] = %+v %+v, want RETURN exits", res.Calls[2], res.Calls[3])
	}

	stack := simulation.NewCallStack(res.Calls)
	if len(stack) != 3 {
		t.Fatalf("call stack len = %d, want 3", len(stack))
	}
	last := stack[len(stack)-1]
	if last.To != testPaymaster || last.Method != "0xc04a6310" {
		t.Errorf("last stack entry = %+v", last)
	}
	ret, ok := last.Return.(string)
	if !ok || len(ret) < 2 {
		t.Errorf("paymaster return data = %v", last.Return)
	}
}

// TestFromErc7562KeyForms covers node encoding variants: decimal usedOpcodes
// keys, numeric contractSize opcode, and array-form extCodeAccessInfo.
func TestFromErc7562KeyForms(t *testing.T) {
	frame := loadErc7562Fixture(t, "erc7562_keyforms.json")

	res, err := tracer.FromErc7562(frame, testEntryPoint, nil)
	if err != nil {
		t.Fatalf("FromErc7562: %v", err)
	}
	if len(res.CallsFromEntryPoint) != 1 {
		t.Fatalf("CallsFromEntryPoint len = %d, want 1", len(res.CallsFromEntryPoint))
	}
	info := res.CallsFromEntryPoint[0]
	if info.Opcodes["SLOAD"] != 1 || info.Opcodes["TIMESTAMP"] != 1 {
		t.Errorf("opcodes = %v, want SLOAD:1 TIMESTAMP:1", info.Opcodes)
	}
	cs := info.ContractSize[testExternal]
	if cs.ContractSize != 64 || cs.Opcode != "CALL" {
		t.Errorf("contractSize = %+v, want {64 CALL}", cs)
	}
	if info.ExtCodeAccessInfo[testExternal] == "" {
		t.Errorf("extCodeAccessInfo missing array-form address: %v", info.ExtCodeAccessInfo)
	}
	if !info.OOG {
		t.Error("OOG flag not propagated")
	}
}

func TestFromErc7562Hook(t *testing.T) {
	frame := loadErc7562Fixture(t, "erc7562_handleops_v09.json")

	hooked := []common.Address{}
	hook := func(f *tracer.Erc7562Frame, info *tracer.CallFromEntryPointInfo) {
		hooked = append(hooked, f.To)
		if f.To == testPaymaster {
			info.PaymasterSignature = true
		}
	}
	res, err := tracer.FromErc7562(frame, testEntryPoint, hook)
	if err != nil {
		t.Fatalf("FromErc7562: %v", err)
	}
	if len(hooked) != 2 {
		t.Fatalf("hook invoked %d times, want 2", len(hooked))
	}
	if !res.CallsFromEntryPoint[1].PaymasterSignature {
		t.Error("hook mutation not visible on result")
	}
}

func TestFromErc7562WrongRoot(t *testing.T) {
	frame := loadErc7562Fixture(t, "erc7562_handleops_v09.json")
	if _, err := tracer.FromErc7562(frame, testSender, nil); err == nil {
		t.Error("expected error for root not targeting the EntryPoint")
	}
	if _, err := tracer.FromErc7562(nil, testEntryPoint, nil); err == nil {
		t.Error("expected error for nil root")
	}
}

func TestMuxedTraceEnvelope(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "muxtracer_envelope.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var muxed tracer.MuxedTrace
	if err := json.Unmarshal(data, &muxed); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if muxed.Erc7562 == nil || muxed.Erc7562.To != testEntryPoint {
		t.Errorf("erc7562 half not parsed: %+v", muxed.Erc7562)
	}
	if len(muxed.CallTracer) == 0 {
		t.Error("callTracer half missing")
	}
}
