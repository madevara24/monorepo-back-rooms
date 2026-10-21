package handler_test

import (
	"crypto/ed25519"
	"database/sql"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	release "github.com/madevara24/monorepo-back-rooms/agent-release"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/handler"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

// newReleaseTestServer is like newTestServer but also wires a release
// signing public key, since most of this file's tests need one
// configured.
func newReleaseTestServer(t *testing.T, pub ed25519.PublicKey) (*httptest.Server, *sql.DB) {
	t.Helper()
	db := storetest.Open(t)
	server := httptest.NewServer(handler.NewMux(&handler.Handler{
		DB: db, Auth: &usecase.Auth{DB: db}, ReleasePublicKey: pub,
	}))
	t.Cleanup(server.Close)
	return server, db
}

func publishReleaseBody(version int, commitSHA, checksum string, priv ed25519.PrivateKey, artifactURL string) map[string]any {
	return map[string]any{
		"version":      version,
		"commit_sha":   commitSHA,
		"checksum":     checksum,
		"signature":    release.Sign(version, commitSHA, checksum, priv),
		"artifact_url": artifactURL,
	}
}

const releaseTestChecksum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestPublishReleaseRejectsWhenKeyNotConfigured(t *testing.T) {
	server, _ := newReleaseTestServer(t, nil)
	_, priv, _ := ed25519.GenerateKey(nil)

	resp := postJSON(t, http.DefaultClient, server.URL+"/api/releases",
		publishReleaseBody(1, "abc123", releaseTestChecksum, priv, "https://example.invalid/mbr-node"))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("POST /api/releases with no public key configured = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestPublishReleaseRejectsAnInvalidSignature(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	_, wrongPriv, _ := ed25519.GenerateKey(nil)
	server, _ := newReleaseTestServer(t, pub)

	resp := postJSON(t, http.DefaultClient, server.URL+"/api/releases",
		publishReleaseBody(1, "abc123", releaseTestChecksum, wrongPriv, "https://example.invalid/mbr-node"))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /api/releases with a wrong-key signature = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestPublishReleaseAcceptsAValidSignatureAndStoresIt(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	server, db := newReleaseTestServer(t, pub)

	resp := postJSON(t, http.DefaultClient, server.URL+"/api/releases",
		publishReleaseBody(1, "abc123", releaseTestChecksum, priv, "https://example.invalid/mbr-node"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/releases with a valid signature = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created struct {
		Version     int    `json:"version"`
		ArtifactURL string `json:"artifact_url"`
	}
	decode(t, resp, &created)
	if created.Version != 1 {
		t.Fatalf("created release version = %d, want 1", created.Version)
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM releases WHERE version = 1`).Scan(&count); err != nil {
		t.Fatalf("count releases: %v", err)
	}
	if count != 1 {
		t.Fatalf("releases rows with version 1 = %d, want 1", count)
	}
}

func TestPublishReleaseRejectsANonMonotonicVersion(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	server, _ := newReleaseTestServer(t, pub)

	resp := postJSON(t, http.DefaultClient, server.URL+"/api/releases",
		publishReleaseBody(5, "abc123", releaseTestChecksum, priv, "https://example.invalid/mbr-node"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/releases version 5 = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	_ = resp.Body.Close()

	for _, version := range []int{5, 4, 1} {
		resp := postJSON(t, http.DefaultClient, server.URL+"/api/releases",
			publishReleaseBody(version, "def456", releaseTestChecksum, priv, "https://example.invalid/mbr-node"))
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("POST /api/releases version %d after version 5 already published = %d, want %d", version, resp.StatusCode, http.StatusConflict)
		}
		_ = resp.Body.Close()
	}
}

func TestAdminReleasesListsNewestFirst(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	server, db := newReleaseTestServer(t, pub)
	client := &http.Client{}

	for _, version := range []int{1, 2} {
		resp := postJSON(t, client, server.URL+"/api/releases",
			publishReleaseBody(version, "commit-"+string(rune('a'+version)), releaseTestChecksum, priv, "https://example.invalid/mbr-node"))
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST /api/releases version %d = %d, want %d", version, resp.StatusCode, http.StatusCreated)
		}
		_ = resp.Body.Close()
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	jarClient := &http.Client{Jar: jar}
	signUp(t, server, jarClient, "admin-releases@example.com")
	promoteToAdmin(t, db, "admin-releases@example.com")

	resp, err := jarClient.Get(server.URL + "/api/admin/releases")
	if err != nil {
		t.Fatalf("GET /api/admin/releases: %v", err)
	}
	var list []struct {
		Version int `json:"version"`
	}
	decode(t, resp, &list)
	if len(list) != 2 || list[0].Version != 2 || list[1].Version != 1 {
		t.Fatalf("GET /api/admin/releases = %+v, want [2, 1]", list)
	}
}
