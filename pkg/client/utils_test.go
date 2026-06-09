package client

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/entrypoint/filter"
	v07filter "github.com/zarbanio/bundler/pkg/entrypoint/v07/filter"
	v08filter "github.com/zarbanio/bundler/pkg/entrypoint/v08/filter"
	v09filter "github.com/zarbanio/bundler/pkg/entrypoint/v09/filter"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestConvertVxReceipt_Nil(t *testing.T) {
	if convertV07Receipt(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
	if convertV08Receipt(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
	if convertV09Receipt(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestConvertVxReceipt_NilInnerReceipt(t *testing.T) {
	// Inner Receipt is nil — converter must not panic.
	in := &v07filter.UserOperationReceipt{Sender: testutils.ValidAddress1, Nonce: "0x1"}
	out := convertV07Receipt(in)
	if out == nil || out.Receipt != nil {
		t.Fatalf("expected non-nil outer with nil inner receipt, got %+v", out)
	}
}

func TestOpToMap_TypedNil(t *testing.T) {
	// A typed-nil *userop.UserOperation wrapped in the generic constraint must not panic.
	var typedNil *filter.UserOperationReceipt = nil
	_ = typedNil // sanity reference
	// We test the actual generic helper via a stub that satisfies the constraint.
	if _, err := opToMap[*userop.UserOperation](nil); err != nil {
		t.Fatalf("opToMap on typed nil should not error: %v", err)
	}
}

func TestIsTypedNil(t *testing.T) {
	if !isTypedNil(nil) {
		t.Fatal("expected untyped nil to be considered nil")
	}
	var p *userop.UserOperation
	if !isTypedNil(p) {
		t.Fatal("expected typed nil pointer to be considered nil")
	}
	if isTypedNil(&userop.UserOperation{}) {
		t.Fatal("expected non-nil pointer to not be considered nil")
	}
}

func TestReEncodeReceipt(t *testing.T) {
	in := &v07filter.UserOperationReceipt{
		Sender:        testutils.ValidAddress1,
		Paymaster:     testutils.ValidAddress2,
		Nonce:         "0x1",
		ActualGasCost: "0x2",
		ActualGasUsed: "0x3",
		Success:       true,
		From:          testutils.ValidAddress3,
	}
	out := convertV07Receipt(in)
	if out == nil {
		t.Fatal("expected non-nil output")
	}
	if out.Sender != in.Sender {
		t.Fatalf("sender mismatch")
	}
	if out.Nonce != in.Nonce {
		t.Fatalf("nonce mismatch")
	}

	v08 := &v08filter.UserOperationReceipt{Sender: testutils.ValidAddress1, Nonce: "0x4"}
	if got := convertV08Receipt(v08); got == nil || got.Nonce != "0x4" {
		t.Fatalf("v08 convert failed")
	}
	v09 := &v09filter.UserOperationReceipt{Sender: testutils.ValidAddress1, Nonce: "0x5"}
	if got := convertV09Receipt(v09); got == nil || got.Nonce != "0x5" {
		t.Fatalf("v09 convert failed")
	}
}

func TestGetUserOpReceiptNoop(t *testing.T) {
	fn := getUserOpReceiptNoop()
	r, err := fn("hash", common.Address{}, 10)
	if err != nil || r != nil {
		t.Fatalf("noop should return nil,nil; got %v, %v", r, err)
	}
}

func TestGetGasEstimateNoop(t *testing.T) {
	fn := getGasEstimateNoop()
	v, c, err := fn(common.Address{}, nil, "", nil)
	if err != nil || v != 0 || c != 0 {
		t.Fatalf("noop should return zero values, got %d/%d/%v", v, c, err)
	}
}

func TestGetUserOpByHashNoop(t *testing.T) {
	fn := getUserOpByHashNoop()
	r, err := fn("hash", common.Address{}, nil, 10)
	if err != nil || r != nil {
		t.Fatalf("noop should return nil,nil; got %v, %v", r, err)
	}
}

func TestGetGasPricesNoop(t *testing.T) {
	fn := getGasPricesNoop()
	gp, err := fn()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if gp.MaxFeePerGas.Sign() != 0 || gp.MaxPriorityFeePerGas.Sign() != 0 {
		t.Fatalf("expected zero gas prices")
	}
}

func TestGetUserOpReceiptWithEthClient_InvalidHash(t *testing.T) {
	// Ensure routing across versions reaches the per-version filter with an invalid hash,
	// which should surface a "Missing/invalid userOpHash" error from each filter package.
	n := testutils.RpcMock(testutils.MethodMocks{})
	defer n.Close()
	r, err := rpc.Dial(n.URL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	eth := ethclient.NewClient(r)
	fn := GetUserOpReceiptWithEthClient(eth)

	for _, ep := range []common.Address{
		common.HexToAddress(v06EP),
		common.HexToAddress(v07EP),
		common.HexToAddress(v08EP),
		common.HexToAddress(v09EP),
	} {
		if _, err := fn("not-a-hash", ep, 10); err == nil {
			t.Fatalf("expected invalid hash error for ep %s", ep.Hex())
		}
	}
}

func TestGetUserOpByHashWithEthClient_InvalidHash(t *testing.T) {
	n := testutils.RpcMock(testutils.MethodMocks{})
	defer n.Close()
	r, err := rpc.Dial(n.URL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	eth := ethclient.NewClient(r)
	fn := GetUserOpByHashWithEthClient(eth)

	for _, ep := range []common.Address{
		common.HexToAddress(v06EP),
		common.HexToAddress(v07EP),
		common.HexToAddress(v08EP),
		common.HexToAddress(v09EP),
	} {
		if _, err := fn("not-a-hash", ep, testutils.ChainID, 10); err == nil {
			t.Fatalf("expected invalid hash error for ep %s", ep.Hex())
		}
	}
}

func TestGetGasEstimateWithEthClient_ConstructsFn(t *testing.T) {
	n := testutils.RpcMock(testutils.MethodMocks{})
	defer n.Close()
	r, err := rpc.Dial(n.URL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	fn := GetGasEstimateWithEthClient(r, nil, testutils.ChainID, nil, "callTracer")
	if fn == nil {
		t.Fatal("expected non-nil GetGasEstimateFunc")
	}
}

func TestGetGasPricesWithEthClient_ConstructsFn(t *testing.T) {
	n := testutils.RpcMock(testutils.MethodMocks{})
	defer n.Close()
	r, err := rpc.Dial(n.URL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	eth := ethclient.NewClient(r)
	fn := GetGasPricesWithEthClient(eth)
	if fn == nil {
		t.Fatal("expected non-nil GetGasPricesFunc")
	}
}

// Ensure the filter type alias compiles in the new world.
var _ *filter.UserOperationReceipt = (*filter.UserOperationReceipt)(nil)
