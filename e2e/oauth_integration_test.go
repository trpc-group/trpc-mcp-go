// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/providers"
)

const (
	testHMACSecret = "test-oauth-secret"
	testClientID   = "test-client"
	testScope      = "mcp.read mcp.write"
)

// TestOAuth2Integration tests the complete OAuth 2.1 flow with MCP server
func TestOAuth2Integration(t *testing.T) {
	// Start mock OAuth authorization server
	oauthServer := startMockOAuthServer(t)
	defer oauthServer.Close()

	// Create OAuth Provider
	provider := createTestOAuthProvider(oauthServer.URL)

	// Start MCP server with OAuth authentication
	mcpServerURL, cleanup := startOAuthMCPServer(t, provider)
	defer cleanup()

	// Test OAuth flows
	t.Run("BearerTokenAuth", func(t *testing.T) {
		testBearerTokenAuth(t, oauthServer.URL, mcpServerURL)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		testInvalidToken(t, mcpServerURL)
	})

	// Test authorization code flow
	t.Run("AuthorizationCodeFlow", func(t *testing.T) {
		testSimpleAuthorizationCodeFlow(t, oauthServer.URL, mcpServerURL)
	})

	t.Run("TokenRefresh", func(t *testing.T) {
		testTokenRefresh(t, oauthServer.URL, mcpServerURL)
	})
}

// startMockOAuthServer starts a mock OAuth authorization server for testing
func startMockOAuthServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// Authorization endpoint
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Simulate user authorization by redirecting directly to callback URL
		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")
		code := "test-auth-code-" + fmt.Sprintf("%d", time.Now().Unix())

		callbackURL := fmt.Sprintf("%s?code=%s&state=%s", redirectURI, code, state)
		http.Redirect(w, r, callbackURL, http.StatusFound)
	})

	// Token endpoint
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}

		grantType := r.FormValue("grant_type")
		code := r.FormValue("code")
		refreshToken := r.FormValue("refresh_token")

		var tokenResponse map[string]interface{}

		switch grantType {
		case "authorization_code":
			if code == "" {
				http.Error(w, "Missing authorization code", http.StatusBadRequest)
				return
			}
			tokenResponse = createTokenResponse(t, "access_token", "refresh_token")

		case "refresh_token":
			if refreshToken == "" {
				http.Error(w, "Missing refresh token", http.StatusBadRequest)
				return
			}
			// Validate refresh token
			if !strings.HasPrefix(refreshToken, "test-refresh-token-") {
				http.Error(w, "Invalid refresh token", http.StatusBadRequest)
				return
			}
			tokenResponse = createTokenResponse(t, "new_access_token", "new_refresh_token")

		default:
			http.Error(w, "Unsupported grant type", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResponse)
	})

	// Client registration endpoint
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		clientInfo := map[string]interface{}{
			"client_id":     testClientID,
			"client_secret": "",
			"redirect_uris": []string{"http://localhost:5173/callback"},
			"grant_types":   []string{"authorization_code", "refresh_token"},
			"scope":         testScope,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(clientInfo)
	})

	// OAuth authorization server metadata endpoint
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Dynamically get server URL
		baseURL := "http://" + r.Host
		metadata := map[string]interface{}{
			"issuer":                                baseURL,
			"authorization_endpoint":                baseURL + "/authorize",
			"token_endpoint":                        baseURL + "/token",
			"registration_endpoint":                 baseURL + "/register",
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
			"scopes_supported":                      []string{"mcp.read", "mcp.write"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(metadata)
	})

	// OpenID Connect configuration endpoint (for compatibility)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Dynamically get server URL
		baseURL := "http://" + r.Host
		// Return same content as OAuth metadata
		metadata := map[string]interface{}{
			"issuer":                                baseURL,
			"authorization_endpoint":                baseURL + "/authorize",
			"token_endpoint":                        baseURL + "/token",
			"registration_endpoint":                 baseURL + "/register",
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
			"scopes_supported":                      []string{"mcp.read", "mcp.write"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(metadata)
	})

	server := httptest.NewServer(mux)
	t.Logf("Mock OAuth server started at: %s", server.URL)
	return server
}

