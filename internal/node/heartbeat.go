package node

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type heartbeatRequest struct {
	NodeID     string             `json:"node_id"`
	Host       string             `json:"host"`
	Capacity   capacityResponse   `json:"capacity"`
	Containers []databaseResponse `json:"containers"`
}

// RunLoop reconciles the agent's view of its containers and sends a
// heartbeat to the control plane, once immediately and then on every
// tick of interval, until ctx is canceled. Both reconcile and heartbeat
// run on this single cadence because they are the same Docker call: the
// task describes each as "every 30s".
func (n *Node) RunLoop(ctx context.Context, controlURL, controlToken string, interval time.Duration) {
	n.tick(ctx, controlURL, controlToken)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.tick(ctx, controlURL, controlToken)
		}
	}
}

func (n *Node) tick(ctx context.Context, controlURL, controlToken string) {
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
	ports := make([]int, 0, len(usedPorts))
	for p := range usedPorts {
		if p >= n.PortMin && p <= n.PortMax {
			ports = append(ports, p)
		}
	}

	out := make([]databaseResponse, len(containers))
	for i, c := range containers {
		out[i] = n.toDatabaseResponse(c)
	}

	body := heartbeatRequest{
		NodeID: n.NodeID,
		Host:   n.HostAddress,
		Capacity: capacityResponse{
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
