// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions

// generateRSAKey generates a new RSA key pair for testing
func generateRSAKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// createTestJWK creates a test JWK from RSA key
func createTestJWK(privateKey *rsa.PrivateKey, keyID string) (jwk.Key, error) {
	// Import the public key part only
	publicKey := &privateKey.PublicKey
	key, err := jwk.FromRaw(publicKey)
	if err != nil {
		return nil, err
	}

	if err := key.Set(jwk.KeyIDKey, keyID); err != nil {
		return nil, err
	}

	if err := key.Set(jwk.AlgorithmKey, "RS256"); err != nil {
		return nil, err
	}

	if err := key.Set(jwk.KeyUsageKey, "sig"); err != nil {
		return nil, err
	}

	return key, nil
}

// createTestToken creates a test JWT token
func createTestToken(privateKey *rsa.PrivateKey, keyID string, claims map[string]interface{}) (string, error) {
	key, err := jwk.FromRaw(privateKey)
	if err != nil {
		return "", err
	}

	if err := key.Set(jwk.KeyIDKey, keyID); err != nil {
		return "", err
	}

	now := time.Now()
	token := jwt.New()

	// Set standard claims
	_ = token.Set(jwt.IssuerKey, "https://example.com")
	_ = token.Set(jwt.SubjectKey, "user123")
	_ = token.Set(jwt.AudienceKey, []string{"https://api.example.com"})
	_ = token.Set(jwt.ExpirationKey, now.Add(time.Hour))
	_ = token.Set(jwt.IssuedAtKey, now)
	_ = token.Set(jwt.JwtIDKey, "jti-123")
	_ = token.Set("client_id", "test-client")
	_ = token.Set("scope", "read write")
	_ = token.Set("kid", keyID)

	// Set custom claims
	for k, v := range claims {
		_ = token.Set(k, v)
	}

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
	if err != nil {
		return "", err
	}

	return string(signed), nil
}

// createTestJWKS creates a test JWKS JSON string
func createTestJWKS(keys ...jwk.Key) string {
	set := jwk.NewSet()
	for _, key := range keys {
		_ = set.AddKey(key)
	}

	buf, _ := json.Marshal(set)
	return string(buf)
}

// Test fixtures
func setupTestKeys(t *testing.T) (*rsa.PrivateKey, jwk.Key, string) {
	privateKey, err := generateRSAKey()
	require.NoError(t, err)

	publicKey, err := createTestJWK(privateKey, "test-key-1")
	require.NoError(t, err)

	jwksJSON := createTestJWKS(publicKey)

	return privateKey, publicKey, jwksJSON
}

func TestTokenVerifierFunc_VerifyAccessToken(t *testing.T) {
	ctx := context.Background()

	// Define a fake verifier function
	fn := TokenVerifierFunc(func(ctx context.Context, token string) (AuthInfo, error) {
		if token == "valid" {
			return AuthInfo{Token: token, ClientID: "test-client"}, nil
		}
		return AuthInfo{}, errors.New("invalid token")
	})

	// Success path
	authInfo, err := fn.VerifyAccessToken(ctx, "valid")
	assert.NoError(t, err)
	assert.Equal(t, "valid", authInfo.Token)
	assert.Equal(t, "test-client", authInfo.ClientID)

	// Failure path
	authInfo, err = fn.VerifyAccessToken(ctx, "invalid")
	assert.Error(t, err)
	assert.Empty(t, authInfo.Token)
}

// Tests for NewLocalTokenVerifier

func TestNewLocalTokenVerifier_WithJWKSString(t *testing.T) {
	ctx := context.Background()
	_, _, jwksJSON := setupTestKeys(t)

	cfg := LocalJWKSConfig{
		JWKS: jwksJSON,
	}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, verifier)
	assert.NotNil(t, verifier.localKeySet)
	assert.False(t, verifier.isRemote)
	assert.Equal(t, 1, verifier.localKeySet.Len())
}

func TestNewLocalTokenVerifier_WithFile(t *testing.T) {
	ctx := context.Background()
	_, _, jwksJSON := setupTestKeys(t)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "jwks-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jwksJSON)
	require.NoError(t, err)
	_ = tmpFile.Close()

	cfg := LocalJWKSConfig{
		File: tmpFile.Name(),
	}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, verifier)
	assert.Equal(t, 1, verifier.localKeySet.Len())
}