// createTestOAuthProvider creates an OAuth provider for testing
func createTestOAuthProvider(oauthServerURL string) server.OAuthServerProvider {
	return providers.NewProxyOAuthServerProvider(providers.ProxyOptions{
		Endpoints: providers.ProxyEndpoints{
			AuthorizationURL: oauthServerURL + "/authorize",
			TokenURL:         oauthServerURL + "/token",
			RegistrationURL:  oauthServerURL + "/register",
		},
		VerifyAccessToken: func(token string) (*server.AuthInfo, error) {
			return verifyTestJWT(token)
		},
		GetClient: func(clientID string) (*auth.OAuthClientInformationFull, error) {
			return &auth.OAuthClientInformationFull{
				OAuthClientMetadata: auth.OAuthClientMetadata{
					RedirectURIs:  []string{"http://localhost:5173/callback"},
					ResponseTypes: []string{"code"},
					GrantTypes:    []string{"authorization_code", "refresh_token"},
					ClientName:    stringPtr("test-client"),
					Scope:         stringPtr(testScope),
				},
				OAuthClientInformation: auth.OAuthClientInformation{
					ClientID:     clientID,
					ClientSecret: "",
				},
			}, nil
		},
	})
}

// startOAuthMCPServer starts an MCP server with OAuth authentication enabled
func startOAuthMCPServer(t *testing.T, provider server.OAuthServerProvider) (string, func()) {
	t.Helper()

	// Create MCP server using standardized approach
	server := mcp.NewServer(
		"OAuth-Test-Server",
		"1.0.0",
		mcp.WithServerPath("/mcp"),
		mcp.WithOAuthRoutes(mcp.OAuthRoutesConfig{
			Provider:        provider,
			IssuerURL:       mustParseURL("http://localhost:3030"),
			BaseURL:         mustParseURL("http://localhost:3000"),
			ScopesSupported: []string{"mcp.read", "mcp.write"},
		}),
		mcp.WithBearerAuth(&mcp.BearerAuthConfig{
			Enabled:        true,
			RequiredScopes: []string{"mcp.read", "mcp.write"},
			Issuer:         "http://localhost:3030",
			Audience:       []string{"http://localhost:3000"},
			Verifier: server.TokenVerifierFunc(func(ctx context.Context, token string) (server.AuthInfo, error) {
				authInfo, err := verifyTestJWT(token)
				if err != nil {
					return server.AuthInfo{}, err
				}
				return *authInfo, nil
			}),
		}),
	)

	// Register test tools using standardized approach
	RegisterTestTools(server)

	// Create HTTP test server
	httpServer := httptest.NewServer(server.HTTPHandler())
	serverURL := httpServer.URL + "/mcp"

	t.Logf("OAuth MCP server started at: %s", serverURL)

	cleanup := func() {
		t.Log("Closing OAuth MCP server")
		httpServer.Close()
	}

	return serverURL, cleanup
}

// testBearerTokenAuth tests Bearer Token authentication using standardized approach
func testBearerTokenAuth(t *testing.T, oauthServerURL, mcpServerURL string) {
	t.Helper()

	// Create client directly with a valid JWT Token
	validToken := createTestJWT(t, "access_token")

	// Create HTTP headers with Bearer Token
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+validToken)

	// Use standardized client creation
	client := CreateTestClient(t, mcpServerURL, func(c *mcp.Client) {
		// Apply OAuth headers
		mcp.WithHTTPHeaders(headers)(c)
	})
	defer CleanupClient(t, client)

	// Initialize client using standardized approach
	InitializeClient(t, client)

	// Test tool invocation using standardized approach
	content := ExecuteTestTool(t, client, "basic-greet", map[string]interface{}{
		"name": "bearer-test",
	})

	require.Len(t, content, 1)
	textContent, ok := content[0].(mcp.TextContent)
	assert.True(t, ok)
	assert.Contains(t, textContent.Text, "Hello, bearer-test")
}

// testInvalidToken tests invalid token handling using standardized approach
func testInvalidToken(t *testing.T, mcpServerURL string) {
	t.Helper()

	// Create client with invalid token
	invalidToken := "invalid.jwt.token"

	// Create HTTP headers with invalid Bearer Token
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+invalidToken)

	// Use standardized client creation
	client := CreateTestClient(t, mcpServerURL, func(c *mcp.Client) {
		// Apply OAuth headers
		mcp.WithHTTPHeaders(headers)(c)
	})
	defer CleanupClient(t, client)

	// Try to initialize client, should fail
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()

	_, err := client.Initialize(ctx, &mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.ProtocolVersion_2025_03_26,
			ClientInfo: mcp.Implementation{
				Name:    "Invalid-Token-Client",
				Version: "1.0.0",
			},
		},
	})

	// Should return authentication error
	assert.Error(t, err)
	// Check if it contains authentication-related error message
	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "unauthorized") ||
			strings.Contains(errorMsg, "401") ||
			strings.Contains(errorMsg, "authentication") ||
			strings.Contains(errorMsg, "auth"),
		"Expected authentication error, got: %s", errorMsg)
}

