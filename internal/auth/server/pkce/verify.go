package pkce

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
)

func ValidatePKCEParams(params server.AuthorizationParams) error {
	if params.CodeChallenge == "" {
		return fmt.Errorf("code_challenge is required")
	}

	// 验证code_challenge长度（RFC 7636: 43-128字符）
	if len(params.CodeChallenge) < 43 || len(params.CodeChallenge) > 128 {
		return fmt.Errorf("code_challenge length must be between 43 and 128 characters")
	}

	// 验证code_challenge格式（BASE64URL）
	if !isValidBase64URL(params.CodeChallenge) {
		return fmt.Errorf("code_challenge must be valid BASE64URL")
	}

	return nil
}

func isValidBase64URL(s string) bool {
	// 长度检查
	if len(s) < 43 || len(s) > 128 {
		return false
	}

	// 字符集验证
	base64URLPattern := `^[A-Za-z0-9_-]+$`
	matched, err := regexp.MatchString(base64URLPattern, s)
	if err != nil || !matched {
		return false
	}

	// 尝试解码验证
	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return false
	}

	// 对于code_challenge，解码后应该是32字节（SHA256哈希）
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
