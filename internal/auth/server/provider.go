// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package server

import (
	"net/http"
	"net/url"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

// AuthorizationParams carries parameters for starting an OAuth authorization request
type AuthorizationParams struct {
	CodeChallenge string   `json:"code_challenge"` // PKCE code challenge from client
	RedirectURI   string   `json:"redirect_uri"`   // Redirect URI registered by the client
	State         string   `json:"state"`          // Optional opaque value to maintain client state between request and callback
	Scopes        []string `json:"scopes"`         // Optional empty slice means not provided
	Resource      *url.URL `json:"resource"`       // Optional nil means not provided
}

// OAuthServerProvider defines a complete OAuth 2.1 server interface including client management authorization token exchange verification and revocation
type OAuthServerProvider interface {

	// ClientsStore returns the store used to read registered OAuth client information
	ClientsStore() *OAuthClientsStore

	// Authorize starts the authorization flow implemented by this server or by redirecting to another authorization server
	// The server must ultimately redirect to the given redirect URI with either a success or an error response per OAuth 2.1
	// On success include query params code and state if provided
	// On error include query param error and may include error_description
	Authorize(client auth.OAuthClientInformationFull, params AuthorizationParams, res http.ResponseWriter, req *http.Request) error

	// ChallengeForAuthorizationCode returns the codeChallenge that was used when the indicated authorization began
	ChallengeForAuthorizationCode(client auth.OAuthClientInformationFull, authorizationCode string) (string, error)

	// ExchangeAuthorizationCode exchanges an authorization code for access tokens
	// Validate code and optional PKCE codeVerifier and optional redirectUri and resource
	ExchangeAuthorizationCode(
		client auth.OAuthClientInformationFull,
		authorizationCode string, codeVerifier *string,
		redirectUri *string,
		resource *url.URL,
	) (*auth.OAuthTokens, error)

	// ExchangeRefreshToken exchanges a refresh token for new access tokens
	// Accept optional scopes and optional resource
	ExchangeRefreshToken(
		client auth.OAuthClientInformationFull,
		refreshToken string,
		scopes []string, // Optional empty slice if not provided
		resource *url.URL, // Optional nil if not provided
	) (*auth.OAuthTokens, error)

	// VerifyAccessToken verifies an access token and returns its associated information
	VerifyAccessToken(token string) (*AuthInfo, error)

	// SupportTokenRevocation indicates optional support for token revocation
	SupportTokenRevocation
}

// SupportTokenRevocation defines optional token revocation capability
type SupportTokenRevocation interface {
	// RevokeToken revokes an access or refresh token
	// If the token is invalid or already revoked this should be a no op
	// Optional method
	RevokeToken(client auth.OAuthClientInformationFull, request auth.OAuthTokenRevocationRequest) error
}
