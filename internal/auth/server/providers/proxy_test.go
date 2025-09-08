// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package providers

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	oauthErrors "trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// shared test fixtures for proxy provider tests
var (
	// validClient models a typical confidential client usable across tests
	validClient = auth.OAuthClientInformationFull{
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		},
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://example.com/callback"},
		},
	}

	// baseOptions is the baseline ProxyOptions used by tests, with hooks injected in TestMain
	baseOptions = ProxyOptions{
		Endpoints: ProxyEndpoints{
			AuthorizationURL: "https://auth.example.com/authorize",
			TokenURL:         "https://auth.example.com/token",
			RevocationURL:    "https://auth.example.com/revoke",
			RegistrationURL:  "https://auth.example.com/register",
		},
		VerifyAccessToken: nil,
		GetClient:         nil,
		Fetch:             nil,
	}

	// mock token payload values reused across assertions
	RefreshToken      = "new-refresh-token"
	ExpiresIn         = int64(3600)
	mockTokenResponse = auth.OAuthTokens{
		AccessToken:  "new-access-token",
		TokenType:    "Bearer",
		ExpiresIn:    &ExpiresIn,
		RefreshToken: &RefreshToken,
	}

	// mockFetch is an overridable HTTP transport used to intercept outbound requests in tests
	mockFetch func(url string, req *http.Request) (*http.Response, error)
)

// TestMain wires per-suite hooks for VerifyAccessToken, GetClient and fetch before running tests
func TestMain(m *testing.M) {
	// set up VerifyAccessToken behavior
	baseOptions.VerifyAccessToken = func(token string) (*server.AuthInfo, error) {
		if token == "valid-token" {
			ExpiresAt := time.Now().Unix() + 3600
			return &server.AuthInfo{
				Token:     token,
				ClientID:  "test-client",
				Scopes:    []string{"read", "write"},
				ExpiresAt: &ExpiresAt,
			}, nil
		}
		if token == "token-with-insufficient-scope" {
			return nil, oauthErrors.NewOAuthError(oauthErrors.ErrInsufficientScope, "Required scopes: read, write", "")
		}
		if token == "valid-token-unexpected" {
			return nil, errors.New("unexpected error")
		}
		return nil, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "Invalid token", "")
	}

	// set up client lookup behavior
	baseOptions.GetClient = func(clientID string) (*auth.OAuthClientInformationFull, error) {
		if clientID == "test-client" {
			return &validClient, nil
		}
		return nil, nil
	}

	// run tests
	code := m.Run()

	// cleanup
	mockFetch = nil
	os.Exit(code)
}

