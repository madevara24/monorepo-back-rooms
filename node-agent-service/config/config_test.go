package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRequireTLSEnv(t *testing.T) {
	const cert, key, ca, tenantCert, tenantKey = "cert", "key", "ca", "tenant-cert", "tenant-key"

	tests := []struct {
		name                                                   string
		tlsCert, tlsKey, clientCA, tenantTLSCert, tenantTLSKey string
		wantErr                                                bool
	}{
		{"all set", cert, key, ca, tenantCert, tenantKey, false},
		{"missing mTLS cert", "", key, ca, tenantCert, tenantKey, true},
		{"missing mTLS key", cert, "", ca, tenantCert, tenantKey, true},
		{"missing client CA", cert, key, "", tenantCert, tenantKey, true},
		{"missing tenant cert", cert, key, ca, "", tenantKey, true},
		{"missing tenant key", cert, key, ca, tenantCert, "", true},
		{"nothing set", "", "", "", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requireTLSEnv(tc.tlsCert, tc.tlsKey, tc.clientCA, tc.tenantTLSCert, tc.tenantTLSKey)
			if tc.wantErr && err == nil {
				t.Fatalf("requireTLSEnv() = nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("requireTLSEnv() = %v, want nil", err)
			}
		})
	}
}

// writeSelfSignedCert writes a throwaway self-signed cert/key pair to dir
// and returns their paths, so tests can exercise file-based validation
// without touching real TLS material.
func writeSelfSignedCert(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPath = filepath.Join(dir, "tenant.crt")
	keyPath = filepath.Join(dir, "tenant.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(keyPath, keyBytes, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

func TestValidateTenantTLSFilesAcceptsAValidPair(t *testing.T) {
	certPath, keyPath := writeSelfSignedCert(t, t.TempDir())

	if err := validateTenantTLSFiles(certPath, keyPath); err != nil {
		t.Fatalf("validateTenantTLSFiles() = %v, want nil", err)
	}
}

func TestValidateTenantTLSFilesRejectsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	if err := validateTenantTLSFiles(filepath.Join(dir, "missing.crt"), filepath.Join(dir, "missing.key")); err == nil {
		t.Fatal("validateTenantTLSFiles() = nil, want error for missing files")
	}
}

// setRequiredTLSEnv sets every env var Load requires besides the
// release public key, so tests can isolate AGENT_RELEASE_PUBLIC_KEY_B64
// as the only thing under test.
func setRequiredTLSEnv(t *testing.T) {
	t.Helper()
	certPath, keyPath := writeSelfSignedCert(t, t.TempDir())
	t.Setenv("AGENT_TLS_CERT", "cert")
	t.Setenv("AGENT_TLS_KEY", "key")
	t.Setenv("AGENT_CLIENT_CA", "ca")
	t.Setenv("AGENT_TENANT_TLS_CERT", certPath)
	t.Setenv("AGENT_TENANT_TLS_KEY", keyPath)
}

func TestLoadAcceptsNoReleasePublicKey(t *testing.T) {
	setRequiredTLSEnv(t)

	if _, err := Load(); err != nil {
		t.Fatalf("Load() with no release public key = %v, want nil", err)
	}
}

func TestLoadAcceptsAWellFormedReleasePublicKey(t *testing.T) {
	setRequiredTLSEnv(t)
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	t.Setenv("AGENT_RELEASE_PUBLIC_KEY_B64", base64.StdEncoding.EncodeToString(pub))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with a well-formed release public key = %v, want nil", err)
	}
	if cfg.ReleasePublicKeyB64 == "" {
		t.Fatal("Load() did not carry through ReleasePublicKeyB64")
	}
}

func TestLoadRejectsAMalformedReleasePublicKey(t *testing.T) {
	setRequiredTLSEnv(t)
	t.Setenv("AGENT_RELEASE_PUBLIC_KEY_B64", "not-base64-!!!")

	if _, err := Load(); err == nil {
		t.Fatal("Load() with a malformed release public key = nil, want error")
	}
}

func TestValidateTenantTLSFilesRejectsMismatchedPair(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	certPath, _ := writeSelfSignedCert(t, dir1)
	_, keyPath := writeSelfSignedCert(t, dir2)

	if err := validateTenantTLSFiles(certPath, keyPath); err == nil {
		t.Fatal("validateTenantTLSFiles() = nil, want error for a cert/key that don't match")
	}
}
