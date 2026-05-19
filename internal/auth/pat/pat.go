// Package pat owns Personal Access Token format: plaintext shape, hashing,
// and parsing from Authorization headers.
package pat

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	// Prefix is the user-visible PAT prefix. Helps secret scanners (GitHub,
	// gitleaks, etc.) detect leaks.
	Prefix = "diarion_pat_"
	// TokenLen is the number of random bytes in the body of a PAT.
	TokenLen = 32
)

// ErrMalformed is returned when a Bearer token doesn't have the Diarion PAT shape.
var ErrMalformed = errors.New("pat: malformed token")

// Generate returns a fresh plaintext PAT (returned to the user once) plus its
// sha256 hex hash (for DB storage).
func Generate() (plaintext, tokenHash string, err error) {
	buf := make([]byte, TokenLen)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("pat.Generate: rand: %w", err)
	}
	body := hex.EncodeToString(buf)
	plaintext = Prefix + body
	tokenHash = Hash(plaintext)
	return plaintext, tokenHash, nil
}

// Hash returns the sha256-hex of the given plaintext PAT.
func Hash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// ParseFromHeader returns the plaintext PAT from an Authorization header value
// of the form "Bearer diarion_pat_…". Returns ErrMalformed on any other shape.
func ParseFromHeader(headerValue string) (string, error) {
	if headerValue == "" {
		return "", ErrMalformed
	}
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(headerValue, bearerPrefix) {
		return "", ErrMalformed
	}
	tok := strings.TrimSpace(headerValue[len(bearerPrefix):])
	if !strings.HasPrefix(tok, Prefix) {
		return "", ErrMalformed
	}
	body := tok[len(Prefix):]
	if len(body) != 2*TokenLen {
		return "", ErrMalformed
	}
	if _, err := hex.DecodeString(body); err != nil {
		return "", ErrMalformed
	}
	return tok, nil
}

// FromRequest is a convenience: extracts and validates the PAT plaintext from
// the standard Authorization header.
func FromRequest(r *http.Request) (string, error) {
	return ParseFromHeader(r.Header.Get("Authorization"))
}
