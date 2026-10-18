package credentials_test

import (
	"encoding/base64"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/internal/credentials"
)

func testKey() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"[:32]))
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c, err := credentials.NewCipher(testKey())
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	ciphertext, err := c.Encrypt("hunter2-hunter2-hunter2-hunter2")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	plaintext, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plaintext != "hunter2-hunter2-hunter2-hunter2" {
		t.Fatalf("Decrypt = %q, want original plaintext", plaintext)
	}
}

func TestEncryptIsNotDeterministic(t *testing.T) {
	c, err := credentials.NewCipher(testKey())
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	a, err := c.Encrypt("same-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	b, err := c.Encrypt("same-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if string(a) == string(b) {
		t.Fatalf("Encrypt produced identical ciphertext twice; nonce is not random")
	}
}

func TestNewCipherRejectsWrongKeyLength(t *testing.T) {
	shortKey := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := credentials.NewCipher(shortKey); err == nil {
		t.Fatalf("NewCipher with a short key = nil error, want one")
	}
}

func TestNewCipherRejectsInvalidBase64(t *testing.T) {
	if _, err := credentials.NewCipher("not-base64!!!"); err == nil {
		t.Fatalf("NewCipher with invalid base64 = nil error, want one")
	}
}

func TestDecryptRejectsTruncatedCiphertext(t *testing.T) {
	c, err := credentials.NewCipher(testKey())
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	if _, err := c.Decrypt([]byte("short")); err == nil {
		t.Fatalf("Decrypt with truncated ciphertext = nil error, want one")
	}
}
