package simulation

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/zarbanio/bundler/pkg/tracer"
)

// nativeFrameForOp builds a minimal erc7562Tracer handleOps frame whose
// top-level call targets the op's sender.
func nativeFrameForOp(sender common.Address, entryPoint common.Address, senderOpcodes map[string]uint64) *tracer.Erc7562Frame {
	return &tracer.Erc7562Frame{
		Type: "CALL",
		From: common.Address{},
		To:   entryPoint,
		Calls: []tracer.Erc7562Frame{
			{
				Type:        "CALL",
				From:        entryPoint,
				To:          sender,
				Input:       hexutil.MustDecode("0xb61d27f6"),
				UsedOpcodes: senderOpcodes,
			},
		},
	}
}

// TestNativeTraceAcceptedV09 is the golden acceptance path: a clean native
// trace adapted via FromErc7562 must pass ProcessTraceResult.
func TestNativeTraceAcceptedV09(t *testing.T) {
	op := validMockOpV09()
	entryPoint := common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009")
	frame := nativeFrameForOp(op.Sender, entryPoint, map[string]uint64{"0x54": 2})

	res, err := tracer.FromErc7562(frame, entryPoint, PaymasterSignatureHook)
	if err != nil {
		t.Fatalf("FromErc7562: %v", err)
	}
	if _, err := ProcessTraceResult(op, entryPoint, mockStakesV09(), res, common.Address{}); err != nil {
		t.Fatalf("ProcessTraceResult: %v", err)
	}
}

// TestNativeTraceBannedOpcodeV09 is the golden rejection path: a native trace
// recording a banned opcode (TIMESTAMP, byte 0x42) must reject after key
// normalization.
func TestNativeTraceBannedOpcodeV09(t *testing.T) {
	op := validMockOpV09()
	entryPoint := common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009")
	frame := nativeFrameForOp(op.Sender, entryPoint, map[string]uint64{"0x42": 1})

	res, err := tracer.FromErc7562(frame, entryPoint, PaymasterSignatureHook)
	if err != nil {
		t.Fatalf("FromErc7562: %v", err)
	}
	_, err = ProcessTraceResult(op, entryPoint, mockStakesV09(), res, common.Address{})
	if err == nil {
		t.Fatal("want banned opcode rejection, got nil")
	}
	if !strings.Contains(err.Error(), "TIMESTAMP") {
		t.Errorf("err = %v, want mention of TIMESTAMP", err)
	}
}

func buildValidatePaymasterInput(sigLen int) []byte {
	selector := hexutil.MustDecode(validatePaymasterUserOpSelector)
	payload := make([]byte, 544)
	// Word 12 (payload bytes 384..416): paymasterSignature offset relative to
	// the PackedUserOperation struct start (payload byte 96). 416 points the
	// length word at payload byte 512.
	payload[415] = 0xa0
	payload[414] = 0x01
	// Length word at payload bytes 512..544.
	payload[543] = byte(sigLen)
	return append(selector, payload...)
}

func TestPaymasterSignatureHookV09(t *testing.T) {
	entryPoint := common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009")
	paymaster := common.HexToAddress("0x2222222222222222222222222222222222222222")

	for _, tc := range []struct {
		name   string
		sigLen int
		want   bool
	}{
		{"with signature", 65, true},
		{"empty signature", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := &tracer.Erc7562Frame{
				Type: "CALL",
				To:   entryPoint,
				Calls: []tracer.Erc7562Frame{
					{
						Type:  "CALL",
						From:  entryPoint,
						To:    paymaster,
						Input: buildValidatePaymasterInput(tc.sigLen),
					},
				},
			}
			res, err := tracer.FromErc7562(frame, entryPoint, PaymasterSignatureHook)
			if err != nil {
				t.Fatalf("FromErc7562: %v", err)
			}
			if got := res.CallsFromEntryPoint[0].PaymasterSignature; got != tc.want {
				t.Errorf("PaymasterSignature = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPaymasterSignatureHookIgnoresOtherSelectors(t *testing.T) {
	entryPoint := common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009")
	frame := &tracer.Erc7562Frame{
		Type: "CALL",
		To:   entryPoint,
		Calls: []tracer.Erc7562Frame{
			{
				Type:  "CALL",
				From:  entryPoint,
				To:    common.HexToAddress("0x2222222222222222222222222222222222222222"),
				Input: buildValidatePaymasterInput(65)[4:], // strip selector
			},
		},
	}
	res, err := tracer.FromErc7562(frame, entryPoint, PaymasterSignatureHook)
	if err != nil {
		t.Fatalf("FromErc7562: %v", err)
	}
	if res.CallsFromEntryPoint[0].PaymasterSignature {
		t.Error("hook must only fire on the validatePaymasterUserOp selector")
	}
}
