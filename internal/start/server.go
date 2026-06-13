package start

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
)

// serveHTTP runs the bundler's HTTP server with production-grade timeouts and a
// graceful shutdown. It blocks until a SIGINT/SIGTERM is received, then drains
// in-flight requests before returning. Returning (rather than blocking forever
// in gin's Run) is what allows the caller's deferred cleanups — BadgerDB close,
// OTEL flush — to actually execute on shutdown.
//
// ReadHeaderTimeout is the primary slowloris guard. WriteTimeout is generous so
// it does not cut legitimate but slow calls (e.g. the debug send-bundle-now RPC
// that waits for inclusion); it is only enabled in debug mode.
func serveHTTP(r *gin.Engine, port int, debugMode bool, logr logr.Logger) {
	writeTimeout := 60 * time.Second
	if debugMode {
		// debug_bundler_sendBundleNow blocks until the bundle is mined, which can
		// exceed any fixed write deadline. Disable the write timeout in debug mode.
		writeTimeout = 0
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           r,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	serverErr := make(chan error, 1)
	go func() {
		logr.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logr.Error(err, "http server failed")
	case sig := <-stop:
		logr.Info("shutdown signal received, draining requests", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logr.Error(err, "graceful shutdown failed, forcing close")
			_ = srv.Close()
		}
	}
}
