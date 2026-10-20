package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/handler"
	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/usecase"
)

// fakeDocker is an in-memory stand-in for the Docker SDK so handlers can
// be tested without a Docker daemon.
type fakeDocker struct {
	mu         sync.Mutex
	containers map[string]entities.Container
	usedPorts  map[int]bool
	nextID     int
	createErr  error
	// startErr simulates ContainerCreate succeeding but ContainerStart
	// failing: the real client must remove the created container in that
	// case, so the fake mirrors that contract by never registering one.
	startErr error
}

func newFakeDocker() *fakeDocker {
	return &fakeDocker{containers: map[string]entities.Container{}, usedPorts: map[int]bool{}}
}

func (f *fakeDocker) ListTenantContainers(context.Context) ([]entities.Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]entities.Container, 0, len(f.containers))
	for _, c := range f.containers {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeDocker) UsedHostPorts(context.Context) (map[int]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	used := make(map[int]bool, len(f.usedPorts))
	for p := range f.usedPorts {
		used[p] = true
	}
	return used, nil
}

func (f *fakeDocker) CreateTenantContainer(_ context.Context, spec entities.ContainerSpec) (entities.Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return entities.Container{}, f.createErr
	}
	if f.startErr != nil {
		// Mirrors sdkClient.CreateTenantContainer: ContainerCreate
		// succeeded (nextID advances, as Docker would assign a new
		// container ID) but ContainerStart failed, so the container is
		// removed before returning and never becomes listable.
		f.nextID++
		return entities.Container{}, f.startErr
	}
	f.nextID++
	c := entities.Container{
		ContainerID: fmt.Sprintf("container-%d", f.nextID),
		DatabaseID:  spec.DatabaseID,
		Port:        spec.Port,
		Status:      "running",
	}
	f.containers[c.ContainerID] = c
	f.usedPorts[spec.Port] = true
	return c, nil
}

func (f *fakeDocker) RemoveTenantContainer(_ context.Context, containerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.containers[containerID]; ok {
		delete(f.usedPorts, c.Port)
		delete(f.containers, containerID)
	}
	return nil
}

// CollectConnectionMetrics is unused by this package's handler-level
// tests, which never run the heartbeat loop; it exists only so fakeDocker
// satisfies repository.DockerClient.
func (f *fakeDocker) CollectConnectionMetrics(context.Context, string) (entities.ConnectionMetrics, error) {
	return entities.ConnectionMetrics{}, fmt.Errorf("not implemented")
}

// fakeVolumes is an in-memory stand-in for the loop-mounted ext4 volumes,
// so handlers can be tested without root privileges.
type fakeVolumes struct {
	mu        sync.Mutex
	created   map[string]int
	removeErr error
}

func newFakeVolumes() *fakeVolumes {
	return &fakeVolumes{created: map[string]int{}}
}

func (v *fakeVolumes) Create(id string, sizeGB int) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.created[id] = sizeGB
	return "/fake/mounts/" + id, nil
}

func (v *fakeVolumes) Remove(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.removeErr != nil {
		return v.removeErr
	}
	delete(v.created, id)
	return nil
}

func (v *fakeVolumes) has(id string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, ok := v.created[id]
	return ok
}

func newTestServer(docker *fakeDocker, volumes *fakeVolumes) *httptest.Server {
	n := &usecase.Node{
		Config: entities.Config{
			NodeID:      "node-1",
			HostAddress: "10.0.0.5",
			Image:       "postgres:17",
			PortMin:     21000,
			PortMax:     21002,
			Version:     "test-commit",
		},
		Docker:  docker,
		Volumes: volumes,
	}
	return httptest.NewServer(handler.NewMux(n))
}

