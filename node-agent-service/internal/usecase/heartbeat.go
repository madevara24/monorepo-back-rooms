package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities"
)

// heartbeatContainer, heartbeatConnectionMetrics, and heartbeatCapacity
// mirror the shapes control-panel-service/internal/handler's
// heartbeatContainer, heartbeatConnectionMetrics, and heartbeatCapacity
// expect in the POST /api/agent/heartbeat body.
type heartbeatContainer struct {
	ID      string                      `json:"id"`
	Host    string                      `json:"host"`
	Port    int                         `json:"port"`
	Status  string                      `json:"status"`
	Metrics *heartbeatConnectionMetrics `json:"metrics,omitempty"`
}

// heartbeatConnectionMetrics is one container's connection-pressure
// snapshot, attached to a heartbeatContainer only on ticks where metrics
// were actually collected (every metricsInterval, not every heartbeat).
type heartbeatConnectionMetrics struct {
	Active                       int       `json:"active"`
	Idle                         int       `json:"idle"`
	IdleInTransaction            int       `json:"idle_in_transaction"`
	MaxConnections               int       `json:"max_connections"`
	SuperuserReservedConnections int       `json:"superuser_reserved_connections"`
	MemoryLimitBytes             int64     `json:"memory_limit_bytes"`
	CollectedAt                  time.Time `json:"collected_at"`
}

type heartbeatCapacity struct {
	PortRangeMin int `json:"port_range_min"`
	PortRangeMax int `json:"port_range_max"`
	PortsTotal   int `json:"ports_total"`
	PortsUsed    int `json:"ports_used"`
}

type heartbeatRequest struct {
	NodeID     string               `json:"node_id"`
	Host       string               `json:"host"`
	Capacity   heartbeatCapacity    `json:"capacity"`
	Containers []heartbeatContainer `json:"containers"`
}

// RunLoop reconciles the agent's view of its containers and sends a
// heartbeat to the control plane, once immediately and then on every
// tick of interval, until ctx is canceled. Both reconcile and heartbeat
// run on this single cadence because they are the same Docker call: the
// task describes each as "every 30s". Connection-pressure metrics are
// collected on their own, coarser metricsInterval cadence (the task wants
// 60s), decoupled from the heartbeat send cadence so changing one does
// not silently change the other.
func (n *Node) RunLoop(ctx context.Context, controlURL, controlToken string, interval, metricsInterval time.Duration) {
	n.tick(ctx, controlURL, controlToken, metricsInterval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.tick(ctx, controlURL, controlToken, metricsInterval)
		}
	}
}

func (n *Node) tick(ctx context.Context, controlURL, controlToken string, metricsInterval time.Duration) {
	containers, err := n.Docker.ListTenantContainers(ctx)
	if err != nil {
		log.Printf("node: reconcile: list tenant containers: %v", err)
		containers = nil
	} else {
		log.Printf("node: reconcile: %d tenant container(s)", len(containers))
	}

	if controlURL == "" {
		return
	}

	usedPorts, err := n.Docker.UsedHostPorts(ctx)
	if err != nil {
		log.Printf("node: heartbeat: list used ports: %v", err)
		usedPorts = nil
	}
	ports := portsInRange(usedPorts, n.PortMin, n.PortMax)

	var metrics map[string]entities.ConnectionMetrics
	collectMetrics := time.Since(n.lastMetricsAt) >= metricsInterval
	if collectMetrics {
		metrics = n.collectMetrics(ctx, containers)
		n.lastMetricsAt = time.Now()
	}

	out := make([]heartbeatContainer, len(containers))
	for i, c := range containers {
		hc := heartbeatContainer{ID: c.DatabaseID, Host: n.HostAddress, Port: c.Port, Status: c.Status}
		if m, ok := metrics[c.DatabaseID]; ok {
			hc.Metrics = &heartbeatConnectionMetrics{
				Active:                       m.Active,
				Idle:                         m.Idle,
				IdleInTransaction:            m.IdleInTransaction,
				MaxConnections:               m.MaxConnections,
				SuperuserReservedConnections: m.SuperuserReservedConnections,
				MemoryLimitBytes:             m.MemoryLimitBytes,
				CollectedAt:                  n.lastMetricsAt,
			}
		}
		out[i] = hc
	}

	body := heartbeatRequest{
		NodeID: n.NodeID,
		Host:   n.HostAddress,
		Capacity: heartbeatCapacity{
			PortRangeMin: n.PortMin,
			PortRangeMax: n.PortMax,
			PortsTotal:   n.PortMax - n.PortMin + 1,
			PortsUsed:    len(ports),
		},
		Containers: out,
	}

	n.sendHeartbeat(ctx, controlURL, controlToken, body)
}

func (n *Node) sendHeartbeat(ctx context.Context, controlURL, controlToken string, body heartbeatRequest) {
	b, err := json.Marshal(body)
	if err != nil {
		log.Printf("node: heartbeat: marshal request: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controlURL+"/api/agent/heartbeat", bytes.NewReader(b))
	if err != nil {
		log.Printf("node: heartbeat: build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+controlToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("node: heartbeat: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		log.Printf("node: heartbeat: control plane returned %d", resp.StatusCode)
	}
}
