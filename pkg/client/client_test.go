package client

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/entrypoint/filter"
	"github.com/zarbanio/bundler/pkg/entrypoint/stake"
	"github.com/zarbanio/bundler/pkg/fees"
	"github.com/zarbanio/bundler/pkg/gas"
	"github.com/zarbanio/bundler/pkg/handlers"
	"github.com/zarbanio/bundler/pkg/mempool"
	"github.com/zarbanio/bundler/pkg/state"
	"github.com/zarbanio/bundler/pkg/userop"
)

const (
	v06EP = "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"
	v07EP = "0x0000000071727De22E5E9d8BAf0edAc6f37da032"
	v08EP = "0x4337084D9E255Ff0702461CF8895CE9E3b5Ff108"
	v09EP = "0x433709009B8330FDa32311DF1C2AFA402eD8D009"
)

func newTestClient(t *testing.T, eps ...common.Address) (*Client, *mempool.Mempool) {
	t.Helper()
	db := testutils.DBMock()
	t.Cleanup(func() { _ = db.Close() })
	mem, err := mempool.New(db)
	if err != nil {
		t.Fatalf("mempool.New: %v", err)
	}

	if len(eps) == 0 {
		eps = []common.Address{common.HexToAddress(v06EP)}
	}

	n := testutils.RpcMock(testutils.MethodMocks{})
	t.Cleanup(n.Close)
	r, err := rpc.Dial(n.URL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	eth := ethclient.NewClient(r)

	ov := gas.NewDefaultOverhead()
	c := New(mem, ov, testutils.ChainID, eps, 100, testutils.DummyEOA, eth, r)
	c.SetGetGasPricesFunc(func() (*fees.GasPrices, error) {
		return &fees.GasPrices{MaxFeePerGas: big.NewInt(1), MaxPriorityFeePerGas: big.NewInt(1)}, nil
	})
	c.SetGetGasEstimateFunc(func(
		ep common.Address,
		op userop.UserOperationInterface,
		version userop.Version,
		sos state.OverrideSet,
	) (uint64, uint64, error) {
		return 100000, 50000, nil
	})
	c.SetGetStakeFunc(stake.GetStakeFuncNoop())
	return c, mem
}

func TestClient_SupportedEntryPoints(t *testing.T) {
	c, _ := newTestClient(t)
	eps, err := c.SupportedEntryPoints()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(eps) != 1 {
		t.Fatalf("expected 1 ep, got %d", len(eps))
	}
}

func TestClient_ChainID(t *testing.T) {
	c, _ := newTestClient(t)
	id, err := c.ChainID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != hexutil.EncodeBig(testutils.ChainID) {
		t.Fatalf("chainID: got %s want %s", id, hexutil.EncodeBig(testutils.ChainID))
	}
}

func TestClient_ParseEntryPointAddress(t *testing.T) {
	c, _ := newTestClient(t)
	if _, err := c.parseEntryPointAddress("0x0000000000000000000000000000000000000000"); err == nil {
		t.Fatal("expected error for unsupported entrypoint")
	}
	if _, err := c.parseEntryPointAddress(v06EP); err != nil {
		t.Fatalf("expected supported entrypoint to parse: %v", err)
	}
}

func TestParseVersionedUserOp_V06(t *testing.T) {
	op, version, err := parseVersionedUserOp(testutils.MockUserOpData, common.HexToAddress(v06EP))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if version != userop.V0_6 {
		t.Fatalf("version: got %s want v0.6", version)
	}
	if op.GetSender() == (common.Address{}) {
		t.Fatalf("sender not set")
	}
}

func TestParseVersionedUserOp_DetectsV07FromFields(t *testing.T) {
	data := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0xfac0000000000000000000000000000000000001",
		"factoryData":                   "0xab",
		"callData":                      "0x",
		"callGasLimit":                  "0x1",
		"verificationGasLimit":          "0x1",
		"preVerificationGas":            "0x1",
		"maxFeePerGas":                  "0x1",
		"maxPriorityFeePerGas":          "0x1",
		"paymaster":                     "0x0000000000000000000000000000000000000000",
		"paymasterVerificationGasLimit": "0x0",
		"paymasterPostOpGasLimit":       "0x0",
		"paymasterData":                 "0x",
		"signature":                     "0x01",
	}
	// EntryPoint is v0.6, but fields signal v0.7+ -> should detect v0.7.
	_, version, err := parseVersionedUserOp(data, common.HexToAddress(v06EP))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if version != userop.V0_7 {
		t.Fatalf("expected v0.7 detection, got %s", version)
	}
}

