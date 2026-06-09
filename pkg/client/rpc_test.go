package client

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/bundler"
	"github.com/zarbanio/bundler/pkg/modules/entities"
)

func newRpcAdapter(t *testing.T) *RpcAdapter {
	t.Helper()
	c, mem := newTestClient(t)
	n := testutils.RpcMock(testutils.MethodMocks{
		"eth_call":             "0x",
		"eth_getBlockByNumber": testutils.NewBlockMock(),
	})
	t.Cleanup(n.Close)
	r, _ := rpc.Dial(n.URL)
	eth := ethclient.NewClient(r)

	db := testutils.DBMock()
	t.Cleanup(func() { _ = db.Close() })
	rep := entities.New(db, eth, nil)
	bun := bundler.New(mem, testutils.ChainID, c.supportedEntryPoints)
	dbg := NewDebug(testutils.DummyEOA, eth, mem, rep, bun, testutils.ChainID, c.supportedEntryPoints[0], testutils.DummyEOA.Address)
	return NewRpcAdapter(c, dbg, r)
}

func TestRpcAdapter_DelegationCoverage(t *testing.T) {
	ra := newRpcAdapter(t)

	if eps, err := ra.Eth_supportedEntryPoints(); err != nil || len(eps) == 0 {
		t.Fatalf("Eth_supportedEntryPoints: %v %v", eps, err)
	}
	if id, err := ra.Eth_chainId(); err != nil || id == "" {
		t.Fatalf("Eth_chainId: %v %v", id, err)
	}

	if _, err := ra.Eth_getUserOperationReceipt(testutils.MockHash); err != nil {
		t.Fatalf("Eth_getUserOperationReceipt: %v", err)
	}
	if _, err := ra.Eth_getUserOperationByHash(testutils.MockHash); err != nil {
		t.Fatalf("Eth_getUserOperationByHash: %v", err)
	}

	if _, err := ra.Eth_sendUserOperation(testutils.MockUserOpData, v06EP); err != nil {
		t.Fatalf("Eth_sendUserOperation: %v", err)
	}

	if _, err := ra.Eth_estimateUserOperationGas(testutils.MockUserOpData, v06EP, nil); err != nil {
		t.Fatalf("Eth_estimateUserOperationGas: %v", err)
	}

	if _, err := ra.Pm_accounts(v06EP); err != nil {
		t.Fatalf("Pm_accounts: %v", err)
	}
	if _, err := ra.Pm_sponsorUserOperation(testutils.MockUserOpData, v06EP, map[string]any{"type": "unknown"}); err == nil {
		t.Fatalf("Pm_sponsorUserOperation: expected error")
	}
}

func TestRpcAdapter_DebugDelegation(t *testing.T) {
	ra := newRpcAdapter(t)

	if _, err := ra.Debug_bundler_clearState(); err != nil {
		t.Fatalf("Debug_bundler_clearState: %v", err)
	}
	if _, err := ra.Debug_bundler_dumpMempool(v06EP); err != nil {
		t.Fatalf("Debug_bundler_dumpMempool: %v", err)
	}
	if _, err := ra.Debug_bundler_setBundlingMode("manual"); err != nil {
		t.Fatalf("Debug_bundler_setBundlingMode: %v", err)
	}
	if _, err := ra.Debug_bundler_setBundlingMode("invalid"); err == nil {
		t.Fatalf("Debug_bundler_setBundlingMode: expected error")
	}
	if _, err := ra.Debug_bundler_setReputation([]any{}, v06EP); err != nil {
		t.Fatalf("Debug_bundler_setReputation: %v", err)
	}
	if _, err := ra.Debug_bundler_dumpReputation(v06EP); err != nil {
		t.Fatalf("Debug_bundler_dumpReputation: %v", err)
	}
	// SendBundleNow on an empty mempool returns "" without error.
	if h, err := ra.Debug_bundler_sendBundleNow(); err != nil {
		t.Fatalf("Debug_bundler_sendBundleNow: %v (hash: %s)", err, h)
	}
}

func TestRpcAdapter_DebugDisabled(t *testing.T) {
	c, _ := newTestClient(t)
	n := testutils.RpcMock(testutils.MethodMocks{})
	defer n.Close()
	r, _ := rpc.Dial(n.URL)
	ra := NewRpcAdapter(c, nil, r)
	if _, err := ra.Debug_bundler_clearState(); err == nil {
		t.Fatal("expected disabled error")
	}
	if _, err := ra.Debug_bundler_dumpMempool(v06EP); err == nil {
		t.Fatal("expected disabled error")
	}
	if _, err := ra.Debug_bundler_sendBundleNow(); err == nil {
		t.Fatal("expected disabled error")
	}
	if _, err := ra.Debug_bundler_setBundlingMode("manual"); err == nil {
		t.Fatal("expected disabled error")
	}
	if _, err := ra.Debug_bundler_setReputation(nil, v06EP); err == nil {
		t.Fatal("expected disabled error")
	}
	if _, err := ra.Debug_bundler_dumpReputation(v06EP); err == nil {
		t.Fatal("expected disabled error")
	}
}

func TestClient_UseLoggerUseModules(t *testing.T) {
	c, _ := newTestClient(t)
	c.UseLogger(c.logger)
	c.UseModules()
	// No assertion - we just need to exercise these helpers for coverage.
	_ = big.NewInt(1)
}
