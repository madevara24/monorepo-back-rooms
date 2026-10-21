package handler

import (
	"net/http"
	"time"

	release "github.com/madevara24/monorepo-back-rooms/agent-release"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
)

type publishReleaseRequest struct {
	Version     int    `json:"version"`
	CommitSHA   string `json:"commit_sha"`
	Checksum    string `json:"checksum"`
	Signature   string `json:"signature"`
	ArtifactURL string `json:"artifact_url"`
}

type releaseResponse struct {
	ID          string    `json:"id"`
	Version     int       `json:"version"`
	CommitSHA   string    `json:"commit_sha"`
	Checksum    string    `json:"checksum"`
	ArtifactURL string    `json:"artifact_url"`
	CreatedAt   time.Time `json:"created_at"`
}

func toReleaseResponse(r entities.Release) releaseResponse {
	return releaseResponse{
		ID:          r.ID,
		Version:     r.Version,
		CommitSHA:   r.CommitSHA,
		Checksum:    r.Checksum,
		ArtifactURL: r.ArtifactURL,
		CreatedAt:   r.CreatedAt,
	}
}

// handlePublishRelease registers a CI-built mbr-node release. There is no
// separate credential: a release manifest's own ed25519 signature,
// checked against ReleasePublicKey, is what lets this endpoint tell a
// genuine CI-signed release from an arbitrary POST. A version that is
// not strictly greater than every release already stored is rejected,
// so the catalog can never go backwards or replay an old release under
// a new row.
func (h *Handler) handlePublishRelease(w http.ResponseWriter, r *http.Request) {
	if h.ReleasePublicKey == nil {
		writeError(w, http.StatusServiceUnavailable, "release publishing is not configured: MBR_RELEASE_PUBLIC_KEY_B64 is unset")
		return
	}

	var req publishReleaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// ArtifactURL is part of the signed payload (see agent-release's
	// CanonicalPayload), so a request that didn't come from CI's signing
	// step — including one that read a published manifest.json and
	// substituted a different artifact_url — fails verification here
	// rather than being distinguishable as a separate "bad artifact_url"
	// case.
	manifest := release.Manifest{
		Version:     req.Version,
		CommitSHA:   req.CommitSHA,
		Checksum:    req.Checksum,
		ArtifactURL: req.ArtifactURL,
		Signature:   req.Signature,
	}
	if err := release.Verify(manifest, h.ReleasePublicKey); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid release manifest: "+err.Error())
		return
	}

	latest, err := repository.LatestReleaseVersion(r.Context(), h.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not look up the latest release")
		return
	}
	if req.Version <= latest {
		writeError(w, http.StatusConflict, "release version must be greater than the latest published version")
		return
	}

	created, err := repository.InsertRelease(r.Context(), h.DB, entities.Release{
		Version:     req.Version,
		CommitSHA:   req.CommitSHA,
		Checksum:    req.Checksum,
		Signature:   req.Signature,
		ArtifactURL: req.ArtifactURL,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store release")
		return
	}

	writeJSON(w, http.StatusCreated, toReleaseResponse(created))
}

// handleAdminReleases lists every published release, newest first, for
// the admin release catalog view.
func (h *Handler) handleAdminReleases(w http.ResponseWriter, r *http.Request) {
	rows, err := repository.ListReleases(r.Context(), h.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list releases")
		return
	}

	out := make([]releaseResponse, len(rows))
	for i, rel := range rows {
		out[i] = toReleaseResponse(rel)
	}
	writeJSON(w, http.StatusOK, out)
}
