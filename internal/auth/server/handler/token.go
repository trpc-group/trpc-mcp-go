// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/time/rate"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/pkce"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/middleware"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// TokenHandlerOptions defines configuration options for the token endpoint
type TokenHandlerOptions struct {
	Provider  server.OAuthServerProvider `json:"provider"`
	RateLimit *rate.Limiter              `json:"rateLimit,omitempty"`
}

// TokenRequest defines the base structure of a token request.
// Every token request must specify a grant_type to indicate which flow is being used.
type TokenRequest struct {
	// GrantType is the type of OAuth grant being requested.
	// Common values include "authorization_code" and "refresh_token".
	GrantType string `form:"grant_type" json:"grant_type" validate:"required"`
}

// AuthorizationCodeGrant represents a token request using the Authorization Code flow.
type AuthorizationCodeGrant struct {
	// Code is the authorization code previously issued to the client.
	Code string `form:"code" json:"code" validate:"required"`

	// CodeVerifier is the PKCE verifier string that matches the original code_challenge.
	CodeVerifier string `form:"code_verifier" json:"code_verifier" validate:"required"`

	// RedirectURI must match the redirect_uri used in the authorization request,
	// if one was included there.
	RedirectURI *string `form:"redirect_uri" json:"redirect_uri,omitempty"`

	// Resource is an optional absolute URL indicating the target resource server.
	Resource *string `form:"resource" json:"resource,omitempty" validate:"omitempty,url"`
}

// RefreshTokenGrant represents a token request using the Refresh Token flow.
type RefreshTokenGrant struct {
	// RefreshToken is the refresh token previously issued to the client.
	RefreshToken string `form:"refresh_token" json:"refresh_token" validate:"required"`

	// Scope is an optional space-delimited list of scopes being requested.
	// If omitted, the scope is assumed to be identical to the scope originally granted.
	Scope *string `form:"scope" json:"scope,omitempty"`

	// Resource is an optional absolute URL indicating the target resource server.
	Resource *string `form:"resource" json:"resource,omitempty" validate:"omitempty,url"`
}

// TokenHandler creates a token endpoint handler with full middleware stack
func TokenHandler(options TokenHandlerOptions) http.HandlerFunc {
	// Create the core handler logic
	coreHandler := createTokenCoreHandler(options)

	// Apply middlewares in order
	var handler http.Handler = coreHandler

	// Apply client authentication middleware
	handler = middleware.AuthenticateClient(middleware.ClientAuthenticationMiddlewareOptions{
		ClientsStore: options.Provider.ClientsStore(),
	})(handler)

	// Apply rate limiting middleware
	limiter := options.RateLimit
	if limiter == nil {
		// Default rate limiting: 50 requests per 15 minutes
		limiter = rate.NewLimiter(rate.Every(15*time.Minute/50), 50)
	}
	handler = middleware.RateLimitMiddleware(limiter)(handler)

	// Apply method restriction middleware (only POST allowed)
	handler = middleware.AllowedMethods([]string{"POST"})(handler)

	// Apply CORS middleware
	handler = middleware.CorsMiddleware(handler)

	// Convert http.Handler to http.HandlerFunc
	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}

