package api_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/internal/api"
	"github.com/madevara24/monorepo-back-rooms/internal/storetest"
)

func newTestServer(t *testing.T) (*httptest.Server, *http.Client, *sql.DB) {
	t.Helper()
	db := storetest.Open(t)

	server := httptest.NewServer(api.NewMux(&api.API{DB: db}))
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return server, &http.Client{Jar: jar}, db
}

func postJSON(t *testing.T, client *http.Client, url string, body map[string]string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
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

func TestSignupLoginMeLogoutFlow(t *testing.T) {
	server, client, _ := newTestServer(t)

	// Weak password is rejected.
	resp := postJSON(t, client, server.URL+"/api/auth/signup", map[string]string{
		"email": "flow@example.com", "password": "short",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("signup with weak password = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	_ = resp.Body.Close()

	// Signup succeeds and starts a session.
	resp = postJSON(t, client, server.URL+"/api/auth/signup", map[string]string{
		"email": "flow@example.com", "password": "a-strong-password",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("signup = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var signupUser struct {
		Email string `json:"email"`
	}
	decode(t, resp, &signupUser)
	if signupUser.Email != "flow@example.com" {
		t.Fatalf("signup email = %q, want %q", signupUser.Email, "flow@example.com")
	}

	// Duplicate signup is rejected.
	resp = postJSON(t, client, server.URL+"/api/auth/signup", map[string]string{
		"email": "flow@example.com", "password": "a-strong-password",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate signup = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	_ = resp.Body.Close()

	// /api/me reflects the signed-in user from the signup session.
	resp, err := client.Get(server.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/me after signup = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	// Logout clears the session.
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/auth/logout", nil)
	if err != nil {
		t.Fatalf("build logout request: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/auth/logout: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	_ = resp.Body.Close()

	resp, err = client.Get(server.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me after logout: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/me after logout = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	_ = resp.Body.Close()

	// Login with wrong password fails, correct password succeeds.
	resp = postJSON(t, client, server.URL+"/api/auth/login", map[string]string{
		"email": "flow@example.com", "password": "wrong-password",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login with wrong password = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	_ = resp.Body.Close()

	resp = postJSON(t, client, server.URL+"/api/auth/login", map[string]string{
		"email": "flow@example.com", "password": "a-strong-password",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	resp, err = client.Get(server.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me after login: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/me after login = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()
}

func TestMeWithoutCookieIsUnauthorized(t *testing.T) {
	server, client, _ := newTestServer(t)

	resp, err := client.Get(server.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/me without a session = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func signUp(t *testing.T, server *httptest.Server, client *http.Client, email string) {
	t.Helper()
	resp := postJSON(t, client, server.URL+"/api/auth/signup", map[string]string{
		"email": email, "password": "a-strong-password",
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("signup for %s = %d, want %d", email, resp.StatusCode, http.StatusCreated)
	}
}

func TestDatabasesListGetAndGating(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "dbs@example.com")

	// Empty list for a brand new user.
	resp, err := client.Get(server.URL + "/api/databases")
	if err != nil {
		t.Fatalf("GET /api/databases: %v", err)
	}
	var list []map[string]any
	decode(t, resp, &list)
	if len(list) != 0 {
		t.Fatalf("GET /api/databases for new user = %v, want empty", list)
	}

	var userID, otherUserID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "dbs@example.com").Scan(&userID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash) VALUES ('owner-of-nothing@example.com', 'x') RETURNING id
	`).Scan(&otherUserID); err != nil {
		t.Fatalf("insert second user: %v", err)
	}

	var dbID string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status) VALUES ($1, 'demo', 5, 'running') RETURNING id
	`, userID).Scan(&dbID); err != nil {
		t.Fatalf("insert database fixture: %v", err)
	}
	var otherDBID string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status) VALUES ($1, 'not-yours', 5, 'running') RETURNING id
	`, otherUserID).Scan(&otherDBID); err != nil {
		t.Fatalf("insert second database fixture: %v", err)
	}

	resp, err = client.Get(server.URL + "/api/databases")
	if err != nil {
		t.Fatalf("GET /api/databases: %v", err)
	}
	decode(t, resp, &list)
	if len(list) != 1 || list[0]["name"] != "demo" {
		t.Fatalf("GET /api/databases = %v, want exactly one row named demo", list)
	}

	resp, err = client.Get(server.URL + "/api/databases/" + dbID)
	if err != nil {
		t.Fatalf("GET /api/databases/{id}: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/databases/{id} for own database = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	resp, err = client.Get(server.URL + "/api/databases/" + otherDBID)
	if err != nil {
		t.Fatalf("GET /api/databases/{id}: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /api/databases/{id} for another user's database = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	_ = resp.Body.Close()

	resp = postJSON(t, client, server.URL+"/api/databases", map[string]string{"name": "new", "size_gb": "5"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("POST /api/databases = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	_ = resp.Body.Close()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/databases/"+dbID, nil)
	if err != nil {
		t.Fatalf("build delete request: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/databases/{id}: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("DELETE /api/databases/{id} = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	_ = resp.Body.Close()

	resp, err = client.Get(server.URL + "/api/databases/" + dbID + "/snapshots")
	if err != nil {
		t.Fatalf("GET /api/databases/{id}/snapshots: %v", err)
	}
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("GET /api/databases/{id}/snapshots = %d, want %d", resp.StatusCode, http.StatusNotImplemented)
	}
	_ = resp.Body.Close()

	resp = postJSON(t, client, server.URL+"/api/databases/"+dbID+"/snapshots", map[string]string{})
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("POST /api/databases/{id}/snapshots = %d, want %d", resp.StatusCode, http.StatusNotImplemented)
	}
	_ = resp.Body.Close()
}

func TestBillingInvoices(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "billing@example.com")

	var userID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "billing@example.com").Scan(&userID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}

	var invoiceID string
	if err := db.QueryRow(`
		INSERT INTO invoices (user_id, status, total_amount_idr, period_start, period_end)
		VALUES ($1, 'open', 150000, '2026-10-01', '2026-10-31')
		RETURNING id
	`, userID).Scan(&invoiceID); err != nil {
		t.Fatalf("insert invoice fixture: %v", err)
	}

	resp, err := client.Get(server.URL + "/api/billing/invoices")
	if err != nil {
		t.Fatalf("GET /api/billing/invoices: %v", err)
	}
	var list []map[string]any
	decode(t, resp, &list)
	if len(list) != 1 || list[0]["status"] != "open" {
		t.Fatalf("GET /api/billing/invoices = %v, want exactly one open invoice", list)
	}

	resp, err = client.Post(server.URL+"/api/billing/invoices/"+invoiceID+"/pay", "application/json", nil)
	if err != nil {
		t.Fatalf("POST pay invoice: %v", err)
	}
	var paid map[string]any
	decode(t, resp, &paid)
	if paid["status"] != "paid" {
		t.Fatalf("pay invoice status = %v, want paid", paid["status"])
	}

	resp, err = client.Post(server.URL+"/api/billing/invoices/"+invoiceID+"/pay", "application/json", nil)
	if err != nil {
		t.Fatalf("POST pay invoice again: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("paying an already-paid invoice = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	_ = resp.Body.Close()

	resp, err = client.Post(server.URL+"/api/billing/invoices/00000000-0000-0000-0000-000000000000/pay", "application/json", nil)
	if err != nil {
		t.Fatalf("POST pay missing invoice: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("paying a missing invoice = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	_ = resp.Body.Close()
}
