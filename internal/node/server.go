package node

import (
	"net/http"
	"sync"
)

// Config holds the agent's static settings, loaded from the environment
// by cmd/node.
type Config struct {
	NodeID      string
	HostAddress string
	Image       string
	PortMin     int
	PortMax     int
	// Version is the build's commit SHA, reported by GET /v1/node.
	Version string
}

// Node holds the dependencies the agent's HTTP handlers and background
// loops need.
type Node struct {
	Config
	Docker  DockerClient
	Volumes VolumeManager

	// createMu serializes database creation so two concurrent requests
	// for different ids cannot race for the same free port.
	createMu sync.Mutex
}

// NewMux builds the agent's HTTP router. It is meant to sit behind the
// mTLS listener cmd/node sets up; it does no authentication of its own.
func NewMux(n *Node) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", n.handleHealthz)
	mux.HandleFunc("GET /v1/node", n.handleGetNode)
	mux.HandleFunc("GET /v1/databases", n.handleListDatabases)
	mux.HandleFunc("POST /v1/databases", n.handleCreateDatabase)
	mux.HandleFunc("DELETE /v1/databases/{id}", n.handleDeleteDatabase)
	mux.HandleFunc("GET /v1/databases/{id}/snapshots", n.handleSnapshotsGated)
	mux.HandleFunc("POST /v1/databases/{id}/snapshots", n.handleSnapshotsGated)

	return mux
}

func (n *Node) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type databaseResponse struct {
	ID     string `json:"id"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Status string `json:"status"`
}

func (n *Node) toDatabaseResponse(c Container) databaseResponse {
	return databaseResponse{ID: c.DatabaseID, Host: n.HostAddress, Port: c.Port, Status: c.Status}
}

type capacityResponse struct {
	PortRangeMin int `json:"port_range_min"`
	PortRangeMax int `json:"port_range_max"`
	PortsTotal   int `json:"ports_total"`
	PortsUsed    int `json:"ports_used"`
}

type nodeResponse struct {
	NodeID     string             `json:"node_id"`
	Host       string             `json:"host"`
	Version    string             `json:"version"`
	Capacity   capacityResponse   `json:"capacity"`
	UsedPorts  []int              `json:"used_ports"`
	Containers []databaseResponse `json:"containers"`
}

func (n *Node) handleGetNode(w http.ResponseWriter, r *http.Request) {
	containers, err := n.Docker.ListTenantContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list containers")
		return
	}
	usedPorts, err := n.Docker.UsedHostPorts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list used ports")
		return
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

	writeJSON(w, http.StatusOK, nodeResponse{
		NodeID:  n.NodeID,
		Host:    n.HostAddress,
		Version: n.Version,
		Capacity: capacityResponse{
			PortRangeMin: n.PortMin,
			PortRangeMax: n.PortMax,
			PortsTotal:   n.PortMax - n.PortMin + 1,
			PortsUsed:    len(ports),
		},
		UsedPorts:  ports,
		Containers: out,
	})
}

func (n *Node) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	containers, err := n.Docker.ListTenantContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list containers")
		return
	}
	out := make([]databaseResponse, len(containers))
	for i, c := range containers {
		out[i] = n.toDatabaseResponse(c)
	}
	writeJSON(w, http.StatusOK, out)
}

type createDatabaseRequest struct {
	ID       string `json:"id"`
	SizeGB   int    `json:"size_gb"`
	Password string `json:"password"`
}

func (n *Node) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	var req createDatabaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" || req.SizeGB <= 0 || req.Password == "" {
		writeError(w, http.StatusBadRequest, "id, size_gb, and password are required")
		return
	}

	n.createMu.Lock()
	defer n.createMu.Unlock()

	ctx := r.Context()

	existing, err := n.Docker.ListTenantContainers(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list containers")
		return
	}
	for _, c := range existing {
		if c.DatabaseID == req.ID {
			writeJSON(w, http.StatusOK, n.toDatabaseResponse(c))
			return
		}
	}

	usedPorts, err := n.Docker.UsedHostPorts(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list used ports")
		return
	}
	port, err := allocatePort(n.PortMin, n.PortMax, usedPorts)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "no free port available")
		return
	}

	hostDataDir, err := n.Volumes.Create(req.ID, req.SizeGB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create volume")
		return
	}

	memoryBytes, nanoCPUs := resourceLimits(req.SizeGB)
	container, err := n.Docker.CreateTenantContainer(ctx, ContainerSpec{
		DatabaseID:  req.ID,
		Image:       n.Image,
		Password:    req.Password,
		Port:        port,
		HostDataDir: hostDataDir,
		MemoryBytes: memoryBytes,
		NanoCPUs:    nanoCPUs,
	})
	if err != nil {
		_ = n.Volumes.Remove(req.ID)
		writeError(w, http.StatusInternalServerError, "could not create container")
		return
	}

	writeJSON(w, http.StatusCreated, n.toDatabaseResponse(container))
}

func (n *Node) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	containers, err := n.Docker.ListTenantContainers(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list containers")
		return
	}

	var found *Container
	for i := range containers {
		if containers[i].DatabaseID == id {
			found = &containers[i]
			break
		}
	}

	if found != nil {
		if err := n.Docker.RemoveTenantContainer(ctx, found.ContainerID); err != nil {
			writeError(w, http.StatusInternalServerError, "could not remove container")
			return
		}
	}

	if err := n.Volumes.Remove(id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not remove volume")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleSnapshotsGated covers every snapshot route; snapshots are out of
// scope for this slice of the agent.
func (n *Node) handleSnapshotsGated(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "snapshots are not implemented yet")
}

// resourceLimits sizes a tenant container's CPU and memory to its plan
// size. The task does not specify pricing tiers, so this is a simple,
// adjustable heuristic: a floor, a per-GB increment, and a ceiling.
func resourceLimits(sizeGB int) (memoryBytes, nanoCPUs int64) {
	const (
		baseMemoryBytes  = 256 * 1024 * 1024
		memoryPerGBBytes = 128 * 1024 * 1024
		maxMemoryBytes   = 4 * 1024 * 1024 * 1024

		baseNanoCPUs  = 250_000_000
		nanoCPUsPerGB = 50_000_000
		maxNanoCPUs   = 2_000_000_000
	)

	memoryBytes = baseMemoryBytes + int64(sizeGB)*memoryPerGBBytes
	if memoryBytes > maxMemoryBytes {
		memoryBytes = maxMemoryBytes
	}
	nanoCPUs = baseNanoCPUs + int64(sizeGB)*nanoCPUsPerGB
	if nanoCPUs > maxNanoCPUs {
		nanoCPUs = maxNanoCPUs
	}
	return memoryBytes, nanoCPUs
}
