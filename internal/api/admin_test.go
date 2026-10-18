package api_test

import (
	"database/sql"
	"net/http"
	"testing"
)

func promoteToAdmin(t *testing.T, db *sql.DB, email string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE users SET is_admin = true WHERE email = $1`, email); err != nil {
		t.Fatalf("promote %s to admin: %v", email, err)
	}
}

func TestAdminRoutesRejectNonAdmin(t *testing.T) {
	server, client, _ := newTestServer(t)
	signUp(t, server, client, "plain-user@example.com")

	for _, path := range []string{
		"/api/admin/overview",
		"/api/admin/users",
		"/api/admin/databases",
		"/api/admin/nodes",
		"/api/admin/jobs",
	} {
		resp, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("GET %s for non-admin = %d, want %d", path, resp.StatusCode, http.StatusForbidden)
		}
		_ = resp.Body.Close()
	}
}

func TestAdminRoutesRejectAnonymous(t *testing.T) {
	server, client, _ := newTestServer(t)

	resp, err := client.Get(server.URL + "/api/admin/overview")
	if err != nil {
		t.Fatalf("GET /api/admin/overview: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/admin/overview without a session = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMeReportsIsAdmin(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "maybe-admin@example.com")

	resp, err := client.Get(server.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	var body map[string]any
	decode(t, resp, &body)
	if body["is_admin"] != false {
		t.Fatalf("is_admin before promotion = %v, want false", body["is_admin"])
	}

	promoteToAdmin(t, db, "maybe-admin@example.com")

	resp, err = client.Get(server.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me after promotion: %v", err)
	}
	decode(t, resp, &body)
	if body["is_admin"] != true {
		t.Fatalf("is_admin after promotion = %v, want true", body["is_admin"])
	}
}

func TestAdminOverviewCountsAcrossTables(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-overview@example.com")
	promoteToAdmin(t, db, "admin-overview@example.com")

	var userID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "admin-overview@example.com").Scan(&userID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO databases (user_id, name, size_gb, status) VALUES ($1, 'running-db', 5, 'running'), ($1, 'deleted-db', 5, 'deleted')
	`, userID); err != nil {
		t.Fatalf("insert database fixtures: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO nodes (id, name, address, token_hash, last_heartbeat) VALUES ('fresh-node', 'fresh-node', 'https://agent.invalid:9443', 'fresh-hash', now())
	`); err != nil {
		t.Fatalf("insert fresh node fixture: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO nodes (id, name, address, token_hash, last_heartbeat) VALUES ('stale-node', 'stale-node', 'https://agent.invalid:9443', 'stale-hash', now() - interval '1 hour')
	`); err != nil {
		t.Fatalf("insert stale node fixture: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jobs (kind, status) VALUES ('provision', 'succeeded')`); err != nil {
		t.Fatalf("insert job fixture: %v", err)
	}

	resp, err := client.Get(server.URL + "/api/admin/overview")
	if err != nil {
		t.Fatalf("GET /api/admin/overview: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/admin/overview = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var overview struct {
		Users             int            `json:"users"`
		DatabasesByStatus map[string]int `json:"databases_by_status"`
		NodesFresh        int            `json:"nodes_fresh"`
		NodesStale        int            `json:"nodes_stale"`
		JobsByStatus      map[string]int `json:"jobs_by_status"`
	}
	decode(t, resp, &overview)

	if overview.Users != 1 {
		t.Fatalf("users = %d, want 1", overview.Users)
	}
	if overview.DatabasesByStatus["running"] != 1 || overview.DatabasesByStatus["deleted"] != 1 {
		t.Fatalf("databases_by_status = %v, want running=1 deleted=1", overview.DatabasesByStatus)
	}
	if overview.NodesFresh != 1 || overview.NodesStale != 1 {
		t.Fatalf("nodes fresh/stale = %d/%d, want 1/1", overview.NodesFresh, overview.NodesStale)
	}
	if overview.JobsByStatus["succeeded"] != 1 {
		t.Fatalf("jobs_by_status[succeeded] = %d, want 1", overview.JobsByStatus["succeeded"])
	}
}

func TestAdminUsersReportsDatabaseCount(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-users@example.com")
	promoteToAdmin(t, db, "admin-users@example.com")

	var userID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "admin-users@example.com").Scan(&userID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO databases (user_id, name, size_gb, status) VALUES ($1, 'a', 5, 'running'), ($1, 'b', 5, 'deleted')
	`, userID); err != nil {
		t.Fatalf("insert database fixtures: %v", err)
	}

	resp, err := client.Get(server.URL + "/api/admin/users")
	if err != nil {
		t.Fatalf("GET /api/admin/users: %v", err)
	}
	var rows []map[string]any
	decode(t, resp, &rows)
	if len(rows) != 1 {
		t.Fatalf("admin users rows = %d, want 1", len(rows))
	}
	if rows[0]["email"] != "admin-users@example.com" {
		t.Fatalf("admin users row email = %v, want admin-users@example.com", rows[0]["email"])
	}
	// database_count includes the deleted row: this is an audit view, not
	// the tenant's own list, which hides deleted rows.
	if rows[0]["database_count"] != float64(2) {
		t.Fatalf("database_count = %v, want 2", rows[0]["database_count"])
	}
}

func TestAdminDatabasesIncludesDeletedRowsAndOwner(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-dbs@example.com")
	promoteToAdmin(t, db, "admin-dbs@example.com")

	var userID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "admin-dbs@example.com").Scan(&userID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO databases (user_id, name, size_gb, status, host, port)
		VALUES ($1, 'gone', 5, 'deleted', '127.0.0.1', 21000)
	`, userID); err != nil {
		t.Fatalf("insert deleted database fixture: %v", err)
	}

	resp, err := client.Get(server.URL + "/api/admin/databases")
	if err != nil {
		t.Fatalf("GET /api/admin/databases: %v", err)
	}
	var rows []map[string]any
	decode(t, resp, &rows)
	if len(rows) != 1 {
		t.Fatalf("admin databases rows = %d, want 1 (including the deleted row)", len(rows))
	}
	if rows[0]["status"] != "deleted" {
		t.Fatalf("admin databases row status = %v, want deleted", rows[0]["status"])
	}
	if rows[0]["owner_email"] != "admin-dbs@example.com" {
		t.Fatalf("admin databases row owner_email = %v, want admin-dbs@example.com", rows[0]["owner_email"])
	}
}

func TestAdminNodesIncludesContainers(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-nodes@example.com")
	promoteToAdmin(t, db, "admin-nodes@example.com")
	insertNode(t, db, "admin-node")

	databaseID := insertRunningDatabaseForNode(t, db, "admin-node")
	if _, err := db.Exec(`
		INSERT INTO node_containers (node_id, database_id, status, port) VALUES ('admin-node', $1, 'running', 21000)
	`, databaseID); err != nil {
		t.Fatalf("insert node_containers fixture: %v", err)
	}

	resp, err := client.Get(server.URL + "/api/admin/nodes")
	if err != nil {
		t.Fatalf("GET /api/admin/nodes: %v", err)
	}
	var rows []map[string]any
	decode(t, resp, &rows)

	var found map[string]any
	for _, row := range rows {
		if row["id"] == "admin-node" {
			found = row
		}
	}
	if found == nil {
		t.Fatalf("admin nodes response missing admin-node: %v", rows)
	}
	containers, ok := found["containers"].([]any)
	if !ok || len(containers) != 1 {
		t.Fatalf("admin-node containers = %v, want exactly one", found["containers"])
	}
	container := containers[0].(map[string]any)
	if container["database_id"] != databaseID {
		t.Fatalf("container database_id = %v, want %v", container["database_id"], databaseID)
	}
}

func TestAdminJobsReturnsRecentJobs(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-jobs@example.com")
	promoteToAdmin(t, db, "admin-jobs@example.com")
	insertNode(t, db, "admin-jobs-node")

	resp := postJSON(t, client, server.URL+"/api/databases", map[string]any{"name": "job-demo", "size_gb": 5})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/databases = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	resp, err := client.Get(server.URL + "/api/admin/jobs")
	if err != nil {
		t.Fatalf("GET /api/admin/jobs: %v", err)
	}
	var rows []map[string]any
	decode(t, resp, &rows)
	if len(rows) != 1 {
		t.Fatalf("admin jobs rows = %d, want 1", len(rows))
	}
	if rows[0]["kind"] != "provision" {
		t.Fatalf("admin jobs row kind = %v, want provision", rows[0]["kind"])
	}
}