// createTokenCoreHandler creates the core token handler logic shared between both versions
func createTokenCoreHandler(options TokenHandlerOptions) http.HandlerFunc {
	// Create a validator
	validate := validator.New()

	return func(w http.ResponseWriter, r *http.Request) {
		// Set cache-control headers
		w.Header().Set("Cache-Control", "no-store")

		// Parsing form data
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			errResp := errors.NewOAuthError(errors.ErrInvalidRequest, "Failed to parse form data", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}

		// Get grant_type from form
		grantType := r.FormValue("grant_type")
		if grantType == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			errResp := errors.NewOAuthError(errors.ErrInvalidRequest, "invalid client credentials", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}

		// Verify basic token request
		tokenReq := TokenRequest{
			GrantType: grantType,
		}

		if err := validate.Struct(tokenReq); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			errResp := errors.NewOAuthError(errors.ErrInvalidRequest, err.Error(), "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}

		// Check client authentication result
		client, ok := middleware.GetAuthenticatedClient(r)
		if !ok {
			// NOW this code will actually execute because middleware didn't terminate
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized) // Proper OAuth error status
			errResp := errors.NewOAuthError(errors.ErrInvalidClient, "invalid client credentials", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}

		switch grantType {
		case "authorization_code":
			handleAuthorizationCodeGrant(w, r, validate, options.Provider, *client)
		case "refresh_token":
			handleRefreshTokenGrant(w, r, validate, options.Provider, *client)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			errResp := errors.NewOAuthError(errors.ErrUnsupportedGrantType, "The grant type is not supported by this authorization server.", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
		}
	}
}

// handleAuthorizationCodeGrant processes authorization code grant
func handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request, validate *validator.Validate, provider server.OAuthServerProvider, client auth.OAuthClientInformationFull) {
	// Parsing the authorization code grant request
	var redirectURI *string
	if uri := r.FormValue("redirect_uri"); uri != "" {
		redirectURI = &uri
	}

	var resource *string
	if res := r.FormValue("resource"); res != "" {
		resource = &res
	}

	grant := AuthorizationCodeGrant{
		Code:         r.FormValue("code"),
		CodeVerifier: r.FormValue("code_verifier"),
		RedirectURI:  redirectURI,
		Resource:     resource,
	}

	if err := validate.Struct(grant); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		switch verrs := err.(type) {
		case validator.ValidationErrors:
			for _, fe := range verrs {
				if fe.Field() == "Resource" && fe.Tag() == "url" {
					errResp := errors.NewOAuthError(errors.ErrInvalidRequest, "resource must be a valid URL", "")
					json.NewEncoder(w).Encode(errResp.ToResponseStruct())
					return
				}
			}
			errResp := errors.NewOAuthError(errors.ErrInvalidRequest, err.Error(), "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		default:
			errResp := errors.NewOAuthError(errors.ErrInvalidRequest, err.Error(), "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}
	}

	// Check if the provider supports skipLocalPKceValidation
	type skipLocalPKceValidation interface {
		GetSkipLocalPkceValidation() bool
	}

	skipLocalValidation := false
	if p, ok := provider.(skipLocalPKceValidation); ok {
		skipLocalValidation = p.GetSkipLocalPkceValidation()
	}

	// Perform local PKCE validation unless explicitly skipped
	if !skipLocalValidation {
		codeChallenge, err := provider.ChallengeForAuthorizationCode(client, grant.Code)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			errResp := errors.NewOAuthError(errors.ErrInvalidGrant, "Failed to retrieve code challenge", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}

		if !pkce.VerifyPKCEChallenge(grant.CodeVerifier, codeChallenge) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			errResp := errors.NewOAuthError(errors.ErrInvalidGrant, "code_verifier does not match the challenge", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}
	}

	var resourceURL *url.URL
	if grant.Resource != nil {
		var err error
		resourceURL, err = url.Parse(*grant.Resource)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			errResp := errors.NewOAuthError(errors.ErrInvalidRequest, "Invalid resource URL", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}
	}

	// Parse the code_verifier to the provider if PKCE validation did not occur locally
	var codeVerifier *string
	if skipLocalValidation {
		codeVerifier = &grant.CodeVerifier
	}

	// Exchange the authorization code for a token
	tokens, err := provider.ExchangeAuthorizationCode(
		client,
		grant.Code,
		codeVerifier,
		grant.RedirectURI,
		resourceURL,
	)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")

		// Return an appropriate OAuth error response based on the error type
		switch {
		case err == errors.ErrInvalidParams || err == errors.ErrMissingParams:
			w.WriteHeader(http.StatusBadRequest)
			errResp := errors.NewOAuthError(errors.ErrInvalidRequest, err.Error(), "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
		case err == errors.ErrInvalidJSONRPCParams:
			w.WriteHeader(http.StatusBadRequest)
			errResp := errors.NewOAuthError(errors.ErrInvalidGrant, err.Error(), "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
		default:
			w.WriteHeader(http.StatusInternalServerError)
			errResp := errors.NewOAuthError(errors.ErrServerError, "Internal Server Error", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tokens)
}

// handleRefreshTokenGrant handles refresh token grant
func handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request, validate *validator.Validate, provider server.OAuthServerProvider, client auth.OAuthClientInformationFull) {
	var scope *string
	if s := r.FormValue("scope"); s != "" {
		scope = &s
	}

	var resource *string
	if res := r.FormValue("resource"); res != "" {
		resource = &res
	}

	grant := RefreshTokenGrant{
		RefreshToken: r.FormValue("refresh_token"),
		Scope:        scope,
		Resource:     resource,
	}

	if err := validate.Struct(grant); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		errResp := errors.NewOAuthError(errors.ErrInvalidRequest, err.Error(), "")
		json.NewEncoder(w).Encode(errResp.ToResponseStruct())
		return
	}

	// Handle scopes
	var scopes []string
	if grant.Scope != nil {
		scopes = strings.Split(*grant.Scope, " ")
	}

	// Handle resource URL
	var resourceURL *url.URL
	if grant.Resource != nil {
		var err error
		resourceURL, err = url.Parse(*grant.Resource)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			errResp := errors.NewOAuthError(errors.ErrInvalidRequest, "Invalid resource URL", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}
	}

	// Swap refresh token
	tokens, err := provider.ExchangeRefreshToken(client, grant.RefreshToken, scopes, resourceURL)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(strings.ToLower(err.Error()), "invalid") {
			w.WriteHeader(http.StatusInternalServerError)
			errResp := errors.NewOAuthError(errors.ErrInvalidGrant, err.Error(), "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
			return
		}

		switch {
		case err == errors.ErrInvalidParams || err == errors.ErrMissingParams:
			w.WriteHeader(http.StatusBadRequest)
			errResp := errors.NewOAuthError(errors.ErrInvalidRequest, err.Error(), "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
		case err == errors.ErrInvalidJSONRPCParams:
			w.WriteHeader(http.StatusBadRequest)
			errResp := errors.NewOAuthError(errors.ErrInvalidGrant, err.Error(), "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
		default:
			w.WriteHeader(http.StatusInternalServerError)
			errResp := errors.NewOAuthError(errors.ErrServerError, "Internal Server Error", "")
			json.NewEncoder(w).Encode(errResp.ToResponseStruct())
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tokens)
}