// testSimpleAuthorizationCodeFlow tests the simplified authorization code flow
func testSimpleAuthorizationCodeFlow(t *testing.T, oauthServerURL, mcpServerURL string) {
	t.Helper()

	// Test authorization endpoint functionality
	t.Run("AuthorizationEndpoint", func(t *testing.T) {
		// Start a simple callback server
		callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Callback received: %s", r.URL.RawQuery)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer callbackServer.Close()

		// Build authorization URL with URL encoding
		params := url.Values{}
		params.Set("client_id", testClientID)
		params.Set("response_type", "code")
		params.Set("redirect_uri", callbackServer.URL+"/callback")
		params.Set("scope", testScope)
		params.Set("state", "test-state")

		authURL := oauthServerURL + "/authorize?" + params.Encode()
		t.Logf("Testing authorization URL: %s", authURL)

		// Create HTTP client that doesn't follow redirects
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		}

		// Access authorization endpoint
		resp, err := client.Get(authURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should redirect to callback URL
		assert.Equal(t, http.StatusFound, resp.StatusCode)

		// Check redirect URL
		location := resp.Header.Get("Location")
		t.Logf("Redirect location: %s", location)
		assert.Contains(t, location, "code=")
		assert.Contains(t, location, "state=test-state")
	})

	// Test token endpoint functionality
	t.Run("TokenEndpoint", func(t *testing.T) {
		// Simulate authorization code exchange for tokens
		formData := url.Values{}
		formData.Set("grant_type", "authorization_code")
		formData.Set("code", "test-auth-code-123")
		formData.Set("redirect_uri", "http://localhost:5173/callback")

		resp, err := http.PostForm(oauthServerURL+"/token", formData)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return success
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Validate response content
		var tokenResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&tokenResp)
		require.NoError(t, err)

		assert.Contains(t, tokenResp, "access_token")
		assert.Contains(t, tokenResp, "refresh_token")
		assert.Equal(t, "Bearer", tokenResp["token_type"])
		assert.Equal(t, testScope, tokenResp["scope"])
	})

	// Test OAuth metadata endpoint
	t.Run("OAuthMetadata", func(t *testing.T) {
		// Test OAuth authorization server metadata
		resp, err := http.Get(oauthServerURL + "/.well-known/oauth-authorization-server")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var metadata map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&metadata)
		require.NoError(t, err)

		assert.Equal(t, oauthServerURL, metadata["issuer"])
		assert.Contains(t, metadata, "authorization_endpoint")
		assert.Contains(t, metadata, "token_endpoint")
		assert.Contains(t, metadata, "scopes_supported")
	})
}