func TestNewLocalTokenVerifier_WithBothJWKSAndFile(t *testing.T) {
	ctx := context.Background()

	// Create two different keys
	_, _, jwksJSON1 := setupTestKeys(t)

	privateKey2, err := generateRSAKey()
	require.NoError(t, err)
	publicKey2, err := createTestJWK(privateKey2, "test-key-2")
	require.NoError(t, err)
	jwksJSON2 := createTestJWKS(publicKey2)

	// Create temporary file with second key
	tmpFile, err := os.CreateTemp("", "jwks-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jwksJSON2)
	require.NoError(t, err)
	_ = tmpFile.Close()

	cfg := LocalJWKSConfig{
		JWKS: jwksJSON1,
		File: tmpFile.Name(),
	}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, verifier)
	assert.Equal(t, 2, verifier.localKeySet.Len()) // Should have both keys
}

func TestNewLocalTokenVerifier_EmptyConfig(t *testing.T) {
	ctx := context.Background()
	cfg := LocalJWKSConfig{}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, verifier)
	assert.Contains(t, err.Error(), "must provide JWKS or File")
}

func TestNewLocalTokenVerifier_InvalidJWKS(t *testing.T) {
	ctx := context.Background()
	cfg := LocalJWKSConfig{
		JWKS: "invalid-json",
	}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, verifier)
	assert.Contains(t, err.Error(), "failed to parse local JWKS")
}

// Tests for NewRemoteTokenVerifier

func TestNewRemoteTokenVerifier_Success(t *testing.T) {
	ctx := context.Background()
	_, _, jwksJSON := setupTestKeys(t)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jwksJSON))
	}))
	defer server.Close()

	cfg := RemoteJWKSConfig{
		URLs: []string{server.URL},
		IssuerToURL: map[string]string{
			"https://example.com": server.URL,
		},
		RefreshInterval: time.Minute,
	}

	verifier, err := newRemoteTokenVerifier(ctx, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, verifier)
	assert.True(t, verifier.isRemote)
	assert.NotNil(t, verifier.cache)
	assert.Equal(t, server.URL, verifier.issuerToURL["https://example.com"])
}

func TestNewRemoteTokenVerifier_EmptyURLs(t *testing.T) {
	ctx := context.Background()
	cfg := RemoteJWKSConfig{}

	verifier, err := newRemoteTokenVerifier(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, verifier)
	assert.Contains(t, err.Error(), "must provide at least one RemoteURL")
}

func TestNewRemoteTokenVerifier_DefaultRefreshInterval(t *testing.T) {
	ctx := context.Background()
	_, _, jwksJSON := setupTestKeys(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jwksJSON))
	}))
	defer server.Close()

	cfg := RemoteJWKSConfig{
		URLs: []string{server.URL},
		// RefreshInterval is 0, should use default (60 minutes)
	}

	verifier, err := newRemoteTokenVerifier(ctx, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, verifier)
}

// Tests for NewTokenVerifier

func TestNewTokenVerifier_LocalOnly(t *testing.T) {
	ctx := context.Background()
	_, _, jwksJSON := setupTestKeys(t)

	cfg := TokenVerifierConfig{
		Local: &LocalJWKSConfig{
			JWKS: jwksJSON,
		},
	}

	verifier, err := NewTokenVerifier(ctx, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, verifier)
	assert.NotNil(t, verifier.localKeySet)
	assert.False(t, verifier.isRemote)
}

func TestNewTokenVerifier_RemoteOnly(t *testing.T) {
	ctx := context.Background()
	_, _, jwksJSON := setupTestKeys(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jwksJSON))
	}))
	defer server.Close()

	cfg := TokenVerifierConfig{
		Remote: &RemoteJWKSConfig{
			URLs: []string{server.URL},
		},
	}

	verifier, err := NewTokenVerifier(ctx, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, verifier)
	assert.True(t, verifier.isRemote)
	assert.NotNil(t, verifier.cache)
}

