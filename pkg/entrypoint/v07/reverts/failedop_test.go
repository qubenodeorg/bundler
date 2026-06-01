package reverts

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type testDataError struct {
	data interface{}
	msg  string
}

func (e *testDataError) Error() string        { return e.msg }
func (e *testDataError) ErrorData() interface{} { return e.data }

func TestNewFailedOpWithRevert(t *testing.T) {
	uint256, _ := abi.NewType("uint256", "", nil)
	stringT, _ := abi.NewType("string", "", nil)
	bytesT, _ := abi.NewType("bytes", "", nil)
	args := abi.Arguments{
		{Name: "opIndex", Type: uint256},
		{Name: "reason", Type: stringT},
		{Name: "inner", Type: bytesT},
	}

	encoded, err := args.Pack(big.NewInt(0), "AA20: test error", common.Hex2Bytes("deadbeef"))
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}

	fop := failedOpWithRevert()
	fullData := append(fop.ID[:4], encoded...)

	mockErr := &testDataError{
		data: "0x" + common.Bytes2Hex(fullData),
		msg:  "execution reverted",
	}

	result, err := NewFailedOpWithRevert(mockErr)
	if err != nil {
		t.Fatalf("NewFailedOpWithRevert failed: %v", err)
	}

	if result.OpIndex != 0 {
		t.Fatalf("got opIndex %d, want 0", result.OpIndex)
	}
	if result.Reason != "AA20: test error" {
		t.Fatalf("got reason %q, want %q", result.Reason, "AA20: test error")
	}
	if len(result.Inner) == 0 || result.Inner[0] != 0xde {
		t.Fatalf("got inner %x, want deadbeef", result.Inner)
	}
}

func TestNewFailedOpWithRevert_InvalidData(t *testing.T) {
	mockErr := &testDataError{
		data: "0xinvalid",
		msg:  "error",
	}
	_, err := NewFailedOpWithRevert(mockErr)
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestNewFailedOpWithRevert_WrongSelector(t *testing.T) {
	mockErr := &testDataError{
		data: "0xcafecafe00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000a414132303a20746573740000000000000000000000000000000000000000000000",
		msg:  "execution reverted",
	}
	_, err := NewFailedOpWithRevert(mockErr)
	if err == nil {
		t.Fatal("expected error for wrong selector")
	}
}

func TestNewFailedOpWithRevert_NonDataError(t *testing.T) {
	_, err := NewFailedOpWithRevert(errors.New("test error"))
	if err == nil {
		t.Fatal("expected error for non-DataError")
	}
}
