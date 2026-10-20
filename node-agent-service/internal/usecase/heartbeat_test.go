package usecase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities"
)

// fakeHeartbeatDocker is a minimal repository.DockerClient stand-in for
// exercising tick's metrics-collection cadence, without a real Docker
// daemon or Postgres container.
type fakeHeartbeatDocker struct {
	containers  []entities.Container
	metrics     map[string]entities.ConnectionMetrics
	metricsErr  error
	collectCall int
}

func (f *fakeHeartbeatDocker) ListTenantContainers(context.Context) ([]entities.Container, error) {
	return f.containers, nil
}

func (f *fakeHeartbeatDocker) UsedHostPorts(context.Context) (map[int]bool, error) {
	return map[int]bool{}, nil
}

func (f *fakeHeartbeatDocker) CreateTenantContainer(context.Context, entities.ContainerSpec) (entities.Container, error) {
	return entities.Container{}, nil
}

func (f *fakeHeartbeatDocker) RemoveTenantContainer(context.Context, string) error {
	return nil
}

func (f *fakeHeartbeatDocker) CollectConnectionMetrics(_ context.Context, containerID string) (entities.ConnectionMetrics, error) {
	f.collectCall++
	if f.metricsErr != nil {
		return entities.ConnectionMetrics{}, f.metricsErr
	}
	return f.metrics[containerID], nil
}

func newTestNode(docker *fakeHeartbeatDocker) *Node {
	return &Node{
		Config: entities.Config{NodeID: "node-1", HostAddress: "10.0.0.5", PortMin: 21000, PortMax: 21999},
		Docker: docker,
	}
}

// decodeHeartbeatBody is a local copy of heartbeatRequest's shape, kept
// separate from the production struct so the test fails if the wire
// format silently changes.
type decodedHeartbeat struct {
	Containers []struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Metrics *struct {
			Active                       int   `json:"active"`
			Idle                         int   `json:"idle"`
			IdleInTransaction            int   `json:"idle_in_transaction"`
			MaxConnections               int   `json:"max_connections"`
			SuperuserReservedConnections int   `json:"superuser_reserved_connections"`
			MemoryLimitBytes             int64 `json:"memory_limit_bytes"`
		} `json:"metrics"`
	} `json:"containers"`
}

func TestTickCollectsMetricsOnFirstTickOnly(t *testing.T) {
	var bodies []decodedHeartbeat
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body decodedHeartbeat
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode heartbeat body: %v", err)
		}
		bodies = append(bodies, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	docker := &fakeHeartbeatDocker{
		containers: []entities.Container{{ContainerID: "c-a", DatabaseID: "db-a", Port: 21000, Status: "running"}},
		metrics:    map[string]entities.ConnectionMetrics{"c-a": {Active: 2, Idle: 1, MaxConnections: 100, SuperuserReservedConnections: 3, MemoryLimitBytes: 512 * 1024 * 1024}},
	}
	n := newTestNode(docker)

	const metricsInterval = time.Hour
	n.tick(context.Background(), server.URL, "token", metricsInterval)
	n.tick(context.Background(), server.URL, "token", metricsInterval)

	if docker.collectCall != 1 {
		t.Fatalf("CollectConnectionMetrics called %d time(s), want exactly 1 (second tick within metricsInterval must skip it)", docker.collectCall)
	}
	if len(bodies) != 2 {
		t.Fatalf("got %d heartbeat(s), want 2", len(bodies))
	}

	first := bodies[0].Containers[0]
	if first.Metrics == nil {
		t.Fatalf("first heartbeat container = %+v, want metrics attached", first)
	}
	if first.Metrics.Active != 2 || first.Metrics.MaxConnections != 100 || first.Metrics.MemoryLimitBytes != 512*1024*1024 {
		t.Fatalf("first heartbeat metrics = %+v, unexpected", first.Metrics)
	}

	second := bodies[1].Containers[0]
	if second.Metrics != nil {
		t.Fatalf("second heartbeat container = %+v, want no metrics (within metricsInterval of the first collection)", second)
	}
}

func TestTickSkipsMetricsForNonRunningContainers(t *testing.T) {
	var body decodedHeartbeat
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode heartbeat body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	docker := &fakeHeartbeatDocker{
		containers: []entities.Container{{ContainerID: "c-b", DatabaseID: "db-b", Port: 21000, Status: "exited"}},
	}
	n := newTestNode(docker)

	n.tick(context.Background(), server.URL, "token", time.Hour)

	if docker.collectCall != 0 {
		t.Fatalf("CollectConnectionMetrics called %d time(s), want 0 for a non-running container", docker.collectCall)
	}
	if body.Containers[0].Metrics != nil {
		t.Fatalf("container = %+v, want no metrics for a non-running container", body.Containers[0])
	}
}

// TestTickSurvivesFailedCollection covers the acceptance criterion that a
// failed collection does not break the heartbeat: the container must
// still be reported, just without metrics.
func TestTickSurvivesFailedCollection(t *testing.T) {
	var statusCode int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body decodedHeartbeat
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode heartbeat body: %v", err)
		}
		if len(body.Containers) != 1 || body.Containers[0].ID != "db-c" {
			t.Fatalf("heartbeat containers = %+v, want exactly db-c", body.Containers)
		}
		if body.Containers[0].Metrics != nil {
			t.Fatalf("container = %+v, want no metrics after a failed collection", body.Containers[0])
		}
		statusCode = http.StatusOK
		w.WriteHeader(statusCode)
	}))
	defer server.Close()

	docker := &fakeHeartbeatDocker{
		containers: []entities.Container{{ContainerID: "c-c", DatabaseID: "db-c", Port: 21000, Status: "running"}},
		metricsErr: context.DeadlineExceeded,
	}
	n := newTestNode(docker)

	n.tick(context.Background(), server.URL, "token", time.Hour)

	if statusCode != http.StatusOK {
		t.Fatalf("heartbeat was not sent after a failed metrics collection")
	}
}
