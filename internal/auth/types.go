// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package auth

import "net/http"

// OAuthClientMetadata defines RFC 7591 OAuth 2.0 Dynamic Client Registration metadata
type OAuthClientMetadata struct {
	RedirectURIs            []string    `json:"redirect_uris"`                        // Allowed redirect URIs for the client
	TokenEndpointAuthMethod string      `json:"token_endpoint_auth_method,omitempty"` // Client auth method at token endpoint
	GrantTypes              []string    `json:"grant_types,omitempty"`                // Supported grant types
	ResponseTypes           []string    `json:"response_types,omitempty"`             // Supported response types
	ClientName              *string     `json:"client_name,omitempty"`                // Human readable client name
	ClientURI               *string     `json:"client_uri,omitempty"`                 // Client homepage URL
	LogoURI                 *string     `json:"logo_uri,omitempty"`                   // Client logo URL
	Scope                   *string     `json:"scope,omitempty"`                      // Default requested scopes as space separated string
	Contacts                []string    `json:"contacts,omitempty"`                   // Admin contact emails
	TosURI                  *string     `json:"tos_uri,omitempty"`                    // Terms of service URL
	PolicyURI               *string     `json:"policy_uri,omitempty"`                 // Privacy policy URL
	JwksURI                 *string     `json:"jwks_uri,omitempty"`                   // URL to client JWKS
	Jwks                    interface{} `json:"jwks,omitempty"`                       // Inline JWKS object
	SoftwareID              *string     `json:"software_id,omitempty"`                // Software identifier
	SoftwareVersion         *string     `json:"software_version,omitempty"`           // Software version
	SoftwareStatement       *string     `json:"software_statement,omitempty"`         // Software statement assertion
}

// OAuthClientInformation defines RFC 7591 OAuth 2.0 Dynamic Client Registration client information
type OAuthClientInformation struct {
	ClientID              string `json:"client_id"`                          // Issued client identifier
	ClientSecret          string `json:"client_secret,omitempty"`            // Issued client secret if applicable
	ClientIDIssuedAt      *int64 `json:"client_id_issued_at,omitempty"`      // Issue time in seconds since epoch
	ClientSecretExpiresAt *int64 `json:"client_secret_expires_at,omitempty"` // Secret expiry time in seconds since epoch
}

// OAuthClientInformationFull defines RFC 7591 OAuth 2.0 Dynamic Client Registration full response
type OAuthClientInformationFull struct {
	OAuthClientMetadata
	OAuthClientInformation
}