func TestNewTokenVerifier_Combined(t *testing.T) {
	ctx := context.Background()
	_, _, jwksJSON := setupTestKeys(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jwksJSON))
	}))
	defer server.Close()

	cfg := TokenVerifierConfig{
		Local: &LocalJWKSConfig{
			JWKS: jwksJSON,
		},
		Remote: &RemoteJWKSConfig{
			URLs: []string{server.URL},
		},
	}

	verifier, err := NewTokenVerifier(ctx, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, verifier)
	assert.True(t, verifier.isRemote)
	assert.NotNil(t, verifier.cache)
	assert.NotNil(t, verifier.localKeySet)
}

// Tests for newIntrospectionTokenVerifier

func TestNewIntrospectionTokenVerifier_Success(t *testing.T) {
	ctx := context.Background()

	cfg := IntrospectionConfig{
		Endpoint:         "http://example.test/introspect",
		Timeout:          2 * time.Second,
		CacheTTL:         3 * time.Second,
		NegativeCacheTTL: 1 * time.Second,
		UseOnJWTFail:     true,
		IssuerToEndpoint: map[string]string{
			"https://issuer.example": "http://example.test/iss-introspect",
		},
		DefaultCredentials: &IntrospectionCredentials{ClientID: "cid", ClientSecret: "sec"},
		IssuerCredentials: map[string]IntrospectionCredentials{
			"https://issuer.example": {ClientID: "icid", ClientSecret: "isec"},
		},
	}

	v, err := newIntrospectionTokenVerifier(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, v)

	assert.True(t, v.introspectionEnabled)
	assert.NotNil(t, v.httpClient)
	assert.Equal(t, cfg.Endpoint, v.defaultIntrospectEP)
	assert.Equal(t, cfg.CacheTTL, v.cacheTTL)
	assert.Equal(t, cfg.NegativeCacheTTL, v.negativeCacheTTL)
	assert.Equal(t, cfg.UseOnJWTFail, v.useIntrospectionOnFail)
	assert.Equal(t, cfg.IssuerToEndpoint["https://issuer.example"], v.issuerToIntrospectEP["https://issuer.example"])
	require.NotNil(t, v.defaultCreds)
	assert.Equal(t, "cid", v.defaultCreds.ClientID)
	assert.Equal(t, "sec", v.defaultCreds.ClientSecret)
	assert.Equal(t, "icid", v.issuerCreds["https://issuer.example"].ClientID)
	assert.Equal(t, "isec", v.issuerCreds["https://issuer.example"].ClientSecret)
}

func TestNewIntrospectionTokenVerifier_Defaults(t *testing.T) {
	ctx := context.Background()

	cfg := IntrospectionConfig{
		Endpoint: "http://example.test/introspect",
		// Timeout, CacheTTL, NegativeCacheTTL left as zero to trigger defaults
	}

	v, err := newIntrospectionTokenVerifier(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, v)

	// Default timeouts
	require.NotNil(t, v.httpClient)
	assert.Equal(t, 5*time.Second, v.httpClient.Timeout)
	assert.Equal(t, 60*time.Second, v.cacheTTL)
	assert.Equal(t, 15*time.Second, v.negativeCacheTTL)
}

// Ensure NewTokenVerifier (introspection-only) constructs an introspection-enabled verifier
func TestNewTokenVerifier_IntrospectionOnly_Constructed(t *testing.T) {
	ctx := context.Background()

	v, err := NewTokenVerifier(ctx, TokenVerifierConfig{
		Introspection: &IntrospectionConfig{Endpoint: "http://example.test/introspect"},
	})
	require.NoError(t, err)
	require.NotNil(t, v)

	assert.True(t, v.introspectionEnabled)
	assert.Nil(t, v.localKeySet)
	assert.False(t, v.isRemote)
}

// Tests for VerifyAccessToken

