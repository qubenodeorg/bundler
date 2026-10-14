package start

import (
	"context"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/zarbanio/bundler/pkg/store"
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
// eth RPC is reachable and the storage backend is usable. It returns 503 when a
// dependency is unavailable so load balancers stop routing to a degraded
// instance without it being restarted.
//
// Check values are stable, sanitized strings ("ok" / "unavailable"); the raw
// dependency error is written to the server log only, never serialized to the
// (potentially public) response, to avoid leaking internal infrastructure detail.
func readinessHandler(eth *ethclient.Client, st store.Store, logr logr.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Guard against an unwired probe: a nil dependency means the instance was
		// never fully initialized, so report not-ready rather than panic on the
		// dereference below.
		if eth == nil || st == nil {
			logr.Error(nil, "readiness check failed: dependencies not initialized")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"ready":  false,
				"checks": gin.H{"eth_client": "unavailable", "store": "unavailable"},
			})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
		defer cancel()

		checks := gin.H{}
		ready := true

		if _, err := eth.BlockNumber(ctx); err != nil {
			logr.Error(err, "readiness check failed", "dependency", "eth_client")
			checks["eth_client"] = "unavailable"
			ready = false
		} else {
			checks["eth_client"] = "ok"
		}

		// A backend ping confirms the store is reachable and usable; it returns
		// ErrUnavailable if the store (e.g. Redis) cannot be contacted.
		if err := st.Ping(ctx); err != nil {
			logr.Error(err, "readiness check failed", "dependency", "store")
			checks["store"] = "unavailable"
			ready = false
		} else {
			checks["store"] = "ok"
		}

		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"ready": ready, "checks": checks})
	}
}
