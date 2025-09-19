// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package pkce

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

// genValidBase64URLDigest generates a valid base64url-encoded SHA256 digest
func genValidBase64URLDigest(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("failed to read random: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func TestIsValidBase64URL_Valid(t *testing.T) {
	s := genValidBase64URLDigest(t)
	if !isValidBase64URL(s) {
		t.Fatalf("expected valid base64url digest, got invalid: %q", s)
	}
}

func TestIsValidBase64URL_InvalidChars(t *testing.T) {
	// '+' and '/' are invalid in base64url
	s := "abcd+efg_hijklmnopqrstuvwxyz0123456789abcd"
	if isValidBase64URL(s) {
		t.Fatalf("expected invalid due to characters, got valid: %q", s)
	}
}

func TestIsValidBase64URL_WrongDecodedLen(t *testing.T) {
	// 31 bytes decoded -> invalid
	decoded31 := make([]byte, 31)
	s31 := base64.RawURLEncoding.EncodeToString(decoded31)
	if isValidBase64URL(s31) {
		t.Fatalf("expected invalid due to decoded len != 32, got valid")
	}

	// 33 bytes decoded -> invalid
	decoded33 := make([]byte, 33)
	s33 := base64.RawURLEncoding.EncodeToString(decoded33)
	if isValidBase64URL(s33) {
		t.Fatalf("expected invalid due to decoded len != 32, got valid")
	}
}

func TestVerifyPKCEChallenge_Match(t *testing.T) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		t.Fatalf("rand read: %v", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	sum := sha256.Sum256([]byte(codeVerifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if !VerifyPKCEChallenge(codeVerifier, expectedChallenge) {
		t.Fatalf("expected challenge to verify")
	}
}

func TestVerifyPKCEChallenge_Mismatch(t *testing.T) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		t.Fatalf("rand read: %v", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	sum := sha256.Sum256([]byte(codeVerifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(sum[:])

	// tamper the verifier
	codeVerifier += "A"

	if VerifyPKCEChallenge(codeVerifier, expectedChallenge) {
		t.Fatalf("expected verification to fail for mismatched verifier")
	}
}

func TestGeneratePKCEChallenge(t *testing.T) {
	pair, err := GeneratePKCEChallenge()
	if err != nil {
		t.Fatalf("GeneratePKCEChallenge returned error: %v", err)
	}
	if pair == nil {
		t.Fatalf("expected non-nil pair")
	}
	if pair.CodeVerifier == "" || pair.CodeChallenge == "" {
		t.Fatalf("expected non-empty verifier and challenge")
	}

	// RFC 7636 requires 43..128 characters for the verifier
	if l := len(pair.CodeVerifier); l < 43 || l > 128 {
		t.Fatalf("code_verifier length must be in [43,128], got %d", l)
	}

	// Challenge should be a base64url-encoded SHA256 digest (valid and 32 bytes decoded)
	if !isValidBase64URL(pair.CodeChallenge) {
		t.Fatalf("code_challenge should be valid base64url digest")
	}

	// Verify Challenge == S256(verifier)
	sum := sha256.Sum256([]byte(pair.CodeVerifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if pair.CodeChallenge != expectedChallenge {
		t.Fatalf("code_challenge mismatch: got %q want %q", pair.CodeChallenge, expectedChallenge)
	}

	// And VerifyPKCEChallenge should pass
	if !VerifyPKCEChallenge(pair.CodeVerifier, pair.CodeChallenge) {
		t.Fatalf("VerifyPKCEChallenge should return true for generated pair")
	}
}

func BenchmarkGeneratePKCEChallenge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := GeneratePKCEChallenge(); err != nil {
			b.Fatalf("GeneratePKCEChallenge error: %v", err)
		}
	}
}
