// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// postFormWithBasicAuth sends a POST request with x-www-form-urlencoded body and HTTP Basic auth
func postFormWithBasicAuth(t *testing.T, h http.Handler, path string, form url.Values, clientID, clientSecret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// postForm sends a POST request with x-www-form-urlencoded body (no client auth)
func postForm(t *testing.T, h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// postFormWithOrigin sends a POST request with x-www-form-urlencoded body, Basic auth, and Origin header for CORS tests
func postFormWithOrigin(t *testing.T, h http.Handler, path string, form url.Values, clientID, clientSecret, origin string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	req.SetBasicAuth(clientID, clientSecret)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// enhancedMockOAuthClientsStore is a simple in-memory clients store used in tests
type enhancedMockOAuthClientsStore struct {
	clients map[string]*auth.OAuthClientInformationFull // client_id -> client record
}

// GetClient returns the client by id or an error if not found.
func (m *enhancedMockOAuthClientsStore) GetClient(clientID string) (*auth.OAuthClientInformationFull, error) {
	client, exists := m.clients[clientID]
	if !exists {
		return nil, fmt.Errorf("client not found")
	}
	return client, nil
}

// enhancedMockOAuthServerProvider simulates an OAuth server provider with toggles for different paths
type enhancedMockOAuthServerProvider struct {
	clientStore               *enhancedMockOAuthClientsStore // backing store for clients
	skipLocalPkceValidation   bool                           // when true, PKCE is not validated locally
	shouldReturnIdToken       bool                           // when true, adds id_token to token response
	shouldFailCodeChallenge   bool                           // when true, ChallengeForAuthorizationCode fails
	shouldFailCodeExchange    bool                           // when true, ExchangeAuthorizationCode fails with invalid_grant
	shouldFailRefreshExchange bool                           // when true, ExchangeRefreshToken fails with invalid_grant
	supportedScopes           []string                       // list of supported scopes (for tests that depend on scope echo)
}

// GetSkipLocalPkceValidation exposes whether local PKCE verification should be skipped
func (m *enhancedMockOAuthServerProvider) GetSkipLocalPkceValidation() bool {
	return m.skipLocalPkceValidation
}

// ClientsStore returns a thin adapter around the in-memory store to satisfy the interface
func (m *enhancedMockOAuthServerProvider) ClientsStore() *server.OAuthClientsStore {
	return server.NewOAuthClientStore(m.clientStore.GetClient)
}

// Authorize simulates authorization success by redirecting with code and echoing state
func (m *enhancedMockOAuthServerProvider) Authorize(client auth.OAuthClientInformationFull, params server.AuthorizationParams, res http.ResponseWriter, req *http.Request) error {
	// Compose a 302 redirect with code + state
	res.Header().Set("Location", "https://redirect-uri.com?code=valid-code&state="+params.State)
	res.WriteHeader(http.StatusFound)
	return nil
}

// ChallengeForAuthorizationCode returns a fixed S256 challenge for "valid-code" and errors otherwise
func (m *enhancedMockOAuthServerProvider) ChallengeForAuthorizationCode(
	client auth.OAuthClientInformationFull,
	authorizationCode string,
) (string, error) {
	if m.shouldFailCodeChallenge {
		return "", errors.ErrInvalidGrant
	}
	switch authorizationCode {
	case "valid-code":
		// Matches code_verifier = "valid-verifier"
		return "A_DCKa0ei4rJGhNfKEbwNpiuHzQP7skGQPZ4CBTkJdQ", nil
	case "expired-code":
		return "", fmt.Errorf("authorization code has expired")
	case "invalid-code":
		return "", fmt.Errorf("authorization code is invalid")
	default:
		return "", fmt.Errorf("unknown authorization code")
	}
}

// ExchangeAuthorizationCode returns mock tokens for "valid-code" and errors for others
func (m *enhancedMockOAuthServerProvider) ExchangeAuthorizationCode(
	client auth.OAuthClientInformationFull,
	authorizationCode string,
	codeVerifier *string,
	redirectUri *string,
	resource *url.URL,
) (*auth.OAuthTokens, error) {
	// Simulate upstream failure toggle
	if m.shouldFailCodeExchange {
		return nil, errors.ErrInvalidGrant
	}

	switch authorizationCode {
	case "valid-code":
		expiresIn := int64(3600)
		refreshToken := "mock-refresh-token"

		tokens := &auth.OAuthTokens{
			AccessToken:  "mock-access-token",
			TokenType:    "bearer",
			ExpiresIn:    &expiresIn,
			RefreshToken: &refreshToken,
		}
		// Optionally attach an ID token for OIDC scenarios
		if m.shouldReturnIdToken {
			idToken := "mock-id-token"
			tokens.IDToken = &idToken
		}
		return tokens, nil
	case "expired-code":
		return nil, fmt.Errorf("authorization code has expired")
	case "invalid-code":
		return nil, fmt.Errorf("authorization code is invalid")
	default:
		return nil, errors.ErrInvalidGrant
	}
}

// ExchangeRefreshToken returns a new access/refresh token pair for a valid refresh token
func (m *enhancedMockOAuthServerProvider) ExchangeRefreshToken(
	client auth.OAuthClientInformationFull,
	refreshToken string,
	scopes []string,
	resource *url.URL,
) (*auth.OAuthTokens, error) {
	if m.shouldFailRefreshExchange {
		return nil, errors.ErrInvalidGrant
	}

	switch refreshToken {
	case "valid-refresh-token":
		expiresIn := int64(3600)
		newRefreshToken := "new-mock-refresh-token"

		tokens := &auth.OAuthTokens{
			AccessToken:  "new-mock-access-token",
			TokenType:    "bearer",
			ExpiresIn:    &expiresIn,
			RefreshToken: &newRefreshToken,
		}
		if len(scopes) > 0 {
			scope := strings.Join(scopes, " ")
			tokens.Scope = &scope
		}
		return tokens, nil
	case "invalid-refresh-token":
		return nil, fmt.Errorf("refresh token is invalid")
	default:
		return nil, errors.ErrInvalidGrant
	}
}

// VerifyAccessToken returns a stubbed AuthInfo when token == "valid-token"
func (m *enhancedMockOAuthServerProvider) VerifyAccessToken(token string) (*server.AuthInfo, error) {
	if token == "valid-token" {
		return &server.AuthInfo{
			ClientID: "valid-client-id",
			Scopes:   []string{"read", "write"},
		}, nil
	}
	return nil, fmt.Errorf("invalid token")
}

// RevokeToken is a no-op in this mock; revocation success is implied
func (m *enhancedMockOAuthServerProvider) RevokeToken(client auth.OAuthClientInformationFull, request auth.OAuthTokenRevocationRequest) error {
	return nil
}

// createMockClient builds a confidential client (client_secret_basic) with a fixed secret
func createMockClient(id string) *auth.OAuthClientInformationFull {
	return &auth.OAuthClientInformationFull{
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID:     id,
			ClientSecret: "valid-secret",
		},
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs:            []string{"https://example.com/callback"},
			TokenEndpointAuthMethod: "client_secret_basic",
		},
	}
}

// createEnhancedMockProvider wires the in-memory clients store and returns a preconfigured provider
func createEnhancedMockProvider() *enhancedMockOAuthServerProvider {
	clients := make(map[string]*auth.OAuthClientInformationFull)
	clients["valid-client"] = createMockClient("valid-client")
	store := &enhancedMockOAuthClientsStore{clients: clients}

	return &enhancedMockOAuthServerProvider{
		clientStore:     store,
		supportedScopes: []string{"read", "write", "profile", "email"},
	}
}

func TestToken_RequiresPostMethod(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	req := httptest.NewRequest(http.MethodGet, "/token", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	assert.Equal(t, "POST", rr.Header().Get("Allow"))
}

func TestToken_RequiresGrantType(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{}
	// Missing grant_type

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_request", errResp["error"])
}

func TestToken_RejectsUnsupportedGrantTypes(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type": {"password"}, // Unsupported grant type
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "unsupported_grant_type", errResp["error"])
	assert.Equal(t, "The grant type is not supported by this authorization server.", errResp["error_description"])
}

func TestToken_RequiresValidClientCredentials_CurrentBehavior(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type": {"authorization_code"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "invalid-client", "wrong-secret")

	// HTTP status should be 401
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))

	assert.Equal(t, "invalid_client", errResp["error"])
	assert.Contains(t, errResp["error_description"].(string), "client")
}

