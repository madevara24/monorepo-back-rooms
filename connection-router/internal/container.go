// Package container is the router's composition root: it reads
// config.Config, loads and starts watching the route map, and wires
// the proxy server and the health HTTP mux for cmd/router to run.
package container

import (
	"fmt"
	"net/http"

	"github.com/madevara24/monorepo-back-rooms/connection-router/config"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/handler"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/proxy"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/routemap"
)

// Container holds every long-lived dependency cmd/router runs: the
// route watcher, the proxy server, and the health HTTP mux.
type Container struct {
	Routes *routemap.Watcher
	Proxy  *proxy.Server
	Mux    *http.ServeMux
}

// New loads cfg.RoutesPath and wires every layer together. commit is
// the build's git SHA, reported by GET /healthz. It fails only if the
// initial route map load fails; there is no previous map to fall back
// to at startup.
func New(cfg config.Config, commit string) (*Container, error) {
	routes, err := routemap.New(cfg.RoutesPath)
	if err != nil {
		return nil, fmt.Errorf("load route map: %w", err)
	}

	metrics := &proxy.Metrics{}
	p := &proxy.Server{
		Routes:           routes,
		Metrics:          metrics,
		DialTimeout:      proxy.DefaultDialTimeout,
		IdleTimeout:      proxy.DefaultIdleTimeout,
		HandshakeTimeout: proxy.DefaultHandshakeTimeout,
		MaxConnections:   proxy.DefaultMaxConnections,
	}

	mux := handler.NewMux(&handler.Handler{Commit: commit, Metrics: metrics, Routes: routes})

	return &Container{Routes: routes, Proxy: p, Mux: mux}, nil
}
