// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package router

import (
	"fmt"
	"net/http"
	"net/url"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/handler"
)

// AuthRouterOptions holds configuration options for the MCP authentication router.
// It configures how OAuth 2.1 endpoints (/authorize, /token, /revoke, /register) are exposed.
type AuthRouterOptions struct {
	// Provider is the OAuth server implementation.
	// It manages client registration, authorization codes, tokens, and verification.
	Provider server.OAuthServerProvider

	// IssuerUrl is the OAuth issuer identifier (RFC 8414).
	// Typically something like "https://auth.example.com".
	IssuerUrl *url.URL

	// BaseUrl is the base URL of this service, used to construct endpoint URLs
	// such as /authorize, /token, etc.
	BaseUrl *url.URL

	// ServiceDocumentationUrl points to human-readable documentation about the service,
	// usually an API docs page.
	ServiceDocumentationUrl *url.URL

	// ScopesSupported lists all scopes supported by this authorization server,
	// for example: ["read", "write"].
	ScopesSupported []string

	// ResourceName is an optional logical name for the protected resource/API.
	ResourceName *string

	// AuthorizationOptions configures the /authorize endpoint (validation, rate limiting, etc.).
	AuthorizationOptions *handler.AuthorizationHandlerOptions

	// ClientRegistrationOptions configures the /register endpoint for dynamic client registration (RFC 7591).
	ClientRegistrationOptions *handler.ClientRegistrationHandlerOptions

	// RevocationOptions configures the /revoke endpoint for token revocation (RFC 7009).
	RevocationOptions *handler.RevocationHandlerOptions

	// TokenOptions configures the /token endpoint for issuing tokens (supports auth code flow, PKCE, etc.).
	TokenOptions *handler.TokenHandlerOptions
}

// AuthMetadataOptions holds configuration options for the MCP authentication metadata endpoints.
// It controls what is published via OAuth 2.1 Authorization Server Metadata (RFC 8414).
type AuthMetadataOptions struct {
	// OAuthMetadata contains the full OAuth 2.1 Authorization Server Metadata,
	// including authorization_endpoint, token_endpoint, scopes_supported, etc.
	OAuthMetadata auth.OAuthMetadata

	// ResourceServerUrl points to the protected resource server,
	// used by clients to discover where to send API requests.
	ResourceServerUrl *url.URL

	// ServiceDocumentationUrl points to human-readable documentation about the service.
	ServiceDocumentationUrl *url.URL

	// ScopesSupported lists the scopes supported by this resource server.
	ScopesSupported []string

	// ResourceName is an optional logical name for the resource server.
	ResourceName *string
}

// checkIssuerUrl validates the issuer URL according to RFC 8414.
func checkIssuerUrl(issuer *url.URL) error {
	// Technically RFC 8414 does not permit a localhost HTTPS exemption,
	// but this will be necessary for ease of testing
	if issuer.Scheme != "https" && issuer.Hostname() != "localhost" && issuer.Hostname() != "127.0.0.1" {
		return fmt.Errorf("issuer URL must be HTTPS")
	}
	if issuer.Fragment != "" {
		return fmt.Errorf("issuer URL must not have a fragment: %s", issuer.String())
	}
	if issuer.RawQuery != "" {
		return fmt.Errorf("issuer URL must not have a query string: %s", issuer.String())
	}
	return nil
}

// supportsClientRegistration checks if the provider supports dynamic client registration
func supportsClientRegistration(provider server.OAuthServerProvider) bool {
	if provider == nil {
		return false
	}

	clientsStore := provider.ClientsStore()
	if clientsStore == nil {
		return false
	}
	// Check if the clients store supports registration
	return clientsStore.SupportsRegistration()
}

// supportsTokenRevocation checks if the provider supports token revocation
func supportsTokenRevocation(provider server.OAuthServerProvider) bool {
	if provider == nil {
		return false
	}
	// Use type assertion to check if the provider implements SupportTokenRevocation interface
	_, ok := provider.(server.SupportTokenRevocation)
	return ok
}

