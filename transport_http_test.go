// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockHTTPTransport is a mock implementation of the httpTransport interface
type MockHTTPTransport struct {
	sendRequestFunc      func(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error)
	sendNotificationFunc func(ctx context.Context, notification *JSONRPCNotification) error
	closeFunc            func() error
	getSessionIDFunc     func() string
	terminateSessionFunc func(ctx context.Context) error
}

func (m *MockHTTPTransport) SendRequest(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	if m.sendRequestFunc != nil {
		return m.sendRequestFunc(ctx, req)
	}
	return nil, nil
}

func (m *MockHTTPTransport) SendNotification(ctx context.Context, notification *JSONRPCNotification) error {
	if m.sendNotificationFunc != nil {
		return m.sendNotificationFunc(ctx, notification)
	}
	return nil
}

func (m *MockHTTPTransport) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *MockHTTPTransport) GetSessionID() string {
	if m.getSessionIDFunc != nil {
		return m.getSessionIDFunc()
	}
	return ""
}

func (m *MockHTTPTransport) TerminateSession(ctx context.Context) error {
	if m.terminateSessionFunc != nil {
		return m.terminateSessionFunc(ctx)
	}
	return nil
}

func TestMockHTTPTransport(t *testing.T) {
	// Create mock transport with custom behaviors
	mockTransport := &MockHTTPTransport{
		sendRequestFunc: func(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
			// Verify request
			assert.Equal(t, "test.method", req.Method)

			// Return mock response
			return &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Result:  "test result",
			}, nil
		},
		sendNotificationFunc: func(ctx context.Context, notification *JSONRPCNotification) error {
			// Verify notification
			assert.Equal(t, "test.notification", notification.Method)
			return nil
		},
		closeFunc: func() error {
			return nil
		},
		getSessionIDFunc: func() string {
			return "test-session-id"
		},
		terminateSessionFunc: func(ctx context.Context) error {
			return nil
		},
	}

	// Test sendRequest
	ctx := context.Background()
	req := newJSONRPCRequest(1, "test.method", nil)
	resp, err := mockTransport.SendRequest(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test result", resp.Result)

	// Test sendNotification
	notification := NewJSONRPCNotificationFromMap("test.notification", nil)
	err = mockTransport.SendNotification(ctx, notification)
	assert.NoError(t, err)

	// Test getSessionID
	sessionID := mockTransport.GetSessionID()
	assert.Equal(t, "test-session-id", sessionID)

	// Test terminateSession
	err = mockTransport.TerminateSession(ctx)
	assert.NoError(t, err)

	// Test close
	err = mockTransport.Close()
	assert.NoError(t, err)
}

func TestNewClientTransport(t *testing.T) {
	// Test creating transport object
	serverURL, _ := url.Parse("http://localhost:3000/mcp")
	config := newDefaultTransportConfig()
	config.serverURL = serverURL
	transport := newStreamableHTTPClientTransport(config)

	// Verify object was created successfully
	assert.NotNil(t, transport)

	// Verify basic properties
	assert.Equal(t, "", transport.sessionID) // Initial session ID should be empty
}

// Test HTTP headers transport option
func TestTransportHTTPHeaders(t *testing.T) {
	// Create test headers
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer test-token")
	headers.Set("User-Agent", "TestTransport/1.0")
	headers.Add("Accept", "application/json")
	headers.Add("Accept", "text/html")

	// Create transport with headers
	serverURL, _ := url.Parse("http://localhost:3000/mcp")
	config := newDefaultTransportConfig()
	config.serverURL = serverURL

	transport := newStreamableHTTPClientTransport(config, withTransportHTTPHeaders(headers))

	// Verify headers are set correctly
	assert.NotNil(t, transport.httpHeaders)
	assert.Equal(t, "Bearer test-token", transport.httpHeaders.Get("Authorization"))
	assert.Equal(t, "TestTransport/1.0", transport.httpHeaders.Get("User-Agent"))

	// Verify multi-value headers
	acceptValues := transport.httpHeaders["Accept"]
	assert.Len(t, acceptValues, 2)
	assert.Contains(t, acceptValues, "application/json")
	assert.Contains(t, acceptValues, "text/html")
}

// Test HTTP headers with other transport options
func TestTransportHTTPHeadersWithOtherOptions(t *testing.T) {
	// Create headers
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer combo-token")

	// Create transport with headers and other options
	serverURL, _ := url.Parse("http://localhost:3000/mcp")
	config := newDefaultTransportConfig()
	config.serverURL = serverURL

	transport := newStreamableHTTPClientTransport(
		config,
		withTransportHTTPHeaders(headers),
		withClientTransportGetSSEEnabled(true),
		withClientTransportPath("/custom"),
	)

	// Verify headers are set
	assert.Equal(t, "Bearer combo-token", transport.httpHeaders.Get("Authorization"))

	// Verify other options are also applied
	assert.True(t, transport.enableGetSSE)
	assert.Equal(t, "/custom", transport.path)
}

// Test HTTP context functions in server transport
func TestServerTransportHTTPContextFuncs(t *testing.T) {
	// Define context keys
	type contextKey string
	const testKey contextKey = "test_key"

	// Define HTTP context function
	testContextFunc := func(ctx context.Context, r *http.Request) context.Context {
		if testHeader := r.Header.Get("X-Test-Header"); testHeader != "" {
			return context.WithValue(ctx, testKey, testHeader)
		}
		return ctx
	}

	// Create server handler with context functions
	handler := &httpServerHandler{
		httpContextFuncs: []HTTPContextFunc{testContextFunc},
	}

	// Verify context functions are set
	assert.NotNil(t, handler.httpContextFuncs)
	assert.Len(t, handler.httpContextFuncs, 1)
}

// Test multiple HTTP context functions in server transport
func TestServerTransportMultipleHTTPContextFuncs(t *testing.T) {
	// Define context functions
	func1 := func(ctx context.Context, r *http.Request) context.Context {
		return context.WithValue(ctx, "key1", "value1")
	}
	func2 := func(ctx context.Context, r *http.Request) context.Context {
		return context.WithValue(ctx, "key2", "value2")
	}
	func3 := func(ctx context.Context, r *http.Request) context.Context {
		return context.WithValue(ctx, "key3", "value3")
	}

	// Create server handler with multiple context functions
	handler := &httpServerHandler{}

	// Apply context functions using the configuration function
	withTransportHTTPContextFuncs([]HTTPContextFunc{func1, func2, func3})(handler)

	// Verify all context functions are set
	assert.NotNil(t, handler.httpContextFuncs)
	assert.Len(t, handler.httpContextFuncs, 3)
}
