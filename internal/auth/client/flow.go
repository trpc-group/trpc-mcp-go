// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/pkce"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// AuthResult describes the outcome of an OAuth flow
type AuthResult string

const (
	AuthResultAuthorized AuthResult = "AUTHORIZED"
	AuthResultRedirect   AuthResult = "REDIRECT"
)

// ClientAuthMethod lists supported client authentication methods for token endpoint
type ClientAuthMethod string

const (
	ClientAuthMethodBasic ClientAuthMethod = "client_secret_basic"
	ClientAuthMethodPost  ClientAuthMethod = "client_secret_post"
	ClientAuthMethodNone  ClientAuthMethod = "none"
)

// metadataDiscoveryOptions carries optional knobs for metadata discovery behavior
type metadataDiscoveryOptions struct {
	ProtocolVersion   *string
	MetadataUrl       *string
	MetadataServerUrl *string
}

// RegisterClientOptions configures dynamic client registration
type RegisterClientOptions struct {
	Metadata       auth.AuthorizationServerMetadata
	ClientMetadata auth.OAuthClientMetadata
	FetchFn        auth.FetchFunc
}

// discoveryUrlType distinguishes between OAuth and OIDC discovery endpoints
type discoveryUrlType string

const (
	discoveryTypeOAuth discoveryUrlType = "oauth"
	discoveryTypeOIDC  discoveryUrlType = "oidc"
)

// discoveryUrl pairs a URL with its discovery type
type discoveryUrl struct {
	URL  *url.URL
	Type discoveryUrlType
}

// StartAuthorizationOptions configures OAuth authorization startup
type StartAuthorizationOptions struct {
	// Metadata contains authorization server configuration (optional)
	Metadata auth.AuthorizationServerMetadata

	// ClientInformation holds the OAuth client credentials
	ClientInformation auth.OAuthClientInformation

	// RedirectURL specifies where to redirect after authorization
	RedirectURL string

	// Scope defines the requested access permissions (optional)
	Scope *string

	// State provides CSRF protection (optional)
	State *string

	// Resource specifies the target resource URL (optional)
	Resource *url.URL
}

// StartAuthorizationResult holds authorization startup results
type StartAuthorizationResult struct {
	// AuthorizationURL is where the user should be redirected for authorization
	AuthorizationURL *url.URL

	// CodeVerifier must be stored securely for the token exchange step
	CodeVerifier string
}

// ExchangeAuthorizationOptions configures exchanging an authorization code for tokens
type ExchangeAuthorizationOptions struct {
	Metadata                auth.AuthorizationServerMetadata            // server config (optional)
	ClientInformation       *auth.OAuthClientInformation                // client credentials
	AuthorizationCode       string                                      // auth code from server
	CodeVerifier            string                                      // PKCE verifier
	RedirectURI             string                                      // must match auth request
	Resource                *url.URL                                    // target resource (optional)
	AddClientAuthentication func(http.Header, url.Values, string) error // custom auth (optional)
	FetchFn                 auth.FetchFunc                              // custom HTTP client (optional)
}

// RefreshAuthorizationOptions configures exchanging a refresh token for new tokens
type RefreshAuthorizationOptions struct {
	Metadata                auth.AuthorizationServerMetadata            // server config (optional)
	ClientInformation       *auth.OAuthClientInformation                // client credentials
	RefreshToken            string                                      // refresh token
	Resource                *url.URL                                    // target resource (optional)
	AddClientAuthentication func(http.Header, url.Values, string) error // custom auth (optional)
	FetchFn                 auth.FetchFunc                              // custom HTTP client (optional)
}

// UnauthorizedError represents an authorization failure that should be surfaced to callers
type UnauthorizedError struct {
	message string
}

// NewUnauthorizedError constructs an UnauthorizedError with a friendly message
func NewUnauthorizedError(message string) *UnauthorizedError {
	if message == "" {
		message = "Unauthorized"
	}
	return &UnauthorizedError{message: message}
}

// Error returns the error message for UnauthorizedError
func (e *UnauthorizedError) Error() string {
	return e.message
}

// selectClientAuthMethod chooses a client auth method based on server support and client secrets
func selectClientAuthMethod(
	clientInformation auth.OAuthClientInformation,
	supportedMethods []string,
) ClientAuthMethod {
	var hasClientSecret bool
	hasClientSecret = clientInformation.ClientSecret != ""
	if len(supportedMethods) == 0 {
		if hasClientSecret {
			return ClientAuthMethodPost
		} else {
			return ClientAuthMethodNone
		}
	}

	if hasClientSecret && slices.Contains(supportedMethods, string(ClientAuthMethodBasic)) {
		return ClientAuthMethodBasic
	}
	if hasClientSecret && slices.Contains(supportedMethods, string(ClientAuthMethodPost)) {
		return ClientAuthMethodPost
	}
	if slices.Contains(supportedMethods, string(ClientAuthMethodNone)) {
		return ClientAuthMethodNone
	}
	if hasClientSecret {
		return ClientAuthMethodPost
	} else {
		return ClientAuthMethodNone
	}
}

