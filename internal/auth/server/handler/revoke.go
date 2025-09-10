// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package handler

import (
	"encoding/json"
	"golang.org/x/time/rate"
	"net/http"
	"strings"
	"time"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/middleware"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// RevocationHandlerOptions configuration for the token revocation endpoint
type RevocationHandlerOptions struct {
	Provider          server.OAuthServerProvider
	RateLimit         *RevocationRateLimitConfig // Set to nil to disable rate limiting for this endpoint
	RequireHTTPS      bool                       // Enforce HTTPS in production (recommended for OAuth 2.1)
	AllowJSONFallback bool                       // Allow JSON format for backward compatibility (non-compliant with RFC 7009)
	EnableMCPHeaders  bool                       // Add MCP-specific headers for MCP 2025-03-26 compliance
}

// RevocationRateLimitConfig rate limiting configuration
type RevocationRateLimitConfig struct {
	WindowMs int // Window duration in milliseconds
	Max      int // Maximum requests per window
}

// RevocationHandler creates a handler for OAuth token revocation with client authentication middleware
func RevocationHandler(opts RevocationHandlerOptions) http.Handler {
	// Check if provider supports token revocation
	revoker, ok := opts.Provider.(server.SupportTokenRevocation)
	if !ok {
		panic("Auth provider does not support revoking tokens")
	}

	// Create the core handler
	coreHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set OAuth 2.1 required security headers
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		// Add MCP-specific headers if enabled
		if opts.EnableMCPHeaders {
			w.Header().Set("X-MCP-Version", "2025-03-26")
			w.Header().Set("X-MCP-Transport", "http")
		}

		// Enforce HTTPS if required (OAuth 2.1 best practice)
		if opts.RequireHTTPS && r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			invalidReqError := errors.NewOAuthError(
				errors.ErrInvalidRequest,
				"HTTPS is required for OAuth 2.1 token revocation",
				"https://datatracker.ietf.org/doc/html/rfc6749#section-3",
			)
			json.NewEncoder(w).Encode(invalidReqError.ToResponseStruct())
			return
		}

		// RFC 7009 Section 2.1: Strict Content-Type validation
		contentType := r.Header.Get("Content-Type")
		isURLEncoded := strings.HasPrefix(contentType, "application/x-www-form-urlencoded")
		isJSON := strings.HasPrefix(contentType, "application/json")

		if !isURLEncoded && (!opts.AllowJSONFallback || !isJSON) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			errorMsg := "Content-Type must be application/x-www-form-urlencoded per RFC 7009"
			if opts.AllowJSONFallback {
				errorMsg = "Content-Type must be application/x-www-form-urlencoded (preferred) or application/json"
			}

			invalidReqError := errors.NewOAuthError(
				errors.ErrInvalidRequest,
				errorMsg,
				"https://datatracker.ietf.org/doc/html/rfc7009#section-2.1",
			)
			json.NewEncoder(w).Encode(invalidReqError.ToResponseStruct())
			return
		}

		// Parse request body based on content type
		var reqBody auth.OAuthTokenRevocationRequest

		if isURLEncoded {
			// RFC 7009 compliant: Parse URL-encoded form data
			if err := r.ParseForm(); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)

				invalidReqError := errors.NewOAuthError(errors.ErrInvalidRequest,
					"Failed to parse application/x-www-form-urlencoded data: "+err.Error(), "")
				json.NewEncoder(w).Encode(invalidReqError.ToResponseStruct())
				return
			}

			// Extract form values
			reqBody.Token = r.FormValue("token")
			reqBody.TokenTypeHint = r.FormValue("token_type_hint")
		} else if opts.AllowJSONFallback && isJSON {
			// Non-standard JSON fallback for backward compatibility
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)

				invalidReqError := errors.NewOAuthError(errors.ErrInvalidRequest,
					"Failed to parse JSON data: "+err.Error(), "")
				json.NewEncoder(w).Encode(invalidReqError.ToResponseStruct())
				return
			}
		}

		// Validate request - token is required (RFC 7009 Section 2.1)
		if err := validateRevocationRequest(reqBody); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(err.(errors.OAuthError).ToResponseStruct())
			return
		}

		// Get authenticated client from context (set by clientAuth middleware)
		client, ok := middleware.GetAuthenticatedClient(r)
		if !ok {
			// This should never happen if middleware is properly configured
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)

			serverError := errors.NewOAuthError(errors.ErrServerError, "Internal Server Error", "")
			json.NewEncoder(w).Encode(serverError.ToResponseStruct())
			return
		}

		// Revoke the token
		err := revoker.RevokeToken(*client, reqBody)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")

			if oauthErr, ok := err.(errors.OAuthError); ok {
				status := http.StatusBadRequest
				if oauthErr.ErrorCode == errors.ErrServerError.Error() {
					status = http.StatusInternalServerError
				}
				w.WriteHeader(status)
				json.NewEncoder(w).Encode(oauthErr.ToResponseStruct())
				return
			}

			w.WriteHeader(http.StatusInternalServerError)
			serverError := errors.NewOAuthError(errors.ErrServerError, "Internal Server Error", "")
			json.NewEncoder(w).Encode(serverError.ToResponseStruct())
			return
		}

		// RFC 7009 Section 2: Success response - HTTP 200 with empty JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})

	// Apply middlewares in order (wrapping from inside out to match TS middleware order)
	var handler http.Handler = coreHandler

	// Apply client authentication middleware (innermost, like TS)
	handler = middleware.AuthenticateClient(middleware.ClientAuthenticationMiddlewareOptions{
		ClientsStore: opts.Provider.ClientsStore(),
	})(handler)

	// Apply rate limiting middleware only if explicitly configured
	if opts.RateLimit != nil {
		windowDuration := time.Duration(opts.RateLimit.WindowMs) * time.Millisecond
		if opts.RateLimit.Max <= 0 {
			panic("RateLimit Max must be greater than 0")
		}

		// Calculate rate to match the window-based approach of express-rate-limit
		limit := rate.Every(windowDuration / time.Duration(opts.RateLimit.Max))
		limiter := rate.NewLimiter(limit, opts.RateLimit.Max)

		handler = middleware.RateLimitMiddleware(limiter)(handler)
	}
	// Note: No default rate limiting applied when opts.RateLimit is nil
	// This matches the TypeScript behavior where rateLimit: false disables it

	// Apply URL-encoded parsing middleware (RFC 7009 compliance)
	handler = middleware.URLEncodedValidationMiddleware(opts.AllowJSONFallback)(handler)

	// Apply method restriction middleware (only POST allowed per RFC 7009)
	handler = middleware.AllowedMethods([]string{"POST"})(handler)

	// Apply CORS middleware (outermost, like TS)
	handler = middleware.CorsMiddleware(handler)

	return handler
}

// validateRevocationRequest validates the OAuth token revocation request per RFC 7009
func validateRevocationRequest(reqBody auth.OAuthTokenRevocationRequest) error {
	// RFC 7009 Section 2.1: token parameter is required
	if reqBody.Token == "" {
		return errors.NewOAuthError(errors.ErrInvalidRequest,
			"token parameter is required per RFC 7009",
			"https://datatracker.ietf.org/doc/html/rfc7009#section-2.1")
	}

	// RFC 7009 Section 2.1: token_type_hint is optional but must be valid if provided
	if reqBody.TokenTypeHint != "" {
		validTypes := map[string]bool{
			"access_token":  true,
			"refresh_token": true,
		}
		if !validTypes[reqBody.TokenTypeHint] {
			return errors.NewOAuthError(
				errors.ErrInvalidRequest,
				"invalid token_type_hint, must be 'access_token' or 'refresh_token' per RFC 7009",
				"https://datatracker.ietf.org/doc/html/rfc7009#section-2.1",
			)
		}
	}

	return nil
}
