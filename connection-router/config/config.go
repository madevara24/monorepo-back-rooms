// Package config reads the router's environment into a typed Config,
// so cmd/router and internal/container.go don't parse os.Getenv
// themselves.
package config

import "os"

// defaultRoutesPath is the SNI host map, bind-mounted read-only into
// the router container (see dokploy-compose.router.yml). It is the
// same file control-panel-service's reconciler writes.
const defaultRoutesPath = "/etc/mbr/routes/mbr-routes.map"

// Config holds every environment-derived setting cmd/router needs to
// start.
type Config struct {
	// ListenAddr is the Postgres-facing data path, e.g. ":15432".
	ListenAddr string
	// HealthAddr is the GET /healthz listener's bind address inside
	// the container, e.g. ":9099". Loopback-only exposure is enforced
	// by Docker's port publishing (dokploy-compose.router.yml maps
	// "127.0.0.1:9099:9099"), not by this bind address: Docker routes
	// published ports to a container's external interface, not its
	// own loopback, so binding to 127.0.0.1 here would make the
	// published port unreachable.
	HealthAddr string
	RoutesPath string
}

// Load reads Config from the environment, applying the router's
// production defaults for local dev.
func Load() Config {
	return Config{
		ListenAddr: getEnv("ROUTER_ADDR", ":15432"),
		HealthAddr: getEnv("ROUTER_HEALTH_ADDR", ":9099"),
		RoutesPath: getEnv("MBR_ROUTES_PATH", defaultRoutesPath),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
