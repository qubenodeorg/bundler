package start

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gin-gonic/gin"
	"github.com/zarbanio/bundler/internal/logger"
	"github.com/zarbanio/bundler/internal/testutils"
	"github.com/zarbanio/bundler/pkg/store"
)

type fakeStore struct {
	pingErr error
}

func (f fakeStore) Mempool() store.MempoolStore       { return nil }
func (f fakeStore) Reputation() store.ReputationStore { return nil }
func (f fakeStore) CodeHash() store.CodeHashStore     { return nil }
func (f fakeStore) Locker() store.Locker              { return store.NoopLocker{} }
func (f fakeStore) Ping(context.Context) error        { return f.pingErr }
func (f fakeStore) Close() error                      { return nil }

func newReadyClient(t *testing.T) *ethclient.Client {
	t.Helper()
	n := testutils.RpcMock(testutils.MethodMocks{"eth_blockNumber": "0x10"})
	t.Cleanup(n.Close)
	r, err := rpc.Dial(n.URL)
	if err != nil {
		t.Fatalf("rpc.Dial: %v", err)
	}
	t.Cleanup(r.Close)
	return ethclient.NewClient(r)
}

func callReady(t *testing.T, h gin.HandlerFunc) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)
	h(c)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return w.Code, body
}

func TestReadiness_Healthy(t *testing.T) {
	h := readinessHandler(newReadyClient(t), fakeStore{}, logger.NewZeroLogr())
	code, body := callReady(t, h)
	if code != http.StatusOK || body["ready"] != true {
		t.Fatalf("code=%d body=%v, want ready", code, body)
	}
	checks := body["checks"].(map[string]any)
	if checks["store"] != "ok" || checks["eth_client"] != "ok" {
		t.Fatalf("checks=%v, want both ok", checks)
	}
}

func TestReadiness_StoreUnavailable(t *testing.T) {
	h := readinessHandler(newReadyClient(t), fakeStore{pingErr: store.ErrUnavailable}, logger.NewZeroLogr())
	code, body := callReady(t, h)
	if code != http.StatusServiceUnavailable || body["ready"] != false {
		t.Fatalf("code=%d body=%v, want not ready", code, body)
	}
	checks := body["checks"].(map[string]any)
	if checks["store"] != "unavailable" {
		t.Fatalf("checks=%v, want store unavailable", checks)
	}
}

func TestReadiness_NilStore(t *testing.T) {
	h := readinessHandler(newReadyClient(t), nil, logger.NewZeroLogr())
	code, _ := callReady(t, h)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d, want 503", code)
	}
}