// CreateOAuthMetadata generates OAuth 2.1 compliant Authorization Server Metadata.
func CreateOAuthMetadata(options struct {
	Provider                server.OAuthServerProvider
	IssuerUrl               *url.URL
	BaseUrl                 *url.URL
	ServiceDocumentationUrl *url.URL
	ScopesSupported         []string
}) (auth.OAuthMetadata, error) {
	if options.Provider == nil {
		return auth.OAuthMetadata{}, fmt.Errorf("provider is required")
	}

	issuer := options.IssuerUrl
	baseUrl := options.BaseUrl

	// Validate issuer URL
	if err := checkIssuerUrl(issuer); err != nil {
		return auth.OAuthMetadata{}, err
	}

	// Determine base URL for endpoints
	var baseUrlForEndpoints *url.URL
	if baseUrl != nil {
		baseUrlForEndpoints = baseUrl
	} else {
		baseUrlForEndpoints = issuer
	}

	// Required endpoints
	authorizationEndpoint := "/authorize"
	tokenEndpoint := "/token"

	authEndpointUrl, _ := url.Parse(authorizationEndpoint)
	tokenEndpointUrl, _ := url.Parse(tokenEndpoint)

	metadata := auth.OAuthMetadata{
		// Core fields
		Issuer:                issuer.String(),
		AuthorizationEndpoint: baseUrlForEndpoints.ResolveReference(authEndpointUrl).String(),
		TokenEndpoint:         baseUrlForEndpoints.ResolveReference(tokenEndpointUrl).String(),

		// OAuth 2.1 requires PKCE support
		ResponseTypesSupported:        []string{"code"}, // OAuth 2.1 removes implicit flow
		CodeChallengeMethodsSupported: []string{"S256"}, // OAuth 2.1 requires S256, plain is deprecated

		// Token endpoint authentication methods
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic"},

		// OAuth 2.1 supported grant types
		GrantTypesSupported: []string{"authorization_code", "refresh_token"},

		// Optional fields
		ScopesSupported: options.ScopesSupported,
	}

	// Add service documentation if provided
	if options.ServiceDocumentationUrl != nil {
		serviceDoc := options.ServiceDocumentationUrl.String()
		metadata.ServiceDocumentation = &serviceDoc
	}

	// Check for optional endpoints based on provider capabilities
	if supportsTokenRevocation(options.Provider) {
		revocationEndpoint := "/revoke"
		revEndpointUrl, _ := url.Parse(revocationEndpoint)
		revEndpoint := baseUrlForEndpoints.ResolveReference(revEndpointUrl).String()
		metadata.RevocationEndpoint = &revEndpoint
		metadata.RevocationEndpointAuthMethodsSupported = []string{"client_secret_post", "client_secret_basic"}
	}

	if supportsClientRegistration(options.Provider) {
		registrationEndpoint := "/register"
		regEndpointUrl, _ := url.Parse(registrationEndpoint)
		regEndpoint := baseUrlForEndpoints.ResolveReference(regEndpointUrl).String()
		metadata.RegistrationEndpoint = &regEndpoint
	}

	return metadata, nil
}

// McpAuthRouter sets up OAuth 2.1 compliant MCP authorization server endpoints
func McpAuthRouter(mux *http.ServeMux, options AuthRouterOptions) error {
	// Create OAuth metadata with error handling
	oauthMetadata, err := CreateOAuthMetadata(struct {
		Provider                server.OAuthServerProvider
		IssuerUrl               *url.URL
		BaseUrl                 *url.URL
		ServiceDocumentationUrl *url.URL
		ScopesSupported         []string
	}{
		Provider:                options.Provider,
		IssuerUrl:               options.IssuerUrl,
		BaseUrl:                 options.BaseUrl,
		ServiceDocumentationUrl: options.ServiceDocumentationUrl,
		ScopesSupported:         options.ScopesSupported,
	})
	if err != nil {
		return fmt.Errorf("failed to create OAuth metadata: %w", err)
	}

	// Authorization endpoint (GET only for OAuth 2.1)
	authorizationURL, _ := url.Parse(oauthMetadata.AuthorizationEndpoint)
	authzOptions := handler.AuthorizationHandlerOptions{
		Provider: options.Provider,
	}
	if options.AuthorizationOptions != nil && options.AuthorizationOptions.RateLimit != nil {
		authzOptions.RateLimit = options.AuthorizationOptions.RateLimit
	}
	mux.Handle(authorizationURL.Path, methodRestrictedHandler("GET", handler.AuthorizationHandler(authzOptions)))

	// Token endpoint (POST only for OAuth 2.1)
	tokenURL, _ := url.Parse(oauthMetadata.TokenEndpoint)
	tokenOptions := handler.TokenHandlerOptions{Provider: options.Provider}
	if options.TokenOptions != nil {
		if options.TokenOptions.RateLimit != nil {
			tokenOptions.RateLimit = options.TokenOptions.RateLimit
		}
	}
	mux.Handle(tokenURL.Path, methodRestrictedHandler("POST", handler.TokenHandler(tokenOptions)))

	// Metadata endpoints
	issuerURL, _ := url.Parse(oauthMetadata.Issuer)
	resourceURL := options.BaseUrl
	if resourceURL == nil {
		resourceURL = issuerURL
	}
	if err := McpAuthMetadataRouter(mux, AuthMetadataOptions{
		OAuthMetadata:           oauthMetadata,
		ResourceServerUrl:       resourceURL,
		ServiceDocumentationUrl: options.ServiceDocumentationUrl,
		ScopesSupported:         options.ScopesSupported,
		ResourceName:            options.ResourceName,
	}); err != nil {
		return fmt.Errorf("failed to setup metadata router: %w", err)
	}

	// Dynamic client registration (optional, POST only)
	if oauthMetadata.RegistrationEndpoint != nil {
		// Ensure ClientsStore() is not nil before mounting /register
		if clientsStore := options.Provider.ClientsStore(); clientsStore != nil {
			registrationURL, _ := url.Parse(*oauthMetadata.RegistrationEndpoint)
			regOpts := handler.ClientRegistrationHandlerOptions{
				ClientsStore: clientsStore,
			}
			if options.ClientRegistrationOptions != nil {
				regOpts = *options.ClientRegistrationOptions
				regOpts.ClientsStore = clientsStore
			} else {
				// OAuth 2.1 recommended rate limiting for client registration
				regOpts.RateLimit = &handler.RegisterRateLimitConfig{
					WindowMs: 60000,
					Max:      10,
				}
			}
			mux.Handle(registrationURL.Path, methodRestrictedHandler("POST", handler.ClientRegistrationHandler(regOpts)))
		}
	}

	// Token revocation endpoint (optional, POST only)
	if oauthMetadata.RevocationEndpoint != nil {
		revocationURL, _ := url.Parse(*oauthMetadata.RevocationEndpoint)

		revOpts := handler.RevocationHandlerOptions{
			Provider: options.Provider,
		}
		if options.RevocationOptions != nil && options.RevocationOptions.RateLimit != nil {
			revOpts.RateLimit = options.RevocationOptions.RateLimit
		}

		mux.Handle(revocationURL.Path, methodRestrictedHandler("POST", handler.RevocationHandler(revOpts)))
	}

	return nil
}