func TestVerifyAccessToken_LocalSuccess(t *testing.T) {
	ctx := context.Background()
	privateKey, _, jwksJSON := setupTestKeys(t)

	cfg := LocalJWKSConfig{
		JWKS: jwksJSON,
	}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	require.NoError(t, err)

	// Create valid token
	tokenStr, err := createTestToken(privateKey, "test-key-1", map[string]interface{}{
		"custom_claim": "custom_value",
	})
	require.NoError(t, err)

	authInfo, err := verifier.VerifyAccessToken(ctx, tokenStr)
	assert.NoError(t, err)
	assert.Equal(t, tokenStr, authInfo.Token)
	assert.Equal(t, "test-client", authInfo.ClientID)
	assert.Equal(t, []string{"read", "write"}, authInfo.Scopes)
	assert.NotNil(t, authInfo.Resource)
	assert.Equal(t, "https://api.example.com", authInfo.Resource.String())
	assert.Equal(t, "custom_value", authInfo.Extra["custom_claim"])
}

func TestVerifyAccessToken_InvalidToken(t *testing.T) {
	ctx := context.Background()
	_, _, jwksJSON := setupTestKeys(t)

	cfg := LocalJWKSConfig{
		JWKS: jwksJSON,
	}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	require.NoError(t, err)

	authInfo, err := verifier.VerifyAccessToken(ctx, "invalid-token")
	assert.Error(t, err)
	assert.Empty(t, authInfo)
}

func TestVerifyAccessToken_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	privateKey, _, jwksJSON := setupTestKeys(t)

	cfg := LocalJWKSConfig{
		JWKS: jwksJSON,
	}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	require.NoError(t, err)

	// Create expired token
	key, err := jwk.FromRaw(privateKey)
	require.NoError(t, err)

	err = key.Set(jwk.KeyIDKey, "test-key-1")
	require.NoError(t, err)

	now := time.Now()
	token := jwt.New()

	_ = token.Set(jwt.IssuerKey, "https://example.com")
	_ = token.Set(jwt.SubjectKey, "user123")
	_ = token.Set(jwt.AudienceKey, []string{"https://api.example.com"})
	_ = token.Set(jwt.ExpirationKey, now.Add(-time.Hour)) // Expired 1 hour ago
	_ = token.Set(jwt.IssuedAtKey, now.Add(-2*time.Hour))
	_ = token.Set(jwt.JwtIDKey, "jti-123")
	_ = token.Set("client_id", "test-client")
	_ = token.Set("scope", "read write")
	_ = token.Set("kid", "test-key-1")

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
	require.NoError(t, err)

	authInfo, err := verifier.VerifyAccessToken(ctx, string(signed))
	assert.Error(t, err)
	assert.Empty(t, authInfo)
}

func TestVerifyAccessToken_MissingRequiredClaims(t *testing.T) {
	ctx := context.Background()
	privateKey, _, jwksJSON := setupTestKeys(t)

	cfg := LocalJWKSConfig{
		JWKS: jwksJSON,
	}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	require.NoError(t, err)

	// Create token missing required claims
	key, err := jwk.FromRaw(privateKey)
	require.NoError(t, err)

	err = key.Set(jwk.KeyIDKey, "test-key-1")
	require.NoError(t, err)

	token := jwt.New()
	_ = token.Set(jwt.IssuerKey, "https://example.com")
	// Missing other required claims
	_ = token.Set("kid", "test-key-1")

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
	require.NoError(t, err)

	authInfo, err := verifier.VerifyAccessToken(ctx, string(signed))
	assert.Error(t, err)
	assert.Empty(t, authInfo)
}

func TestVerifyAccessToken_NoMatchingKey(t *testing.T) {
	ctx := context.Background()
	privateKey, _, jwksJSON := setupTestKeys(t)

	cfg := LocalJWKSConfig{
		JWKS: jwksJSON,
	}

	verifier, err := newLocalTokenVerifier(ctx, cfg)
	require.NoError(t, err)

	// Create token with different key ID
	tokenStr, err := createTestToken(privateKey, "different-key-id", nil)
	require.NoError(t, err)

	authInfo, err := verifier.VerifyAccessToken(ctx, tokenStr)
	assert.Error(t, err)
	assert.Empty(t, authInfo)
}

// Tests for extractScopes

func TestExtractScopes_StringFormat(t *testing.T) {
	token := jwt.New()
	_ = token.Set("scope", "read write admin")

	scopes, err := extractScopes(token)
	assert.NoError(t, err)
	assert.Equal(t, []string{"read", "write", "admin"}, scopes)
}

