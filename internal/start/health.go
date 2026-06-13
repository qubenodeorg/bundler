package start

import (
	"context"
	"net/http"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
)

// healthCheckTimeout bounds the dependency probes in the readiness handler so a
// slow upstream cannot make the endpoint hang.
const healthCheckTimeout = 2 * time.Second

// livenessHandler reports whether the process is up. It has no dependencies and
// must stay cheap: orchestrators restart the container when it fails, so it
// should only report unhealthy when the process itself is broken.
func livenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// readinessHandler reports whether the bundler can serve traffic: the upstream
// eth RPC is reachable and the mempool DB is usable. It returns 503 when a
// dependency is unavailable so load balancers stop routing to a degraded
// instance without it being restarted.
func readinessHandler(eth *ethclient.Client, db *badger.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
		defer cancel()

		checks := gin.H{}
		ready := true

		if _, err := eth.BlockNumber(ctx); err != nil {
			checks["eth_client"] = err.Error()
			ready = false
		} else {
			checks["eth_client"] = "ok"
		}

		// A no-op read transaction confirms the DB is open and usable; it returns
		// an error (e.g. ErrDBClosed) if the store is unavailable.
		if err := db.View(func(txn *badger.Txn) error { return nil }); err != nil {
			checks["db"] = err.Error()
			ready = false
		} else {
			checks["db"] = "ok"
		}

		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"ready": ready, "checks": checks})
	}
}
