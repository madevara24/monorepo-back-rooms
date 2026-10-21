// Package entities holds the agent's domain models: its static config,
// and the tenant container spec and state repository and usecase pass
// around.
package entities

// Config holds the agent's static settings, loaded from the environment
// by config.Load.
type Config struct {
	NodeID      string
	HostAddress string
	Image       string
	PortMin     int
	PortMax     int
	// Version is the build's commit SHA, reported by GET /v1/node.
	Version string
	// ReleasePublicKeyConfigured reports whether AGENT_RELEASE_PUBLIC_KEY_B64
	// was set, so GET /v1/node lets ops confirm a node is ready for
	// MBR-23's updater before it ships.
	ReleasePublicKeyConfigured bool
	// TenantTLSCertPath and TenantTLSKeyPath are host paths to the
	// wildcard cert and key that every tenant container mounts
	// read-only so Postgres can require TLS.
	TenantTLSCertPath string
	TenantTLSKeyPath  string
}

// Container is the subset of a tenant container's Docker state the agent
// exposes over its API.
type Container struct {
	ContainerID string
	DatabaseID  string
	Port        int
	Status      string
}

// ConnectionMetrics is one tenant container's backend connection counts
// and limits, collected read-only over the container's local socket, plus
// the container's configured memory limit (the plan's memory limit).
type ConnectionMetrics struct {
	Active                       int
	Idle                         int
	IdleInTransaction            int
	MaxConnections               int
	SuperuserReservedConnections int
	MemoryLimitBytes             int64
}

// ContainerSpec describes the tenant container to create.
type ContainerSpec struct {
	DatabaseID  string
	Image       string
	Password    string
	Port        int
	HostDataDir string
	MemoryBytes int64
	NanoCPUs    int64
	// TLSCertPath and TLSKeyPath are host paths to the wildcard cert and
	// key; they are bind-mounted read-only into the container so Postgres
	// can require TLS on every tenant connection.
	TLSCertPath string
	TLSKeyPath  string
}