func TestExtractScopes_ArrayFormat(t *testing.T) {
	token := jwt.New()
	_ = token.Set("scope", []string{"read", "write", "admin"})

	scopes, err := extractScopes(token)
	assert.NoError(t, err)
	assert.Equal(t, []string{"read", "write", "admin"}, scopes)
}

func TestExtractScopes_EmptyString(t *testing.T) {
	token := jwt.New()
	_ = token.Set("scope", "")

	scopes, err := extractScopes(token)
	assert.NoError(t, err)
	assert.Empty(t, scopes)
}

func TestExtractScopes_EmptyArray(t *testing.T) {
	token := jwt.New()
	_ = token.Set("scope", []string{})

	scopes, err := extractScopes(token)
	assert.NoError(t, err)
	assert.Empty(t, scopes)
}

// Tests for extractResource

func TestExtractResource_ValidURL(t *testing.T) {
	token := jwt.New()
	_ = token.Set(jwt.AudienceKey, []string{"https://api.example.com/resource"})

	resource, err := extractResource(token)
	assert.NoError(t, err)
	assert.NotNil(t, resource)
	assert.Equal(t, "https://api.example.com/resource", resource.String())
}

func TestExtractResource_URLWithFragment(t *testing.T) {
	token := jwt.New()
	_ = token.Set(jwt.AudienceKey, []string{"https://api.example.com/resource#fragment"})

	resource, err := extractResource(token)
	assert.NoError(t, err)
	assert.NotNil(t, resource)
	assert.Equal(t, "https://api.example.com/resource", resource.String()) // Fragment should be removed
}

func TestExtractResource_MissingAudience(t *testing.T) {
	token := jwt.New()

	resource, err := extractResource(token)
	assert.Error(t, err)
	assert.Nil(t, resource)
}

// Tests for extractExtra

func TestExtractExtra_WithCustomClaims(t *testing.T) {
	token := jwt.New()
	_ = token.Set(jwt.IssuerKey, "https://example.com") // Standard claim
	_ = token.Set("custom_claim1", "value1")            // Custom claim
	_ = token.Set("custom_claim2", 123)                 // Custom claim
	_ = token.Set("client_id", "test-client")           // Standard claim

	extra := extractExtra(token)
	assert.NotNil(t, extra)
	assert.Equal(t, "value1", extra["custom_claim1"])
	assert.Equal(t, 123, extra["custom_claim2"])
	assert.NotContains(t, extra, "iss")       // Standard claims should be excluded
	assert.NotContains(t, extra, "client_id") // Standard claims should be excluded
}

func TestExtractExtra_NoCustomClaims(t *testing.T) {
	token := jwt.New()
	_ = token.Set(jwt.IssuerKey, "https://example.com")
	_ = token.Set("client_id", "test-client")

	extra := extractExtra(token)
	assert.Nil(t, extra) // Should return nil for omitempty
}

// Integration tests

func TestVerifyAccessToken_RemoteJWKS(t *testing.T) {
	ctx := context.Background()
	privateKey, _, jwksJSON := setupTestKeys(t)

	// Create test server for JWKS
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jwksJSON))
	}))
	defer server.Close()

	cfg := RemoteJWKSConfig{
		URLs: []string{server.URL},
		IssuerToURL: map[string]string{
			"https://example.com": server.URL,
		},
	}

	verifier, err := newRemoteTokenVerifier(ctx, cfg)
	require.NoError(t, err)

	// Create valid token
	tokenStr, err := createTestToken(privateKey, "test-key-1", nil)
	require.NoError(t, err)

	authInfo, err := verifier.VerifyAccessToken(ctx, tokenStr)
	assert.NoError(t, err)
	assert.Equal(t, tokenStr, authInfo.Token)
	assert.Equal(t, "test-client", authInfo.ClientID)
}

