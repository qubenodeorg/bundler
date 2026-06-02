package simulation

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestBannedOpCodesContainsTSTORE(t *testing.T) {
	if !BannedOpCodeSet.Contains("TSTORE") {
		t.Fatal("BannedOpCodeSet should contain TSTORE (OP-070)")
	}
}

func TestBannedOpCodesContainsTLOAD(t *testing.T) {
	if !BannedOpCodeSet.Contains("TLOAD") {
		t.Fatal("BannedOpCodeSet should contain TLOAD (OP-070)")
	}
}

func TestBannedOpCodesHelper(t *testing.T) {
	s := BannedOpCodes()
	if !s.Contains("TSTORE") {
		t.Fatal("BannedOpCodes() should include TSTORE")
	}
}

func TestBannedUnstakedOpCodesHelper(t *testing.T) {
	s := BannedUnstakedOpCodes()
	if !s.Contains("SELFBALANCE") {
		t.Fatal("BannedUnstakedOpCodes() should include SELFBALANCE")
	}
}

func TestCreate2OpCode(t *testing.T) {
	if Create2OpCode != "CREATE2" {
		t.Fatalf("Create2OpCode: got %s, want CREATE2", Create2OpCode)
	}
}

func TestRip7212PrecompileNotZero(t *testing.T) {
	if Rip7212Precompile == (common.Address{}) {
		t.Fatal("Rip7212Precompile should not be zero")
	}
}

func TestRevertOpCode(t *testing.T) {
	if RevertOpCode != "REVERT" {
		t.Fatalf("RevertOpCode: got %s, want REVERT", RevertOpCode)
	}
}

func TestReturnOpCode(t *testing.T) {
	if ReturnOpCode != "RETURN" {
		t.Fatalf("ReturnOpCode: got %s, want RETURN", ReturnOpCode)
	}
}

func TestEntityStakesType(t *testing.T) {
	stakes := EntityStakes{
		common.HexToAddress("0x1234"): {Deposit: big.NewInt(100), Stake: big.NewInt(50)},
	}
	if len(stakes) != 1 {
		t.Fatalf("EntityStakes: got len %d, want 1", len(stakes))
	}
	if stakes[common.HexToAddress("0x1234")].Deposit.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("EntityStakes: got deposit %d, want 100", stakes[common.HexToAddress("0x1234")].Deposit)
	}
}
