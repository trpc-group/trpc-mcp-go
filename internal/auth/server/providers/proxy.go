// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-playground/validator/v10"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// ProxyEndpoints defines the OAuth 2.0/2.1 server endpoints used by the proxy.
// It contains the URLs for various OAuth operations.
type ProxyEndpoints struct {
	// AuthorizationURL is the URL of the OAuth 2.0/2.1 authorization endpoint.
	// This is where users are redirected to authorize the client.
	// "https://auth.example.com/authorize"
	AuthorizationURL string `json:"authorizationUrl"`

	// TokenURL is the URL of the OAuth 2.0/2.1 token endpoint.
	// This is where the client exchanges an authorization code for an access token.
	// "https://auth.example.com/token"
	TokenURL string `json:"tokenUrl"`

	// RevocationURL is the optional URL of the OAuth 2.0 token revocation endpoint.
	// If provided, it's used to revoke access tokens or refresh tokens.
	// "https://auth.example.com/revoke"
	RevocationURL string `json:"revocationUrl,omitempty"`

	// RegistrationURL is the optional URL of the OAuth 2.0 dynamic client registration endpoint.
	// If provided, it allows clients to register with the authorization server dynamically. 。
	// "https://auth.example.com/register"
	RegistrationURL string `json:"registrationUrl,omitempty"`
}

// ProxyOptions defines configuration options for the proxy OAuth server
type ProxyOptions struct {
	// Endpoints presents Endpoint configuration for proxy OAuth operations
	Endpoints ProxyEndpoints

	// VerifyAccessToken verifies access tokens and return auth info
	VerifyAccessToken func(token string) (*server.AuthInfo, error)

	// GetClient fetches client information from the upstream server
	GetClient func(clientID string) (*auth.OAuthClientInformationFull, error)

	// Fetch customs fetch implementation used for all network requests, optional
	Fetch auth.FetchFunc
}

// ProxyOAuthServerProvider defines proxy OAuth server provider
type ProxyOAuthServerProvider struct {
	// endpoints defines proxy endpoint configuration
	endpoints ProxyEndpoints

	// verifyAccessToken verifies access tokens
	verifyAccessToken func(token string) (*server.AuthInfo, error)

	// getClient get client's information
	getClient func(clientID string) (*auth.OAuthClientInformationFull, error)

	// SkipLocalPkceValidation determines whether to skip local PKCE validation.
	// If true, the server will not perform PKCE validation locally and will pass the code_verifier to the upstream server.
	// NOTE: This should only be true if the upstream server is performing the actual PKCE validation.
	// 可选字段，默认false / Optional field, defaults to false
	SkipLocalPkceValidation bool `json:"skipLocalPkceValidation,omitempty"`

	// Custom fetch implementation, optional
	fetch auth.FetchFunc
}