func TestParseVersionedUserOp_BadData(t *testing.T) {
	_, _, err := parseVersionedUserOp(map[string]any{"sender": "not-an-address"}, common.HexToAddress(v06EP))
	if err == nil {
		t.Fatal("expected error for bad data")
	}
}

func TestParseVersionedUserOp_NilMapReturnsError(t *testing.T) {
	_, _, err := parseVersionedUserOp(nil, common.HexToAddress(v06EP))
	if err == nil {
		t.Fatal("expected error for nil op map")
	}
}

func TestClient_SendUserOperation_V06(t *testing.T) {
	c, mem := newTestClient(t)
	hash, err := c.SendUserOperation(testutils.MockUserOpData, v06EP)
	if err != nil {
		t.Fatalf("SendUserOperation: %v", err)
	}
	if !strings.HasPrefix(hash, "0x") {
		t.Fatalf("hash not in hex: %s", hash)
	}

	ops, err := mem.Dump(common.HexToAddress(v06EP))
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 op in mempool, got %d", len(ops))
	}
}

func TestClient_SendUserOperation_UnsupportedEP(t *testing.T) {
	c, _ := newTestClient(t)
	_, err := c.SendUserOperation(testutils.MockUserOpData, "0x0000000000000000000000000000000000000099")
	if err == nil {
		t.Fatal("expected unsupported EntryPoint error")
	}
}

func TestClient_SendUserOperation_BadOp(t *testing.T) {
	c, _ := newTestClient(t)
	_, err := c.SendUserOperation(map[string]any{"sender": "nope"}, v06EP)
	if err == nil {
		t.Fatal("expected bad op error")
	}
}

func TestClient_EstimateUserOperationGas_V06(t *testing.T) {
	c, _ := newTestClient(t)
	est, err := c.EstimateUserOperationGas(testutils.MockUserOpData, v06EP, nil)
	if err != nil {
		t.Fatalf("EstimateUserOperationGas: %v", err)
	}
	if est.VerificationGasLimit.Cmp(big.NewInt(100000)) != 0 {
		t.Fatalf("VerificationGasLimit: got %s", est.VerificationGasLimit)
	}
	if est.CallGasLimit.Cmp(big.NewInt(50000)) != 0 {
		t.Fatalf("CallGasLimit: got %s", est.CallGasLimit)
	}
	// v0.6 should not populate paymaster fields.
	if est.PaymasterVerificationGasLimit != nil {
		t.Fatalf("expected nil PaymasterVerificationGasLimit for v0.6, got %s", est.PaymasterVerificationGasLimit)
	}
}

func TestClient_EstimateUserOperationGas_V07PopulatesPaymasterFields(t *testing.T) {
	c, _ := newTestClient(t, common.HexToAddress(v07EP))
	c.ov = gas.NewDefaultOverheadV07()
	v07Data := map[string]any{
		"sender":                        "0xa13D69573f994bf662C2714560c44dd7266FC547",
		"nonce":                         "0x0",
		"factory":                       "0xfac0000000000000000000000000000000000001",
		"factoryData":                   "0xab",
		"callData":                      "0x",
		"callGasLimit":                  "0x1",
		"verificationGasLimit":          "0x1",
		"preVerificationGas":            "0x1",
		"maxFeePerGas":                  "0x1",
		"maxPriorityFeePerGas":          "0x1",
		"paymaster":                     "0xb000000000000000000000000000000000000002",
		"paymasterVerificationGasLimit": "0x100",
		"paymasterPostOpGasLimit":       "0x200",
		"paymasterData":                 "0xcafe",
		"signature":                     "0x01",
	}
	est, err := c.EstimateUserOperationGas(v07Data, v07EP, nil)
	if err != nil {
		t.Fatalf("EstimateUserOperationGas: %v", err)
	}
	if est.PaymasterVerificationGasLimit == nil || est.PaymasterVerificationGasLimit.Cmp(big.NewInt(0x100)) != 0 {
		t.Fatalf("PaymasterVerificationGasLimit not set correctly: %v", est.PaymasterVerificationGasLimit)
	}
	if est.PaymasterPostOpGasLimit == nil || est.PaymasterPostOpGasLimit.Cmp(big.NewInt(0x200)) != 0 {
		t.Fatalf("PaymasterPostOpGasLimit not set correctly: %v", est.PaymasterPostOpGasLimit)
	}
}

func TestClient_EstimateUserOperationGas_AppliesGasPriceOverride(t *testing.T) {
	c, _ := newTestClient(t)
	data := map[string]any{}
	for k, v := range testutils.MockUserOpData {
		data[k] = v
	}
	data["maxFeePerGas"] = "0x0"
	data["maxPriorityFeePerGas"] = "0x0"
	if _, err := c.EstimateUserOperationGas(data, v06EP, nil); err != nil {
		t.Fatalf("EstimateUserOperationGas: %v", err)
	}
}

