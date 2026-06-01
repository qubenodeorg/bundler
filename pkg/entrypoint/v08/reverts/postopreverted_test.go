package reverts

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestNewPostOpReverted(t *testing.T) {
	bytesT, _ := abi.NewType("bytes", "", nil)
	args := abi.Arguments{
		{Name: "returnData", Type: bytesT},
	}

	encoded, err := args.Pack(common.Hex2Bytes("cafebabe"))
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}

	por := postOpReverted()
	fullData := append(por.ID[:4], encoded...)

	mockErr := &testDataError{
		data: "0x" + common.Bytes2Hex(fullData),
		msg:  "execution reverted",
	}

	result, err := NewPostOpReverted(mockErr)
	if err != nil {
		t.Fatalf("NewPostOpReverted failed: %v", err)
	}

	if len(result.ReturnData) == 0 {
		t.Fatal("returnData should not be empty")
	}
}

func TestNewPostOpReverted_NonDataError(t *testing.T) {
	_, err := NewPostOpReverted(errors.New("test error"))
	if err == nil {
		t.Fatal("expected error for non-DataError")
	}
}

func TestNewPostOpReverted_InvalidHex(t *testing.T) {
	mockErr := &testDataError{
		data: "nothex",
		msg:  "error",
	}
	_, err := NewPostOpReverted(mockErr)
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}
