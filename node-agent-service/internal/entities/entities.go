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
