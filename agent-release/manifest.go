// Package release defines the signed mbr-node release manifest: the
// shape CI produces and both control-panel-service (on ingest) and
// node-agent-service (before an update, once MBR-23 lands) verify
// against. It is the one home for the canonical signed payload, so the
// signing side (agent-release/cmd/sign) and every verifying side stay in
// lockstep.
package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
)

// Manifest is one signed mbr-node build: the version, the binary's
// checksum, where to download it, and the ed25519 signature over all
// three. ArtifactURL is part of the signed payload (not just Checksum)
// so that a release row can never be pointed at a different download
// location than the one CI actually signed: without this, anyone who
// reads a published manifest.json (a public GitHub Release asset)
// could race CI's own POST /api/releases with the same valid signature
// but a different artifact_url, and win if they arrive first — the
// version-monotonicity check would then reject CI's own, legitimate
// registration.
type Manifest struct {
	Version     int    `json:"version"`
	CommitSHA   string `json:"commit_sha"`
	Checksum    string `json:"checksum"`
	ArtifactURL string `json:"artifact_url"`
	Signature   string `json:"signature"`
}

// CanonicalPayload is the exact bytes a release's signature covers.
// Version, CommitSHA, Checksum, and ArtifactURL all feed it, so a
// signature cannot be replayed across a different version, commit,
// binary, or download location.
func CanonicalPayload(version int, commitSHA, checksum, artifactURL string) []byte {
	return []byte(strconv.Itoa(version) + "\n" + commitSHA + "\n" + checksum + "\n" + artifactURL)
}

// ChecksumSHA256 returns the lowercase hex SHA-256 digest of r.
func ChecksumSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("hash release artifact: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Sign returns the base64 ed25519 signature over version, commitSHA,
// checksum, and artifactURL.
func Sign(version int, commitSHA, checksum, artifactURL string, priv ed25519.PrivateKey) string {
	sig := ed25519.Sign(priv, CanonicalPayload(version, commitSHA, checksum, artifactURL))
	return base64.StdEncoding.EncodeToString(sig)
}

// Verify reports whether m's signature is valid for pub. It checks the
// checksum format (64 lowercase hex characters) and the artifact URL's
// scheme before checking the signature, so a malformed manifest fails
// with a clearer error than an ed25519 verification failure.
func Verify(m Manifest, pub ed25519.PublicKey) error {
	if m.Version <= 0 {
		return errors.New("version must be positive")
	}
	if m.CommitSHA == "" {
		return errors.New("commit_sha is required")
	}
	if !isHexSHA256(m.Checksum) {
		return errors.New("checksum must be 64 lowercase hex characters")
	}
	if err := validateArtifactURL(m.ArtifactURL); err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(pub, CanonicalPayload(m.Version, m.CommitSHA, m.Checksum, m.ArtifactURL), sig) {
		return errors.New("invalid signature")
	}
	return nil
}

func validateArtifactURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("artifact_url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return errors.New("artifact_url must be an absolute http(s) URL")
	}
	if u.Host == "" {
		return errors.New("artifact_url must be an absolute http(s) URL")
	}
	return nil
}

func isHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ParsePublicKeyB64 decodes a standard-base64-encoded ed25519 public key.
func ParsePublicKeyB64(s string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key must be %d bytes, got %d", ed25519.PublicKeySize, len(b))
	}
	return ed25519.PublicKey(b), nil
}

// ParsePrivateKeyB64 decodes a standard-base64-encoded ed25519 private
// key (the seed-plus-public-key form crypto/ed25519 returns from
// GenerateKey).
func ParsePrivateKeyB64(s string) (ed25519.PrivateKey, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(b))
	}
	return ed25519.PrivateKey(b), nil
}
