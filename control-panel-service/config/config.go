// Package config reads the control plane's environment into a typed
// Config, so cmd/api and internal/container.go don't parse os.Getenv
// themselves.
package config

import "os"

// defaultRoutesPath is the SNI proxy's host map on KKB, bind-mounted
// read-write into the mbr-api container (see dokploy-compose.yml).
const defaultRoutesPath = "/etc/mbr/routes/mbr-routes.map"

// Config holds every environment-derived setting cmd/api needs to start.
type Config struct {
	Addr               string
	DatabaseURL        string
	CredentialKeyB64   string
	AgentCAB64         string
	AgentClientCertB64 string
	AgentClientKeyB64  string
	CookieSecure       bool
	RoutesPath         string
	DemoMode           bool
	DemoAdminEmail     string
	DemoAdminPassword  string
}

// Load reads Config from the environment, applying the same defaults
// the service has always used for local dev.
func Load() Config {
	return Config{
		Addr:               getEnv("API_ADDR", ":8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://mbr:mbr@localhost:5432/mbr?sslmode=disable"),
		CredentialKeyB64:   os.Getenv("CREDENTIAL_KEY"),
		AgentCAB64:         os.Getenv("AGENT_CA_B64"),
		AgentClientCertB64: os.Getenv("AGENT_CLIENT_CERT_B64"),
		AgentClientKeyB64:  os.Getenv("AGENT_CLIENT_KEY_B64"),
		CookieSecure:       os.Getenv("API_COOKIE_SECURE") == "true",
		RoutesPath:         getEnv("MBR_ROUTES_PATH", defaultRoutesPath),
		DemoMode:           os.Getenv("DEMO_MODE") == "true",
		DemoAdminEmail:     os.Getenv("DEMO_ADMIN_EMAIL"),
		DemoAdminPassword:  os.Getenv("DEMO_ADMIN_PASSWORD"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
