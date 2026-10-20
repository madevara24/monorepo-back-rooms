// Command router runs the MBR protocol-aware connection router: a
// TCP-level proxy that reads just enough of a client's TLS/Postgres
// handshake to route on SNI, then forwards every other byte between
// the client and the tenant it resolves to. It never terminates TLS
// and holds no certificate or key.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/madevara24/monorepo-back-rooms/connection-router/config"
	container "github.com/madevara24/monorepo-back-rooms/connection-router/internal"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/routemap"
)

// commit is stamped at build time with -ldflags "-X main.commit=<sha>".
var commit = "unknown"

func main() {
	cfg := config.Load()

	c, err := container.New(cfg, commit)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go c.Routes.Run(ctx)
	go watchSIGHUP(ctx, c.Routes)

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Fatalf("listen on %s: %v", cfg.ListenAddr, err)
	}
	proxyDone := make(chan error, 1)
	go func() { proxyDone <- c.Proxy.Serve(ctx, ln) }()

	healthServer := &http.Server{Addr: cfg.HealthAddr, Handler: c.Mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = healthServer.Shutdown(shutdownCtx)
	}()

	log.Printf("router listening on %s, health on %s, routes %s", cfg.ListenAddr, cfg.HealthAddr, cfg.RoutesPath)
	if err := healthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}

	if err := <-proxyDone; err != nil {
		log.Printf("proxy: serve: %v", err)
	}
}

// watchSIGHUP lets an operator force a route-map reload without
// waiting on fsnotify, e.g. if the mount's change events ever get
// lost across a bind-mount remount.
func watchSIGHUP(ctx context.Context, routes *routemap.Watcher) {
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer signal.Stop(sighup)
	for {
		select {
		case <-ctx.Done():
			return
		case <-sighup:
			routes.Reload()
		}
	}
}