func TestProxyOAuthServerProvider(t *testing.T) {
	provider := NewProxyOAuthServerProvider(baseOptions)

	// Mock codeVerifier and redirectURI
	codeVerifier := "test-verifier"
	redirectURI := "https://example.com/callback"

	t.Run("Authorization", func(t *testing.T) {
		t.Run("Redirects to authorization endpoint with correct parameters", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			resource, _ := url.Parse("https://api.example.com/resource")
			err := provider.Authorize(validClient, server.AuthorizationParams{
				RedirectURI:   "https://example.com/callback",
				CodeChallenge: "test-challenge",
				State:         "test-state",
				Scopes:        []string{"read", "write"},
				Resource:      resource,
			}, rr, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the status code and Location header
			if rr.Code != http.StatusFound {
				t.Errorf("expected status code %d, got %d", http.StatusFound, rr.Code)
			}

			gotURL := rr.Header().Get("Location")
			// Debug output
			t.Logf("got redirect URL: %s", gotURL)
			expectedURL, _ := url.Parse("https://auth.example.com/authorize")
			q := expectedURL.Query()
			q.Set("client_id", "test-client")
			q.Set("response_type", "code")
			q.Set("redirect_uri", "https://example.com/callback")
			q.Set("code_challenge", "test-challenge")
			q.Set("code_challenge_method", "S256")
			q.Set("state", "test-state")
			q.Set("scope", "read write")
			q.Set("resource", "https://api.example.com/resource")
			expectedURL.RawQuery = q.Encode()

			if gotURL != expectedURL.String() {
				t.Errorf("expected redirect URL %s, got %s", expectedURL.String(), gotURL)
			}
		})
	})

	t.Run("Token Exchange", func(t *testing.T) {
		t.Run("Exchanges authorization code for tokens", func(t *testing.T) {
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				t.Logf("request body: %s", string(body)) // 调试请求体
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access-token","token_type":"Bearer","expires_in":3600,"refresh_token":"new-refresh-token"}`)),
				}, nil
			}
			provider.fetch = mockFetch

			tokens, err := provider.ExchangeAuthorizationCode(validClient, "test-code", &codeVerifier, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Debug output
			t.Logf("tokens: %+v", tokens)
			if tokens.AccessToken != mockTokenResponse.AccessToken {
				t.Errorf("expected access_token %s, got %s", mockTokenResponse.AccessToken, tokens.AccessToken)
			}
			if tokens.TokenType != mockTokenResponse.TokenType {
				t.Errorf("expected token_type %s, got %s", mockTokenResponse.TokenType, tokens.TokenType)
			}
			if tokens.ExpiresIn == nil || *tokens.ExpiresIn != *mockTokenResponse.ExpiresIn {
				t.Errorf("expected expires_in %d, got %v", *mockTokenResponse.ExpiresIn, tokens.ExpiresIn)
			}
			if tokens.RefreshToken == nil || *tokens.RefreshToken != *mockTokenResponse.RefreshToken {
				t.Errorf("expected refresh_token %s, got %v", *mockTokenResponse.RefreshToken, tokens.RefreshToken)
			}
		})

		t.Run("Includes redirect_uri in token request when provided", func(t *testing.T) {
			var calledBody string
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				calledBody = string(body)
				// Debug output
				t.Logf("request body: %s", calledBody)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access-token","token_type":"Bearer","expires_in":3600,"refresh_token":"new-refresh-token"}`)),
				}, nil
			}
			provider.fetch = mockFetch

			_, err := provider.ExchangeAuthorizationCode(validClient, "test-code", &codeVerifier, &redirectURI, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(calledBody, "redirect_uri=https%3A%2F%2Fexample.com%2Fcallback") {
				t.Errorf("expected redirect_uri in body, got %s", calledBody)
			}
		})

		t.Run("Handles token exchange failure", func(t *testing.T) {
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			provider.fetch = mockFetch

			_, err := provider.ExchangeAuthorizationCode(validClient, "test-code", &codeVerifier, nil, nil)
			// Debug output
			t.Logf("error: %v", err)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var oauthErr oauthErrors.OAuthError
			if !errors.As(err, &oauthErr) {
				t.Errorf("expected error to be of type oauthErrors.OAuthError, got %T", err)
			}
			if oauthErr.ErrorCode != oauthErrors.ErrServerError.Error() {
				t.Errorf("expected OAuthError with code %s, got %s", oauthErrors.ErrServerError.Error(), oauthErr.ErrorCode)
			}
		})

		t.Run("Includes resource parameter in authorization code exchange", func(t *testing.T) {
			var calledBody string
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				if url != "https://auth.example.com/token" {
					t.Errorf("expected URL %s, got %s", "https://auth.example.com/token", url)
				}
				if req.Method != http.MethodPost {
					t.Errorf("expected method POST, got %s", req.Method)
				}
				if contentType := req.Header.Get("Content-Type"); contentType != "application/x-www-form-urlencoded" {
					t.Errorf("expected Content-Type application/x-www-form-urlencoded, got %s", contentType)
				}
				body, _ := io.ReadAll(req.Body)
				calledBody = string(body)
				t.Logf("request body: %s", calledBody)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access-token","token_type":"Bearer","expires_in":3600,"refresh_token":"new-refresh-token"}`)),
				}, nil
			}
			provider.fetch = mockFetch

			resource, _ := url.Parse("https://api.example.com/resource")
			codeVerifier := "test-verifier"
			redirectURI := "https://example.com/callback"
			tokens, err := provider.ExchangeAuthorizationCode(validClient, "test-code", &codeVerifier, &redirectURI, resource)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(calledBody, "resource=https%3A%2F%2Fapi.example.com%2Fresource") {
				t.Errorf("expected resource parameter in body, got %s", calledBody)
			}
			if tokens.AccessToken != mockTokenResponse.AccessToken {
				t.Errorf("expected access_token %s, got %s", mockTokenResponse.AccessToken, tokens.AccessToken)
			}
			if tokens.TokenType != mockTokenResponse.TokenType {
				t.Errorf("expected token_type %s, got %s", mockTokenResponse.TokenType, tokens.TokenType)
			}
			if tokens.ExpiresIn == nil || *tokens.ExpiresIn != *mockTokenResponse.ExpiresIn {
				t.Errorf("expected expires_in %d, got %v", *mockTokenResponse.ExpiresIn, tokens.ExpiresIn)
			}
			if tokens.RefreshToken == nil || *tokens.RefreshToken != *mockTokenResponse.RefreshToken {
				t.Errorf("expected refresh_token %s, got %v", *mockTokenResponse.RefreshToken, tokens.RefreshToken)
			}
		})

		t.Run("Handles authorization code exchange without resource parameter", func(t *testing.T) {
			var calledBody string
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				calledBody = string(body)
				t.Logf("request body: %s", calledBody) // 调试输出
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access-token","token_type":"Bearer","expires_in":3600,"refresh_token":"new-refresh-token"}`)),
				}, nil
			}
			provider.fetch = mockFetch

			tokens, err := provider.ExchangeAuthorizationCode(validClient, "test-code", &codeVerifier, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Contains(calledBody, "resource=") {
				t.Errorf("expected no resource parameter in body, got %s", calledBody)
			}
			if tokens.AccessToken != mockTokenResponse.AccessToken {
				t.Errorf("expected access_token %s, got %s", mockTokenResponse.AccessToken, tokens.AccessToken)
			}
			if tokens.TokenType != mockTokenResponse.TokenType {
				t.Errorf("expected token_type %s, got %s", mockTokenResponse.TokenType, tokens.TokenType)
			}
			if tokens.ExpiresIn == nil || *tokens.ExpiresIn != *mockTokenResponse.ExpiresIn {
				t.Errorf("expected expires_in %d, got %v", *mockTokenResponse.ExpiresIn, tokens.ExpiresIn)
			}
			if tokens.RefreshToken == nil || *tokens.RefreshToken != *mockTokenResponse.RefreshToken {
				t.Errorf("expected refresh_token %s, got %v", *mockTokenResponse.RefreshToken, tokens.RefreshToken)
			}
		})

		t.Run("Includes resource parameter in refresh token exchange", func(t *testing.T) {
			var calledBody string
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				calledBody = string(body)
				// Debug output
				t.Logf("request body: %s", calledBody)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access-token","token_type":"Bearer","expires_in":3600,"refresh_token":"new-refresh-token"}`)),
				}, nil
			}
			provider.fetch = mockFetch

			resource, _ := url.Parse("https://api.example.com/resource")
			tokens, err := provider.ExchangeRefreshToken(validClient, "test-refresh-token", []string{"read", "write"}, resource)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(calledBody, "resource=https%3A%2F%2Fapi.example.com%2Fresource") {
				t.Errorf("expected resource in body, got %s", calledBody)
			}
			if tokens.AccessToken != mockTokenResponse.AccessToken {
				t.Errorf("expected access_token %s, got %s", mockTokenResponse.AccessToken, tokens.AccessToken)
			}
			if tokens.TokenType != mockTokenResponse.TokenType {
				t.Errorf("expected token_type %s, got %s", mockTokenResponse.TokenType, tokens.TokenType)
			}
			if tokens.ExpiresIn == nil || *tokens.ExpiresIn != *mockTokenResponse.ExpiresIn {
				t.Errorf("expected expires_in %d, got %v", *mockTokenResponse.ExpiresIn, tokens.ExpiresIn)
			}
			if tokens.RefreshToken == nil || *tokens.RefreshToken != *mockTokenResponse.RefreshToken {
				t.Errorf("expected refresh_token %s, got %v", *mockTokenResponse.RefreshToken, tokens.RefreshToken)
			}
		})
	})

	t.Run("Client Registration", func(t *testing.T) {
		t.Run("Registers new client", func(t *testing.T) {
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				// Debug output
				t.Logf("register request body: %s", string(body))
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"client_id":"new-client","client_secret":"new-secret","redirect_uris":["https://new-client.com/callback"]}`)),
				}, nil
			}
			provider.fetch = mockFetch

			newClient := auth.OAuthClientInformationFull{
				OAuthClientInformation: auth.OAuthClientInformation{
					ClientID: "new-client",
				},
				OAuthClientMetadata: auth.OAuthClientMetadata{
					RedirectURIs: []string{"https://new-client.com/callback"},
				},
			}
			result, err := provider.ClientsStore().RegisterClient(newClient)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ClientID != newClient.ClientID {
				t.Errorf("expected client_id %s, got %s", newClient.ClientID, result.ClientID)
			}
		})

		t.Run("Handles registration failure", func(t *testing.T) {
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			provider.fetch = mockFetch

			newClient := auth.OAuthClientInformationFull{
				OAuthClientInformation: auth.OAuthClientInformation{
					ClientID: "new-client",
				},
				OAuthClientMetadata: auth.OAuthClientMetadata{
					RedirectURIs: []string{"https://new-client.com/callback"},
				},
			}
			_, err := provider.ClientsStore().RegisterClient(newClient)
			t.Logf("error: %v", err) // Debug output
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var serverErr oauthErrors.OAuthError
			if !errors.As(err, &serverErr) {
				t.Errorf("expected error to be of type oauthErrors.OAuthError, got %T", err)
			}
			if serverErr.ErrorCode != oauthErrors.ErrServerError.Error() {
				t.Errorf("expected OAuthError with code %s, got %s", oauthErrors.ErrServerError.Error(), serverErr.ErrorCode)
			}
		})
	})

	t.Run("Token Revocation", func(t *testing.T) {
		t.Run("Revokes token", func(t *testing.T) {
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				t.Logf("revoke request body: %s", string(body)) // 调试输出
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			provider.fetch = mockFetch

			err := provider.RevokeToken(validClient, auth.OAuthTokenRevocationRequest{
				Token:         "token-to-revoke",
				TokenTypeHint: "access_token",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("Handles revocation failure", func(t *testing.T) {
			mockFetch = func(url string, req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			provider.fetch = mockFetch

			err := provider.RevokeToken(validClient, auth.OAuthTokenRevocationRequest{
				Token: "invalid-token",
			})
			t.Logf("error: %v", err) // 调试输出
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var serverErr oauthErrors.OAuthError
			if !errors.As(err, &serverErr) {
				t.Errorf("expected error to be of type oauthErrors.OAuthError, got %T", err)
			}
			if serverErr.ErrorCode != oauthErrors.ErrServerError.Error() {
				t.Errorf("expected OAuthError with code %s, got %s", oauthErrors.ErrServerError.Error(), serverErr.ErrorCode)
			}
		})
	})

	t.Run("Token Verification", func(t *testing.T) {
		t.Run("Verifies valid token", func(t *testing.T) {
			var calledToken string
			options := baseOptions
			options.VerifyAccessToken = func(token string) (*server.AuthInfo, error) {
				calledToken = token
				return baseOptions.VerifyAccessToken(token)
			}
			provider := NewProxyOAuthServerProvider(options)

			authInfo, err := provider.VerifyAccessToken("valid-token")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if calledToken != "valid-token" {
				t.Errorf("expected VerifyAccessToken called with 'valid-token', got %q", calledToken)
			}
			t.Logf("authInfo: %+v", authInfo) // 调试输出
			if authInfo.ClientID != "test-client" {
				t.Errorf("expected clientId test-client, got %s", authInfo.ClientID)
			}
		})

		t.Run("Passes through InvalidTokenError", func(t *testing.T) {
			var calledToken string
			options := baseOptions
			options.VerifyAccessToken = func(token string) (*server.AuthInfo, error) {
				calledToken = token // 记录调用参数
				if token == "valid-token" {
					ExpiresAt := time.Now().Unix() + 3600
					return &server.AuthInfo{
						Token:     token,
						ClientID:  "test-client",
						Scopes:    []string{"read", "write"},
						ExpiresAt: &ExpiresAt,
					}, nil
				}
				return nil, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "Invalid token", "")
			}
			provider := NewProxyOAuthServerProvider(options)

			_, err := provider.VerifyAccessToken("invalid-token")
			t.Logf("error: %v", err)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if calledToken != "invalid-token" {
				t.Errorf("expected VerifyAccessToken called with 'invalid-token', got %q", calledToken)
			}
			var invalidTokenErr oauthErrors.OAuthError
			if !errors.As(err, &invalidTokenErr) {
				t.Errorf("expected error to be of type oauthErrors.OAuthError, got %T", err)
			}
			if invalidTokenErr.ErrorCode != oauthErrors.ErrInvalidToken.Error() {
				t.Errorf("expected OAuthError with code %s, got %s", oauthErrors.ErrInvalidToken.Error(), invalidTokenErr.ErrorCode)
			}
		})

		t.Run("Passes through InsufficientScopeError", func(t *testing.T) {
			var calledToken string
			options := baseOptions // 复制全局配置
			options.VerifyAccessToken = func(token string) (*server.AuthInfo, error) {
				calledToken = token
				return nil, oauthErrors.NewOAuthError(oauthErrors.ErrInsufficientScope, "Required scopes: read, write", "")
			}
			provider := NewProxyOAuthServerProvider(options)

			_, err := provider.VerifyAccessToken("token-with-insufficient-scope")
			t.Logf("error: %v", err)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var insufficientScopeErr oauthErrors.OAuthError
			if !errors.As(err, &insufficientScopeErr) {
				t.Errorf("expected error to be of type oauthErrors.OAuthError, got %T", err)
			}
			if insufficientScopeErr.ErrorCode != "insufficient_scope" {
				t.Errorf("expected OAuthError with code %s, got %s", "insufficient_scope", insufficientScopeErr.ErrorCode)
			}
			if insufficientScopeErr.Message != "Required scopes: read, write" {
				t.Errorf("expected OAuthError with description %s, got %s", "Required scopes: read, write", insufficientScopeErr.Message)
			}
			if calledToken != "token-with-insufficient-scope" {
				t.Errorf("expected VerifyAccessToken called with %s, got %s", "token-with-insufficient-scope", calledToken)
			}
		})

		t.Run("Passes through unexpected errors", func(t *testing.T) {
			var calledToken string
			// Copy global configuration
			options := baseOptions
			options.VerifyAccessToken = func(token string) (*server.AuthInfo, error) {
				calledToken = token
				return baseOptions.VerifyAccessToken(token)
			}
			provider := NewProxyOAuthServerProvider(options)

			// Invoke VerifyAccessToken
			_, err := provider.VerifyAccessToken("valid-token-unexpected")
			t.Logf("error: %v", err) // 调试输出
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var oauthErr oauthErrors.OAuthError
			if errors.As(err, &oauthErr) {
				t.Errorf("expected error to be non-OAuth error, got oauthErrors.OAuthError")
			}
			if err.Error() != "unexpected error" {
				t.Errorf("expected error message %s, got %s", "unexpected error", err.Error())
			}
			if calledToken != "valid-token-unexpected" {
				t.Errorf("expected VerifyAccessToken called with %s, got %s", "valid-token-unexpected", calledToken)
			}
		})
	})
}
