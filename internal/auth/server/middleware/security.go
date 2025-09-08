// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/time/rate"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// SecurityMiddlewareOption holds pluggable security dependencies for middleware
type SecurityMiddlewareOption struct {
	verifier server.TokenVerifier // Token verifier used by middleware that need direct verification
}

// Authorizer defines a unified authorization decision interface
// Implementations should return nil when access is allowed and an OAuthError when denied
type Authorizer interface {
	Authorize(authInfo server.AuthInfo, resource string, action string) error
}

// ScopePermissionMapper maps OAuth scopes to internal permissions
// The returned slice represents permissions granted by the provided scopes
type ScopePermissionMapper interface {
	MapScopes(scopes []string) []string
}

// DefaultScopeMapper is a simple mapper using a static scope→permissions table
type DefaultScopeMapper struct {
	Mapping map[string][]string // Per-scope permission list
}

// MapScopes expands scopes into a flattened permission list using Mapping
func (m *DefaultScopeMapper) MapScopes(scopes []string) []string {
	// Accumulate permissions granted by each scope
	var perms []string
	for _, scope := range scopes {
		if mapped, ok := m.Mapping[scope]; ok {
			perms = append(perms, mapped...)
		}
	}
	return perms
}

// PolicyAuthorizer authorizes by checking if the required permission exists after scope mapping
type PolicyAuthorizer struct {
	ScopeMapper ScopePermissionMapper // Pluggable scope→permission mapper
}

// Authorize checks whether authInfo scopes grant the required {resource}:{action} permission
func (a *PolicyAuthorizer) Authorize(authInfo server.AuthInfo, resource string, action string) error {
	// Convert scopes to internal permissions
	perms := a.ScopeMapper.MapScopes(authInfo.Scopes)

	// Build required permission string like urn:mcp:workspace:xyz:read
	required := fmt.Sprintf("%s:%s", resource, action)

	// Return success when permission is present
	for _, p := range perms {
		if p == required {
			return nil
		}
	}

	// Otherwise return standardized insufficient_scope error
	return errors.NewOAuthError(errors.ErrInsufficientScope,
		fmt.Sprintf("Missing permission %s", required), "")
}

// responseWriterWithStatus wraps http.ResponseWriter to capture the final status code
type responseWriterWithStatus struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader intercepts WriteHeader calls to store the status code
func (rw *responseWriterWithStatus) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CorsMiddleware applies permissive CORS headers similar to express default behavior
// It returns 204 for OPTIONS preflight while forwarding non-preflight requests downstream
func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read Origin header to detect cross origin requests
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Not a CORS request so proceed without CORS headers
			next.ServeHTTP(w, r)
			return
		}

		// Set basic CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,HEAD,PUT,PATCH,POST,DELETE")

		// Handle preflight with 204 and zero content length
		if r.Method == http.MethodOptions {
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Forward actual request
		next.ServeHTTP(w, r)
	})
}

// RateLimitMiddleware applies a token bucket limiter to incoming requests
// When the limiter denies a request a 429 JSON OAuth error is returned
func RateLimitMiddleware(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Short circuit when the limiter does not allow the request
			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				// Build standardized OAuth error payload
				tooManyRequestsError := errors.NewOAuthError(
					errors.ErrTooManyRequests,
					"You have exceeded the rate limit for token revocation requests",
					"",
				)
				_ = json.NewEncoder(w).Encode(tooManyRequestsError.ToResponseStruct())
				return
			}

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// ContentTypeValidationMiddleware validates the Content-Type header against an allowlist
// When allowJSONFallback is true application/json is accepted in addition to allowedTypes[0]
func ContentTypeValidationMiddleware(allowedTypes []string, allowJSONFallback bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contentType := r.Header.Get("Content-Type")

			// Content-Type header is required for these endpoints
			if contentType == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)

				invalidReqError := errors.NewOAuthError(
					errors.ErrInvalidRequest,
					"Content-Type header is required",
					"",
				)
				_ = json.NewEncoder(w).Encode(invalidReqError.ToResponseStruct())
				return
			}

			// Check prefix match to allow charset parameters
			var isValid bool
			for _, allowedType := range allowedTypes {
				if strings.HasPrefix(contentType, allowedType) {
					isValid = true
					break
				}
			}

			// Optionally accept JSON when configured
			if !isValid && allowJSONFallback && strings.HasPrefix(contentType, "application/json") {
				isValid = true
			}

			// Reject unsupported content types with a helpful message
			if !isValid {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)

				errorMsg := fmt.Sprintf("Content-Type must be one of: %s", strings.Join(allowedTypes, ", "))
				if allowJSONFallback && len(allowedTypes) > 0 {
					errorMsg = fmt.Sprintf("Content-Type must be %s (preferred) or application/json", allowedTypes[0])
				}

				invalidReqError := errors.NewOAuthError(
					errors.ErrInvalidRequest,
					errorMsg,
					"",
				)
				_ = json.NewEncoder(w).Encode(invalidReqError.ToResponseStruct())
				return
			}

			// Forward to the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// URLEncodedValidationMiddleware enforces application/x-www-form-urlencoded for RFC 7009 style endpoints
func URLEncodedValidationMiddleware(allowJSONFallback bool) func(http.Handler) http.Handler {
	return ContentTypeValidationMiddleware([]string{"application/x-www-form-urlencoded"}, allowJSONFallback)
}

// JSONValidationMiddleware enforces application/json for endpoints that only accept JSON
func JSONValidationMiddleware() func(http.Handler) http.Handler {
	return ContentTypeValidationMiddleware([]string{"application/json"}, false)
}

// AuthorizationMiddleware performs authorization using the provided Authorizer for a {resource, action} pair
// It requires a validated AuthInfo in context and returns 401 or 403 with OAuth style error when denied
func AuthorizationMiddleware(authorizer Authorizer, resource string, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read validated auth info placed in context by upstream auth middleware
			authInfo, ok := GetAuthInfo(r.Context())
			if !ok {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="No authentication info found"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Evaluate authorization decision for the requested resource and action
			err := authorizer.Authorize(authInfo, resource, action)
			if err != nil {
				// Return standardized insufficient_scope response
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope"`)
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(err.(errors.OAuthError).ToResponseStruct())

				// Optionally extract subject for audit or side effects
				_ = extractSubject(authInfo)
				return
			}

			// Optionally extract subject for audit or side effects
			_ = extractSubject(authInfo)

			// Authorized so continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}
