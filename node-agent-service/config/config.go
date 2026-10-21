// Package config reads the agent's environment into a typed Config and
// validates its TLS material, so cmd/node and internal/container.go
// don't parse os.Getenv themselves.
package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strconv"

	release "github.com/madevara24/monorepo-back-rooms/agent-release"
)

// Config holds every environment-derived setting cmd/node needs to
// start.
type Config struct {
	Addr        string
	DataDir     string
	NodeID      string
	HostAddress string
	Image       string
	PortMin     int
	PortMax     int

	TLSCert  string
	TLSKey   string
	ClientCA string

	// TenantTLSCert and TenantTLSKey are host paths to the wildcard cert
	// and key every tenant container mounts read-only so Postgres can
	// require TLS.
	TenantTLSCert string
	TenantTLSKey  string

	ControlURL   string
	ControlToken string

	// ReleasePublicKeyB64 is the base64 ed25519 public key that verifies
	// a signed mbr-node release manifest. It is optional until MBR-23's
	// updater lands; if set, it must already be a well-formed key so a
	// typo is caught at startup rather than at the first update attempt.
	ReleasePublicKeyB64 string
}

// Load reads Config from the environment and validates its TLS
// material: the agent's own mTLS listener cert/key/client CA, and the
// tenant wildcard cert/key every tenant container mounts.
func Load() (Config, error) {
	nodeID := os.Getenv("AGENT_NODE_ID")
	if nodeID == "" {
		if hostname, err := os.Hostname(); err == nil {
			nodeID = hostname
		}
	}

	portMin, err := getEnvInt("AGENT_PORT_MIN", 21000)
	if err != nil {
		return Config{}, err
	}
	portMax, err := getEnvInt("AGENT_PORT_MAX", 21999)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Addr:                getEnv("AGENT_ADDR", ":9443"),
		DataDir:             getEnv("AGENT_DATA_DIR", "/var/lib/mbr-node"),
		NodeID:              nodeID,
		HostAddress:         os.Getenv("AGENT_HOST_ADDRESS"),
		Image:               getEnv("AGENT_IMAGE", "postgres:17"),
		PortMin:             portMin,
		PortMax:             portMax,
		TLSCert:             os.Getenv("AGENT_TLS_CERT"),
		TLSKey:              os.Getenv("AGENT_TLS_KEY"),
		ClientCA:            os.Getenv("AGENT_CLIENT_CA"),
		TenantTLSCert:       os.Getenv("AGENT_TENANT_TLS_CERT"),
		TenantTLSKey:        os.Getenv("AGENT_TENANT_TLS_KEY"),
		ControlURL:          os.Getenv("CONTROL_URL"),
		ControlToken:        os.Getenv("AGENT_CONTROL_TOKEN"),
		ReleasePublicKeyB64: os.Getenv("AGENT_RELEASE_PUBLIC_KEY_B64"),
	}

	if err := requireTLSEnv(cfg.TLSCert, cfg.TLSKey, cfg.ClientCA, cfg.TenantTLSCert, cfg.TenantTLSKey); err != nil {
		return Config{}, err
	}
	if err := validateTenantTLSFiles(cfg.TenantTLSCert, cfg.TenantTLSKey); err != nil {
		return Config{}, err
	}
	if cfg.ReleasePublicKeyB64 != "" {
		if _, err := release.ParsePublicKeyB64(cfg.ReleasePublicKeyB64); err != nil {
			return Config{}, fmt.Errorf("invalid AGENT_RELEASE_PUBLIC_KEY_B64: %w", err)
		}
	}

	return cfg, nil
}

// requireTLSEnv validates the agent's mTLS listener env (cert, key, and
// client CA) and the tenant wildcard TLS env (cert and key mounted into
// every tenant container) are all set. Infra is responsible for placing
// the tenant cert/key on the host; the agent only requires that it knows
// where to find them.
func requireTLSEnv(tlsCert, tlsKey, clientCA, tenantTLSCert, tenantTLSKey string) error {
	if tlsCert == "" || tlsKey == "" || clientCA == "" {
		return errors.New("AGENT_TLS_CERT, AGENT_TLS_KEY, and AGENT_CLIENT_CA are required")
	}
	if tenantTLSCert == "" || tenantTLSKey == "" {
		return errors.New("AGENT_TENANT_TLS_CERT and AGENT_TENANT_TLS_KEY are required")
	}
	return nil
}

// validateTenantTLSFiles confirms the tenant wildcard cert and key exist
// and parse as a valid key pair. Without this, a bad path or corrupt
// file would not fail until the first CreateTenantContainer call (and
// then every subsequent one), turning a startup-time problem into a
// provisioning outage discovered in production.
func validateTenantTLSFiles(certPath, keyPath string) error {
	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		return fmt.Errorf("load tenant TLS cert/key: %w", err)
	}
	return nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, v, err)
	}
	return n, nil
}