// OAuthProtectedResourceMetadata defines RFC 9728 OAuth Protected Resource metadata
type OAuthProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`                                             // Resource identifier URI
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`                      // Authorization server issuers supporting this resource
	JWKSURI                *string  `json:"jwks_uri,omitempty"`                                   // JWKS URI used by the resource
	ScopesSupported        []string `json:"scopes_supported,omitempty"`                           // Supported scopes
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`                   // Supported bearer presentation methods
	ResourceSigningAlgs    []string `json:"resource_signing_alg_values_supported,omitempty"`      // Supported signing algorithms
	ResourceName           *string  `json:"resource_name,omitempty"`                              // Human friendly resource name
	ResourceDocumentation  *string  `json:"resource_documentation,omitempty"`                     // Documentation URL
	ResourcePolicyURI      *string  `json:"resource_policy_uri,omitempty"`                        // Policy URL
	ResourceTOSURI         *string  `json:"resource_tos_uri,omitempty"`                           // Terms of service URL
	TLSCertBoundAT         *bool    `json:"tls_client_certificate_bound_access_tokens,omitempty"` // Whether MTLS bound AT are required or supported
	AuthzDetailsTypes      []string `json:"authorization_details_types_supported,omitempty"`      // Supported authorization details types
	DPoPSigningAlgs        []string `json:"dpop_signing_alg_values_supported,omitempty"`          // Supported DPoP signing algorithms
	DPoPBoundATRequired    *bool    `json:"dpop_bound_access_tokens_required,omitempty"`          // Whether DPoP bound access tokens are required
}

// OAuthTokens defines the OAuth 2.1 token response
type OAuthTokens struct {
	AccessToken  string  `json:"access_token"`            // Access token value required non empty
	IDToken      *string `json:"id_token,omitempty"`      // OIDC ID token optional non empty if present
	TokenType    string  `json:"token_type"`              // Token type for example Bearer required non empty
	ExpiresIn    *int64  `json:"expires_in,omitempty"`    // Access token lifetime in seconds optional positive if present
	Scope        *string `json:"scope,omitempty"`         // Granted scope as space separated string optional non empty if present
	RefreshToken *string `json:"refresh_token,omitempty"` // Refresh token optional non empty if present
}

// OAuthTokenRevocationRequest represents a token revocation request payload
type OAuthTokenRevocationRequest struct {
	Token         string `json:"token"`                     // Token to revoke
	TokenTypeHint string `json:"token_type_hint,omitempty"` // Optional token type hint
}

// AuthorizationServerMetadata represents OAuth 2.0 or OpenID Connect server metadata
type AuthorizationServerMetadata interface {
	// GetIssuer returns the issuer identifier
	GetIssuer() string
	// GetAuthorizationEndpoint returns the authorization endpoint URL
	GetAuthorizationEndpoint() string
	// GetTokenEndpoint returns the token endpoint URL
	GetTokenEndpoint() string
	// GetResponseTypesSupported returns supported response types
	GetResponseTypesSupported() []string
	// GetGrantTypesSupported returns supported grant types
	GetGrantTypesSupported() []string
	// GetTokenEndpointAuthMethodsSupported returns supported client auth methods for the token endpoint
	GetTokenEndpointAuthMethodsSupported() []string
}

// OAuthMetadata defines OAuth 2.0 Authorization Server Metadata per RFC 8414
type OAuthMetadata struct {
	Issuer                                             string   `json:"issuer"`                                                             // Issuer identifier
	AuthorizationEndpoint                              string   `json:"authorization_endpoint"`                                             // Authorization endpoint URL
	TokenEndpoint                                      string   `json:"token_endpoint"`                                                     // Token endpoint URL
	RegistrationEndpoint                               *string  `json:"registration_endpoint,omitempty"`                                    // Dynamic client registration endpoint
	ScopesSupported                                    []string `json:"scopes_supported,omitempty"`                                         // Supported scopes
	ResponseTypesSupported                             []string `json:"response_types_supported"`                                           // Supported response types
	ResponseModesSupported                             []string `json:"response_modes_supported,omitempty"`                                 // Supported response modes
	GrantTypesSupported                                []string `json:"grant_types_supported,omitempty"`                                    // Supported grant types
	TokenEndpointAuthMethodsSupported                  []string `json:"token_endpoint_auth_methods_supported,omitempty"`                    // Supported token endpoint auth methods
	TokenEndpointAuthSigningAlgValuesSupported         []string `json:"token_endpoint_auth_signing_alg_values_supported,omitempty"`         // Supported signing algs for client auth
	ServiceDocumentation                               *string  `json:"service_documentation,omitempty"`                                    // Service documentation URL
	RevocationEndpoint                                 *string  `json:"revocation_endpoint,omitempty"`                                      // Token revocation endpoint
	RevocationEndpointAuthMethodsSupported             []string `json:"revocation_endpoint_auth_methods_supported,omitempty"`               // Supported auth methods for revocation
	RevocationEndpointAuthSigningAlgValuesSupported    []string `json:"revocation_endpoint_auth_signing_alg_values_supported,omitempty"`    // Supported signing algs for revocation
	IntrospectionEndpoint                              *string  `json:"introspection_endpoint,omitempty"`                                   // Token introspection endpoint
	IntrospectionEndpointAuthMethodsSupported          []string `json:"introspection_endpoint_auth_methods_supported,omitempty"`            // Supported auth methods for introspection
	IntrospectionEndpointAuthSigningAlgValuesSupported []string `json:"introspection_endpoint_auth_signing_alg_values_supported,omitempty"` // Supported signing algs for introspection
	CodeChallengeMethodsSupported                      []string `json:"code_challenge_methods_supported,omitempty"`                         // Supported PKCE methods
}

// GetIssuer returns the issuer identifier
func (m OAuthMetadata) GetIssuer() string {
	return m.Issuer
}

// GetAuthorizationEndpoint returns the authorization endpoint URL
func (m OAuthMetadata) GetAuthorizationEndpoint() string {
	return m.AuthorizationEndpoint
}

// GetTokenEndpoint returns the token endpoint URL
func (m OAuthMetadata) GetTokenEndpoint() string {
	return m.TokenEndpoint
}

// GetResponseTypesSupported returns supported response types
func (m OAuthMetadata) GetResponseTypesSupported() []string {
	return m.ResponseTypesSupported
}

// GetGrantTypesSupported returns supported grant types
func (m OAuthMetadata) GetGrantTypesSupported() []string {
	return m.GrantTypesSupported
}

// GetTokenEndpointAuthMethodsSupported returns supported client auth methods for the token endpoint
func (m OAuthMetadata) GetTokenEndpointAuthMethodsSupported() []string {
	return m.TokenEndpointAuthMethodsSupported
}

// OpenIdProviderMetadata defines OpenID Connect Discovery 1.0 provider metadata
type OpenIdProviderMetadata struct {
	Issuer                                     string   `json:"issuer"`                                                     // Issuer identifier
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`                                     // Authorization endpoint URL
	TokenEndpoint                              string   `json:"token_endpoint"`                                             // Token endpoint URL
	UserinfoEndpoint                           *string  `json:"userinfo_endpoint,omitempty"`                                // Userinfo endpoint URL
	JwksURI                                    string   `json:"jwks_uri"`                                                   // JWKS URI
	RegistrationEndpoint                       *string  `json:"registration_endpoint,omitempty"`                            // Dynamic client registration endpoint
	ScopesSupported                            []string `json:"scopes_supported,omitempty"`                                 // Supported scopes
	ResponseTypesSupported                     []string `json:"response_types_supported"`                                   // Supported response types
	ResponseModesSupported                     []string `json:"response_modes_supported,omitempty"`                         // Supported response modes
	GrantTypesSupported                        []string `json:"grant_types_supported,omitempty"`                            // Supported grant types
	AcrValuesSupported                         []string `json:"acr_values_supported,omitempty"`                             // Supported ACR values
	SubjectTypesSupported                      []string `json:"subject_types_supported"`                                    // Supported subject types
	IdTokenSigningAlgValuesSupported           []string `json:"id_token_signing_alg_values_supported"`                      // Supported ID token signing algs
	IdTokenEncryptionAlgValuesSupported        []string `json:"id_token_encryption_alg_values_supported,omitempty"`         // Supported ID token encryption algs
	IdTokenEncryptionEncValuesSupported        []string `json:"id_token_encryption_enc_values_supported,omitempty"`         // Supported ID token encryption enc values
	UserinfoSigningAlgValuesSupported          []string `json:"userinfo_signing_alg_values_supported,omitempty"`            // Supported userinfo signing algs
	UserinfoEncryptionAlgValuesSupported       []string `json:"userinfo_encryption_alg_values_supported,omitempty"`         // Supported userinfo encryption algs
	UserinfoEncryptionEncValuesSupported       []string `json:"userinfo_encryption_enc_values_supported,omitempty"`         // Supported userinfo encryption enc values
	RequestObjectSigningAlgValuesSupported     []string `json:"request_object_signing_alg_values_supported,omitempty"`      // Supported request object signing algs
	RequestObjectEncryptionAlgValuesSupported  []string `json:"request_object_encryption_alg_values_supported,omitempty"`   // Supported request object encryption algs
	RequestObjectEncryptionEncValuesSupported  []string `json:"request_object_encryption_enc_values_supported,omitempty"`   // Supported request object encryption enc values
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported,omitempty"`            // Supported token endpoint auth methods
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported,omitempty"` // Supported signing algs for token endpoint auth
	DisplayValuesSupported                     []string `json:"display_values_supported,omitempty"`                         // Supported display values
	ClaimTypesSupported                        []string `json:"claim_types_supported,omitempty"`                            // Supported claim types
	ClaimsSupported                            []string `json:"claims_supported,omitempty"`                                 // Supported claims
	ServiceDocumentation                       *string  `json:"service_documentation,omitempty"`                            // Service documentation URL
	ClaimsLocalesSupported                     []string `json:"claims_locales_supported,omitempty"`                         // Supported claims locales
	UiLocalesSupported                         []string `json:"ui_locales_supported,omitempty"`                             // Supported UI locales
	ClaimsParameterSupported                   *bool    `json:"claims_parameter_supported,omitempty"`                       // Whether claims parameter is supported
	RequestParameterSupported                  *bool    `json:"request_parameter_supported,omitempty"`                      // Whether request parameter is supported
	RequestUriParameterSupported               *bool    `json:"request_uri_parameter_supported,omitempty"`                  // Whether request_uri is supported
	RequireRequestUriRegistration              *bool    `json:"require_request_uri_registration,omitempty"`                 // Whether request_uri registration is required
	OpPolicyUri                                *string  `json:"op_policy_uri,omitempty"`                                    // OP policy URL
	OpTosUri                                   *string  `json:"op_tos_uri,omitempty"`                                       // OP terms of service URL
}

