package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestGenerateHashAndVerifyNodeToken(t *testing.T) {
	token, err := GenerateNodeToken()
	if err != nil {
		t.Fatalf("GenerateNodeToken() error = %v", err)
	}
	if !strings.HasPrefix(token, TokenPrefix) {
		t.Fatalf("token prefix = %q, want %q", token[:len(TokenPrefix)], TokenPrefix)
	}

	hash, err := HashNodeToken(token)
	if err != nil {
		t.Fatalf("HashNodeToken() error = %v", err)
	}
	if hash == token {
		t.Fatal("hash must not equal raw token")
	}
	if err := VerifyNodeToken(token, hash); err != nil {
		t.Fatalf("VerifyNodeToken() error = %v", err)
	}
	if err := VerifyNodeToken(token+"x", hash); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyNodeToken(wrong) error = %v, want ErrInvalidHash", err)
	}
}

func TestNodeTokenWhitespaceNormalization(t *testing.T) {
	token, err := GenerateNodeToken()
	if err != nil {
		t.Fatalf("GenerateNodeToken() error = %v", err)
	}
	hash, err := HashNodeToken(" \t" + token + "\n")
	if err != nil {
		t.Fatalf("HashNodeToken() error = %v", err)
	}
	if err := VerifyNodeToken(token, hash); err != nil {
		t.Fatalf("VerifyNodeToken() error = %v", err)
	}
}

func TestFingerprintNodeToken(t *testing.T) {
	token, err := GenerateNodeToken()
	if err != nil {
		t.Fatalf("GenerateNodeToken() error = %v", err)
	}

	first, err := FingerprintNodeToken(token)
	if err != nil {
		t.Fatalf("FingerprintNodeToken() error = %v", err)
	}
	second, err := FingerprintNodeToken(token)
	if err != nil {
		t.Fatalf("FingerprintNodeToken() error = %v", err)
	}
	if first != second {
		t.Fatalf("fingerprint not stable: %q != %q", first, second)
	}
	if len(first) != fingerprintSize {
		t.Fatalf("fingerprint length = %d, want %d", len(first), fingerprintSize)
	}
}

func TestEmptyNodeToken(t *testing.T) {
	if _, err := HashNodeToken(" "); !errors.Is(err, ErrEmptyToken) {
		t.Fatalf("HashNodeToken(empty) error = %v, want ErrEmptyToken", err)
	}
	if err := VerifyNodeToken("", "hash"); !errors.Is(err, ErrEmptyToken) {
		t.Fatalf("VerifyNodeToken(empty) error = %v, want ErrEmptyToken", err)
	}
	if _, err := FingerprintNodeToken("\n"); !errors.Is(err, ErrEmptyToken) {
		t.Fatalf("FingerprintNodeToken(empty) error = %v, want ErrEmptyToken", err)
	}
}
