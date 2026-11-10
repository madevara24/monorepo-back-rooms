package contracts

import "time"

// HeartbeatRequest is the POST /api/agent/heartbeat body: an mbr-node
// agent's periodic report of its capacity and the tenant containers it
// currently sees. It carries no agent version field, so a heartbeat never
// updates nodes.version.
type HeartbeatRequest struct {
	NodeID     string               `json:"node_id"`
	Host       string               `json:"host"`
	Capacity   Capacity             `json:"capacity"`
	Containers []HeartbeatContainer `json:"containers"`
}

// HeartbeatContainer is one tenant container as seen by the agent.
type HeartbeatContainer struct {
	ID      string                      `json:"id"`
	Host    string                      `json:"host"`
	Port    int                         `json:"port"`
	Status  string                      `json:"status"`
	Metrics *HeartbeatConnectionMetrics `json:"metrics,omitempty"`
}

// HeartbeatConnectionMetrics is one container's connection-pressure
// snapshot, attached to a HeartbeatContainer only on ticks where metrics
// were actually collected (every metricsInterval, not every heartbeat).
type HeartbeatConnectionMetrics struct {
	Active                       int       `json:"active"`
	Idle                         int       `json:"idle"`
	IdleInTransaction            int       `json:"idle_in_transaction"`
	MaxConnections               int       `json:"max_connections"`
	SuperuserReservedConnections int       `json:"superuser_reserved_connections"`
	MemoryLimitBytes             int64     `json:"memory_limit_bytes"`
	CollectedAt                  time.Time `json:"collected_at"`
}

// Capacity is a node's port capacity. It is not heartbeat-specific: the
// agent's node-status response reuses it too.
type Capacity struct {
	PortRangeMin int `json:"port_range_min"`
	PortRangeMax int `json:"port_range_max"`
	PortsTotal   int `json:"ports_total"`
	PortsUsed    int `json:"ports_used"`
}