// AuthOptions contains configuration options for the OAuth authorization process
type AuthOptions struct {
	ServerUrl           string    // OAuth server URL
	ResourceMetadataUrl *string   // Resource metadata URL
	AuthorizationCode   *string   // Authorization code to exchange
	Scope               *string   // Requested scopes as space separated string
	ProtocolVersion     *string   // OAuth protocol version string
	FetchFn             FetchFunc // Custom HTTP request function
}

// DiscoveryOptions contains options for discovering OAuth server metadata
type DiscoveryOptions struct {
	ServerUrl           string    // Base server URL for discovery
	ResourceMetadataUrl *string   // Resource metadata URL for RFC 9728
	FetchFn             FetchFunc // Custom HTTP request function
	ProtocolVersion     *string   // Protocol version hint
}

// GetIssuer returns the issuer identifier
func (m OpenIdProviderMetadata) GetIssuer() string {
	return m.Issuer
}

// GetAuthorizationEndpoint returns the authorization endpoint URL
func (m OpenIdProviderMetadata) GetAuthorizationEndpoint() string {
	return m.AuthorizationEndpoint
}

// GetTokenEndpoint returns the token endpoint URL
func (m OpenIdProviderMetadata) GetTokenEndpoint() string {
	return m.TokenEndpoint
}