// Authorize handles an OAuth authorization request by constructing the query
// parameters and redirecting the user agent to the configured authorization endpoint.
func (p *ProxyOAuthServerProvider) Authorize(client auth.OAuthClientInformationFull, params server.AuthorizationParams, res http.ResponseWriter, req *http.Request) error {
	// Validate the configured authorization endpoint URL
	targetURL, err := url.Parse(p.endpoints.AuthorizationURL)
	if err != nil {
		return fmt.Errorf("invalid authorization URL: %v", err)
	}

	// Build required OAuth query parameters
	query := url.Values{
		"client_id":             {client.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {params.RedirectURI},
		"code_challenge":        {params.CodeChallenge},
		"code_challenge_method": {"S256"},
	}

	// Add optional parameters when present
	if params.State != "" {
		query.Set("state", params.State)
	}
	if len(params.Scopes) > 0 {
		query.Set("scope", strings.Join(params.Scopes, " "))
	}
	if params.Resource != nil {
		query.Set("resource", params.Resource.String())
	}

	// Attach encoded query to the target URL
	targetURL.RawQuery = query.Encode()

	// Perform a 302 redirect to the upstream authorization endpoint
	http.Redirect(res, req, targetURL.String(), http.StatusFound)
	return nil
}

// VerifyAccessToken proxies token verification to the configured verifier function.
func (p *ProxyOAuthServerProvider) VerifyAccessToken(token string) (*server.AuthInfo, error) {
	// Delegate to injected verifier to allow custom verification strategies
	return p.verifyAccessToken(token)
}

// NewProxyOAuthServerProvider creates a new ProxyOAuthServerProvider using the provided options.
// By default SkipLocalPkceValidation is set to true to defer PKCE verification to the upstream server.
func NewProxyOAuthServerProvider(options ProxyOptions) *ProxyOAuthServerProvider {
	// Populate provider with endpoints, dependency functions, and optional fetch
	provider := &ProxyOAuthServerProvider{
		endpoints:               options.Endpoints,
		verifyAccessToken:       options.VerifyAccessToken,
		getClient:               options.GetClient,
		fetch:                   options.Fetch,
		SkipLocalPkceValidation: true,
	}
	// Return the ready to use provider
	return provider
}

// doFetch executes an HTTP request using the custom fetch function if provided,
// otherwise falls back to the default HTTP client.
func (p *ProxyOAuthServerProvider) doFetch(req *http.Request) (*http.Response, error) {
	// Prefer custom fetch to allow callers to add auth, retries, or instrumentation
	if p.fetch != nil {
		return p.fetch(req.URL.String(), req)
	}
	// Fallback to a vanilla http.Client
	client := &http.Client{}
	return client.Do(req)
}

// RevokeToken sends a token revocation request to the configured revocation endpoint.
// If the revocation endpoint is not configured, an error is returned.
func (p *ProxyOAuthServerProvider) RevokeToken(client auth.OAuthClientInformationFull, request auth.OAuthTokenRevocationRequest) error {
	// Ensure the revocation endpoint exists
	if p.endpoints.RevocationURL == "" {
		return fmt.Errorf("no revocation endpoint configured")
	}

	// Build form-encoded body with required parameters
	params := url.Values{
		"token":     {request.Token},
		"client_id": {client.ClientID},
	}

	// Include client_secret when available for confidential clients
	if client.ClientSecret != "" {
		params.Set("client_secret", client.ClientSecret)
	}

	// Optionally include token_type_hint to help the AS
	if request.TokenTypeHint != "" {
		params.Set("token_type_hint", request.TokenTypeHint)
	}

	// Create POST request with application/x-www-form-urlencoded payload
	req, err := http.NewRequest("POST", p.endpoints.RevocationURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("create request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request via custom fetch or default client
	resp, err := p.doFetch(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Expect 200 OK per RFC 7009
	if resp.StatusCode != http.StatusOK {
		return errors.NewOAuthError(errors.ErrServerError, fmt.Sprintf("Token revocation failed: %v", resp.StatusCode), "")
	}

	// No body parsing required for successful revocation
	return nil
}

// ClientsStore returns an OAuthClientsStore wired for lookup and optional dynamic client registration
// depending on whether a registration endpoint is configured.
func (p *ProxyOAuthServerProvider) ClientsStore() *server.OAuthClientsStore {
	var store *server.OAuthClientsStore

	// If registration URL is configured, enable dynamic client registration proxy
	if p.endpoints.RegistrationURL != "" {
		// Define registration function that forwards the registration request upstream
		registerClient := func(client auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error) {
			// Serialize client metadata to JSON request body
			body, err := json.Marshal(client)
			if err != nil {
				// TODO: add logging for serialization error
				return nil, fmt.Errorf("failed to marshal client: %v", err)
			}

			// Create HTTP POST to upstream registration endpoint
			req, err := http.NewRequest("POST", p.endpoints.RegistrationURL, bytes.NewReader(body))
			if err != nil {
				// TODO: add logging for request creation error
				return nil, fmt.Errorf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			// Execute registration request
			resp, err := p.doFetch(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			// Expect 200 OK with client registration response
			if resp.StatusCode != http.StatusOK {
				return nil, errors.NewOAuthError(errors.ErrServerError, fmt.Errorf("client registration failed: %v", resp.StatusCode).Error(), "")
			}

			// Decode response JSON into full client record
			var data auth.OAuthClientInformationFull
			if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
				// TODO: add logging for decode error
				return nil, fmt.Errorf("failed to decode response: %v", err)
			}

			// Return registered client info to caller
			return &data, nil
		}

		// Build a store that supports both lookup and registration
		store = server.NewOAuthClientStoreSupportDynamicRegistration(p.getClient, registerClient)
	} else {
		// Build a lookup-only store when registration is not supported
		store = server.NewOAuthClientStore(p.getClient)
	}

	return store
}

// ChallengeForAuthorizationCode returns the PKCE code_challenge for a previously initiated authorization.
// In a proxy setup this is not stored locally and we defer validation to the upstream server.
func (p *ProxyOAuthServerProvider) ChallengeForAuthorizationCode(client auth.OAuthClientInformationFull, authorizationCode string) (string, error) {
	// No local storage of code_challenge in proxy mode
	// Upstream AS validates code_verifier against its stored challenge
	return "", nil
}

// ExchangeAuthorizationCode exchanges an authorization code for tokens by forwarding
// the request to the upstream token endpoint and returning the parsed response.
func (p *ProxyOAuthServerProvider) ExchangeAuthorizationCode(client auth.OAuthClientInformationFull, authorizationCode string, codeVerifier *string, redirectUri *string, resource *url.URL) (*auth.OAuthTokens, error) {
	// Ensure a token endpoint is configured
	if p.endpoints.TokenURL == "" {
		return nil, fmt.Errorf("no token endpoint configured")
	}

	// Build form parameters required by the authorization_code grant
	params := url.Values{
		"grant_type": {"authorization_code"},
		"client_id":  {client.ClientID},
		"code":       {authorizationCode},
	}

	// Include client_secret for confidential clients
	if client.ClientSecret != "" {
		params.Set("client_secret", client.ClientSecret)
	}

	// Forward PKCE code_verifier when provided
	if codeVerifier != nil {
		params.Set("code_verifier", *codeVerifier)
	}

	// Include redirect_uri when provided to satisfy AS validation
	if redirectUri != nil {
		params.Set("redirect_uri", *redirectUri)
	}

	// Forward resource indicator when present
	if resource != nil {
		params.Set("resource", resource.String())
	}

	// Create POST request to token endpoint with form-encoded body
	req, err := http.NewRequest("POST", p.endpoints.TokenURL, bytes.NewReader([]byte(params.Encode())))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request via fetch helper
	resp, err := p.doFetch(req)
	if err != nil {
		return nil, errors.NewOAuthError(errors.ErrServerError, fmt.Sprintf("token exchange failed: %v", err), "")
	}
	defer resp.Body.Close()

	// Expect 200 OK for successful token response
	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewOAuthError(errors.ErrServerError, fmt.Sprintf("token exchange failed: %v", resp.StatusCode), "")
	}

	// Decode token response JSON into OAuthTokens
	var data auth.OAuthTokens
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// Return parsed tokens to caller
	return &data, nil
}

// ExchangeRefreshToken exchanges a refresh token for a new access token by calling
// the upstream token endpoint and validating the response payload.
func (p *ProxyOAuthServerProvider) ExchangeRefreshToken(
	client auth.OAuthClientInformationFull,
	refreshToken string,
	scopes []string,   // Optional empty slice if not provided
	resource *url.URL, // Optional nil if not provided
) (*auth.OAuthTokens, error) {
	// Assemble form parameters for the refresh_token grant
	params := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {client.ClientID},
		"refresh_token": {refreshToken},
	}

	// Include client_secret for confidential clients
	if client.ClientSecret != "" {
		params.Set("client_secret", client.ClientSecret)
	}

	// Optionally narrow or expand scopes as requested
	if len(scopes) > 0 {
		params.Set("scope", strings.Join(scopes, " "))
	}

	// Forward resource indicator when present
	if resource != nil {
		params.Set("resource", resource.String())
	}

	// Create a context bound POST request to the token endpoint
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, p.endpoints.TokenURL, bytes.NewBufferString(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Choose custom fetch when provided, otherwise use default client
	fetch := p.fetch
	if fetch == nil {
		fetch = func(url string, req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}
	}

	// Execute the HTTP request
	resp, err := fetch(p.endpoints.TokenURL, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Expect a successful status code from the token endpoint
	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewOAuthError(errors.ErrServerError, fmt.Sprintf("token refresh failed: %v", resp.StatusCode), "")
	}

	// Decode the token response body
	var data auth.OAuthTokens
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// Validate the decoded structure using validator/v10
	if err := validateOAuthTokens(&data); err != nil {
		return nil, fmt.Errorf("validation failed: %v", err)
	}

	// Return validated tokens
	return &data, nil
}

// validateOAuthTokens validates the OAuthTokens struct using github.com/go-playground/validator.
// This can be extended with custom field validators if needed.
func validateOAuthTokens(tokens *auth.OAuthTokens) error {
	// Initialize a new validator instance and run struct validation
	validate := validator.New()
	if err := validate.Struct(tokens); err != nil {
		return fmt.Errorf("validation errors: %v", err)
	}
	return nil
}

// GetSkipLocalPkceValidation exposes whether local PKCE verification should be skipped.
// Token handlers can use this to decide if code_verifier must be validated locally or forwarded.
func (p *ProxyOAuthServerProvider) GetSkipLocalPkceValidation() bool {
	// Return the current setting as provided during construction
	return p.SkipLocalPkceValidation
}
