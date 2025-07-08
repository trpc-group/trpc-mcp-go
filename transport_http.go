// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// HTTPReqHandler is a custom HTTP request handler interface.
type HTTPReqHandler interface {
	Handle(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error)
}

// defaultHTTPReqHandler is the default implementation of HTTPReqHandler
type defaultHTTPReqHandler struct{}

func (h *defaultHTTPReqHandler) Handle(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req.WithContext(ctx))
}

// NewDefaultHTTPReqHandler creates a new default HTTP request handler
func NewDefaultHTTPReqHandler() HTTPReqHandler {
	return &defaultHTTPReqHandler{}
}

// Common errors
var (
	// ErrStreamingNotSupported is returned when the HTTP implementation doesn't support streaming
	ErrStreamingNotSupported = errors.New("streaming is not supported by the underlying HTTP implementation")

	// ErrMissingResultField is returned when a response is missing the required result field
	ErrMissingResultField = errors.New("response missing result field")

	// ErrInvalidRequestBody is returned when the request body cannot be parsed
	ErrInvalidRequestBody = errors.New("invalid request body")

	// ErrInvalidContentType is returned when the content type is not supported
	ErrInvalidContentType = errors.New("invalid content type")

	// ErrSessionNotFound is returned when a requested session cannot be found
	ErrSessionNotFound = errors.New("session not found")

	// ErrInvalidSessionID is returned when a session ID is invalid
	ErrInvalidSessionID = errors.New("invalid session ID")

	// ErrSessionExpired is returned when attempting to use an expired session
	ErrSessionExpired = errors.New("session expired")

	// ErrSSENotSupported is returned when SSE is not supported or enabled
	ErrSSENotSupported = errors.New("SSE not supported")

	// ErrInvalidEventFormat is returned when an SSE event has invalid format
	ErrInvalidEventFormat = errors.New("invalid SSE event format")

	// ErrResponseSerialization is returned when response serialization fails
	ErrResponseSerialization = errors.New("failed to serialize response")

	// ErrNotificationSerialization is returned when notification serialization fails
	ErrNotificationSerialization = errors.New("failed to serialize notification")

	// ErrRequestSerialization is returned when request serialization fails
	ErrRequestSerialization = errors.New("failed to serialize request")

	// ErrHTTPRequestCreation is returned when HTTP request creation fails
	ErrHTTPRequestCreation = errors.New("failed to create HTTP request")

	// ErrHTTPRequestFailed is returned when an HTTP request fails
	ErrHTTPRequestFailed = errors.New("HTTP request failed")

	// ErrResponseParsing is returned when response parsing fails
	ErrResponseParsing = errors.New("failed to parse response")

	// ErrInvalidResponseType is returned when response type is invalid
	ErrInvalidResponseType = errors.New("invalid response type")
)

// transport represents the interface for the communication transport layer
type transport interface {
	// Start the transport
	start(ctx context.Context) error

	// Send a request and wait for a response
	sendRequest(ctx context.Context, req *JSONRPCRequest) (*json.RawMessage, error)

	// Send a notification (no response expected)
	sendNotification(ctx context.Context, notification *JSONRPCNotification) error

	// Send a response
	sendResponse(ctx context.Context, resp *JSONRPCResponse) error

	// Close the transport
	close() error
}

// httpTransport represents the interface for HTTP transport
type httpTransport interface {
	transport

	// Get the session ID
	getSessionID() string

	// Set the session ID
	setSessionID(sessionID string)

	// Terminate the session
	terminateSession(ctx context.Context) error
}