func postJSON(t *testing.T, url string, body map[string]any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func decode(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

func TestHealthz(t *testing.T) {
	server := newTestServer(newFakeDocker(), newFakeVolumes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	var body map[string]string
	decode(t, resp, &body)
	if body["status"] != "ok" {
		t.Fatalf("healthz status = %q, want ok", body["status"])
	}
}

func TestGetNode(t *testing.T) {
	docker := newFakeDocker()
	server := newTestServer(docker, newFakeVolumes())
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/databases", map[string]any{
		"id": "db-a", "size_gb": 5, "password": "secret",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /v1/databases = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	_ = resp.Body.Close()

	resp, err := http.Get(server.URL + "/v1/node")
	if err != nil {
		t.Fatalf("GET /v1/node: %v", err)
	}
	var out struct {
		NodeID   string `json:"node_id"`
		Host     string `json:"host"`
		Version  string `json:"version"`
		Capacity struct {
			PortRangeMin int `json:"port_range_min"`
			PortRangeMax int `json:"port_range_max"`
			PortsTotal   int `json:"ports_total"`
			PortsUsed    int `json:"ports_used"`
		} `json:"capacity"`
		UsedPorts  []int `json:"used_ports"`
		Containers []struct {
			ID string `json:"id"`
		} `json:"containers"`
	}
	decode(t, resp, &out)

	if out.NodeID != "node-1" || out.Host != "10.0.0.5" || out.Version != "test-commit" {
		t.Fatalf("GET /v1/node = %+v, want node-1/10.0.0.5/test-commit", out)
	}
	if out.Capacity.PortRangeMin != 21000 || out.Capacity.PortRangeMax != 21002 || out.Capacity.PortsTotal != 3 {
		t.Fatalf("GET /v1/node capacity = %+v, want range 21000-21002", out.Capacity)
	}
	if out.Capacity.PortsUsed != 1 || len(out.UsedPorts) != 1 {
		t.Fatalf("GET /v1/node used ports = %+v, want exactly one", out)
	}
	if len(out.Containers) != 1 || out.Containers[0].ID != "db-a" {
		t.Fatalf("GET /v1/node containers = %+v, want exactly db-a", out.Containers)
	}
}

func TestCreateDatabaseIsIdempotent(t *testing.T) {
	docker := newFakeDocker()
	volumes := newFakeVolumes()
	server := newTestServer(docker, volumes)
	defer server.Close()

	reqBody := map[string]any{"id": "db-b", "size_gb": 10, "password": "hunter2"}

	resp := postJSON(t, server.URL+"/v1/databases", reqBody)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first POST /v1/databases = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var first databaseResponse
	decode(t, resp, &first)
	if first.ID != "db-b" || first.Host != "10.0.0.5" || first.Status != "running" || first.Port < 21000 || first.Port > 21002 {
		t.Fatalf("create response = %+v, unexpected", first)
	}
	if !volumes.has("db-b") {
		t.Fatalf("volume for db-b was not created")
	}

	resp = postJSON(t, server.URL+"/v1/databases", reqBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("repeat POST /v1/databases = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var second databaseResponse
	decode(t, resp, &second)
	if second != first {
		t.Fatalf("repeat create response = %+v, want identical %+v", second, first)
	}
	if docker.nextID != 1 {
		t.Fatalf("repeat POST created a second container (nextID = %d)", docker.nextID)
	}
}

func TestCreateDatabaseAllocatesFreePort(t *testing.T) {
	docker := newFakeDocker()
	docker.usedPorts[21000] = true
	server := newTestServer(docker, newFakeVolumes())
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/databases", map[string]any{
		"id": "db-c", "size_gb": 1, "password": "pw",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /v1/databases = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out databaseResponse
	decode(t, resp, &out)
	if out.Port != 21001 {
		t.Fatalf("allocated port = %d, want 21001 (21000 already used)", out.Port)
	}
}

func TestCreateDatabaseRejectsInvalidBody(t *testing.T) {
	server := newTestServer(newFakeDocker(), newFakeVolumes())
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/databases", map[string]any{"id": "", "size_gb": 0, "password": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /v1/databases with empty body = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	_ = resp.Body.Close()
}

func TestCreateDatabaseCleansUpVolumeOnContainerFailure(t *testing.T) {
	docker := newFakeDocker()
	docker.createErr = fmt.Errorf("docker is on fire")
	volumes := newFakeVolumes()
	server := newTestServer(docker, volumes)
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/databases", map[string]any{
		"id": "db-d", "size_gb": 5, "password": "pw",
	})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("POST /v1/databases with failing docker = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	if volumes.has("db-d") {
		t.Fatalf("volume for db-d was not cleaned up after container creation failed")
	}
}

// TestCreateDatabaseRetryableAfterStartFailure covers ContainerCreate
// succeeding but ContainerStart failing: no container or volume should
// survive, and the id must not get stuck as "already exists" — a repeat
// POST with the same id must retry creation rather than treating the
// failed attempt as idempotent success.
func TestCreateDatabaseRetryableAfterStartFailure(t *testing.T) {
	docker := newFakeDocker()
	docker.startErr = fmt.Errorf("container created but failed to start")
	volumes := newFakeVolumes()
	server := newTestServer(docker, volumes)
	defer server.Close()

	reqBody := map[string]any{"id": "db-f", "size_gb": 5, "password": "pw"}

	resp := postJSON(t, server.URL+"/v1/databases", reqBody)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("POST /v1/databases with failing start = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	if volumes.has("db-f") {
		t.Fatalf("volume for db-f was not cleaned up after container start failed")
	}

	resp, err := http.Get(server.URL + "/v1/databases")
	if err != nil {
		t.Fatalf("GET /v1/databases: %v", err)
	}
	var list []databaseResponse
	decode(t, resp, &list)
	if len(list) != 0 {
		t.Fatalf("GET /v1/databases after failed start = %+v, want no orphaned container", list)
	}

	docker.startErr = nil
	resp = postJSON(t, server.URL+"/v1/databases", reqBody)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("retry POST /v1/databases after clearing start error = %d, want %d (id must be retryable, not stuck)", resp.StatusCode, http.StatusCreated)
	}
	var out databaseResponse
	decode(t, resp, &out)
	if out.ID != "db-f" || out.Status != "running" {
		t.Fatalf("retry create response = %+v, unexpected", out)
	}
	if !volumes.has("db-f") {
		t.Fatalf("volume for db-f was not recreated on retry")
	}
}

func TestListAndDeleteDatabase(t *testing.T) {
	docker := newFakeDocker()
	volumes := newFakeVolumes()
	server := newTestServer(docker, volumes)
	defer server.Close()

	resp := postJSON(t, server.URL+"/v1/databases", map[string]any{
		"id": "db-e", "size_gb": 5, "password": "pw",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /v1/databases = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	_ = resp.Body.Close()

	resp, err := http.Get(server.URL + "/v1/databases")
	if err != nil {
		t.Fatalf("GET /v1/databases: %v", err)
	}
	var list []databaseResponse
	decode(t, resp, &list)
	if len(list) != 1 || list[0].ID != "db-e" {
		t.Fatalf("GET /v1/databases = %+v, want exactly db-e", list)
	}

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/databases/db-e", nil)
	if err != nil {
		t.Fatalf("build delete request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /v1/databases/db-e: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /v1/databases/db-e = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	_ = resp.Body.Close()
	if volumes.has("db-e") {
		t.Fatalf("volume for db-e was not removed")
	}

	// Deleting again is idempotent.
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("second DELETE /v1/databases/db-e: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("second DELETE /v1/databases/db-e = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	_ = resp.Body.Close()
}

func TestSnapshotRoutesAreGated(t *testing.T) {
	server := newTestServer(newFakeDocker(), newFakeVolumes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/v1/databases/db-x/snapshots")
	if err != nil {
		t.Fatalf("GET snapshots: %v", err)
	}
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("GET snapshots = %d, want %d", resp.StatusCode, http.StatusNotImplemented)
	}
	_ = resp.Body.Close()

	resp = postJSON(t, server.URL+"/v1/databases/db-x/snapshots", map[string]any{})
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("POST snapshots = %d, want %d", resp.StatusCode, http.StatusNotImplemented)
	}
	_ = resp.Body.Close()
}

type databaseResponse struct {
	ID     string `json:"id"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Status string `json:"status"`
}
