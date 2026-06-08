package filter

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestEIP7702AccountInitializedEvent_Fields(t *testing.T) {
	event := EIP7702AccountInitializedEvent{
		UserOpHash: common.HexToHash("0x1234"),
		Sender:     common.HexToAddress("0xabcd"),
		Delegate:   common.HexToAddress("0xef01"),
	}

	if event.UserOpHash == (common.Hash{}) {
		t.Error("UserOpHash should not be empty")
	}
	if event.Sender == (common.Address{}) {
		t.Error("Sender should not be empty")
	}
	if event.Delegate == (common.Address{}) {
		t.Error("Delegate should not be empty")
	}
}

func TestFilterEIP7702AccountInitialized_NilClient(t *testing.T) {
	_, err := FilterEIP7702AccountInitialized(nil, "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", common.HexToAddress("0x433709009b8330fda32311df1c2afa402ed8d009"), 100)
	if err == nil {
		t.Error("expected error for nil client, got nil")
	}
}

func TestFilterEIP7702AccountInitialized_InvalidHash(t *testing.T) {
	_, err := FilterEIP7702AccountInitialized(nil, "not-a-hash", common.HexToAddress("0x433709009b8330fda32311df1c2afa402ed8d009"), 100)
	if err == nil {
		t.Error("expected error for invalid hash, got nil")
	}
}
