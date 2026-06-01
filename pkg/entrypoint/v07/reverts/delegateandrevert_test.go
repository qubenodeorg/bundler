package reverts

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestNewDelegateAndRevert(t *testing.T) {
	boolT, _ := abi.NewType("bool", "", nil)
	bytesT, _ := abi.NewType("bytes", "", nil)
	args := abi.Arguments{
		{Name: "success", Type: boolT},
		{Name: "ret", Type: bytesT},
	}

	encoded, err := args.Pack(true, common.Hex2Bytes("00010203"))
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}

	dar := delegateAndRevert()
	fullData := append(dar.ID[:4], encoded...)

	mockErr := &testDataError{
		data: "0x" + common.Bytes2Hex(fullData),
		msg:  "execution reverted",
	}

	result, err := NewDelegateAndRevert(mockErr)
	if err != nil {
		t.Fatalf("NewDelegateAndRevert failed: %v", err)
	}

	if !result.Success {
		t.Fatal("expected success=true")
	}
	if len(result.Ret) == 0 {
		t.Fatal("ret should not be empty")
	}
}

func TestNewDelegateAndRevert_Failure(t *testing.T) {
	boolT, _ := abi.NewType("bool", "", nil)
	bytesT, _ := abi.NewType("bytes", "", nil)
	args := abi.Arguments{
		{Name: "success", Type: boolT},
		{Name: "ret", Type: bytesT},
	}

	encoded, err := args.Pack(false, []byte{})
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}

	dar := delegateAndRevert()
	fullData := append(dar.ID[:4], encoded...)

	mockErr := &testDataError{
		data: "0x" + common.Bytes2Hex(fullData),
		msg:  "execution reverted",
	}

	result, err := NewDelegateAndRevert(mockErr)
	if err != nil {
		t.Fatalf("NewDelegateAndRevert failed: %v", err)
	}

	if result.Success {
		t.Fatal("expected success=false")
	}
}

func TestNewDelegateAndRevert_NonDataError(t *testing.T) {
	_, err := NewDelegateAndRevert(errors.New("test error"))
	if err == nil {
		t.Fatal("expected error for non-DataError")
	}
}