// GetResponseTypesSupported returns supported response types
func (m OpenIdProviderMetadata) GetResponseTypesSupported() []string {
	return m.ResponseTypesSupported
}

// GetGrantTypesSupported returns supported grant types
func (m OpenIdProviderMetadata) GetGrantTypesSupported() []string {
	return m.GrantTypesSupported
}

// GetTokenEndpointAuthMethodsSupported returns supported client auth methods for the token endpoint
func (m OpenIdProviderMetadata) GetTokenEndpointAuthMethodsSupported() []string {
	return m.TokenEndpointAuthMethodsSupported
}

// OpenIdProviderDiscoveryMetadata merges OpenID Provider metadata with OAuth 2.0 fields for discovery
type OpenIdProviderDiscoveryMetadata struct {
	OpenIdProviderMetadata                 // Embedded OIDC provider metadata
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"` // Supported PKCE methods
}

// GetIssuer returns the issuer identifier
func (m OpenIdProviderDiscoveryMetadata) GetIssuer() string {
	return m.OpenIdProviderMetadata.Issuer
}

// GetAuthorizationEndpoint returns the authorization endpoint URL
func (m OpenIdProviderDiscoveryMetadata) GetAuthorizationEndpoint() string {
	return m.OpenIdProviderMetadata.AuthorizationEndpoint
}

// GetTokenEndpoint returns the token endpoint URL
func (m OpenIdProviderDiscoveryMetadata) GetTokenEndpoint() string {
	return m.OpenIdProviderMetadata.TokenEndpoint
}

// GetResponseTypesSupported returns supported response types
func (m OpenIdProviderDiscoveryMetadata) GetResponseTypesSupported() []string {
	return m.OpenIdProviderMetadata.ResponseTypesSupported
}

// GetGrantTypesSupported returns supported grant types
func (m OpenIdProviderDiscoveryMetadata) GetGrantTypesSupported() []string {
	return m.OpenIdProviderMetadata.GrantTypesSupported
}

// GetTokenEndpointAuthMethodsSupported returns supported client auth methods for the token endpoint
func (m OpenIdProviderDiscoveryMetadata) GetTokenEndpointAuthMethodsSupported() []string {
	return m.OpenIdProviderMetadata.TokenEndpointAuthMethodsSupported
}

// FetchFunc is a customizable HTTP fetch function used by discovery and auth flows
type FetchFunc func(url string, req *http.Request) (*http.Response, error)
