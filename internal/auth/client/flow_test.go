// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

// mockOAuthClientProvider provides a test double implementing the OAuth client provider interfaces
type mockOAuthClientProvider struct {
	clientInfo    *auth.OAuthClientInformation
	tokens        *auth.OAuthTokens
	codeVerifier  string
	redirectURL   string
	clientMeta    auth.OAuthClientMetadata
	saveTokensErr error
	saveCodeErr   error
	redirectErr   error
	invalidateErr error
	stateValue    string
	stateErr      error
}

// ClientInformation returns the current OAuth client credentials used by the client
func (m *mockOAuthClientProvider) ClientInformation() *auth.OAuthClientInformation {
	return m.clientInfo
}

// Tokens returns the currently stored OAuth tokens
func (m *mockOAuthClientProvider) Tokens() (*auth.OAuthTokens, error) {
	return m.tokens, nil
}

// SaveTokens persists newly issued OAuth tokens
func (m *mockOAuthClientProvider) SaveTokens(tokens auth.OAuthTokens) error {
	m.tokens = &tokens
	return m.saveTokensErr
}

// SaveCodeVerifier persists the PKCE code verifier for later token exchange
func (m *mockOAuthClientProvider) SaveCodeVerifier(verifier string) error {
	m.codeVerifier = verifier
	return m.saveCodeErr
}

// CodeVerifier returns the stored PKCE code verifier
func (m *mockOAuthClientProvider) CodeVerifier() (string, error) {
	return m.codeVerifier, nil
}

// RedirectURL returns the client redirect URL registered with the authorization server
func (m *mockOAuthClientProvider) RedirectURL() string {
	return m.redirectURL
}

// ClientMetadata returns the OAuth client metadata used for dynamic registration
func (m *mockOAuthClientProvider) ClientMetadata() auth.OAuthClientMetadata {
	return m.clientMeta
}

// RedirectToAuthorization performs a redirect to the authorization URL in real implementations
func (m *mockOAuthClientProvider) RedirectToAuthorization(authURL *url.URL) error {
	return m.redirectErr
}

// InvalidateCredentials invalidates cached credentials according to the provided scope
func (m *mockOAuthClientProvider) InvalidateCredentials(scope string) error {
	return m.invalidateErr
}

// SaveClientInformation persists full client information returned by dynamic registration
func (m *mockOAuthClientProvider) SaveClientInformation(info auth.OAuthClientInformationFull) error {
	m.clientInfo = &auth.OAuthClientInformation{
		ClientID:     info.ClientID,
		ClientSecret: info.ClientSecret,
	}
	return nil
}

// AddClientAuthentication attaches client authentication to token requests
func (m *mockOAuthClientProvider) AddClientAuthentication(headers http.Header, params url.Values, serverUrl string) error {
	return nil
}

// State returns a CSRF protection state value for authorization requests
func (m *mockOAuthClientProvider) State() (string, error) {
	return m.stateValue, m.stateErr
}

// ValidateResourceURL validates or adjusts the default resource URL using optional metadata
func (m *mockOAuthClientProvider) ValidateResourceURL(defaultResource *url.URL, metadata *auth.OAuthProtectedResourceMetadata) (*url.URL, error) {
	return defaultResource, nil
}