func TestVerifyAccessToken_MixedMode_LocalKeyFound(t *testing.T) {
	ctx := context.Background()
	privateKey, _, jwksJSON := setupTestKeys(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jwksJSON))
	}))
	defer server.Close()

	cfg := TokenVerifierConfig{
		Local: &LocalJWKSConfig{
			JWKS: jwksJSON,
		},
		Remote: &RemoteJWKSConfig{
			URLs: []string{server.URL},
			IssuerToURL: map[string]string{
				"https://example.com": server.URL,
			},
		},
	}

	verifier, err := NewTokenVerifier(ctx, cfg)
	require.NoError(t, err)

	// Create valid token
	tokenStr, err := createTestToken(privateKey, "test-key-1", nil)
	require.NoError(t, err)

	authInfo, err := verifier.VerifyAccessToken(ctx, tokenStr)
	assert.NoError(t, err)
	assert.Equal(t, tokenStr, authInfo.Token)
}

// Introspection-only mode: no JWKS configured, only introspection is used.
func TestVerifyAccessToken_IntrospectionOnly_Mode(t *testing.T) {
	ctx := context.Background()

	// Fake introspection endpoint which returns active token and minimal payload
	introspectCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		introspectCalls++
		_ = r.ParseForm()
		token := r.FormValue("token")
		// return an active response regardless of token content
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"active":    true,
			"scope":     "read write",
			"client_id": "cli-123",
			"exp":       float64(time.Now().Add(5 * time.Minute).Unix()),
			"aud":       "https://api.example.com",
		}
		// echo part to ensure parser tolerates arbitrary fields
		if token != "" {
			resp["token_hash"] = len(token)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	v, err := NewTokenVerifier(ctx, TokenVerifierConfig{
		Introspection: &IntrospectionConfig{
			Endpoint:         server.URL,
			Timeout:          2 * time.Second,
			CacheTTL:         2 * time.Second,
			NegativeCacheTTL: 1 * time.Second,
			UseOnJWTFail:     true,
		},
	})
	require.NoError(t, err)

	// Opaque token scenario
	ai, err := v.VerifyAccessToken(ctx, "opaque-token-abc")
	assert.NoError(t, err)
	assert.Equal(t, "cli-123", ai.ClientID)
	assert.ElementsMatch(t, []string{"read", "write"}, ai.Scopes)
	assert.NotNil(t, ai.ExpiresAt)

	// Cache hit path
	ai2, err := v.VerifyAccessToken(ctx, "opaque-token-abc")
	assert.NoError(t, err)
	assert.Equal(t, ai.ClientID, ai2.ClientID)
	assert.LessOrEqual(t, introspectCalls, 2) // first call + maybe cache check
}

// Key rotation: first JWKS does not contain target kid, second fetch returns rotated JWKS.
func TestVerifyAccessToken_RemoteJWKS_KeyRotation_RefreshOnKidMiss(t *testing.T) {
	ctx := context.Background()

	// old key (won't match token)
	oldPriv, err := generateRSAKey()
	require.NoError(t, err)
	oldPub, err := createTestJWK(oldPriv, "old-key")
	require.NoError(t, err)

	// new key (used to sign token)
	newPriv, err := generateRSAKey()
	require.NoError(t, err)
	newPub, err := createTestJWK(newPriv, "new-key")
	require.NoError(t, err)

	jwksOld := createTestJWKS(oldPub)
	jwksNew := createTestJWKS(newPub)

	// JWKS server: first call -> old, subsequent -> new
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if callCount == 0 {
			_, _ = w.Write([]byte(jwksOld))
		} else {
			_, _ = w.Write([]byte(jwksNew))
		}
		callCount++
	}))
	defer server.Close()

	cfg := RemoteJWKSConfig{
		URLs: []string{server.URL},
		IssuerToURL: map[string]string{
			"https://example.com": server.URL,
		},
		RefreshInterval: time.Minute,
	}

	verifier, err := newRemoteTokenVerifier(ctx, cfg)
	require.NoError(t, err)

	// Token signed by new key (kid=new-key). First cache lookup sees old JWKS
	tokenStr, err := createTestToken(newPriv, "new-key", nil)
	require.NoError(t, err)

	authInfo, err := verifier.VerifyAccessToken(ctx, tokenStr)
	assert.NoError(t, err)
	assert.Equal(t, tokenStr, authInfo.Token)

	// Expect at least two server calls: initial cache fetch + forced refresh
	assert.GreaterOrEqual(t, callCount, 2)
}
