package checks

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestValidateERC7562Rules_NilOp(t *testing.T) {
	// Should not panic with nil op
	if err := ValidateERC7562Rules(nil, common.Address{}); err != nil {
		t.Errorf("expected nil for placeholder implementation, got %v", err)
	}
}

func TestValidateERC7562Rules_ValidOp(t *testing.T) {
	op := &mockAuthOp{sender: common.HexToAddress("0x1234")}
	if err := ValidateERC7562Rules(op, common.HexToAddress("0xabcd")); err != nil {
		t.Errorf("expected nil for placeholder implementation, got %v", err)
	}
}

func TestValidateOP051ExtCodeSize_EntryPoint(t *testing.T) {
	ep := common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789")
	err := ValidateOP051ExtCodeSize("account", ep, ep)
	if err == nil {
		t.Fatal("expected error for EXTCODESIZE on EntryPoint, got nil")
	}
	if !strings.Contains(err.Error(), "OP-051") {
		t.Errorf("expected error to contain 'OP-051', got: %v", err)
	}
}

func TestValidateOP051ExtCodeSize_OtherAddress(t *testing.T) {
	ep := common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789")
	other := common.HexToAddress("0x1234567890123456789012345678901234567890")
	if err := ValidateOP051ExtCodeSize("account", other, ep); err != nil {
		t.Errorf("expected nil for EXTCODESIZE on non-EntryPoint, got %v", err)
	}
}

func TestValidateSTO033StakedStorageRead(t *testing.T) {
	if err := ValidateSTO033StakedStorageRead("factory"); err != nil {
		t.Errorf("expected nil for placeholder, got %v", err)
	}
}

func TestValidateEREP020StakedFactory_ZeroAddress(t *testing.T) {
	if err := ValidateEREP020StakedFactory(common.Address{}, false); err != nil {
		t.Errorf("expected nil for zero-address factory, got %v", err)
	}
}

func TestValidateEREP020StakedFactory_Unstaked(t *testing.T) {
	factory := common.HexToAddress("0x1234567890123456789012345678901234567890")
	err := ValidateEREP020StakedFactory(factory, false)
	if err == nil {
		t.Fatal("expected error for unstaked factory, got nil")
	}
	if !strings.Contains(err.Error(), "EREP-020") {
		t.Errorf("expected error to contain 'EREP-020', got: %v", err)
	}
}

func TestValidateEREP020StakedFactory_Staked(t *testing.T) {
	factory := common.HexToAddress("0x1234567890123456789012345678901234567890")
	if err := ValidateEREP020StakedFactory(factory, true); err != nil {
		t.Errorf("expected nil for staked factory, got %v", err)
	}
}