func TestSelectClientAuthMethod(t *testing.T) {
	tests := []struct {
		name             string
		clientInfo       auth.OAuthClientInformation
		supportedMethods []string
		expected         ClientAuthMethod
	}{
		{
			name: "basic auth preferred with secret",
			clientInfo: auth.OAuthClientInformation{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			supportedMethods: []string{"client_secret_basic", "client_secret_post"},
			expected:         ClientAuthMethodBasic,
		},
		{
			name: "post auth when basic not supported",
			clientInfo: auth.OAuthClientInformation{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			supportedMethods: []string{"client_secret_post"},
			expected:         ClientAuthMethodPost,
		},
		{
			name: "none auth for public client",
			clientInfo: auth.OAuthClientInformation{
				ClientID: "test-client",
			},
			supportedMethods: []string{"none"},
			expected:         ClientAuthMethodNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectClientAuthMethod(tt.clientInfo, tt.supportedMethods)
			if result != tt.expected {
				t.Errorf("selectClientAuthMethod() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestApplyClientAuthentication(t *testing.T) {
	clientInfo := auth.OAuthClientInformation{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}

	t.Run("basic auth", func(t *testing.T) {
		headers := http.Header{}
		params := url.Values{}

		err := applyClientAuthentication(ClientAuthMethodBasic, clientInfo, headers, params)
		if err != nil {
			t.Fatalf("applyClientAuthentication() error = %v", err)
		}

		authHeader := headers.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Basic ") {
			t.Errorf("Expected Basic authorization header, got %s", authHeader)
		}
	})

	t.Run("post auth", func(t *testing.T) {
		headers := http.Header{}
		params := url.Values{}

		err := applyClientAuthentication(ClientAuthMethodPost, clientInfo, headers, params)
		if err != nil {
			t.Fatalf("applyClientAuthentication() error = %v", err)
		}

		if params.Get("client_id") != "test-client" {
			t.Errorf("Expected client_id parameter, got %s", params.Get("client_id"))
		}
		if params.Get("client_secret") != "test-secret" {
			t.Errorf("Expected client_secret parameter, got %s", params.Get("client_secret"))
		}
	})
}

func TestParseErrorResponse(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expectError bool
		expectCode  string
	}{
		{
			name:        "valid oauth error from bytes",
			input:       []byte(`{"error":"invalid_client","error_description":"Client authentication failed"}`),
			expectError: false,
			expectCode:  "invalid_client",
		},
		{
			name:        "missing error field",
			input:       `{"error_description":"Missing error field"}`,
			expectError: true,
		},
		{
			name:        "invalid json",
			input:       `invalid json`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oauthErr, err := parseErrorResponse(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("parseErrorResponse() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseErrorResponse() unexpected error = %v", err)
			}

			if oauthErr.ErrorCode != tt.expectCode {
				t.Errorf("parseErrorResponse() error code = %v, want %v", oauthErr.ErrorCode, tt.expectCode)
			}
		})
	}
}

func TestBuildDiscoveryUrls(t *testing.T) {
	tests := []struct {
		name          string
		serverURL     string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "root path server",
			serverURL:     "https://auth.example.com",
			expectedCount: 2,
		},
		{
			name:          "server with path",
			serverURL:     "https://auth.example.com/tenant1",
			expectedCount: 4,
		},
		{
			name:        "invalid URL",
			serverURL:   "://invalid-url",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls, err := buildDiscoveryUrls(tt.serverURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("buildDiscoveryUrls() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("buildDiscoveryUrls() unexpected error = %v", err)
			}

			if len(urls) != tt.expectedCount {
				t.Errorf("buildDiscoveryUrls() returned %d URLs, want %d", len(urls), tt.expectedCount)
			}
		})
	}
}

func TestStartAuthorization(t *testing.T) {
	metadata := &auth.OAuthMetadata{
		Issuer:                        "https://auth.example.com",
		AuthorizationEndpoint:         "https://auth.example.com/authorize",
		TokenEndpoint:                 "https://auth.example.com/token",
		ResponseTypesSupported:        []string{"code"},
		CodeChallengeMethodsSupported: []string{"S256"},
	}

	options := StartAuthorizationOptions{
		Metadata: metadata,
		ClientInformation: auth.OAuthClientInformation{
			ClientID: "test-client",
		},
		RedirectURL: "https://client.example.com/callback",
	}

	result, err := StartAuthorization("https://auth.example.com", options)
	if err != nil {
		t.Fatalf("startAuthorization() error = %v", err)
	}

	if result.AuthorizationURL == nil {
		t.Error("startAuthorization() AuthorizationURL is nil")
	}

	if result.CodeVerifier == "" {
		t.Error("startAuthorization() CodeVerifier is empty")
	}

	// Check URL parameters
	params := result.AuthorizationURL.Query()
	if params.Get("response_type") != "code" {
		t.Errorf("Expected response_type=code, got %s", params.Get("response_type"))
	}
	if params.Get("client_id") != "test-client" {
		t.Errorf("Expected client_id=test-client, got %s", params.Get("client_id"))
	}
}

func TestDiscoverAuthorizationServerMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadata := auth.OAuthMetadata{
			Issuer:                 "https://auth.example.com",
			AuthorizationEndpoint:  "https://auth.example.com/authorize",
			TokenEndpoint:          "https://auth.example.com/token",
			ResponseTypesSupported: []string{"code"},
			GrantTypesSupported:    []string{"authorization_code", "refresh_token"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metadata)
	}))
	defer server.Close()

	result, err := DiscoverAuthorizationServerMetadata(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatalf("DiscoverAuthorizationServerMetadata() error = %v", err)
	}

	if result.GetIssuer() != "https://auth.example.com" {
		t.Errorf("Expected issuer https://auth.example.com, got %s", result.GetIssuer())
	}
}

