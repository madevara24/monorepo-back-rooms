// Package usecase implements the agent's business logic: provisioning,
// deleting, listing, and reconciling tenant containers, and the
// heartbeat loop that reports them to the control plane. It orchestrates
// repository's DockerClient and VolumeManager without owning either.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/repository"
)

// Node holds the dependencies the agent's handlers and background loops
// need.
type Node struct {
	entities.Config
	Docker  repository.DockerClient
	Volumes repository.VolumeManager

	// createMu serializes database creation so two concurrent requests
	// for different ids cannot race for the same free port.
	createMu sync.Mutex

	// lastMetricsAt is when connection metrics were last collected. It is
	// only read and written from RunLoop's single goroutine, so it needs
	// no lock.
	lastMetricsAt time.Time
}

// collectMetrics gathers a connection-pressure snapshot for every running
// container, skipping (and logging, not failing) any container that is
// not running or whose collection errors, so one wedged or deleted tenant
// cannot break metrics collection for the rest or the heartbeat itself.
func (n *Node) collectMetrics(ctx context.Context, containers []entities.Container) map[string]entities.ConnectionMetrics {
	out := make(map[string]entities.ConnectionMetrics, len(containers))
	for _, c := range containers {
		if c.Status != "running" {
			continue
		}
		m, err := n.Docker.CollectConnectionMetrics(ctx, c.ContainerID)
		if err != nil {
			log.Printf("node: metrics: collect %s: %v", c.DatabaseID, err)
			continue
		}
		out[c.DatabaseID] = m
	}
	return out
}

// ListDatabases returns every tenant container the agent currently sees.
func (n *Node) ListDatabases(ctx context.Context) ([]entities.Container, error) {
	return n.Docker.ListTenantContainers(ctx)
}

// Status is the agent's current capacity and the tenant containers it
// sees, the data behind GET /v1/node.
type Status struct {
	NodeID       string
	Host         string
	Version      string
	PortRangeMin int
	PortRangeMax int
	PortsTotal   int
	PortsUsed    int
	UsedPorts    []int
	Containers   []entities.Container
}

// GetStatus reports the agent's capacity and the tenant containers it
// currently sees.
func (n *Node) GetStatus(ctx context.Context) (Status, error) {
	containers, err := n.Docker.ListTenantContainers(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("list tenant containers: %w", err)
	}
	usedPorts, err := n.Docker.UsedHostPorts(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("list used ports: %w", err)
	}

	ports := portsInRange(usedPorts, n.PortMin, n.PortMax)

	return Status{
		NodeID:       n.NodeID,
		Host:         n.HostAddress,
		Version:      n.Version,
		PortRangeMin: n.PortMin,
		PortRangeMax: n.PortMax,
		PortsTotal:   n.PortMax - n.PortMin + 1,
		PortsUsed:    len(ports),
		UsedPorts:    ports,
		Containers:   containers,
	}, nil
}

// CreateDatabase provisions (or, if id already exists, confirms) a
// tenant container sized to sizeGB. created is false when an existing
// container was returned instead of a new one.
func (n *Node) CreateDatabase(ctx context.Context, id string, sizeGB int, password string) (container entities.Container, created bool, err error) {
	n.createMu.Lock()
	defer n.createMu.Unlock()

	existing, err := n.Docker.ListTenantContainers(ctx)
	if err != nil {
		return entities.Container{}, false, fmt.Errorf("list tenant containers: %w", err)
	}
	for _, c := range existing {
		if c.DatabaseID == id {
			return c, false, nil
		}
	}

	usedPorts, err := n.Docker.UsedHostPorts(ctx)
	if err != nil {
		return entities.Container{}, false, fmt.Errorf("list used ports: %w", err)
	}
	port, err := allocatePort(n.PortMin, n.PortMax, usedPorts)
	if err != nil {
		return entities.Container{}, false, ErrNoFreePort
	}

	hostDataDir, err := n.Volumes.Create(id, sizeGB)
	if err != nil {
		return entities.Container{}, false, fmt.Errorf("create volume: %w", err)
	}

	memoryBytes, nanoCPUs := resourceLimits(sizeGB)
	container, err = n.Docker.CreateTenantContainer(ctx, entities.ContainerSpec{
		DatabaseID:  id,
		Image:       n.Image,
		Password:    password,
		Port:        port,
		HostDataDir: hostDataDir,
		MemoryBytes: memoryBytes,
		NanoCPUs:    nanoCPUs,
		TLSCertPath: n.TenantTLSCertPath,
		TLSKeyPath:  n.TenantTLSKeyPath,
	})
	if err != nil {
		_ = n.Volumes.Remove(id)
		return entities.Container{}, false, fmt.Errorf("create container: %w", err)
	}

	return container, true, nil
}

// ResetDemo removes every tenant container and loop volume on this host,
// then sweeps orphan images and mounts. It is idempotent and is only
// reachable when demo mode is enabled.
func (n *Node) ResetDemo(ctx context.Context) (entities.ResetCounts, error) {
	n.createMu.Lock()
	defer n.createMu.Unlock()

	containers, err := n.Docker.ListTenantContainers(ctx)
	if err != nil {
		return entities.ResetCounts{}, fmt.Errorf("list tenant containers: %w", err)
	}

	var counts entities.ResetCounts
	var errs []error
	for _, c := range containers {
		if err := n.Docker.RemoveTenantContainer(ctx, c.ContainerID); err != nil {
			errs = append(errs, fmt.Errorf("remove container %s: %w", c.DatabaseID, err))
			continue
		}
		counts.Containers++
		if err := n.Volumes.Remove(c.DatabaseID); err != nil {
			errs = append(errs, fmt.Errorf("remove volume %s: %w", c.DatabaseID, err))
			continue
		}
		counts.Volumes++
	}

	remaining, err := n.Docker.ListTenantContainers(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("verify containers: %w", err))
	}
	keep := make(map[string]bool, len(remaining))
	for _, c := range remaining {
		keep[c.DatabaseID] = true
	}
	orphans, err := n.Volumes.CleanOrphans(ctx, keep)
	counts.Orphans = orphans
	if err != nil {
		errs = append(errs, fmt.Errorf("clean orphan volumes: %w", err))
	}
	if len(remaining) > 0 {
		errs = append(errs, fmt.Errorf("%d tenant container(s) remain", len(remaining)))
	}

	return counts, errors.Join(errs...)
}

// DeleteDatabase removes a tenant container and its volume. It is a
// no-op, not an error, if id does not exist.
func (n *Node) DeleteDatabase(ctx context.Context, id string) error {
	containers, err := n.Docker.ListTenantContainers(ctx)
	if err != nil {
		return fmt.Errorf("list tenant containers: %w", err)
	}

	var found *entities.Container
	for i := range containers {
		if containers[i].DatabaseID == id {
			found = &containers[i]
			break
		}
	}

	if found != nil {
		if err := n.Docker.RemoveTenantContainer(ctx, found.ContainerID); err != nil {
			return fmt.Errorf("remove container: %w", err)
		}
	}

	if err := n.Volumes.Remove(id); err != nil {
		return fmt.Errorf("remove volume: %w", err)
	}
	return nil
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

// portsInRange returns the subset of used that falls within [min, max].
func portsInRange(used map[int]bool, min, max int) []int {
	ports := make([]int, 0, len(used))
	for p := range used {
		if p >= min && p <= max {
			ports = append(ports, p)
		}
	}
	return ports
}
