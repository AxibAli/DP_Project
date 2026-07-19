package auth

import (
	"crypto/rand"
	"encoding/base64"
)

// tokenBytes is the amount of entropy (256 bits) used for session tokens
// and CSRF tokens — enough to make guessing/brute-forcing infeasible.
const tokenBytes = 32

// GenerateToken returns a cryptographically random, URL-safe token string.
func GenerateToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
