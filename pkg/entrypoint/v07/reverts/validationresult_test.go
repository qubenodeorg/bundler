package reverts

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestNewValidationResult(t *testing.T) {
	returnInfo, _ := abi.NewType("tuple", "ReturnInfo", []abi.ArgumentMarshaling{
		{Name: "preOpGas", Type: "uint256"},
		{Name: "prefund", Type: "uint256"},
		{Name: "sigFailed", Type: "bool"},
		{Name: "validAfter", Type: "uint48"},
		{Name: "validUntil", Type: "uint48"},
		{Name: "paymasterContext", Type: "bytes"},
	})
	stakeInfo, _ := abi.NewType("tuple", "StakeInfo", []abi.ArgumentMarshaling{
		{Name: "stake", Type: "uint256"},
		{Name: "unstakeDelaySec", Type: "uint256"},
	})

	args := abi.Arguments{
		{Name: "returnInfo", Type: returnInfo},
		{Name: "senderInfo", Type: stakeInfo},
		{Name: "factoryInfo", Type: stakeInfo},
		{Name: "paymasterInfo", Type: stakeInfo},
	}

	encoded, err := args.Pack(
		struct {
			PreOpGas         *big.Int
			Prefund          *big.Int
			SigFailed        bool
			ValidAfter       *big.Int
			ValidUntil       *big.Int
			PaymasterContext []byte
		}{
			PreOpGas:         big.NewInt(50000),
			Prefund:          big.NewInt(100000),
			SigFailed:        false,
			ValidAfter:       big.NewInt(0),
			ValidUntil:       big.NewInt(1000000),
			PaymasterContext: []byte{},
		},
		struct {
			Stake           *big.Int
			UnstakeDelaySec *big.Int
		}{Stake: big.NewInt(1e18), UnstakeDelaySec: big.NewInt(86400)},
		struct {
			Stake           *big.Int
			UnstakeDelaySec *big.Int
		}{Stake: big.NewInt(0), UnstakeDelaySec: big.NewInt(0)},
		struct {
			Stake           *big.Int
			UnstakeDelaySec *big.Int
		}{Stake: big.NewInt(1e17), UnstakeDelaySec: big.NewInt(3600)},
	)
	if err != nil {
		t.Fatalf("pack failed: %v", err)
	}

	vr := validationResult()
	fullData := append(vr.ID[:4], encoded...)

	mockErr := &testDataError{
		data: "0x" + common.Bytes2Hex(fullData),
		msg:  "execution reverted",
	}

	result, err := NewValidationResult(mockErr)
	if err != nil {
		t.Fatalf("NewValidationResult failed: %v", err)
	}

	if result.ReturnInfo == nil {
		t.Fatal("ReturnInfo should not be nil")
	}
	if result.ReturnInfo.PreOpGas.Cmp(big.NewInt(50000)) != 0 {
		t.Fatalf("got PreOpGas %s, want 50000", result.ReturnInfo.PreOpGas.String())
	}
	if result.ReturnInfo.Prefund.Cmp(big.NewInt(100000)) != 0 {
		t.Fatalf("got Prefund %s, want 100000", result.ReturnInfo.Prefund.String())
	}

	if result.SenderInfo == nil {
		t.Fatal("SenderInfo should not be nil")
	}
	if result.SenderInfo.Stake.Cmp(big.NewInt(1e18)) != 0 {
		t.Fatalf("got SenderInfo.Stake %s, want 1000000000000000000", result.SenderInfo.Stake.String())
	}

	if result.FactoryInfo == nil {
		t.Fatal("FactoryInfo should not be nil")
	}
	if result.PaymasterInfo == nil {
		t.Fatal("PaymasterInfo should not be nil")
	}
}

func TestNewValidationResult_NonDataError(t *testing.T) {
	_, err := NewValidationResult(errors.New("test error"))
	if err == nil {
		t.Fatal("expected error for non-DataError")
	}
}
