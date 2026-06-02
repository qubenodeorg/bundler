package tracer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestBundlerCollectorReturn_RoundTripMinimal(t *testing.T) {
	in := BundlerCollectorReturn{
		CallsFromEntryPoint: nil,
		Keccak:              nil,
		Calls:               nil,
		Logs:                nil,
		Debug:               nil,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out BundlerCollectorReturn
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.CallsFromEntryPoint) != 0 || len(out.Keccak) != 0 || len(out.Calls) != 0 {
		t.Errorf("unmarshal lost data: %+v", out)
	}
}

func TestBundlerCollectorReturn_DeserializeEIP7702(t *testing.T) {
	raw := `{
        "callsFromEntryPoint": [
            {
                "topLevelMethodSig": "0x1a5d1cd7",
                "topLevelTargetAddress": "0x433709009b8330fda32311df1c2afa402ed8d009",
                "opcodes": {},
                "access": {},
                "contractSize": {},
                "extCodeAccessInfo": {},
                "oog": false,
                "eip7702Auth": true
            }
        ],
        "keccak": [],
        "calls": [],
        "logs": [],
        "debug": []
    }`
	var got BundlerCollectorReturn
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.CallsFromEntryPoint) != 1 {
		t.Fatalf("expected 1 call, got %d", len(got.CallsFromEntryPoint))
	}
	if !got.CallsFromEntryPoint[0].EIP7702Auth {
		t.Error("EIP7702Auth not set after unmarshal")
	}
}

func TestBundlerCollectorReturn_DeserializePaymasterSignature(t *testing.T) {
	raw := `{
        "callsFromEntryPoint": [
            {
                "topLevelMethodSig": "0x",
                "topLevelTargetAddress": "0x1111111111111111111111111111111111111111",
                "opcodes": {},
                "access": {},
                "contractSize": {},
                "extCodeAccessInfo": {},
                "oog": false,
                "paymasterSignature": true
            }
        ],
        "keccak": [],
        "calls": [],
        "logs": [],
        "debug": []
    }`
	var got BundlerCollectorReturn
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.CallsFromEntryPoint[0].PaymasterSignature {
		t.Error("PaymasterSignature not set after unmarshal")
	}
}

func TestBundlerCollectorReturn_DeserializeTSTORE(t *testing.T) {
	raw := `{
        "callsFromEntryPoint": [
            {
                "topLevelMethodSig": "0x",
                "topLevelTargetAddress": "0x1111111111111111111111111111111111111111",
                "opcodes": {},
                "access": {},
                "contractSize": {},
                "extCodeAccessInfo": {},
                "oog": false,
                "tstore": 3,
                "tload": 1
            }
        ],
        "keccak": [],
        "calls": [],
        "logs": [],
        "debug": []
    }`
	var got BundlerCollectorReturn
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.CallsFromEntryPoint[0].TSTORE != 3 {
		t.Errorf("TSTORE: got %v, want 3", got.CallsFromEntryPoint[0].TSTORE)
	}
	if got.CallsFromEntryPoint[0].TLOAD != 1 {
		t.Errorf("TLOAD: got %v, want 1", got.CallsFromEntryPoint[0].TLOAD)
	}
}

func TestBundlerCollectorReturn_OmitZeroTSTORE(t *testing.T) {
	in := CallFromEntryPointInfo{}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "tstore") {
		t.Errorf("TSTORE should be omitted when zero; got: %s", s)
	}
	if strings.Contains(s, "tload") {
		t.Errorf("TLOAD should be omitted when zero; got: %s", s)
	}
	if strings.Contains(s, "eip7702Auth") {
		t.Errorf("eip7702Auth should be omitted when false; got: %s", s)
	}
}

func TestBundlerExecutionReturn_RoundTripMinimal(t *testing.T) {
	in := BundlerExecutionReturn{}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out BundlerExecutionReturn
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ValidationOOG || out.ExecutionOOG {
		t.Errorf("unexpected OOG flags: %+v", out)
	}
}

func TestBundlerExecutionReturn_DeserializeCurrentUserOpHash(t *testing.T) {
	raw := `{
        "reverts": [],
        "validationOOG": false,
        "executionOOG": false,
        "executionGasLimit": 100000,
        "currentUserOpHash": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
        "currentUserOpHashes": ["0xdeadbeef"],
        "output": "0x",
        "error": ""
    }`
	var got BundlerExecutionReturn
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.CurrentUserOpHash == "" {
		t.Error("CurrentUserOpHash not set")
	}
	if len(got.CurrentUserOpHashes) != 1 {
		t.Errorf("CurrentUserOpHashes: got %d, want 1", len(got.CurrentUserOpHashes))
	}
}

func TestBundlerExecutionReturn_OmitEmptyCurrentUserOpHash(t *testing.T) {
	in := BundlerExecutionReturn{}
	b, _ := json.Marshal(in)
	if strings.Contains(string(b), "currentUserOpHash") {
		t.Errorf("currentUserOpHash should be omitted when empty; got: %s", string(b))
	}
}

func TestBundlerExecutionReturn_DeserializeUserOpEvent(t *testing.T) {
	raw := `{
        "reverts": [],
        "validationOOG": false,
        "executionOOG": false,
        "executionGasLimit": 0,
        "userOperationEvent": {
            "topics": ["0x01","0x02"],
            "data": "0xabcd"
        },
        "output": "0x",
        "error": ""
    }`
	var got BundlerExecutionReturn
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.UserOperationEvent == nil {
		t.Fatal("UserOperationEvent is nil")
	}
	if len(got.UserOperationEvent.Topics) != 2 {
		t.Errorf("Topics: got %d, want 2", len(got.UserOperationEvent.Topics))
	}
	if got.UserOperationEvent.Data != "0xabcd" {
		t.Errorf("Data: got %q, want %q", got.UserOperationEvent.Data, "0xabcd")
	}
}

func TestCallInfo_Defaults(t *testing.T) {
	c := CallInfo{}
	if c.Type != "" || c.From != (common.Address{}) || c.To != (common.Address{}) {
		t.Errorf("unexpected non-zero defaults: %+v", c)
	}
}

func TestLogInfo_RoundTrip(t *testing.T) {
	in := LogInfo{Topics: []string{"0xa", "0xb"}, Data: "0xc"}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out LogInfo
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Data != "0xc" {
		t.Errorf("Data: got %q, want %q", out.Data, "0xc")
	}
}

func TestAccessInfo_RoundTrip(t *testing.T) {
	in := AccessInfo{
		Reads:  HexMap{"0x1": "0xff"},
		Writes: Counts{"0x1": 1},
	}
	b, _ := json.Marshal(in)
	var out AccessInfo
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Reads["0x1"] != "0xff" {
		t.Errorf("Reads[0x1]: got %q, want %q", out.Reads["0x1"], "0xff")
	}
	if out.Writes["0x1"] != 1 {
		t.Errorf("Writes[0x1]: got %v, want 1", out.Writes["0x1"])
	}
}
