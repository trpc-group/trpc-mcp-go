// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package client

import (
	"net/http"
	"net/url"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

// OAuthClientProvider defines core OAuth 2.0 client operations
// Provides client configuration, token management, and authorization flow handling
type OAuthClientProvider interface {
	// RedirectURL returns the client redirect URL for authorization
	RedirectURL() string

	// ClientMetadata returns static client metadata such as redirect URIs
	ClientMetadata() auth.OAuthClientMetadata

	// ClientInformation returns current client credentials if available
	ClientInformation() *auth.OAuthClientInformation

	// Tokens returns the current access and refresh tokens
	Tokens() (*auth.OAuthTokens, error)

	// SaveTokens persists the given OAuth tokens
	SaveTokens(tokens auth.OAuthTokens) error

	// RedirectToAuthorization handles redirection to the authorization endpoint
	RedirectToAuthorization(authorizationUrl *url.URL) error

	// SaveCodeVerifier persists the PKCE code verifier for later token exchange
	SaveCodeVerifier(codeVerifier string) error

	// CodeVerifier retrieves the stored PKCE code verifier
	CodeVerifier() (string, error)
}

// OAuthStateProvider adds state parameter management for CSRF protection.
type OAuthStateProvider interface {
	State() (string, error)
}

// OAuthClientInfoProvider handles dynamic client credential storage.
type OAuthClientInfoProvider interface {
	SaveClientInformation(clientInformation auth.OAuthClientInformationFull) error
}

// OAuthClientAuthProvider enables custom client authentication methods.
type OAuthClientAuthProvider interface {
	AddClientAuthentication(headers http.Header, params url.Values, tokenUrl string) error
}

// OAuthResourceValidator validates resource URLs for specific server requirements.
type OAuthResourceValidator interface {
	ValidateResourceURL(serverUrl *url.URL, resourceMetadata *auth.OAuthProtectedResourceMetadata) (*url.URL, error)
}

// OAuthCredentialInvalidator handles logout and credential revocation.
type OAuthCredentialInvalidator interface {
	InvalidateCredentials(scope string) error
}
