// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package server

import "net/url"

// AuthInfo holds information about a validated access token
// and is provided to request handlers
type AuthInfo struct {
	// Token is the original access token string
	Token string `json:"token"`

	// ClientID is the client identifier associated with this token
	ClientID string `json:"clientId"`

	// Subject is the principal (end-user or client) the token represents
	// Typically comes from the JWT 'sub' claim or introspection response
	Subject string `json:"subject,omitempty"`

	// Scopes are the permission scopes granted with this token
	Scopes []string `json:"scopes"`

	// ExpiresAt is the token expiration time in seconds since Unix epoch
	// If nil it means no expiration was provided
	ExpiresAt *int64 `json:"expiresAt,omitempty"`

	// Resource is the RFC 8707 resource server identifier for which this token is valid
	// If set it must match the resource identifier of the MCP server excluding any fragment
	// If nil it means no resource was provided
	Resource *url.URL `json:"resource,omitempty"`

	// Extra contains any additional data attached to this token
	// Used for passing custom claims or metadata alongside authentication info
	// If nil it means no extra data was provided
	Extra map[string]interface{} `json:"extra,omitempty"`
}