// McpAuthMetadataRouter sets up OAuth 2.1 compliant metadata endpoints
func McpAuthMetadataRouter(mux *http.ServeMux, options AuthMetadataOptions) error {
	issuerURL, _ := url.Parse(options.OAuthMetadata.Issuer)
	if err := checkIssuerUrl(issuerURL); err != nil {
		return fmt.Errorf("invalid issuer URL in metadata: %w", err)
	}

	// Create protected resource metadata
	protectedResourceMetadata := auth.OAuthProtectedResourceMetadata{
		Resource: options.ResourceServerUrl.String(),
		AuthorizationServers: []string{
			options.OAuthMetadata.Issuer,
		},
		ScopesSupported: options.ScopesSupported,
	}

	// Add optional fields
	if options.ResourceName != nil {
		protectedResourceMetadata.ResourceName = options.ResourceName
	}

	if options.ServiceDocumentationUrl != nil {
		resourceDoc := options.ServiceDocumentationUrl.String()
		protectedResourceMetadata.ResourceDocumentation = &resourceDoc
	}

	// Protected resource metadata endpoint (GET only)
	mux.Handle("/.well-known/oauth-protected-resource",
		methodRestrictedHandler("GET", handler.MetadataHandler(protectedResourceMetadata)))

	// Authorization server metadata endpoint (GET only, for backward compatibility)
	mux.Handle("/.well-known/oauth-authorization-server",
		methodRestrictedHandler("GET", handler.MetadataHandler(options.OAuthMetadata)))

	return nil
}

// GetOAuthProtectedResourceMetadataUrl constructs the OAuth 2.0 Protected Resource Metadata URL from a given server URL
func GetOAuthProtectedResourceMetadataUrl(serverUrl *url.URL) string {
	metadataUrl, _ := url.Parse("/.well-known/oauth-protected-resource")
	return serverUrl.ResolveReference(metadataUrl).String()
}

// InstallMCPAuthRoutes convenience function to simplify OAuth 2.1 compliant route installation
func InstallMCPAuthRoutes(
	mux *http.ServeMux,
	issuerBaseURL string,
	resourceServerURL string,
	provider server.OAuthServerProvider,
	scopesSupported []string,
	resourceName *string,
	serviceDocURL *string,
) error {
	issuerURL, err := url.Parse(issuerBaseURL)
	if err != nil {
		return fmt.Errorf("invalid issuer URL: %w", err)
	}

	var baseURL *url.URL
	if resourceServerURL != "" {
		baseURL, err = url.Parse(resourceServerURL)
		if err != nil {
			return fmt.Errorf("invalid resource server URL: %w", err)
		}
	}

	var serviceDocumentationUrl *url.URL
	if serviceDocURL != nil {
		serviceDocumentationUrl, err = url.Parse(*serviceDocURL)
		if err != nil {
			return fmt.Errorf("invalid service documentation URL: %w", err)
		}
	}

	options := AuthRouterOptions{
		Provider:                provider,
		IssuerUrl:               issuerURL,
		BaseUrl:                 baseURL,
		ServiceDocumentationUrl: serviceDocumentationUrl,
		ScopesSupported:         scopesSupported,
		ResourceName:            resourceName,
	}

	return McpAuthRouter(mux, options)
}

// methodRestrictedHandler returns an HTTP handler that restricts requests
// to the specified HTTP method. If the request method does not match
func methodRestrictedHandler(allowedMethod string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != allowedMethod {
			w.Header().Set("Allow", allowedMethod)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.ServeHTTP(w, r)
	})
}
