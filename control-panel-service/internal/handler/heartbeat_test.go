package handler_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"
)

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func insertNodeWithToken(t *testing.T, db *sql.DB, id, token string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO nodes (id, name, address, token_hash) VALUES ($1, $1, 'https://agent.invalid:9443', $2)
	`, id, hashToken(token))
	if err != nil {
		t.Fatalf("insert node fixture: %v", err)
	}
}

func heartbeat(t *testing.T, serverURL, token string, body map[string]any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal heartbeat body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, serverURL+"/api/agent/heartbeat", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("build heartbeat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/agent/heartbeat: %v", err)
	}
	return resp
}

func TestAgentHeartbeatRejectsMissingOrUnknownToken(t *testing.T) {
	server, _, db := newTestServer(t)
	insertNodeWithToken(t, db, "node-auth", "the-real-token")

	resp := heartbeat(t, server.URL, "", map[string]any{"node_id": "node-auth"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("heartbeat without a token = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	_ = resp.Body.Close()

	resp = heartbeat(t, server.URL, "not-the-real-token", map[string]any{"node_id": "node-auth"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("heartbeat with an unknown token = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	_ = resp.Body.Close()
}

func TestAgentHeartbeatUpdatesCapacityAndUpsertsContainers(t *testing.T) {
	server, _, db := newTestServer(t)
	insertNodeWithToken(t, db, "node-hb", "hb-token")

	databaseID := insertRunningDatabaseForNode(t, db, "node-hb")

	resp := heartbeat(t, server.URL, "hb-token", map[string]any{
		"node_id": "node-hb",
		"host":    "10.0.0.5",
		"capacity": map[string]any{
			"port_range_min": 21000,
			"port_range_max": 21999,
			"ports_total":    1000,
			"ports_used":     1,
		},
		"containers": []map[string]any{
			{"id": databaseID, "host": "10.0.0.5", "port": 21000, "status": "running"},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	var capacityJSON []byte
	var lastHeartbeatSet bool
	if err := db.QueryRow(`
		SELECT capacity, last_heartbeat IS NOT NULL FROM nodes WHERE id = 'node-hb'
	`).Scan(&capacityJSON, &lastHeartbeatSet); err != nil {
		t.Fatalf("look up node: %v", err)
	}
	if !lastHeartbeatSet {
		t.Fatalf("last_heartbeat was not set by the heartbeat")
	}
	var capacity map[string]any
	if err := json.Unmarshal(capacityJSON, &capacity); err != nil {
		t.Fatalf("unmarshal capacity: %v", err)
	}
	if capacity["ports_used"] != float64(1) {
		t.Fatalf("capacity.ports_used = %v, want 1", capacity["ports_used"])
	}

	var containerCount int
	if err := db.QueryRow(`
		SELECT count(*) FROM node_containers WHERE node_id = 'node-hb' AND database_id = $1 AND status = 'running' AND port = 21000
	`, databaseID).Scan(&containerCount); err != nil {
		t.Fatalf("count node_containers: %v", err)
	}
	if containerCount != 1 {
		t.Fatalf("node_containers rows for %s = %d, want 1", databaseID, containerCount)
	}

	// A second heartbeat that no longer reports the container removes its
	// node_containers row (the node no longer sees it) but does not
	// touch the databases row itself.
	resp = heartbeat(t, server.URL, "hb-token", map[string]any{
		"node_id": "node-hb",
		"host":    "10.0.0.5",
		"capacity": map[string]any{
			"port_range_min": 21000,
			"port_range_max": 21999,
			"ports_total":    1000,
			"ports_used":     0,
		},
		"containers": []map[string]any{},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second heartbeat = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	if err := db.QueryRow(`
		SELECT count(*) FROM node_containers WHERE node_id = 'node-hb'
	`).Scan(&containerCount); err != nil {
		t.Fatalf("count node_containers after reconcile: %v", err)
	}
	if containerCount != 0 {
		t.Fatalf("node_containers rows after reconcile = %d, want 0 (stale row not removed)", containerCount)
	}
}

// TestAgentHeartbeatRecordsVersion covers the agent reporting its build
// version in the heartbeat body, so the control plane can store it on
// the node record for GET /api/admin/nodes.
func TestAgentHeartbeatRecordsVersion(t *testing.T) {
	server, _, db := newTestServer(t)
	insertNodeWithToken(t, db, "node-version", "version-token")

	resp := heartbeat(t, server.URL, "version-token", map[string]any{
		"node_id": "node-version",
		"host":    "10.0.0.5",
		"version": "abc1234",
		"capacity": map[string]any{
			"port_range_min": 21000, "port_range_max": 21999, "ports_total": 1000, "ports_used": 0,
		},
		"containers": []map[string]any{},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat with version = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	var version string
	if err := db.QueryRow(`SELECT version FROM nodes WHERE id = 'node-version'`).Scan(&version); err != nil {
		t.Fatalf("look up node: %v", err)
	}
	if version != "abc1234" {
		t.Fatalf("nodes.version = %q, want %q", version, "abc1234")
	}
}

// TestAgentHeartbeatRecordsConnectionMetrics covers the agent attaching a
// metrics snapshot to a container entry: the control plane must record it
// in database_metrics.
func TestAgentHeartbeatRecordsConnectionMetrics(t *testing.T) {
	server, _, db := newTestServer(t)
	insertNodeWithToken(t, db, "node-metrics", "metrics-token")
	databaseID := insertRunningDatabaseForNode(t, db, "node-metrics")

	resp := heartbeat(t, server.URL, "metrics-token", map[string]any{
		"node_id": "node-metrics",
		"host":    "10.0.0.5",
		"capacity": map[string]any{
			"port_range_min": 21000, "port_range_max": 21999, "ports_total": 1000, "ports_used": 1,
		},
		"containers": []map[string]any{
			{
				"id": databaseID, "host": "10.0.0.5", "port": 21000, "status": "running",
				"metrics": map[string]any{
					"active": 71, "idle": 2, "idle_in_transaction": 0,
					"max_connections": 100, "superuser_reserved_connections": 3,
					"memory_limit_bytes": 536870912, "collected_at": "2026-01-01T00:00:00Z",
				},
			},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat with metrics = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	var active, maxConnections int
	if err := db.QueryRow(`
		SELECT active, max_connections FROM database_metrics WHERE database_id = $1
	`, databaseID).Scan(&active, &maxConnections); err != nil {
		t.Fatalf("look up recorded metric: %v", err)
	}
	if active != 71 || maxConnections != 100 {
		t.Fatalf("recorded metric active/max = %d/%d, want 71/100", active, maxConnections)
	}
}

func insertRunningDatabaseForNode(t *testing.T, db *sql.DB, nodeID string) string {
	t.Helper()
	var userID string
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash) VALUES ('hb-owner@example.com', 'x') RETURNING id
	`).Scan(&userID); err != nil {
		t.Fatalf("insert owner for heartbeat fixture: %v", err)
	}
	var id string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status, node_id)
		VALUES ($1, 'hb-demo', 5, 'running', $2)
		RETURNING id
	`, userID, nodeID).Scan(&id); err != nil {
		t.Fatalf("insert running database fixture: %v", err)
	}
	return id
}
