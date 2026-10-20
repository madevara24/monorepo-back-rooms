// Package credentials encrypts tenant database passwords at rest with
// AES-256-GCM, keyed by the control plane's CREDENTIAL_KEY.
package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Cipher encrypts and decrypts tenant database passwords with AES-256-GCM.
type Cipher struct {
	gcm cipher.AEAD
}

// NewCipher builds a Cipher from CREDENTIAL_KEY: a base64-encoded 32-byte
// AES-256 key.
func NewCipher(keyB64 string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("decode CREDENTIAL_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("CREDENTIAL_KEY must decode to 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("build AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("build AES-GCM: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

// Encrypt returns the nonce-prefixed ciphertext for plaintext.
func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return c.gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt reverses Encrypt.
func (c *Cipher) Decrypt(ciphertext []byte) (string, error) {
	nonceSize := c.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := c.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}
