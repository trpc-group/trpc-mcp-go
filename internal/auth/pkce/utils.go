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
	"fmt"
	"regexp"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
)

// PKCEChallenge holds PKCE code verifier and challenge
type PKCEChallenge struct {
	// CodeVerifier is the high-entropy cryptographic random string
	CodeVerifier string
	// CodeChallenge is the derived challenge from the code verifier
	CodeChallenge string
}

// ValidatePKCEParams validates the PKCE parameters provided in the authorization request.
func ValidatePKCEParams(params server.AuthorizationParams) error {
	if params.CodeChallenge == "" {
		return fmt.Errorf("code_challenge is required")
	}

	// Verify code_challenge length (RFC 7636: 43-128 characters)
	if len(params.CodeChallenge) < 43 || len(params.CodeChallenge) > 128 {
		return fmt.Errorf("code_challenge length must be between 43 and 128 characters")
	}

	// Verify code_challenge format (BASE64URL)
	if !isValidBase64URL(params.CodeChallenge) {
		return fmt.Errorf("code_challenge must be valid BASE64URL")
	}

	return nil
}

// isValidBase64URL checks whether the given string is a valid Base64URL-encoded value
// and decodes to exactly 32 bytes (the output size of SHA-256).
func isValidBase64URL(s string) bool {
	// Length check
	if len(s) < 43 || len(s) > 128 {
		return false
	}

	// Character set validation
	base64URLPattern := `^[A-Za-z0-9_-]+$`
	matched, err := regexp.MatchString(base64URLPattern, s)
	if err != nil || !matched {
		return false
	}

	// Try decoding verification
	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return false
	}

	// For code_challenge, it should be 32 bytes after decoding (SHA256 hash)
	if len(decoded) != 32 {
		return false
	}

	return true
}

// VerifyPKCEChallenge verifies the PKCE code_verifier against the code_challenge
func VerifyPKCEChallenge(codeVerifier, codeChallenge string) bool {
	if codeVerifier == "" || codeChallenge == "" {
		return false
	}

	// Create SHA256 hash of the code_verifier
	hash := sha256.Sum256([]byte(codeVerifier))

	// Base64 URL encode the hash
	computedChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return computedChallenge == codeChallenge
}

// GeneratePKCEChallenge generates a new PKCE pair (code_verifier and code_challenge).
func GeneratePKCEChallenge() (*PKCEChallenge, error) {
	// Generate 43-128 character code_verifier (RFC 7636)
	verifierBytes := make([]byte, 32) // 32 bytes = 43 chars in base64url
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate code_challenge using S256 method
	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &PKCEChallenge{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
	}, nil
}
