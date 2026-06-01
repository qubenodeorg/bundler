package reverts

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestNewExecutionResult(t *testing.T) {
	uint256, _ := abi.NewType("uint256", "", nil)
	uint48, _ := abi.NewType("uint48", "", nil)
	boolean, _ := abi.NewType("bool", "", nil)
	bytes, _ := abi.NewType("bytes", "", nil)
	args := abi.Arguments{
		{Name: "preOpGas", Type: uint256},
		{Name: "paid", Type: uint256},
		{Name: "validAfter", Type: uint48},
		{Name: "validUntil", Type: uint48},
		{Name: "targetSuccess", Type: boolean},
		{Name: "targetResult", Type: bytes},
	}

	encoded, err := args.Pack(
		big.NewInt(50000),
		big.NewInt(100000),
		big.NewInt(0),
		big.NewInt(1000000),
		true,
		common.Hex2Bytes("deadbeef"),
	)
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}

	er := executionResult()
	fullData := append(er.ID[:4], encoded...)

	mockErr := &testDataError{
		data: "0x" + common.Bytes2Hex(fullData),
		msg:  "execution reverted",
	}

	result, err := NewExecutionResult(mockErr)
	if err != nil {
		t.Fatalf("NewExecutionResult failed: %v", err)
	}

	if result.PreOpGas.Cmp(big.NewInt(50000)) != 0 {
		t.Fatalf("got PreOpGas %s, want 50000", result.PreOpGas.String())
	}
	if result.Paid.Cmp(big.NewInt(100000)) != 0 {
		t.Fatalf("got Paid %s, want 100000", result.Paid.String())
	}
	if !result.TargetSuccess {
		t.Fatal("expected TargetSuccess=true")
	}
	if len(result.TargetResult) == 0 {
		t.Fatal("TargetResult should not be empty")
	}
}

func TestNewExecutionResult_NonDataError(t *testing.T) {
	_, err := NewExecutionResult(errors.New("test error"))
	if err == nil {
		t.Fatal("expected error for non-DataError")
	}
}

func TestNewExecutionResult_InvalidHex(t *testing.T) {
	mockErr := &testDataError{
		data: "invalid",
		msg:  "error",
	}
	_, err := NewExecutionResult(mockErr)
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}