func TestAuthWithExistingTokens(t *testing.T) {
	provider := &mockOAuthClientProvider{
		clientInfo: &auth.OAuthClientInformation{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		},
		tokens: &auth.OAuthTokens{
			AccessToken:  "valid-access-token",
			RefreshToken: stringPtr("valid-refresh-token"),
		},
		redirectURL: "https://client.example.com/callback",
		clientMeta: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://client.example.com/callback"},
		},
	}

	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/oauth-authorization-server"):
			metadata := &auth.OAuthMetadata{
				Issuer:                        serverURL, // Use server URL as issuer
				AuthorizationEndpoint:         serverURL + "/authorize",
				TokenEndpoint:                 serverURL + "/token",
				ResponseTypesSupported:        []string{"code"},
				GrantTypesSupported:           []string{"authorization_code", "refresh_token"},
				CodeChallengeMethodsSupported: []string{"S256"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(metadata)
		case r.URL.Path == "/token":
			// Simulate successful token refresh
			tokens := auth.OAuthTokens{
				AccessToken:  "new-access-token",
				RefreshToken: stringPtr("new-refresh-token"),
				TokenType:    "Bearer",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tokens)
		case strings.HasSuffix(r.URL.Path, "/.well-known/oauth-protected-resource"):
			// Return 404 for resource metadata (optional)
			w.WriteHeader(404)
		default:
			t.Logf("Unexpected request to: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	serverURL = server.URL

	options := auth.AuthOptions{
		ServerUrl: server.URL,
		FetchFn: func(url string, req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		},
	}

	result, err := Auth(provider, options)
	if err != nil {
		t.Fatalf("Auth() error = %v", err)
	}

	if *result != AuthResultAuthorized {
		t.Errorf("Expected AuthResultAuthorized, got %v", *result)
	}

	// Verify tokens were updated
	if provider.tokens.AccessToken != "new-access-token" {
		t.Errorf("Expected access token to be updated to 'new-access-token', got %s", provider.tokens.AccessToken)
	}
}

func TestRegisterClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		clientInfo := auth.OAuthClientInformationFull{
			OAuthClientInformation: auth.OAuthClientInformation{
				ClientID:     "generated-client-id",
				ClientSecret: "generated-client-secret",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clientInfo)
	}))
	defer server.Close()

	registrationEndpoint := server.URL + "/register"
	metadata := &auth.OAuthMetadata{
		RegistrationEndpoint: &registrationEndpoint,
	}

	options := RegisterClientOptions{
		Metadata: metadata,
		ClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://client.example.com/callback"},
		},
		FetchFn: func(url string, req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		},
	}

	result, err := RegisterClient(context.Background(), server.URL, options)
	if err != nil {
		t.Fatalf("RegisterClient() error = %v", err)
	}

	if result.ClientID != "generated-client-id" {
		t.Errorf("Expected client_id 'generated-client-id', got %s", result.ClientID)
	}
}