func TestToken_AcceptsValidClientCredentials(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"code_verifier": {"valid-verifier"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestToken_AuthorizationCode_RequiresCodeParameter(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type": {"authorization_code"},
		// Missing code
		"code_verifier": {"valid-verifier"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_request", errResp["error"])
}

func TestToken_AuthorizationCode_RequiresCodeVerifierParameter(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type": {"authorization_code"},
		"code":       {"valid-code"},
		// Missing code_verifier
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_request", errResp["error"])
}

func TestToken_AuthorizationCode_VerifiesPKCEChallenge(t *testing.T) {
	provider := createEnhancedMockProvider()
	provider.shouldFailCodeChallenge = false // Ensure challenge retrieval works

	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"code_verifier": {"invalid-verifier"}, // This won't match the challenge
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_grant", errResp["error"])
	assert.Contains(t, errResp["error_description"], "code_verifier")
}

func TestToken_AuthorizationCode_RejectsExpiredCode(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"expired-code"},
		"code_verifier": {"valid-verifier"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_grant", errResp["error"])
}

func TestToken_AuthorizationCode_RejectsInvalidCode(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"invalid-code"},
		"code_verifier": {"valid-verifier"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_grant", errResp["error"])
}

func TestToken_AuthorizationCode_ReturnsTokensForValidExchange(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"code_verifier": {"valid-verifier"},
		"resource":      {"https://api.example.com/resource"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusOK, rr.Code)

	var tokens map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &tokens))
	assert.Equal(t, "mock-access-token", tokens["access_token"])
	assert.Equal(t, "bearer", tokens["token_type"])
	assert.Equal(t, float64(3600), tokens["expires_in"])
	assert.Equal(t, "mock-refresh-token", tokens["refresh_token"])
}

func TestToken_AuthorizationCode_ReturnsIdTokenWhenProvided(t *testing.T) {
	provider := createEnhancedMockProvider()
	provider.shouldReturnIdToken = true

	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"code_verifier": {"valid-verifier"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusOK, rr.Code)

	var tokens map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &tokens))
	assert.Equal(t, "mock-id-token", tokens["id_token"])
}