func TestClient_GetUserOperationReceipt_NotFound(t *testing.T) {
	c, _ := newTestClient(t)
	// Default noop receipt func returns nil receipt, no error.
	r, err := c.GetUserOperationReceipt(testutils.MockHash)
	if err != nil {
		t.Fatalf("GetUserOperationReceipt: %v", err)
	}
	if r != nil {
		t.Fatalf("expected nil receipt")
	}
}

func TestClient_GetUserOperationReceipt_FoundViaSetter(t *testing.T) {
	c, _ := newTestClient(t)
	want := &filter.UserOperationReceipt{Sender: testutils.ValidAddress1}
	c.SetGetUserOpReceiptFunc(func(hash string, ep common.Address, blkRange uint64) (*filter.UserOperationReceipt, error) {
		return want, nil
	})
	got, err := c.GetUserOperationReceipt(testutils.MockHash)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestClient_GetUserOperationReceipt_FallsThroughEntryPoints(t *testing.T) {
	c, _ := newTestClient(t, common.HexToAddress(v06EP), common.HexToAddress(v07EP))
	want := &filter.UserOperationReceipt{Sender: testutils.ValidAddress1}
	calls := 0
	c.SetGetUserOpReceiptFunc(func(hash string, ep common.Address, blkRange uint64) (*filter.UserOperationReceipt, error) {
		calls++
		if calls == 1 {
			return nil, nil
		}
		return want, nil
	})
	got, err := c.GetUserOperationReceipt(testutils.MockHash)
	if err != nil || got != want || calls != 2 {
		t.Fatalf("fallback: got=%v err=%v calls=%d", got, err, calls)
	}
}

func TestClient_GetUserOperationByHash_FallsThroughEntryPoints(t *testing.T) {
	c, _ := newTestClient(t, common.HexToAddress(v06EP), common.HexToAddress(v07EP))
	want := &HashLookupResult{EntryPoint: v07EP}
	calls := 0
	c.SetGetUserOpByHashFunc(func(hash string, ep common.Address, chain *big.Int, blkRange uint64) (*HashLookupResult, error) {
		calls++
		if calls == 1 {
			return nil, nil
		}
		return want, nil
	})
	got, err := c.GetUserOperationByHash(testutils.MockHash)
	if err != nil || got != want || calls != 2 {
		t.Fatalf("fallback: got=%v err=%v calls=%d", got, err, calls)
	}
}

func TestClient_GetUserOperationByHash_FoundViaSetter(t *testing.T) {
	c, _ := newTestClient(t)
	want := &HashLookupResult{EntryPoint: v06EP}
	c.SetGetUserOpByHashFunc(func(hash string, ep common.Address, chain *big.Int, blkRange uint64) (*HashLookupResult, error) {
		return want, nil
	})
	got, err := c.GetUserOperationByHash(testutils.MockHash)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestClient_Accounts(t *testing.T) {
	c, _ := newTestClient(t)
	res, err := c.Accounts(v06EP)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	if len(res) == 0 {
		t.Fatalf("expected at least 1 paymaster")
	}
	if _, err := c.Accounts("0x0000000000000000000000000000000000000099"); err == nil {
		t.Fatal("expected unsupported EP error")
	}
}

func TestClient_SponsorUserOperation_BadType(t *testing.T) {
	c, _ := newTestClient(t)
	_, err := c.SponsorUserOperation(
		testutils.MockUserOpData,
		v06EP,
		map[string]any{"type": "unknown"},
	)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestClient_SponsorUserOperation_UnsupportedEP(t *testing.T) {
	c, _ := newTestClient(t)
	_, err := c.SponsorUserOperation(
		testutils.MockUserOpData,
		"0x0000000000000000000000000000000000000099",
		map[string]any{"type": "payg"},
	)
	if err == nil {
		t.Fatal("expected unsupported EP error")
	}
}

func TestClient_SponsorUserOperation_BadOp(t *testing.T) {
	c, _ := newTestClient(t)
	_, err := c.SponsorUserOperation(
		map[string]any{"sender": "bad"},
		v06EP,
		map[string]any{"type": "payg"},
	)
	if err == nil {
		t.Fatal("expected bad op error")
	}
}

func TestClient_SponsorUserOperation_BadContext(t *testing.T) {
	c, _ := newTestClient(t)
	_, err := c.SponsorUserOperation(
		testutils.MockUserOpData,
		v06EP,
		map[string]any{},
	)
	if err == nil {
		t.Fatal("expected bad context error")
	}
}

// helper to ensure unused imports stay used
var _ = handlers.SponsorUserOperationResponse{}
