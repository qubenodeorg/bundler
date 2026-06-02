package methods

import (
	"testing"
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
