package methods

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestHandleOpsSelector(t *testing.T) {
	if HandleOpsSelector == "" {
		t.Fatal("HandleOpsSelector should not be empty")
	}
	if len(HandleOpsSelector) != 10 {
		t.Fatalf("HandleOpsSelector: got len %d, want 10", len(HandleOpsSelector))
	}
}

func TestHandleOpsMethodName(t *testing.T) {
	if HandleOpsMethod.Name != "handleOps" {
		t.Fatalf("HandleOpsMethod.Name: got %s, want handleOps", HandleOpsMethod.Name)
	}
}

func TestPackedUserOpArrType(t *testing.T) {
	if packedUserOpArr.T != abi.SliceTy {
		t.Fatalf("packedUserOpArr: got type %v, want slice type (tuple[])", packedUserOpArr.T)
	}
	if packedUserOpArr.Elem.T != abi.TupleTy {
		t.Fatalf("packedUserOpArr.Elem: got type %v, want tuple type", packedUserOpArr.Elem.T)
	}
}

func TestAddressType(t *testing.T) {
	if address.String() != "address" {
		t.Fatalf("address: got %s, want address", address.String())
	}
}
