// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package client

import (
	"context"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

// ctxKey defines a private type for context keys to avoid collisions
type ctxKey int

const (
	// ctxKeyClientAuthInfo is the context key used to store ClientAuthInfo
	ctxKeyClientAuthInfo ctxKey = iota
	// ctxKeyClientAuthErr is the context key used to store authentication errors
	ctxKeyClientAuthErr
)

// ClientAuthInfo holds OAuth client authentication details
type ClientAuthInfo struct {
	AccessToken  string
	RefreshToken *string
	ExpiresAt    *time.Time
	Scopes       []string
	Extra        map[string]interface{}
}

// WithAuthInfo stores authentication information in the context
func WithAuthInfo(ctx context.Context, info *ClientAuthInfo) context.Context {
	if info == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyClientAuthInfo, info)
}

// GetAuthInfo retrieves authentication information from the context
func GetAuthInfo(ctx context.Context) (*ClientAuthInfo, bool) {
	v := ctx.Value(ctxKeyClientAuthInfo)
	if v == nil {
		return nil, false
	}
	info, ok := v.(*ClientAuthInfo)
	return info, ok && info != nil
}

// WithAuthErr stores an authentication error in the context
func WithAuthErr(ctx context.Context, err error) context.Context {
	if err == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyClientAuthErr, err)
}

// ConvertTokensToAuthInfo converts OAuth tokens into ClientAuthInfo
func ConvertTokensToAuthInfo(tokens *auth.OAuthTokens) *ClientAuthInfo {
	if tokens == nil || tokens.AccessToken == "" {
		return nil
	}

	authInfo := &ClientAuthInfo{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Scopes:       parseTokenScopes(tokens),
		Extra:        make(map[string]interface{}),
	}

	if tokens.ExpiresIn != nil {
		expiresAt := time.Now().Add(time.Duration(*tokens.ExpiresIn) * time.Second)
		authInfo.ExpiresAt = &expiresAt
	}

	return authInfo
}

// IsTokenExpired checks if the token is expired or near expiry
func IsTokenExpired(authInfo *ClientAuthInfo) bool {
	if authInfo == nil {
		return true // nil means expired
	}
	if authInfo.ExpiresAt == nil {
		return false // no expiry means never expired
	}
	// expire if within 30 seconds of expiry
	return !authInfo.ExpiresAt.After(time.Now().Add(30 * time.Second))
}

// parseTokenScopes extracts scopes from tokens (currently placeholder)
func parseTokenScopes(tokens *auth.OAuthTokens) []string {
	return []string{}
}
