package relay

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/internal/logger"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/entrypoint/transaction"
	"github.com/zarbanio/bundler/pkg/modules"
	"github.com/zarbanio/bundler/pkg/userop"
)

func newRpcEth(t *testing.T) *ethclient.Client {
	t.Helper()
	n := testutils.RpcMock(testutils.MethodMocks{
		"eth_gasPrice":              "0x1",
		"eth_getTransactionCount":   "0x1",
		"eth_estimateGas":           "0x1",
		"eth_sendRawTransaction":    testutils.MockHash,
		"eth_getTransactionReceipt": testutils.NewTransactionReceiptMock(),
		"eth_blockNumber":           "0x1",
		"eth_getBlockByNumber":      testutils.NewBlockMock(),
	})
	t.Cleanup(n.Close)
	r, err := rpc.Dial(n.URL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	return ethclient.NewClient(r)
}

func TestRelayer_SendUserOperation_V06(t *testing.T) {
	eth := newRpcEth(t)
	l := logger.NewZeroLogr()
	r := New(testutils.DummyEOA, eth, testutils.ChainID, testutils.DummyEOA.Address, l)
	r.SetWaitTimeout(0)

	op := testutils.MockValidInitUserOp()
	ctx := modules.NewBatchHandlerContext(
		[]*userop.UserOperation{op},
		// Zero EntryPoint -> defaults to v0.6 routing.
		common.Address{},
		testutils.ChainID,
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
	)
	if err := r.SendUserOperation()(ctx); err != nil {
		t.Fatalf("relayer SendUserOperation: %v", err)
	}
	if ctx.Data["txn_hash"] == nil {
		t.Fatalf("expected txn_hash to be set")
	}
}

func TestRelayer_SendUserOperation_V07Routing(t *testing.T) {
	eth := newRpcEth(t)
	l := logger.NewZeroLogr()
	r := New(testutils.DummyEOA, eth, testutils.ChainID, testutils.DummyEOA.Address, l)
	r.SetWaitTimeout(0)

	op := testutils.MockValidInitUserOp()
	v07ep := common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032")
	ctx := modules.NewBatchHandlerContext(
		[]*userop.UserOperation{op},
		v07ep,
		testutils.ChainID,
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
	)
	if err := r.SendUserOperation()(ctx); err != nil {
		t.Fatalf("relayer SendUserOperation v0.7: %v", err)
	}
}

func TestRelayer_SendUserOperation_V08Routing(t *testing.T) {
	eth := newRpcEth(t)
	l := logger.NewZeroLogr()
	r := New(testutils.DummyEOA, eth, testutils.ChainID, testutils.DummyEOA.Address, l)
	r.SetWaitTimeout(0)

	op := testutils.MockValidInitUserOp()
	v08ep := common.HexToAddress("0x4337084D9E255Ff0702461CF8895CE9E3b5Ff108")
	ctx := modules.NewBatchHandlerContext(
		[]*userop.UserOperation{op},
		v08ep,
		testutils.ChainID,
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
	)
	if err := r.SendUserOperation()(ctx); err != nil {
		t.Fatalf("relayer SendUserOperation v0.8: %v", err)
	}
}

func TestRelayer_SendUserOperation_V09Routing(t *testing.T) {
	eth := newRpcEth(t)
	l := logger.NewZeroLogr()
	r := New(testutils.DummyEOA, eth, testutils.ChainID, testutils.DummyEOA.Address, l)
	r.SetWaitTimeout(0)

	op := testutils.MockValidInitUserOp()
	v09ep := common.HexToAddress("0x433709009B8330FDa32311DF1C2AFA402eD8D009")
	ctx := modules.NewBatchHandlerContext(
		[]*userop.UserOperation{op},
		v09ep,
		testutils.ChainID,
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
	)
	if err := r.SendUserOperation()(ctx); err != nil {
		t.Fatalf("relayer SendUserOperation v0.9: %v", err)
	}
}

func TestRelayer_EmptyBatch(t *testing.T) {
	eth := newRpcEth(t)
	l := logger.NewZeroLogr()
	r := New(testutils.DummyEOA, eth, testutils.ChainID, testutils.DummyEOA.Address, l)
	r.SetWaitTimeout(0)

	ctx := modules.NewBatchHandlerContext(
		nil,
		common.Address{},
		testutils.ChainID,
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
	)
	if err := r.SendUserOperation()(ctx); err != nil {
		t.Fatalf("relayer SendUserOperation (empty): %v", err)
	}
	if _, ok := ctx.Data["txn_hash"]; ok {
		t.Fatalf("did not expect txn_hash for empty batch")
	}
}

func TestRelayer_OptsConversions(t *testing.T) {
	base := &transaction.Opts{
		EOA:         testutils.DummyEOA,
		ChainID:     testutils.ChainID,
		EntryPoint:  testutils.ValidAddress1,
		Beneficiary: testutils.ValidAddress2,
		BaseFee:     big.NewInt(7),
		Tip:         big.NewInt(8),
		GasPrice:    big.NewInt(9),
		GasLimit:    1234,
		NoSend:      true,
	}
	if got := toV07Opts(base); got.EntryPoint != base.EntryPoint || got.GasLimit != base.GasLimit {
		t.Fatalf("toV07Opts mismatch")
	}
	if got := toV08Opts(base); got.EntryPoint != base.EntryPoint || got.Tip.Cmp(base.Tip) != 0 {
		t.Fatalf("toV08Opts mismatch")
	}
	if got := toV09Opts(base); got.Beneficiary != base.Beneficiary || got.GasPrice.Cmp(base.GasPrice) != 0 {
		t.Fatalf("toV09Opts mismatch")
	}
}
