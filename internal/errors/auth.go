// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package errors

import (
	"errors"
)

// OAuthErrorCode represents an OAuth 2.1 error code
type OAuthErrorCode error

// OAuthError represents a structured OAuth 2.1 error
type OAuthError struct {
	ErrorCode string
	Message   string
	ErrorURI  string
}

// OAuthErrorResponse represents the JSON response for OAuth errors
type OAuthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

// Standard OAuth error codes
var (
	ErrInvalidRequest          OAuthErrorCode = errors.New("invalid_request")
	ErrInvalidClient           OAuthErrorCode = errors.New("invalid_client")
	ErrInvalidGrant            OAuthErrorCode = errors.New("invalid_grant")
	ErrUnauthorizedClient      OAuthErrorCode = errors.New("unauthorized_client")
	ErrUnsupportedGrantType    OAuthErrorCode = errors.New("unsupported_grant_type")
	ErrInvalidScope            OAuthErrorCode = errors.New("invalid_scope")
	ErrAccessDenied            OAuthErrorCode = errors.New("access_denied")
	ErrServerError             OAuthErrorCode = errors.New("server_error")
	ErrTemporarilyUnavailable  OAuthErrorCode = errors.New("temporarily_unavailable")
	ErrUnsupportedResponseType OAuthErrorCode = errors.New("unsupported_response_type")
	ErrUnsupportedTokenType    OAuthErrorCode = errors.New("unsupported_token_type")
	ErrInvalidToken            OAuthErrorCode = errors.New("invalid_token")
	ErrMethodNotAllowed        OAuthErrorCode = errors.New("method_not_allowed")
	ErrTooManyRequests         OAuthErrorCode = errors.New("too_many_requests")
	ErrInvalidClientMetadata   OAuthErrorCode = errors.New("invalid_client_metadata")
	ErrInsufficientScope       OAuthErrorCode = errors.New("insufficient_scope")
)

// OAuthErrorMapping maps error strings to their corresponding OAuthErrorCode
// This replaces the need for large switch statements when parsing error responses
var OAuthErrorMapping = map[string]OAuthErrorCode{
	"invalid_request":           ErrInvalidRequest,
	"invalid_client":            ErrInvalidClient,
	"invalid_grant":             ErrInvalidGrant,
	"unauthorized_client":       ErrUnauthorizedClient,
	"unsupported_grant_type":    ErrUnsupportedGrantType,
	"invalid_scope":             ErrInvalidScope,
	"access_denied":             ErrAccessDenied,
	"server_error":              ErrServerError,
	"temporarily_unavailable":   ErrTemporarilyUnavailable,
	"unsupported_response_type": ErrUnsupportedResponseType,
	"unsupported_token_type":    ErrUnsupportedTokenType,
	"invalid_token":             ErrInvalidToken,
	"method_not_allowed":        ErrMethodNotAllowed,
	"too_many_requests":         ErrTooManyRequests,
	"invalid_client_metadata":   ErrInvalidClientMetadata,
	"insufficient_scope":        ErrInsufficientScope,
}

// NewOAuthError creates a new OAuthError
func NewOAuthError(errCode OAuthErrorCode, message string, uri string) OAuthError {
	err := OAuthError{
		ErrorCode: errCode.Error(),
	}
	if uri != "" {
		err.ErrorURI = uri
	}
	if message != "" {
		err.Message = message
	}
	return err
}

// ToResponseStruct converts OAuthError into OAuthErrorResponse for JSON encoding
func (o OAuthError) ToResponseStruct() *OAuthErrorResponse {
	return &OAuthErrorResponse{
		Error:            o.ErrorCode,
		ErrorDescription: o.Message,
		ErrorURI:         o.ErrorURI,
	}
}

// Error implements the error interface
func (o OAuthError) Error() string {
	return o.ErrorCode
}
