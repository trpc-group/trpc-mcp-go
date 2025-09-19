// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// audienceMatchLocal matches resource against allowed audience values (trim trailing '#')
func audienceMatchLocal(resource string, allowed []string) bool {
	resource = strings.TrimSuffix(strings.TrimSpace(resource), "#")
	for _, a := range allowed {
		if resource == strings.TrimSuffix(strings.TrimSpace(a), "#") {
			return true
		}
	}
	return false
}

// BearerAuthMiddlewareOptions defines configuration for the Bearer auth middleware
type BearerAuthMiddlewareOptions struct {
	// Verifier is used to validate the access token
	Verifier server.TokenVerifierInterface

	// RequiredScopes lists scopes that must all be present in the token
	RequiredScopes []string

	// ResourceMetadataURL is optionally included in the WWW-Authenticate header
	ResourceMetadataURL *string

	// Issuer restricts accepted tokens to this issuer (optional)
	Issuer string

	// Audience restricts accepted tokens to this audience/resource (optional)
	Audience []string
}

// RequireBearerAuth returns an HTTP middleware that validates Bearer tokens on incoming requests
func RequireBearerAuth(options BearerAuthMiddlewareOptions) func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// setErrorResponse writes a JSON OAuth error and appropriate status and headers
			setErrorResponse := func(w http.ResponseWriter, err errors.OAuthError, statusCode int) {
				// Set WWW-Authenticate only for 401 or 403 to align with TS implementation
				if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
					wwwAuthValue := fmt.Sprintf(`Bearer error="%s", error_description="%s"`, err.ErrorCode, err.Message)
					if options.ResourceMetadataURL != nil {
						wwwAuthValue += fmt.Sprintf(`, resource_metadata="%s"`, *options.ResourceMetadataURL)
					}
					// Append scope for insufficient_scope
					if err.ErrorCode == errors.ErrInsufficientScope.Error() && len(options.RequiredScopes) > 0 {
						wwwAuthValue += fmt.Sprintf(`, scope="%s"`, strings.Join(options.RequiredScopes, " "))
					}
					w.Header().Set("WWW-Authenticate", wwwAuthValue)
				}
				// Write JSON body with error details
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(statusCode)
				_ = json.NewEncoder(w).Encode(err.ToResponseStruct())
			}

			// Read Authorization header and ensure presence
			authHeader := req.Header.Get("Authorization")
			if authHeader == "" {
				setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidToken, "Missing Authorization header", ""), http.StatusUnauthorized)
				return
			}

			// Expect "Bearer <token>" format and extract the token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" || parts[1] == "" {
				setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidToken, "Invalid Authorization header format, expected 'Bearer TOKEN'", ""), http.StatusUnauthorized)
				return
			}
			token := parts[1]

			// Verify token using provided verifier
			authInfo, err := options.Verifier.VerifyAccessToken(req.Context(), token)
			if err != nil {
				// Map verifier error to HTTP status via OAuth error code
				if oauthErr, ok := err.(errors.OAuthError); ok {
					switch oauthErr.ErrorCode {
					case errors.ErrInvalidToken.Error():
						setErrorResponse(w, oauthErr, http.StatusUnauthorized)
					case errors.ErrInsufficientScope.Error():
						setErrorResponse(w, oauthErr, http.StatusForbidden)
					case errors.ErrServerError.Error():
						setErrorResponse(w, oauthErr, http.StatusInternalServerError)
					default:
						setErrorResponse(w, oauthErr, http.StatusBadRequest)
					}
				} else {
					// Default unknown errors to invalid_token (401) to avoid leaking internals
					invalid := errors.NewOAuthError(errors.ErrInvalidToken, "Invalid access token", "")
					setErrorResponse(w, invalid, http.StatusUnauthorized)
				}
				return
			}

			// Optional issuer guarantee
			if options.Issuer != "" {
				if authInfo.Extra != nil {
					if iss, _ := authInfo.Extra["iss"].(string); iss != "" && iss != options.Issuer {
						setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidToken, "Invalid token issuer", ""), http.StatusUnauthorized)
						return
					}
				}
			}

			// Optional audience/resource check (RFC 8707 simplified)
			if len(options.Audience) > 0 && authInfo.Resource != nil {
				if !audienceMatchLocal(authInfo.Resource.String(), options.Audience) {
					setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidToken, "Invalid token audience", ""), http.StatusUnauthorized)
					return
				}
			}

			// Enforce required scopes if configured
			if len(options.RequiredScopes) > 0 {
				for _, scope := range options.RequiredScopes {
					found := false
					for _, tokenScope := range authInfo.Scopes {
						if tokenScope == scope {
							found = true
							break
						}
					}
					if !found {
						setErrorResponse(w, errors.NewOAuthError(errors.ErrInsufficientScope, "Insufficient scope", ""), http.StatusForbidden)
						return
					}
				}
			}

			// Ensure token has an expiration time and is not expired
			if authInfo.ExpiresAt == nil || *authInfo.ExpiresAt == 0 {
				setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidToken, "Token has no expiration time", ""), http.StatusUnauthorized)
				return
			}
			if *authInfo.ExpiresAt <= time.Now().Unix() {
				setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidToken, "Token has expired", ""), http.StatusUnauthorized)
				return
			}

			// Attach validated auth info to the request context under AuthInfoKey (avoid token propagation)
			authInfo.Token = ""
			ctx := context.WithValue(req.Context(), AuthInfoKey, authInfo)
			req = req.WithContext(ctx)

			// Delegate to next handler
			next.ServeHTTP(w, req)
		})
	}
}
