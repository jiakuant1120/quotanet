// Package auth provides QuotaNet node token helpers.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	TokenPrefix      = "qnc_"
	defaultTokenSize = 32
	fingerprintSize  = 12
)

var (
	ErrEmptyToken  = errors.New("quotanet node token is required")
	ErrInvalidHash = errors.New("quotanet node token hash is invalid")
)

// GenerateNodeToken returns a new opaque client token. The raw token should be
// shown once to the operator and only its bcrypt hash should be persisted.
func GenerateNodeToken() (string, error) {
	raw := make([]byte, defaultTokenSize)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate quotanet node token: %w", err)
	}
	return TokenPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// HashNodeToken returns a bcrypt hash suitable for storing in quotanet_nodes.
func HashNodeToken(token string) (string, error) {
	normalized, err := normalizeToken(token)
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(normalized), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash quotanet node token: %w", err)
	}
	return string(hash), nil
}

// VerifyNodeToken compares a raw node token with a stored bcrypt hash.
func VerifyNodeToken(token, hash string) error {
	normalized, err := normalizeToken(token)
	if err != nil {
		return err
	}
	if strings.TrimSpace(hash) == "" {
		return ErrInvalidHash
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(normalized)); err != nil {
		return ErrInvalidHash
	}
	return nil
}

// FingerprintNodeToken returns a short, non-secret token fingerprint for logs.
func FingerprintNodeToken(token string) (string, error) {
	normalized, err := normalizeToken(token)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:fingerprintSize], nil
}

func normalizeToken(token string) (string, error) {
	normalized := strings.TrimSpace(token)
	if normalized == "" {
		return "", ErrEmptyToken
	}
	return normalized, nil
}
