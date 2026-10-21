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
	"strconv"
)

// Manifest is one signed mbr-node build: the version, the binary's
// checksum, and the ed25519 signature over both. ArtifactURL is not part
// of the signed payload; its integrity comes from Checksum once
// downloaded, not from the signature.
type Manifest struct {
	Version     int    `json:"version"`
	CommitSHA   string `json:"commit_sha"`
	Checksum    string `json:"checksum"`
	Signature   string `json:"signature"`
	ArtifactURL string `json:"artifact_url,omitempty"`
}

// CanonicalPayload is the exact bytes a release's signature covers.
// Version, CommitSHA, and Checksum all feed it, so a signature cannot be
// replayed across a different version, commit, or binary.
func CanonicalPayload(version int, commitSHA, checksum string) []byte {
	return []byte(strconv.Itoa(version) + "\n" + commitSHA + "\n" + checksum)
}

// ChecksumSHA256 returns the lowercase hex SHA-256 digest of r.
func ChecksumSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("hash release artifact: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Sign returns the base64 ed25519 signature over version, commitSHA, and
// checksum.
func Sign(version int, commitSHA, checksum string, priv ed25519.PrivateKey) string {
	sig := ed25519.Sign(priv, CanonicalPayload(version, commitSHA, checksum))
	return base64.StdEncoding.EncodeToString(sig)
}

// Verify reports whether m's signature is valid for pub. It checks the
// checksum format (64 lowercase hex characters) before checking the
// signature, so a malformed manifest fails with a clearer error than an
// ed25519 verification failure.
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
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(pub, CanonicalPayload(m.Version, m.CommitSHA, m.Checksum), sig) {
		return errors.New("invalid signature")
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