func TestAuthWithoutClientInfo(t *testing.T) {
	provider := &mockOAuthClientProvider{
		clientInfo:  nil, // No existing client info
		redirectURL: "https://client.example.com/callback",
		clientMeta: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://client.example.com/callback"},
		},
	}

	// Capture the server URL using a variable
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/oauth-authorization-server"):
			metadata := &auth.OAuthMetadata{
				Issuer:                        serverURL,
				AuthorizationEndpoint:         serverURL + "/authorize",
				TokenEndpoint:                 serverURL + "/token",
				RegistrationEndpoint:          stringPtr(serverURL + "/register"),
				ResponseTypesSupported:        []string{"code"},
				GrantTypesSupported:           []string{"authorization_code", "refresh_token"},
				CodeChallengeMethodsSupported: []string{"S256"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(metadata)
		case r.URL.Path == "/register":
			clientInfo := auth.OAuthClientInformationFull{
				OAuthClientInformation: auth.OAuthClientInformation{
					ClientID:     "registered-client-id",
					ClientSecret: "registered-client-secret",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(clientInfo)
		case strings.HasSuffix(r.URL.Path, "/.well-known/oauth-protected-resource"):
			// Return 404 for resource metadata (optional)
			w.WriteHeader(404)
		default:
			t.Logf("Unexpected request to: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	serverURL = server.URL

	options := auth.AuthOptions{
		ServerUrl: server.URL,
		FetchFn: func(url string, req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		},
	}

	result, err := Auth(provider, options)
	if err != nil {
		t.Fatalf("Auth() error = %v", err)
	}

	if *result != AuthResultRedirect {
		t.Errorf("Expected AuthResultRedirect, got %v", *result)
	}

	// Verify client information was saved after registration
	if provider.clientInfo == nil {
		t.Error("Expected client information to be saved after registration")
	}
	if provider.clientInfo.ClientID != "registered-client-id" {
		t.Errorf("Expected registered client ID, got %s", provider.clientInfo.ClientID)
	}
}

func TestAuthWithoutRefreshToken(t *testing.T) {
	provider := &mockOAuthClientProvider{
		clientInfo: &auth.OAuthClientInformation{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		},
		tokens: &auth.OAuthTokens{
			AccessToken: "valid-access-token",
			// No refresh token
		},
		redirectURL: "https://client.example.com/callback",
		clientMeta: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://client.example.com/callback"},
		},
	}

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/oauth-authorization-server"):
			metadata := &auth.OAuthMetadata{
				Issuer:                        serverURL,
				AuthorizationEndpoint:         serverURL + "/authorize",
				TokenEndpoint:                 serverURL + "/token",
				ResponseTypesSupported:        []string{"code"},
				GrantTypesSupported:           []string{"authorization_code", "refresh_token"},
				CodeChallengeMethodsSupported: []string{"S256"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(metadata)
		case strings.HasSuffix(r.URL.Path, "/.well-known/oauth-protected-resource"):
			w.WriteHeader(404)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	serverURL = server.URL

	options := auth.AuthOptions{
		ServerUrl: server.URL,
		FetchFn: func(url string, req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		},
	}

	result, err := Auth(provider, options)
	if err != nil {
		t.Fatalf("Auth() error = %v", err)
	}

	if *result != AuthResultRedirect {
		t.Errorf("Expected AuthResultRedirect, got %v", *result)
	}
}

func BenchmarkSelectClientAuthMethod(b *testing.B) {
	clientInfo := auth.OAuthClientInformation{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}
	supportedMethods := []string{"client_secret_basic", "client_secret_post"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selectClientAuthMethod(clientInfo, supportedMethods)
	}
}
