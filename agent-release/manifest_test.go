package release

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
)

func testKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return pub, priv
}

const testArtifactURL = "https://github.com/madevara24/monorepo-back-rooms/releases/download/mbr-node-v1/mbr-node"

func signedManifest(version int, commitSHA, checksum, artifactURL string, priv ed25519.PrivateKey) Manifest {
	return Manifest{
		Version:     version,
		CommitSHA:   commitSHA,
		Checksum:    checksum,
		ArtifactURL: artifactURL,
		Signature:   Sign(version, commitSHA, checksum, artifactURL, priv),
	}
}

const testChecksum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestVerifyAcceptsAValidSignature(t *testing.T) {
	pub, priv := testKeypair(t)
	m := signedManifest(1, "abc123", testChecksum, testArtifactURL, priv)

	if err := Verify(m, pub); err != nil {
		t.Fatalf("Verify = %v, want nil", err)
	}
}

func TestVerifyRejectsAWrongKey(t *testing.T) {
	_, priv := testKeypair(t)
	otherPub, _ := testKeypair(t)
	m := signedManifest(1, "abc123", testChecksum, testArtifactURL, priv)

	if err := Verify(m, otherPub); err == nil {
		t.Fatal("Verify with the wrong public key = nil, want an error")
	}
}

func TestVerifyRejectsATamperedChecksum(t *testing.T) {
	pub, priv := testKeypair(t)
	m := signedManifest(1, "abc123", testChecksum, testArtifactURL, priv)
	m.Checksum = strings.Repeat("f", 64)

	if err := Verify(m, pub); err == nil {
		t.Fatal("Verify with a tampered checksum = nil, want an error")
	}
}

func TestVerifyRejectsATamperedVersion(t *testing.T) {
	pub, priv := testKeypair(t)
	m := signedManifest(1, "abc123", testChecksum, testArtifactURL, priv)
	m.Version = 2

	if err := Verify(m, pub); err == nil {
		t.Fatal("Verify with a tampered version = nil, want an error")
	}
}

func TestVerifyRejectsATamperedCommit(t *testing.T) {
	pub, priv := testKeypair(t)
	m := signedManifest(1, "abc123", testChecksum, testArtifactURL, priv)
	m.CommitSHA = "def456"

	if err := Verify(m, pub); err == nil {
		t.Fatal("Verify with a tampered commit_sha = nil, want an error")
	}
}

// TestVerifyRejectsATamperedArtifactURL is the regression test for the
// release-hijack scenario: a published manifest.json's artifact_url
// must not be swappable by a third party who only ever saw the public,
// already-signed manifest.
func TestVerifyRejectsATamperedArtifactURL(t *testing.T) {
	pub, priv := testKeypair(t)
	m := signedManifest(1, "abc123", testChecksum, testArtifactURL, priv)
	m.ArtifactURL = "https://attacker.invalid/mbr-node"

	if err := Verify(m, pub); err == nil {
		t.Fatal("Verify with a tampered artifact_url = nil, want an error")
	}
}

func TestVerifyRejectsAMalformedChecksum(t *testing.T) {
	pub, priv := testKeypair(t)
	m := signedManifest(1, "abc123", "not-a-sha256-hex-checksum", testArtifactURL, priv)

	if err := Verify(m, pub); err == nil {
		t.Fatal("Verify with a malformed checksum = nil, want an error")
	}
}

func TestVerifyRejectsAZeroVersion(t *testing.T) {
	pub, priv := testKeypair(t)
	m := signedManifest(0, "abc123", testChecksum, testArtifactURL, priv)

	if err := Verify(m, pub); err == nil {
		t.Fatal("Verify with version 0 = nil, want an error")
	}
}

func TestVerifyRejectsANonHTTPArtifactURL(t *testing.T) {
	pub, priv := testKeypair(t)
	m := signedManifest(1, "abc123", testChecksum, "javascript:alert(1)", priv)

	if err := Verify(m, pub); err == nil {
		t.Fatal("Verify with a non-http(s) artifact_url = nil, want an error")
	}
}

func TestVerifyRejectsAnEmptyArtifactURL(t *testing.T) {
	pub, priv := testKeypair(t)
	m := signedManifest(1, "abc123", testChecksum, "", priv)

	if err := Verify(m, pub); err == nil {
		t.Fatal("Verify with an empty artifact_url = nil, want an error")
	}
}

func TestChecksumSHA256(t *testing.T) {
	sum, err := ChecksumSHA256(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ChecksumSHA256: %v", err)
	}
	if sum != testChecksum {
		t.Fatalf("ChecksumSHA256(\"\") = %q, want %q", sum, testChecksum)
	}
}

func TestParsePublicKeyB64RoundTrip(t *testing.T) {
	pub, _ := testKeypair(t)

	parsed, err := ParsePublicKeyB64(base64.StdEncoding.EncodeToString(pub))
	if err != nil {
		t.Fatalf("ParsePublicKeyB64: %v", err)
	}
	if !parsed.Equal(pub) {
		t.Fatal("ParsePublicKeyB64 round-trip did not match the original key")
	}
}

func TestParsePublicKeyB64RejectsWrongLength(t *testing.T) {
	if _, err := ParsePublicKeyB64("dG9vc2hvcnQ="); err == nil {
		t.Fatal("ParsePublicKeyB64 with a too-short key = nil, want an error")
	}
}

func TestParsePrivateKeyB64RoundTrip(t *testing.T) {
	_, priv := testKeypair(t)

	parsed, err := ParsePrivateKeyB64(base64.StdEncoding.EncodeToString(priv))
	if err != nil {
		t.Fatalf("ParsePrivateKeyB64: %v", err)
	}
	if !parsed.Equal(priv) {
		t.Fatal("ParsePrivateKeyB64 round-trip did not match the original key")
	}
}