// testTokenRefresh tests token refresh functionality
func testTokenRefresh(t *testing.T, oauthServerURL, mcpServerURL string) {
	t.Helper()

	// Create a client with a refresh token
	refreshToken := "test-refresh-token-" + fmt.Sprintf("%d", time.Now().Unix())

	// Create OAuth Provider that supports token refresh
	provider := providers.NewProxyOAuthServerProvider(providers.ProxyOptions{
		Endpoints: providers.ProxyEndpoints{
			AuthorizationURL: oauthServerURL + "/authorize",
			TokenURL:         oauthServerURL + "/token",
			RegistrationURL:  oauthServerURL + "/register",
		},
		VerifyAccessToken: func(token string) (*server.AuthInfo, error) {
			return verifyTestJWT(token)
		},
		GetClient: func(clientID string) (*auth.OAuthClientInformationFull, error) {
			return &auth.OAuthClientInformationFull{
				OAuthClientMetadata: auth.OAuthClientMetadata{
					RedirectURIs:  []string{"http://localhost:5173/callback"},
					ResponseTypes: []string{"code"},
					GrantTypes:    []string{"authorization_code", "refresh_token"},
					ClientName:    stringPtr("test-client"),
					Scope:         stringPtr(testScope),
				},
				OAuthClientInformation: auth.OAuthClientInformation{
					ClientID:     clientID,
					ClientSecret: "",
				},
			}, nil
		},
	})

	// Start MCP server with OAuth
	mcpServerURL, cleanup := startOAuthMCPServer(t, provider)
	defer cleanup()

	// Test token refresh flow
	t.Run("RefreshTokenFlow", func(t *testing.T) {
		// Simulate refresh token request
		refreshReq := map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": refreshToken,
		}

		// Send refresh request to OAuth server
		formData := url.Values{}
		for k, v := range refreshReq {
			formData.Set(k, v)
		}
		resp, err := http.PostForm(oauthServerURL+"/token", formData)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Validate response
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var tokenResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&tokenResp)
		require.NoError(t, err)

		// Validate returned tokens
		assert.Contains(t, tokenResp, "access_token")
		assert.Contains(t, tokenResp, "refresh_token")
		assert.Equal(t, "Bearer", tokenResp["token_type"])
		assert.Equal(t, testScope, tokenResp["scope"])

		// Validate new access token is valid
		newAccessToken, ok := tokenResp["access_token"].(string)
		require.True(t, ok)

		// Create client with new access token using standardized approach
		headers := make(http.Header)
		headers.Set("Authorization", "Bearer "+newAccessToken)

		client := CreateTestClient(t, mcpServerURL, func(c *mcp.Client) {
			mcp.WithHTTPHeaders(headers)(c)
		})
		defer CleanupClient(t, client)

		// Initialize client using standardized approach
		InitializeClient(t, client)

		// Test tool invocation using standardized approach
		content := ExecuteTestTool(t, client, "basic-greet", map[string]interface{}{
			"name": "refresh-test",
		})

		require.Len(t, content, 1)
		textContent, ok := content[0].(mcp.TextContent)
		assert.True(t, ok)
		assert.Contains(t, textContent.Text, "Hello, refresh-test")
	})

	// Test invalid refresh token
	t.Run("InvalidRefreshToken", func(t *testing.T) {
		invalidRefreshReq := map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": "invalid-refresh-token",
		}

		formData := url.Values{}
		for k, v := range invalidRefreshReq {
			formData.Set(k, v)
		}
		resp, err := http.PostForm(oauthServerURL+"/token", formData)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return error (400 Bad Request)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// createTokenResponse creates a test token response with JWT access token
func createTokenResponse(t *testing.T, accessToken, refreshToken string) map[string]interface{} {
	t.Helper()

	return map[string]interface{}{
		"access_token":  createTestJWT(t, accessToken),
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"scope":         testScope,
	}
}

// createTestJWT creates a test JWT token with specified token type
func createTestJWT(t *testing.T, tokenType string) string {
	t.Helper()

	claims := jwt.MapClaims{
		"iss":        "http://localhost:3030",
		"aud":        []string{"http://localhost:3000"},
		"sub":        testClientID,
		"scope":      "mcp.read mcp.write", // Ensure correct scope
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(time.Hour).Unix(),
		"token_type": tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(testHMACSecret))
	require.NoError(t, err)
	return signedToken
}

// verifyTestJWT verifies and parses a test JWT token
func verifyTestJWT(tokenString string) (*server.AuthInfo, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(testHMACSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Scopes
		scopes := []string{}
		if scope, ok := claims["scope"].(string); ok {
			scopes = strings.Fields(scope)
		}

		// ExpiresAt
		var expiresAtPtr *int64
		if v, ok := claims["exp"].(float64); ok {
			vv := int64(v)
			expiresAtPtr = &vv
		}

		// Audience -> Resource (first value)
		var resourceURL *url.URL
		if audVal, ok := claims["aud"]; ok {
			switch v := audVal.(type) {
			case string:
				if u, err := url.Parse(v); err == nil {
					resourceURL = u
				}
			case []interface{}:
				if len(v) > 0 {
					if s, ok := v[0].(string); ok {
						if u, err := url.Parse(s); err == nil {
							resourceURL = u
						}
					}
				}
			case []string:
				if len(v) > 0 {
					if u, err := url.Parse(v[0]); err == nil {
						resourceURL = u
					}
				}
			}
		}

		// ClientID
		clientID, _ := claims["sub"].(string)

		// Extra (include iss and client_id)
		extra := map[string]interface{}{}
		if iss, ok := claims["iss"].(string); ok {
			extra["iss"] = iss
		}
		if clientID != "" {
			extra["client_id"] = clientID
		}

		return &server.AuthInfo{
			ClientID:  clientID,
			Scopes:    scopes,
			ExpiresAt: expiresAtPtr,
			Resource:  resourceURL,
			Extra:     extra,
		}, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

// mustParseURL parses a URL string and panics if parsing fails
func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
