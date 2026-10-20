package api_test

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/internal/routes"
)

func doRequest(t *testing.T, client *http.Client, method, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

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

func TestAdminWriteRoutesRejectNonAdmin(t *testing.T) {
	server, client, _ := newTestServer(t)
	signUp(t, server, client, "plain-write-user@example.com")

	resp := postJSON(t, client, server.URL+"/api/admin/databases", map[string]any{
		"user_id": "00000000-0000-0000-0000-000000000000", "name": "demo", "size_gb": 5,
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /api/admin/databases for non-admin = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	_ = resp.Body.Close()

	resp = doRequest(t, client, http.MethodDelete, server.URL+"/api/admin/databases/00000000-0000-0000-0000-000000000000")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("DELETE /api/admin/databases/{id} for non-admin = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	_ = resp.Body.Close()

	resp = doRequest(t, client, http.MethodPost, server.URL+"/api/admin/databases/00000000-0000-0000-0000-000000000000/retry")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /api/admin/databases/{id}/retry for non-admin = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	_ = resp.Body.Close()
}

func TestAdminCreateDatabaseProvisionsForGivenUser(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-create@example.com")
	promoteToAdmin(t, db, "admin-create@example.com")
	insertNode(t, db, "node-admin-create")

	var ownerID string
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash) VALUES ('admin-create-owner@example.com', 'x') RETURNING id
	`).Scan(&ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}

	resp := postJSON(t, client, server.URL+"/api/admin/databases", map[string]any{
		"user_id": ownerID, "name": "admin-made", "size_gb": 5,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/admin/databases = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created struct {
		ID               string `json:"id"`
		Host             string `json:"host"`
		Port             int    `json:"port"`
		Database         string `json:"database"`
		User             string `json:"user"`
		Password         string `json:"password"`
		ConnectionString string `json:"connection_string"`
		JDBCURL          string `json:"jdbc_url"`
	}
	decode(t, resp, &created)
	if created.ID == "" {
		t.Fatalf("create response id is empty")
	}
	if len(created.Password) != 32 {
		t.Fatalf("create response password length = %d, want 32", len(created.Password))
	}

	var userID, status, slug string
	if err := db.QueryRow(`SELECT user_id, status, slug FROM databases WHERE id = $1`, created.ID).Scan(&userID, &status, &slug); err != nil {
		t.Fatalf("look up created database: %v", err)
	}
	if userID != ownerID {
		t.Fatalf("created database user_id = %q, want %q", userID, ownerID)
	}
	if status != "provisioning" {
		t.Fatalf("created database status = %q, want provisioning", status)
	}
	if want := routes.PublicHost(slug); created.Host != want {
		t.Fatalf("create response host = %q, want %q", created.Host, want)
	}
	if created.Port != routes.PublicPort {
		t.Fatalf("create response port = %d, want %d", created.Port, routes.PublicPort)
	}
	if created.Database != "postgres" {
		t.Fatalf("create response database = %q, want postgres", created.Database)
	}
	if created.User != "postgres" {
		t.Fatalf("create response user = %q, want postgres", created.User)
	}
	if want := routes.ConnectionString(slug, created.Password); created.ConnectionString != want {
		t.Fatalf("create response connection_string = %q, want %q", created.ConnectionString, want)
	}
	if want := routes.JDBCConnectionString(slug); created.JDBCURL != want {
		t.Fatalf("create response jdbc_url = %q, want %q", created.JDBCURL, want)
	}

	var jobCount int
	if err := db.QueryRow(`
		SELECT count(*) FROM jobs WHERE kind = 'provision' AND payload->>'database_id' = $1
	`, created.ID).Scan(&jobCount); err != nil {
		t.Fatalf("count provision jobs: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("provision jobs queued = %d, want 1", jobCount)
	}
}

func TestAdminCreateDatabaseValidatesSizeGB(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-create-sizes@example.com")
	promoteToAdmin(t, db, "admin-create-sizes@example.com")
	insertNode(t, db, "node-admin-sizes")

	var ownerID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "admin-create-sizes@example.com").Scan(&ownerID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}

	for _, size := range []int{0, 21, -1} {
		resp := postJSON(t, client, server.URL+"/api/admin/databases", map[string]any{
			"user_id": ownerID, "name": "demo", "size_gb": size,
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("POST /api/admin/databases with size_gb=%d = %d, want %d", size, resp.StatusCode, http.StatusBadRequest)
		}
		_ = resp.Body.Close()
	}
}

func TestAdminCreateDatabaseUnknownUserIsNotFound(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-create-unknown@example.com")
	promoteToAdmin(t, db, "admin-create-unknown@example.com")
	insertNode(t, db, "node-admin-unknown")

	resp := postJSON(t, client, server.URL+"/api/admin/databases", map[string]any{
		"user_id": "00000000-0000-0000-0000-000000000000", "name": "demo", "size_gb": 5,
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST /api/admin/databases for unknown user = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	_ = resp.Body.Close()
}

func TestAdminDeleteDatabaseForcesAnyOwnersRunningOrFailedRow(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-delete@example.com")
	promoteToAdmin(t, db, "admin-delete@example.com")

	var ownerID string
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash) VALUES ('admin-delete-owner@example.com', 'x') RETURNING id
	`).Scan(&ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	var databaseID string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status, host, port)
		VALUES ($1, 'owned-elsewhere', 5, 'running', '127.0.0.1', 21000)
		RETURNING id
	`, ownerID).Scan(&databaseID); err != nil {
		t.Fatalf("insert running database fixture: %v", err)
	}

	resp := doRequest(t, client, http.MethodDelete, server.URL+"/api/admin/databases/"+databaseID)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("DELETE /api/admin/databases/{id} = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	var deleted map[string]any
	decode(t, resp, &deleted)
	if deleted["status"] != "deleting" {
		t.Fatalf("delete response status = %v, want deleting", deleted["status"])
	}

	var jobCount int
	if err := db.QueryRow(`
		SELECT count(*) FROM jobs WHERE kind = 'delete' AND payload->>'database_id' = $1
	`, databaseID).Scan(&jobCount); err != nil {
		t.Fatalf("count delete jobs: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("delete jobs queued = %d, want 1", jobCount)
	}
}

func TestAdminDeleteDatabaseMissingIsNotFound(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-delete-missing@example.com")
	promoteToAdmin(t, db, "admin-delete-missing@example.com")

	resp := doRequest(t, client, http.MethodDelete, server.URL+"/api/admin/databases/00000000-0000-0000-0000-000000000000")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("DELETE /api/admin/databases/{id} for missing id = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	_ = resp.Body.Close()
}

func TestAdminDeleteDatabaseWrongStateIsConflict(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-delete-wrong-state@example.com")
	promoteToAdmin(t, db, "admin-delete-wrong-state@example.com")

	var ownerID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "admin-delete-wrong-state@example.com").Scan(&ownerID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}
	var databaseID string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status) VALUES ($1, 'still-provisioning', 5, 'provisioning') RETURNING id
	`, ownerID).Scan(&databaseID); err != nil {
		t.Fatalf("insert provisioning database fixture: %v", err)
	}

	resp := doRequest(t, client, http.MethodDelete, server.URL+"/api/admin/databases/"+databaseID)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("DELETE /api/admin/databases/{id} for a provisioning row = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	_ = resp.Body.Close()
}

func TestAdminRetryDatabaseReprovisionsAFailedProvision(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-retry-provision@example.com")
	promoteToAdmin(t, db, "admin-retry-provision@example.com")

	var ownerID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "admin-retry-provision@example.com").Scan(&ownerID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}
	var databaseID string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status) VALUES ($1, 'never-ran', 5, 'failed') RETURNING id
	`, ownerID).Scan(&databaseID); err != nil {
		t.Fatalf("insert failed database fixture: %v", err)
	}

	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/admin/databases/"+databaseID+"/retry")
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /api/admin/databases/{id}/retry = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	var retried map[string]any
	decode(t, resp, &retried)
	if retried["status"] != "provisioning" {
		t.Fatalf("retry response status = %v, want provisioning", retried["status"])
	}

	var jobCount int
	if err := db.QueryRow(`
		SELECT count(*) FROM jobs WHERE kind = 'provision' AND payload->>'database_id' = $1
	`, databaseID).Scan(&jobCount); err != nil {
		t.Fatalf("count provision jobs: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("provision jobs queued = %d, want 1", jobCount)
	}
}

func TestAdminRetryDatabaseRequeuesAFailedDelete(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-retry-delete@example.com")
	promoteToAdmin(t, db, "admin-retry-delete@example.com")

	var ownerID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "admin-retry-delete@example.com").Scan(&ownerID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}
	var databaseID string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status, host, port)
		VALUES ($1, 'delete-kept-failing', 5, 'failed', '127.0.0.1', 21000)
		RETURNING id
	`, ownerID).Scan(&databaseID); err != nil {
		t.Fatalf("insert failed database fixture: %v", err)
	}

	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/admin/databases/"+databaseID+"/retry")
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /api/admin/databases/{id}/retry = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	var retried map[string]any
	decode(t, resp, &retried)
	if retried["status"] != "deleting" {
		t.Fatalf("retry response status = %v, want deleting", retried["status"])
	}

	var jobCount int
	if err := db.QueryRow(`
		SELECT count(*) FROM jobs WHERE kind = 'delete' AND payload->>'database_id' = $1
	`, databaseID).Scan(&jobCount); err != nil {
		t.Fatalf("count delete jobs: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("delete jobs queued = %d, want 1", jobCount)
	}
}

func TestAdminRetryDatabaseMissingIsNotFound(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-retry-missing@example.com")
	promoteToAdmin(t, db, "admin-retry-missing@example.com")

	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/admin/databases/00000000-0000-0000-0000-000000000000/retry")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST /api/admin/databases/{id}/retry for missing id = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	_ = resp.Body.Close()
}

func TestAdminRetryDatabaseWrongStateIsConflict(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-retry-wrong-state@example.com")
	promoteToAdmin(t, db, "admin-retry-wrong-state@example.com")

	var ownerID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "admin-retry-wrong-state@example.com").Scan(&ownerID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}
	var databaseID string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status, host, port)
		VALUES ($1, 'still-running', 5, 'running', '127.0.0.1', 21000)
		RETURNING id
	`, ownerID).Scan(&databaseID); err != nil {
		t.Fatalf("insert running database fixture: %v", err)
	}

	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/admin/databases/"+databaseID+"/retry")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("POST /api/admin/databases/{id}/retry for a running row = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	_ = resp.Body.Close()
}