func TestToken_RefreshToken_RequiresRefreshTokenParameter(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type": {"refresh_token"},
		// Missing refresh_token
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_request", errResp["error"])
}

func TestToken_RefreshToken_RejectsInvalidRefreshToken(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"invalid-refresh-token"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_grant", errResp["error"])
}

func TestToken_RefreshToken_ReturnsNewTokensForValidRefresh(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"valid-refresh-token"},
		"resource":      {"https://api.example.com/resource"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusOK, rr.Code)

	var tokens map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &tokens))
	assert.Equal(t, "new-mock-access-token", tokens["access_token"])
	assert.Equal(t, "bearer", tokens["token_type"])
	assert.Equal(t, float64(3600), tokens["expires_in"])
	assert.Equal(t, "new-mock-refresh-token", tokens["refresh_token"])
}

func TestToken_RefreshToken_RespectsRequestedScopes(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"valid-refresh-token"},
		"scope":         {"profile email"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusOK, rr.Code)

	var tokens map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &tokens))
	assert.Equal(t, "profile email", tokens["scope"])
}

func TestToken_IncludesCORSHeaders(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"code_verifier": {"valid-verifier"},
	}

	rr := postFormWithOrigin(t, handler, "/token", form, "valid-client", "valid-secret", "https://example.com")

	assert.Equal(t, http.StatusOK, rr.Code)

	// Check CORS headers
	assert.Contains(t, rr.Header().Get("Access-Control-Allow-Origin"), "*")
}

func TestToken_RateLimiting(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 1), // Only 1 request allowed
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"code_verifier": {"valid-verifier"},
	}

	// First request should succeed
	rr1 := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Second request should be rate-limited
	rr2 := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
}

func TestToken_SetsCacheControlHeaders(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"code_verifier": {"valid-verifier"},
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}

func TestToken_RejectsOPTIONSMethod(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	req := httptest.NewRequest(http.MethodOptions, "/token", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestToken_ValidatesResourceParameter(t *testing.T) {
	provider := createEnhancedMockProvider()
	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"code_verifier": {"valid-verifier"},
		"resource":      {"invalid-url"}, // Invalid URL
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_request", errResp["error"])
	assert.Contains(t, errResp["error_description"], "resource")
}

func TestToken_SkipLocalPKCEValidation(t *testing.T) {
	provider := createEnhancedMockProvider()
	provider.skipLocalPkceValidation = true

	handler := TokenHandler(TokenHandlerOptions{
		Provider:  provider,
		RateLimit: rate.NewLimiter(rate.Every(15*time.Minute/50), 50),
	})

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"valid-code"},
		"code_verifier": {"any-verifier"}, // Should be passed through without local validation
	}

	rr := postFormWithBasicAuth(t, handler, "/token", form, "valid-client", "valid-secret")

	assert.Equal(t, http.StatusOK, rr.Code)
}
