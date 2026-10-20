// Command api runs the MBR control-plane HTTP API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/config"
	container "github.com/madevara24/monorepo-back-rooms/control-panel-service/internal"
)

// commit is stamped at build time with -ldflags "-X main.commit=<sha>".
var commit = "unknown"

// routeReconcileInterval is how often the SNI route map is rewritten from
// the current set of running databases, on top of the reconcile that
// already runs right after a provision or delete job completes.
const routeReconcileInterval = 10 * time.Second

// metricsRetentionInterval is how often old connection-pressure snapshots
// are pruned. It does not need to run often: the retention window is 14
// days.
const metricsRetentionInterval = 1 * time.Hour

func main() {
	cfg := config.Load()

	c, err := container.New(cfg, commit)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = c.DB.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go c.Worker.Run(ctx)
	go c.Reconciler.Run(ctx, routeReconcileInterval)
	go c.MetricsRetention.Run(ctx, metricsRetentionInterval)

	server := &http.Server{Addr: cfg.Addr, Handler: c.Mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("api listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
