// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	oauthErrors "trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

type ctxKey int
type ctxKeyScope int

const (
	ctxKeyAuthInfo ctxKey = iota
	ctxKeyAuthErr
	ctxKeyRequiredScope ctxKeyScope = 1
)

// WithAuthInfo writes authentication information into the context
func WithAuthInfo(ctx context.Context, info *AuthInfo) context.Context {
	// If no info provided, just return the original context
	if info == nil {
		return ctx
	}
	// Store authentication info in the context with a private key
	return context.WithValue(ctx, ctxKeyAuthInfo, info)
}

// GetAuthInfo retrieves authentication information from the context
func GetAuthInfo(ctx context.Context) (*AuthInfo, bool) {
	// Extract value from context
	v := ctx.Value(ctxKeyAuthInfo)
	// Return false if not set
	if v == nil {
		return nil, false
	}
	// Type assert to *AuthInfo
	info, ok := v.(*AuthInfo)
	return info, ok && info != nil
}

// WithAuthErr stores an authentication error into the context
func WithAuthErr(ctx context.Context, err error) context.Context {
	// No error means no need to wrap context
	if err == nil {
		return ctx
	}
	// Store error in context for later retrieval
	return context.WithValue(ctx, ctxKeyAuthErr, err)
}

// GetAuthErr retrieves an authentication error from the context
func GetAuthErr(ctx context.Context) error {
	// Extract error from context
	v := ctx.Value(ctxKeyAuthErr)
	// If not set, return nil (no error)
	if v == nil {
		return nil
	}
	// Type assert to error
	if err, ok := v.(error); ok {
		return err
	}
	// Fallback: return generic error if type is invalid
	return errors.New("auth err")
}

// WriteAuthChallenge writes a Bearer authentication challenge response header
// and sends the specified HTTP status code
func WriteAuthChallenge(w http.ResponseWriter, status int, code, desc, scope string) {
	// Build WWW-Authenticate header per RFC 6750
	val := fmt.Sprintf(`Bearer realm="mcp", error="%s", error_description="%s"`, code, desc)
	// Append required scope if present
	if scope != "" {
		val += fmt.Sprintf(`, scope="%s"`, scope)
	}
	// Add headers to prevent caching
	w.Header().Set("WWW-Authenticate", val)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	// Return error with HTTP status
	http.Error(w, http.StatusText(status), status)
}

// DetermineAuthError maps OAuth error types to HTTP status codes
// and standardized error code/description strings
func DetermineAuthError(err error) (int, string, string) {
	// Match specific OAuth error cases
	switch {
	case errors.Is(err, oauthErrors.ErrInsufficientScope):
		// Missing required scope: 403 Forbidden
		return http.StatusForbidden, "insufficient_scope", "Token lacks required scope"
	case errors.Is(err, oauthErrors.ErrInvalidRequest):
		// Invalid/malformed request: 400 Bad Request
		return http.StatusBadRequest, "invalid_request", "Missing or malformed authorization"
	case errors.Is(err, oauthErrors.ErrInvalidClient):
		// Invalid client authentication: 401 Unauthorized
		return http.StatusUnauthorized, "invalid_client", "Client authentication failed"
	case errors.Is(err, oauthErrors.ErrInvalidToken):
		// Invalid/expired token: 401 Unauthorized
		return http.StatusUnauthorized, "invalid_token", "The access token is invalid or expired"
	default:
		// Default: classify as invalid_token for security
		return http.StatusUnauthorized, "invalid_token", "Token verification failed"
	}
}

// WithRequiredScope stores the minimum required scope into the context
func WithRequiredScope(ctx context.Context, scope string) context.Context {
	// No scope provided, no modification
	if scope == "" {
		return ctx
	}
	// Store required scope for later validation
	return context.WithValue(ctx, ctxKeyRequiredScope, scope)
}

// GetRequiredScope retrieves the required scope from the context
func GetRequiredScope(ctx context.Context) (string, bool) {
	// Extract scope value
	v := ctx.Value(ctxKeyRequiredScope)
	// Assert type to string
	s, ok := v.(string)
	// Return non-empty scope if valid
	return s, ok && s != ""
}