// applyClientAuthentication applies the chosen client auth to headers and or form parameters
func applyClientAuthentication(
	method ClientAuthMethod,
	clientInformation auth.OAuthClientInformation,
	headers http.Header,
	params url.Values,
) error {
	clientID := clientInformation.ClientID
	clientSecret := clientInformation.ClientSecret

	switch method {
	case ClientAuthMethodBasic:
		return applyBasicAuth(clientID, clientSecret, headers)
	case ClientAuthMethodPost:
		applyPostAuth(clientID, clientSecret, params)
		return nil
	case ClientAuthMethodNone:
		applyPublicAuth(clientID, params)
		return nil
	default:
		return fmt.Errorf("unsupported client authentication method: %s", method)
	}
}

// applyBasicAuth adds HTTP Basic Authorization using client id and secret
func applyBasicAuth(clientID, clientSecret string, headers http.Header) error {
	if clientSecret == "" {
		return fmt.Errorf("client_secret_basic authentication requires a client_secret")
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	headers.Set("Authorization", "Basic "+credentials)
	return nil
}

// applyPostAuth writes client credentials into the token form payload
func applyPostAuth(clientID, clientSecret string, params url.Values) {
	params.Set("client_id", clientID)
	if clientSecret != "" {
		params.Set("client_secret", clientSecret)
	}
}

// applyPublicAuth writes public client id into the token form payload
func applyPublicAuth(clientID string, params url.Values) {
	params.Set("client_id", clientID)
}

// parseErrorResponse converts an OAuth style JSON error payload into an OAuthError
func parseErrorResponse(input interface{}) (*errors.OAuthError, error) {
	var responseBody []byte
	var err error

	// Handle different input types
	switch v := input.(type) {
	case []byte:
		responseBody = v
	case string:
		responseBody = []byte(v)
	case *http.Response:
		defer v.Body.Close()
		responseBody, err = io.ReadAll(v.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported input type: %T", input)
	}

	// Try to parse as OAuth error response
	var oauthErrorResp errors.OAuthErrorResponse
	if err := json.Unmarshal(responseBody, &oauthErrorResp); err != nil {
		// Not a valid OAuth error response format
		return nil, fmt.Errorf("failed to parse OAuth error response: %w", err)
	}

	// Validate required error field
	if oauthErrorResp.Error == "" {
		return nil, fmt.Errorf("invalid OAuth error response: missing error field")
	}

	// Map error code to OAuthErrorCode using the mapping table
	errorCode, exists := errors.OAuthErrorMapping[oauthErrorResp.Error]
	if !exists {
		// Unknown error code, default to server error
		errorCode = errors.ErrServerError
	}

	// Create OAuthError with parsed information
	oauthError := errors.NewOAuthError(errorCode, oauthErrorResp.ErrorDescription, oauthErrorResp.ErrorURI)
	return &oauthError, nil
}

// Auth performs high level authentication with retries and credential invalidation on specific errors
func Auth(provider OAuthClientProvider, options auth.AuthOptions) (*AuthResult, error) {
	result, err := authInternal(provider, options)
	if err != nil {
		if stderrors.Is(err, errors.ErrInvalidClient) || stderrors.Is(err, errors.ErrUnauthorizedClient) {
			if invalidator, ok := provider.(OAuthCredentialInvalidator); ok {
				if invalidateErr := invalidator.InvalidateCredentials("all"); invalidateErr != nil {
					return nil, invalidateErr
				}
			}
			return authInternal(provider, options)
		} else if stderrors.Is(err, errors.ErrInvalidGrant) {
			if invalidator, ok := provider.(OAuthCredentialInvalidator); ok {
				if invalidateErr := invalidator.InvalidateCredentials("tokens"); invalidateErr != nil {
					return nil, invalidateErr
				}
			}
			return authInternal(provider, options)
		}
		return nil, err
	}
	return result, err
}

// authInternal runs the core auth logic including discovery registration refresh and redirect setup
func authInternal(provider OAuthClientProvider, options auth.AuthOptions) (*AuthResult, error) {
	var resourceMetadata *auth.OAuthProtectedResourceMetadata
	var authorizationServerUrl string
	metadata, err := DiscoverOAuthProtectedResourceMetadata(options.ServerUrl, &auth.DiscoveryOptions{
		ResourceMetadataUrl: options.ResourceMetadataUrl,
	}, options.FetchFn)
	if err == nil {
		resourceMetadata = metadata
		if len(resourceMetadata.AuthorizationServers) > 0 {
			authorizationServerUrl = resourceMetadata.AuthorizationServers[0]
		}
	}
	if authorizationServerUrl == "" {
		authorizationServerUrl = options.ServerUrl
	}

	resource, err := selectResourceURL(options.ServerUrl, provider, resourceMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to select resource URL: %w", err)
	}

	serverMetadata, err := DiscoverAuthorizationServerMetadata(context.Background(), authorizationServerUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to discover authorization server metadata: %w", err)
	}
	clientInformation := provider.ClientInformation()

	if clientInformation == nil {
		if options.AuthorizationCode != nil {
			return nil, stderrors.New("existing OAuth client information is required when exchanging an authorization code")
		}

		if _, ok := provider.(OAuthClientInfoProvider); !ok {
			return nil, stderrors.New("OAuth client information must be saveable for dynamic registration")
		}

		fullInformation, err := RegisterClient(context.Background(), authorizationServerUrl, RegisterClientOptions{
			Metadata:       serverMetadata,
			ClientMetadata: provider.ClientMetadata(),
			FetchFn:        options.FetchFn,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to register client: %w", err)
		}

		if clientInfoProvider, ok := provider.(OAuthClientInfoProvider); ok {
			if err := clientInfoProvider.SaveClientInformation(*fullInformation); err != nil {
				return nil, fmt.Errorf("failed to save client information: %w", err)
			}
		}
		clientInformation = &auth.OAuthClientInformation{
			ClientID:     fullInformation.ClientID,
			ClientSecret: fullInformation.ClientSecret,
		}
	}
	tokens, err := provider.Tokens()
	if err != nil {
		return nil, fmt.Errorf("failed to get tokens: %w", err)
	}

	if tokens != nil && tokens.RefreshToken != nil && *tokens.RefreshToken != "" {
		var addClientAuth func(http.Header, url.Values, string) error
		if authProvider, ok := provider.(OAuthClientAuthProvider); ok {
			addClientAuth = authProvider.AddClientAuthentication
		}

		newTokens, err := RefreshAuthorization(authorizationServerUrl, RefreshAuthorizationOptions{
			Metadata:                serverMetadata,
			ClientInformation:       clientInformation,
			RefreshToken:            *tokens.RefreshToken,
			Resource:                resource,
			AddClientAuthentication: addClientAuth,
			FetchFn:                 options.FetchFn,
		})
		if err != nil {
			var oauthErr *errors.OAuthError
			if !stderrors.As(err, &oauthErr) {
				// Network/non-OAuth errors, continue auth flow
			} else {
				return nil, err
			}
		} else {
			if err := provider.SaveTokens(*newTokens); err != nil {
				return nil, fmt.Errorf("failed to save refreshed tokens: %w", err)
			}
			result := AuthResultAuthorized
			return &result, nil
		}
	}
	var state *string
	if stateProvider, ok := provider.(OAuthStateProvider); ok {
		stateValue, err := stateProvider.State()
		if err != nil {
			return nil, fmt.Errorf("failed to get state: %w", err)
		}
		state = &stateValue
	}
	scope := options.Scope
	if scope == nil {
		clientMetadata := provider.ClientMetadata()
		if clientMetadata.Scope != nil {
			scope = clientMetadata.Scope
		}
	}

	authorizationResult, err := StartAuthorization(authorizationServerUrl, StartAuthorizationOptions{
		Metadata:          serverMetadata,
		ClientInformation: *clientInformation,
		State:             state,
		RedirectURL:       provider.RedirectURL(),
		Scope:             scope,
		Resource:          resource,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start authorization: %w", err)
	}

	if err := provider.SaveCodeVerifier(authorizationResult.CodeVerifier); err != nil {
		return nil, fmt.Errorf("failed to save code verifier: %w", err)
	}

	if err := provider.RedirectToAuthorization(authorizationResult.AuthorizationURL); err != nil {
		return nil, fmt.Errorf("failed to redirect to authorization: %w", err)
	}

	result := AuthResultRedirect
	return &result, nil
}

// selectResourceURL determines the resource parameter to use validating against protected resource metadata
func selectResourceURL(serverUrl string, provider OAuthClientProvider, resourceMetadata *auth.OAuthProtectedResourceMetadata) (*url.URL, error) {
	defaultResource, err := auth.ResourceURLFromServerURL(serverUrl)
	if err != nil {
		return nil, err
	}

	// Use custom validator if available
	if validator, ok := provider.(OAuthResourceValidator); ok {
		return validator.ValidateResourceURL(defaultResource, resourceMetadata)
	}

	// Include resource param only when metadata exists
	if resourceMetadata == nil {
		return nil, nil // No resource param needed
	}

	// Check metadata resource compatibility
	allowed, err := auth.CheckResourceAllowed(auth.CheckResourceAllowedParams{
		RequestedResource:  defaultResource,
		ConfiguredResource: resourceMetadata.Resource,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to validate resource: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("protected resource %s does not match expected %s",
			resourceMetadata.Resource, defaultResource.String())
	}

	// Use metadata resource - server expects this
	return url.Parse(resourceMetadata.Resource)
}

// DiscoverOAuthProtectedResourceMetadata loads OAuth Protected Resource metadata with path aware fallback
func DiscoverOAuthProtectedResourceMetadata(serverUrl string, opts *auth.DiscoveryOptions, fetchFn auth.FetchFunc) (*auth.OAuthProtectedResourceMetadata, error) {
	if fetchFn == nil {
		fetchFn = func(urlStr string, req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}
	}

	response, err := discoverMetadataWithFallback(
		serverUrl,
		"oauth-protected-resource",
		fetchFn,
		&metadataDiscoveryOptions{
			ProtocolVersion: getProtocolVersion(opts),
			MetadataUrl:     getResourceMetadataUrl(opts),
		},
	)
	if err != nil {
		return nil, err
	}

	if response == nil || response.StatusCode == 404 {
		return nil, fmt.Errorf("Resource server does not implement OAuth 2.0 Protected Resource Metadata.")
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d trying to load well-known OAuth protected resource metadata.", response.StatusCode)
	}

	defer response.Body.Close()
	var metadata auth.OAuthProtectedResourceMetadata
	if err := json.NewDecoder(response.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata response: %w", err)
	}

	return &metadata, nil
}

// discoverMetadataWithFallback tries path aware discovery then falls back to root well known when applicable
func discoverMetadataWithFallback(
	serverUrl interface{},
	wellKnownType string, // "oauth-authorization-server" or "oauth-protected-resource"
	fetchFn auth.FetchFunc,
	opts *metadataDiscoveryOptions,
) (*http.Response, error) {
	issuer, err := parseURL(serverUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	protocolVersion := "2025-03-26" // LATEST_PROTOCOL_VERSION
	if opts != nil && opts.ProtocolVersion != nil {
		protocolVersion = *opts.ProtocolVersion
	}

	var targetUrl *url.URL
	if opts != nil && opts.MetadataUrl != nil {
		targetUrl, err = url.Parse(*opts.MetadataUrl)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata URL: %w", err)
		}
	} else {
		// Try path-aware discovery
		wellKnownPath := buildWellKnownPath(wellKnownType, issuer.Path)
		baseUrl := issuer
		if opts != nil && opts.MetadataServerUrl != nil {
			baseUrl, err = url.Parse(*opts.MetadataServerUrl)
			if err != nil {
				return nil, fmt.Errorf("invalid metadata server URL: %w", err)
			}
		}
		targetUrl, _ = url.Parse(wellKnownPath)
		targetUrl = baseUrl.ResolveReference(targetUrl)
		targetUrl.RawQuery = issuer.RawQuery
	}

	response, err := tryMetadataDiscovery(targetUrl, protocolVersion, fetchFn)
	if err != nil {
		return nil, err
	}

	// If path-aware discovery fails with 404 and we're not at root, try fallback to root discovery
	if (opts == nil || opts.MetadataUrl == nil) && shouldAttemptFallback(response, issuer.Path) {
		rootUrl, _ := url.Parse(fmt.Sprintf("/.well-known/%s", wellKnownType))
		rootUrl = issuer.ResolveReference(rootUrl)
		response, err = tryMetadataDiscovery(rootUrl, protocolVersion, fetchFn)
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}

// tryMetadataDiscovery issues a discovery request with protocol headers and returns the HTTP response
func tryMetadataDiscovery(targetUrl *url.URL, protocolVersion string, fetchFn auth.FetchFunc) (*http.Response, error) {
	req, err := http.NewRequest("GET", targetUrl.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("MCP-Protocol-Version", protocolVersion)

	return fetchWithCorsRetry(targetUrl, req.Header, fetchFn)
}

// fetchWithCorsRetry helper function to handle CORS retry logic
func fetchWithCorsRetry(targetUrl *url.URL, headers http.Header, fetchFn auth.FetchFunc) (*http.Response, error) {
	req, err := http.NewRequest("GET", targetUrl.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Copy headers
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	response, err := fetchFn(targetUrl.String(), req)
	if err != nil {
		// If it's a network error (similar to TypeError in TypeScript), try retry without headers
		if isNetworkError(err) && len(headers) > 0 {
			return fetchWithCorsRetry(targetUrl, http.Header{}, fetchFn)
		}
		return nil, err
	}

	return response, nil
}
func shouldAttemptFallback(response *http.Response, pathname string) bool {
	return response == nil || (response.StatusCode == 404 && pathname != "/")
}

// buildWellKnownPath builds well-known path for authentication-related metadata discovery
func buildWellKnownPath(wellKnownPrefix, pathname string) string {
	// Remove trailing slash from pathname to avoid double slashes
	if strings.HasSuffix(pathname, "/") {
		pathname = strings.TrimSuffix(pathname, "/")
	}

	return fmt.Sprintf("/.well-known/%s%s", wellKnownPrefix, pathname)
}

// parseURL accepts string or *url.URL and returns a parsed URL
func parseURL(u interface{}) (*url.URL, error) {
	switch v := u.(type) {
	case string:
		return url.Parse(v)
	case *url.URL:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported URL type")
	}
}

// getProtocolVersion resolves the requested protocol version from options if provided
func getProtocolVersion(opts *auth.DiscoveryOptions) *string {
	if opts != nil && opts.ProtocolVersion != nil {
		return opts.ProtocolVersion
	}
	return nil
}

// getResourceMetadataUrl resolves an explicit resource metadata URL from options if provided
func getResourceMetadataUrl(opts *auth.DiscoveryOptions) *string {
	if opts != nil && opts.ResourceMetadataUrl != nil {
		return opts.ResourceMetadataUrl
	}
	return nil
}

// isNetworkError determines if it's a network error (simulating TypeError check in TypeScript)
func isNetworkError(err error) bool {
	// In Go, network errors usually contain these keywords
	errorStr := strings.ToLower(err.Error())
	return strings.Contains(errorStr, "network") ||
		strings.Contains(errorStr, "connection") ||
		strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "refused")
}

// buildDiscoveryUrls generates candidate OAuth and OIDC discovery URLs for a given authorization server URL
func buildDiscoveryUrls(authorizationServerURL string) ([]discoveryUrl, error) {
	parsedURL, err := url.Parse(authorizationServerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization server URL: %w", err)
	}

	hasPath := parsedURL.Path != "/" && parsedURL.Path != ""
	var urlsToTry []discoveryUrl

	if !hasPath {
		// Root path: https://example.com/.well-known/oauth-authorization-server
		oauthURL, _ := url.Parse(parsedURL.Scheme + "://" + parsedURL.Host + "/.well-known/oauth-authorization-server")
		urlsToTry = append(urlsToTry, discoveryUrl{URL: oauthURL, Type: discoveryTypeOAuth})

		// OIDC: https://example.com/.well-known/openid-configuration
		oidcURL, _ := url.Parse(parsedURL.Scheme + "://" + parsedURL.Host + "/.well-known/openid-configuration")
		urlsToTry = append(urlsToTry, discoveryUrl{URL: oidcURL, Type: discoveryTypeOIDC})

		return urlsToTry, nil
	}

	// Strip trailing slash from pathname to avoid double slashes
	pathname := parsedURL.Path
	if strings.HasSuffix(pathname, "/") {
		pathname = pathname[:len(pathname)-1]
	}

	// 1. OAuth metadata at the given URL
	// Insert well-known before the path: https://example.com/.well-known/oauth-authorization-server/tenant1
	oauthWithPath, _ := url.Parse(parsedURL.Scheme + "://" + parsedURL.Host + "/.well-known/oauth-authorization-server" + pathname)
	urlsToTry = append(urlsToTry, discoveryUrl{URL: oauthWithPath, Type: discoveryTypeOAuth})

	// Root path: https://example.com/.well-known/oauth-authorization-server
	oauthRoot, _ := url.Parse(parsedURL.Scheme + "://" + parsedURL.Host + "/.well-known/oauth-authorization-server")
	urlsToTry = append(urlsToTry, discoveryUrl{URL: oauthRoot, Type: discoveryTypeOAuth})

	// 3. OIDC metadata endpoints
	// RFC 8414 style: Insert /.well-known/openid-configuration before the path
	oidcWithPath, _ := url.Parse(parsedURL.Scheme + "://" + parsedURL.Host + "/.well-known/openid-configuration" + pathname)
	urlsToTry = append(urlsToTry, discoveryUrl{URL: oidcWithPath, Type: discoveryTypeOIDC})

	// OIDC Discovery 1.0 style: Append /.well-known/openid-configuration after the path
	oidcAfterPath, _ := url.Parse(parsedURL.Scheme + "://" + parsedURL.Host + pathname + "/.well-known/openid-configuration")
	urlsToTry = append(urlsToTry, discoveryUrl{URL: oidcAfterPath, Type: discoveryTypeOIDC})

	return urlsToTry, nil
}

// DiscoveryURL represents a metadata endpoint
type DiscoveryURL struct {
	URL  *url.URL
	Type string // "oauth" or "oidc"
}

// DiscoverAuthorizationServerMetadata discovers OAuth or OIDC metadata and verifies minimal capabilities
func DiscoverAuthorizationServerMetadata(ctx context.Context, authServerUrl string, options *auth.DiscoveryOptions) (auth.AuthorizationServerMetadata, error) {
	// Build discovery URLs
	discoveryUrls, err := buildDiscoveryUrls(authServerUrl)
	if err != nil {
		return nil, err
	}

	// Create default fetch function
	fetchFunc := func(urlStr string, req *http.Request) (*http.Response, error) {
		return http.DefaultClient.Do(req)
	}

	// Try each discovery URL
	for _, discoveryUrl := range discoveryUrls {
		// Create headers
		headers := http.Header{
			"Accept": []string{"application/json"},
		}

		// Try to fetch metadata
		resp, err := fetchWithCorsRetry(discoveryUrl.URL, headers, fetchFunc)
		if err != nil {
			if isNetworkError(err) {
				continue
			}
			return nil, fmt.Errorf("failed to fetch metadata from %s: %w", discoveryUrl.URL.String(), err)
		}
		defer resp.Body.Close()

		// Check status code
		if resp.StatusCode == 404 {
			continue
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, discoveryUrl.URL.String())
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		// Parse metadata based on type
		if discoveryUrl.Type == "oidc" {
			// Try to parse as OpenID Connect metadata
			var metadata auth.OpenIdProviderDiscoveryMetadata
			if err := json.Unmarshal(body, &metadata); err != nil {
				continue // Try next URL
			}

			// Validate required fields for OIDC
			if metadata.Issuer == "" || metadata.AuthorizationEndpoint == "" || metadata.TokenEndpoint == "" {
				continue // Try next URL
			}

			// Check if S256 PKCE is supported for OIDC
			supportsS256 := false
			for _, method := range metadata.CodeChallengeMethodsSupported {
				if method == "S256" {
					supportsS256 = true
					break
				}
			}
			if !supportsS256 {
				return nil, fmt.Errorf("OIDC provider does not support S256 PKCE")
			}

			return &metadata, nil
		} else {
			// Try to parse as OAuth 2.0 metadata
			var metadata auth.OAuthMetadata
			if err := json.Unmarshal(body, &metadata); err != nil {
				continue // Try next URL
			}

			// Validate required fields for OAuth 2.0
			if metadata.Issuer == "" || metadata.AuthorizationEndpoint == "" || metadata.TokenEndpoint == "" {
				continue // Try next URL
			}

			return &metadata, nil
		}
	}

	return nil, fmt.Errorf("failed to discover authorization server metadata from %s", authServerUrl)
}

// RegisterClient performs dynamic client registration and returns full client information
func RegisterClient(
	ctx context.Context,
	authorizationServerUrl string,
	options RegisterClientOptions,
) (*auth.OAuthClientInformationFull, error) {
	var registrationUrl *url.URL
	var err error

	// Determine registration endpoint URL
	if options.Metadata != nil {
		// Check if dynamic client registration is supported
		var registrationEndpoint string

		// Get registration endpoint based on metadata type
		switch metadata := options.Metadata.(type) {
		case *auth.OAuthMetadata:
			if metadata.RegistrationEndpoint == nil {
				return nil, fmt.Errorf("incompatible auth server: does not support dynamic client registration")
			}
			registrationEndpoint = *metadata.RegistrationEndpoint
		case *auth.OpenIdProviderMetadata:
			if metadata.RegistrationEndpoint == nil {
				return nil, fmt.Errorf("incompatible auth server: does not support dynamic client registration")
			}
			registrationEndpoint = *metadata.RegistrationEndpoint
		case *auth.OpenIdProviderDiscoveryMetadata:
			if metadata.RegistrationEndpoint == nil {
				return nil, fmt.Errorf("incompatible auth server: does not support dynamic client registration")
			}
			registrationEndpoint = *metadata.RegistrationEndpoint
		default:
			return nil, fmt.Errorf("unsupported metadata type")
		}

		registrationUrl, err = url.Parse(registrationEndpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid registration endpoint URL: %w", err)
		}
	} else {
		// Use default registration path
		baseUrl, err := url.Parse(authorizationServerUrl)
		if err != nil {
			return nil, fmt.Errorf("invalid authorization server URL: %w", err)
		}
		registrationUrl, err = baseUrl.Parse("/register")
		if err != nil {
			return nil, fmt.Errorf("failed to construct registration URL: %w", err)
		}
	}

	// Serialize client metadata
	requestBody, err := json.Marshal(options.ClientMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal client metadata: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", registrationUrl.String(), strings.NewReader(string(requestBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Select fetch function
	fetchFn := options.FetchFn
	if fetchFn == nil {
		fetchFn = func(url string, req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}
	}

	// Send request
	resp, err := fetchFn(registrationUrl.String(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if !isSuccessStatusCode(resp.StatusCode) {
		// Try to parse OAuth error response
		var oauthError errors.OAuthError
		if err := json.Unmarshal(responseBody, &oauthError); err == nil {
			return nil, &oauthError
		}
		return nil, fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse success response
	var clientInfo auth.OAuthClientInformationFull
	if err := json.Unmarshal(responseBody, &clientInfo); err != nil {
		return nil, fmt.Errorf("failed to parse registration response: %w", err)
	}

	return &clientInfo, nil
}

// isSuccessStatusCode checks if HTTP status code indicates success
func isSuccessStatusCode(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// StartAuthorization starts OAuth 2.0 authorization flow
func StartAuthorization(
	authorizationServerUrl string,
	options StartAuthorizationOptions,
) (*StartAuthorizationResult, error) {
	const responseType = "code"
	const codeChallengeMethod = "S256"

	var authorizationURL *url.URL
	var err error

	// Determine authorization endpoint URL
	if options.Metadata != nil {
		authorizationURL, err = url.Parse(options.Metadata.GetAuthorizationEndpoint())
		if err != nil {
			return nil, fmt.Errorf("invalid authorization endpoint: %w", err)
		}

		// Verify server supports "code" response type
		responseTypesSupported := options.Metadata.GetResponseTypesSupported()
		supportsCode := false
		for _, rt := range responseTypesSupported {
			if rt == responseType {
				supportsCode = true
				break
			}
		}
		if !supportsCode {
			return nil, fmt.Errorf(
				"incompatible auth server: does not support response type %s",
				responseType,
			)
		}

		// Verify server supports S256 PKCE method
		var codeChallengeMethodsSupported []string

		// Check different types of metadata
		switch metadata := options.Metadata.(type) {
		case *auth.OAuthMetadata:
			codeChallengeMethodsSupported = metadata.CodeChallengeMethodsSupported
		case *auth.OpenIdProviderDiscoveryMetadata:
			codeChallengeMethodsSupported = metadata.CodeChallengeMethodsSupported
		}

		if len(codeChallengeMethodsSupported) > 0 {
			supportsS256 := false
			for _, method := range codeChallengeMethodsSupported {
				if method == codeChallengeMethod {
					supportsS256 = true
					break
				}
			}
			if !supportsS256 {
				return nil, fmt.Errorf(
					"incompatible auth server: does not support code challenge method %s",
					codeChallengeMethod,
				)
			}
		}
	} else {
		// If no metadata, use default /authorize endpoint
		baseURL, err := url.Parse(authorizationServerUrl)
		if err != nil {
			return nil, fmt.Errorf("invalid authorization server URL: %w", err)
		}
		authorizationURL = baseURL.ResolveReference(&url.URL{Path: "/authorize"})
	}

	// Generate PKCE challenge
	challenge, err := pkce.GeneratePKCEChallenge()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE challenge: %w", err)
	}

	// Build query parameters
	params := url.Values{}
	params.Set("response_type", responseType)
	params.Set("client_id", options.ClientInformation.ClientID)
	params.Set("redirect_uri", options.RedirectURL)
	params.Set("code_challenge", challenge.CodeChallenge)
	params.Set("code_challenge_method", codeChallengeMethod)

	// Add optional parameters
	if options.Scope != nil && *options.Scope != "" {
		params.Set("scope", *options.Scope)

		// OpenID Connect requirement: if scope contains 'offline_access', need to add consent prompt
		if strings.Contains(*options.Scope, "offline_access") {
			params.Set("prompt", "consent")
		}
	}

	if options.State != nil && *options.State != "" {
		params.Set("state", *options.State)
	}

	if options.Resource != nil {
		params.Set("resource", options.Resource.String())
	}

	// Set query parameters
	authorizationURL.RawQuery = params.Encode()

	return &StartAuthorizationResult{
		AuthorizationURL: authorizationURL,
		CodeVerifier:     challenge.CodeVerifier,
	}, nil
}

// ExchangeAuthorization exchanges an authorization code for tokens applying appropriate client authentication
func ExchangeAuthorization(
	authorizationServerUrl string,
	options ExchangeAuthorizationOptions,
) (*auth.OAuthTokens, error) {
	const grantType = "authorization_code"

	// Determine token endpoint URL
	var tokenURL *url.URL
	var err error

	if options.Metadata != nil {
		tokenEndpoint := options.Metadata.GetTokenEndpoint()
		if tokenEndpoint == "" {
			return nil, fmt.Errorf("token endpoint not found in metadata")
		}
		tokenURL, err = url.Parse(tokenEndpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid token endpoint: %w", err)
		}

		// Verify server supports authorization_code grant type
		grantTypesSupported := options.Metadata.GetGrantTypesSupported()
		if len(grantTypesSupported) > 0 {
			supportsAuthCode := false
			for _, gt := range grantTypesSupported {
				if gt == grantType {
					supportsAuthCode = true
					break
				}
			}
			if !supportsAuthCode {
				return nil, fmt.Errorf(
					"incompatible auth server: does not support grant type %s",
					grantType,
				)
			}
		}
	} else {
		// Use default /token endpoint
		baseURL, err := url.Parse(authorizationServerUrl)
		if err != nil {
			return nil, fmt.Errorf("invalid authorization server URL: %w", err)
		}
		tokenURL = baseURL.ResolveReference(&url.URL{Path: "/token"})
	}

	// Prepare request headers and parameters
	headers := http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
	}
	params := url.Values{
		"grant_type":    []string{grantType},
		"code":          []string{options.AuthorizationCode},
		"redirect_uri":  []string{options.RedirectURI},
		"code_verifier": []string{options.CodeVerifier},
	}

	// Apply client authentication
	if options.AddClientAuthentication != nil {
		if err := options.AddClientAuthentication(headers, params, authorizationServerUrl); err != nil {
			return nil, fmt.Errorf("failed to apply client authentication: %w", err)
		}
	} else {
		// Determine and apply client authentication method
		var supportedMethods []string
		if options.Metadata != nil {
			supportedMethods = options.Metadata.GetTokenEndpointAuthMethodsSupported()

		}
		authMethod := selectClientAuthMethod(*options.ClientInformation, supportedMethods)
		if err := applyClientAuthentication(authMethod, *options.ClientInformation, headers, params); err != nil {
			return nil, fmt.Errorf("failed to apply client authentication: %w", err)
		}
	}

	// Add resource parameter (if provided)
	if options.Resource != nil {
		params.Set("resource", options.Resource.String())
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", tokenURL.String(), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header = headers

	// Select fetch function
	fetchFn := options.FetchFn
	if fetchFn == nil {
		fetchFn = func(url string, req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}
	}

	// Send request
	resp, err := fetchFn(tokenURL.String(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if !isSuccessStatusCode(resp.StatusCode) {
		// Try to parse OAuth error response
		var oauthError errors.OAuthError
		if err := json.Unmarshal(responseBody, &oauthError); err == nil {
			return nil, &oauthError
		}
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse success response
	var tokens auth.OAuthTokens
	if err := json.Unmarshal(responseBody, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokens, nil
}

// RefreshAuthorization exchanges a refresh token for a new access token and propagates refresh token when absent
func RefreshAuthorization(
	authorizationServerUrl string,
	options RefreshAuthorizationOptions,
) (*auth.OAuthTokens, error) {
	const grantType = "refresh_token"

	// Determine token endpoint URL
	var tokenURL *url.URL
	var err error

	if options.Metadata != nil {
		tokenEndpoint := options.Metadata.GetTokenEndpoint()
		if tokenEndpoint == "" {
			return nil, fmt.Errorf("token endpoint not found in metadata")
		}
		tokenURL, err = url.Parse(tokenEndpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid token endpoint: %w", err)
		}

		// Verify server supports refresh_token grant type
		grantTypesSupported := options.Metadata.GetGrantTypesSupported()
		if len(grantTypesSupported) > 0 {
			supportsRefreshToken := false
			for _, gt := range grantTypesSupported {
				if gt == grantType {
					supportsRefreshToken = true
					break
				}
			}
			if !supportsRefreshToken {
				return nil, fmt.Errorf(
					"incompatible auth server: does not support grant type %s",
					grantType,
				)
			}
		}
	} else {
		// Use default /token endpoint
		baseURL, err := url.Parse(authorizationServerUrl)
		if err != nil {
			return nil, fmt.Errorf("invalid authorization server URL: %w", err)
		}
		tokenURL = baseURL.ResolveReference(&url.URL{Path: "/token"})
	}

	// Prepare request headers and parameters
	headers := http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
	}
	params := url.Values{
		"grant_type":    []string{grantType},
		"refresh_token": []string{options.RefreshToken},
	}

	// Apply client authentication
	if options.AddClientAuthentication != nil {
		if err := options.AddClientAuthentication(headers, params, authorizationServerUrl); err != nil {
			return nil, fmt.Errorf("failed to apply client authentication: %w", err)
		}
	} else {
		// Determine and apply client authentication method
		var supportedMethods []string
		if options.Metadata != nil {
			supportedMethods = options.Metadata.GetTokenEndpointAuthMethodsSupported()
		}
		authMethod := selectClientAuthMethod(*options.ClientInformation, supportedMethods)
		if err := applyClientAuthentication(authMethod, *options.ClientInformation, headers, params); err != nil {
			return nil, fmt.Errorf("failed to apply client authentication: %w", err)
		}
	}

	// Add resource parameter (if provided)
	if options.Resource != nil {
		params.Set("resource", options.Resource.String())
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", tokenURL.String(), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header = headers

	// Select fetch function
	fetchFn := options.FetchFn
	if fetchFn == nil {
		fetchFn = func(url string, req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}
	}

	// Send request
	resp, err := fetchFn(tokenURL.String(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to send refresh request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if !isSuccessStatusCode(resp.StatusCode) {
		// Try to parse OAuth error response
		var oauthError errors.OAuthError
		if err := json.Unmarshal(responseBody, &oauthError); err == nil {
			return nil, &oauthError
		}
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse success response
	var tokens auth.OAuthTokens
	if err := json.Unmarshal(responseBody, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// If response doesn't contain new refresh token, keep the original one
	if tokens.RefreshToken == nil || *tokens.RefreshToken == "" {
		tokens.RefreshToken = &options.RefreshToken
	}

	return &tokens, nil
}
