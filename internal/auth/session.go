package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// SessionTTL is how long a session stays valid after login.
const SessionTTL = 7 * 24 * time.Hour

// CookieName is the session cookie the API sets and reads.
const CookieName = "mbr_session"

// GenerateToken returns a new random opaque session token. The caller
// stores only its hash (see HashToken) and sends the raw token to the
// client in the session cookie.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns the SHA-256 hex digest of a raw session token, which
// is what gets stored in and looked up from the sessions table. Storing
// the hash instead of the raw token means a database read alone cannot be
// replayed as a session cookie.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
