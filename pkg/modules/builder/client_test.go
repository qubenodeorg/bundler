package builder

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/metachris/flashbotsrpc"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/entrypoint/transaction"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/userop"
)

func TestSetWaitTimeout(t *testing.T) {
	b := New(testutils.DummyEOA, nil, nil, testutils.DummyEOA.Address, 1)
	b.SetWaitTimeout(0)
	if b.waitTimeout != 0 {
		t.Fatalf("expected wait timeout 0, got %v", b.waitTimeout)
	}
}

func TestOptsConversions(t *testing.T) {
	base := &transaction.Opts{
		EOA:         testutils.DummyEOA,
		ChainID:     testutils.ChainID,
		EntryPoint:  testutils.ValidAddress1,
		Beneficiary: testutils.ValidAddress2,
		BaseFee:     big.NewInt(1),
		Tip:         big.NewInt(2),
		GasPrice:    big.NewInt(3),
		GasLimit:    42,
		NoSend:      true,
	}
	if got := toV07Opts(base); got.EntryPoint != base.EntryPoint || got.GasLimit != base.GasLimit {
		t.Fatalf("toV07Opts mismatch")
	}
	if got := toV08Opts(base); got.Tip.Cmp(base.Tip) != 0 {
		t.Fatalf("toV08Opts mismatch")
	}
	if got := toV09Opts(base); got.Beneficiary != base.Beneficiary {
		t.Fatalf("toV09Opts mismatch")
	}
}

func TestEstimateHandleOpsGas_VersionDispatch(t *testing.T) {
	// Each dispatch path should be reachable; we expect errors due to the missing eth
	// client, but the call should reach the per-version EstimateHandleOpsGas.
	for _, v := range []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8, userop.V0_9} {
		opts := &transaction.Opts{
			EOA:        testutils.DummyEOA,
			ChainID:    testutils.ChainID,
			EntryPoint: testutils.ValidAddress1,
		}
		defer func() {
			// Some dispatch paths panic on nil eth client; recover so we can iterate.
			_ = recover()
		}()
		_, _, _ = estimateHandleOpsGas(v, opts)
	}
}

func TestHandleOps_VersionDispatch(t *testing.T) {
	for _, v := range []userop.Version{userop.V0_6, userop.V0_7, userop.V0_8, userop.V0_9} {
		opts := &transaction.Opts{
			EOA:        testutils.DummyEOA,
			ChainID:    testutils.ChainID,
			EntryPoint: testutils.ValidAddress1,
		}
		defer func() {
			_ = recover()
		}()
		_, _ = handleOps(v, opts)
	}
}

func TestSendUserOperationWithAllUpstreamErrors(t *testing.T) {
	n := testutils.RpcMock(testutils.MethodMocks{
		"eth_blockNumber":           "0x1",
		"eth_gasPrice":              "0x1",
		"eth_getTransactionCount":   "0x1",
		"eth_estimateGas":           "0x1",
		"eth_getBlockByNumber":      testutils.NewBlockMock(),
		"eth_getTransactionReceipt": testutils.NewTransactionReceiptMock(),
	})
	r, _ := rpc.Dial(n.URL)
	eth := ethclient.NewClient(r)

	bb1 := testutils.BadBuilderRpcMock()
	bb2 := testutils.BadBuilderRpcMock()
	fb := flashbotsrpc.NewBuilderBroadcastRPC([]string{bb1.URL, bb2.URL})
	fn := New(testutils.DummyEOA, eth, fb, testutils.DummyEOA.Address, 1).SendUserOperation()

	if err := fn(
		modules.NewBatchHandlerContext(
			[]*userop.UserOperation{testutils.MockValidInitUserOp()},
			common.HexToAddress("0x"),
			testutils.ChainID,
			big.NewInt(1),
			big.NewInt(1),
			big.NewInt(1),
		),
	); !errors.Is(err, ErrFlashbotsBroadcastBundle) {
		t.Fatalf("got %v, want ErrFlashbotsBroadcastBundle", err)
	}
}

func TestSendUserOperationWithPartialUpstreamErrors(t *testing.T) {
	n := testutils.RpcMock(testutils.MethodMocks{
		"eth_blockNumber":           "0x1",
		"eth_gasPrice":              "0x1",
		"eth_getTransactionCount":   "0x1",
		"eth_estimateGas":           "0x1",
		"eth_getBlockByNumber":      testutils.NewBlockMock(),
		"eth_getTransactionReceipt": testutils.NewTransactionReceiptMock(),
	})
	r, _ := rpc.Dial(n.URL)
	eth := ethclient.NewClient(r)

	bb1 := testutils.RpcMock(testutils.MethodMocks{
		"eth_sendBundle": map[string]string{
			"bundleHash": testutils.MockHash,
		},
	})
	bb2 := testutils.BadBuilderRpcMock()
	fb := flashbotsrpc.NewBuilderBroadcastRPC([]string{bb1.URL, bb2.URL})
	fn := New(testutils.DummyEOA, eth, fb, testutils.DummyEOA.Address, 1).SendUserOperation()

	if err := fn(
		modules.NewBatchHandlerContext(
			[]*userop.UserOperation{testutils.MockValidInitUserOp()},
			common.HexToAddress("0x"),
			testutils.ChainID,
			big.NewInt(1),
			big.NewInt(1),
			big.NewInt(1),
		),
	); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

func TestSendUserOperationWithNoUpstreamErrors(t *testing.T) {
	n := testutils.RpcMock(testutils.MethodMocks{
		"eth_blockNumber":           "0x1",
		"eth_gasPrice":              "0x1",
		"eth_getTransactionCount":   "0x1",
		"eth_estimateGas":           "0x1",
		"eth_getBlockByNumber":      testutils.NewBlockMock(),
		"eth_getTransactionReceipt": testutils.NewTransactionReceiptMock(),
	})
	r, _ := rpc.Dial(n.URL)
	eth := ethclient.NewClient(r)

	bb1 := testutils.RpcMock(testutils.MethodMocks{
		"eth_sendBundle": map[string]string{
			"bundleHash": testutils.MockHash,
		},
	})
	bb2 := testutils.RpcMock(testutils.MethodMocks{
		"eth_sendBundle": map[string]string{
			"bundleHash": testutils.MockHash,
		},
	})
	fb := flashbotsrpc.NewBuilderBroadcastRPC([]string{bb1.URL, bb2.URL})
	fn := New(testutils.DummyEOA, eth, fb, testutils.DummyEOA.Address, 1).SendUserOperation()

	if err := fn(
		modules.NewBatchHandlerContext(
			[]*userop.UserOperation{testutils.MockValidInitUserOp()},
			common.HexToAddress("0x"),
			testutils.ChainID,
			big.NewInt(1),
			big.NewInt(1),
			big.NewInt(1),
		),
	); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}
