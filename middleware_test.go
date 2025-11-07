// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMiddlewareBasic tests basic middleware functionality
func TestMiddlewareBasic(t *testing.T) {
	handler := newMCPHandler()

	// Track middleware execution
	var executionOrder []string

	// Register middleware
	middleware1 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionOrder = append(executionOrder, "middleware1-before")
			result, err := next(ctx, req)
			executionOrder = append(executionOrder, "middleware1-after")
			return result, err
		}
	}

	middleware2 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionOrder = append(executionOrder, "middleware2-before")
			result, err := next(ctx, req)
			executionOrder = append(executionOrder, "middleware2-after")
			return result, err
		}
	}

	handler.use(middleware1)
	handler.use(middleware2)

	// Create a ping request
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{
			Method: MethodPing,
		},
	}

	// Create context with session
	ctx := context.Background()
	session := newSession()
	ctx = withClientSession(ctx, session)

	// Execute request
	_, err := handler.handleRequest(ctx, req, session)
	require.NoError(t, err)

	// Verify execution order (onion model)
	expectedOrder := []string{
		"middleware1-before",
		"middleware2-before",
		"middleware2-after",
		"middleware1-after",
	}
	assert.Equal(t, expectedOrder, executionOrder)
}

// TestMiddlewareSessionAccess tests accessing session from context in middleware
func TestMiddlewareSessionAccess(t *testing.T) {
	handler := newMCPHandler()

	var capturedSessionID string

	// Middleware that accesses session from context
	sessionMiddleware := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			session := ClientSessionFromContext(ctx)
			if session != nil {
				capturedSessionID = session.GetID()
			}
			return next(ctx, req)
		}
	}

	handler.use(sessionMiddleware)

	// Create request
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{
			Method: MethodPing,
		},
	}

	// Create context with session
	ctx := context.Background()
	session := newSession()
	expectedSessionID := session.GetID()
	ctx = withClientSession(ctx, session)

	// Execute request
	_, err := handler.handleRequest(ctx, req, session)
	require.NoError(t, err)

	// Verify session was accessible in middleware
	assert.Equal(t, expectedSessionID, capturedSessionID)
}

// TestMiddlewareErrorHandling tests error handling in middleware
func TestMiddlewareErrorHandling(t *testing.T) {
	handler := newMCPHandler()

	var errorResponseCaptured bool

	// Middleware that detects error responses
	errorMiddleware := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			result, err := next(ctx, req)
			// Check if result is an error response
			if errResp, ok := result.(*JSONRPCError); ok {
				errorResponseCaptured = true
				_ = errResp // Use errResp to avoid unused variable warning
			}
			return result, err
		}
	}

	handler.use(errorMiddleware)

	// Register a tool that returns an error
	toolManager := newToolManager()
	toolManager.registerTool(
		&Tool{Name: "error-tool"},
		func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return nil, errors.New("test error")
		},
	)
	handler.toolManager = toolManager

	// Create tool call request
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{
			Method: MethodToolsCall,
		},
		Params: map[string]interface{}{
			"name": "error-tool",
		},
	}

	// Create context with session
	ctx := context.Background()
	session := newSession()
	ctx = withClientSession(ctx, session)

	// Execute request
	result, err := handler.handleRequest(ctx, req, session)

	// Should not return error (errors are converted to JSON-RPC error responses)
	assert.NoError(t, err)
	// Should return an error response
	assert.IsType(t, &JSONRPCError{}, result)
	// Middleware should have detected the error response
	assert.True(t, errorResponseCaptured)
}

// TestMiddlewareStateful tests stateful middleware (using closures)
func TestMiddlewareStateful(t *testing.T) {
	handler := newMCPHandler()

	callCount := 0

	// Stateful middleware using closure
	counterMiddleware := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			callCount++
			return next(ctx, req)
		}
	}

	handler.use(counterMiddleware)

	// Create request
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{
			Method: MethodPing,
		},
	}

	// Create context with session
	ctx := context.Background()
	session := newSession()
	ctx = withClientSession(ctx, session)

	// Execute request multiple times
	for i := 0; i < 3; i++ {
		_, err := handler.handleRequest(ctx, req, session)
		require.NoError(t, err)
	}

	// Verify counter was incremented
	assert.Equal(t, 3, callCount)
}

// TestMiddlewareNoSession tests middleware without session in context
func TestMiddlewareNoSession(t *testing.T) {
	handler := newMCPHandler()

	var sessionFound bool

	// Middleware that checks for session
	sessionCheckMiddleware := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			session := ClientSessionFromContext(ctx)
			sessionFound = (session != nil)
			return next(ctx, req)
		}
	}

	handler.use(sessionCheckMiddleware)

	// Create request
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{
			Method: MethodPing,
		},
	}

	// Create context WITHOUT session
	ctx := context.Background()

	// Execute request (pass nil session)
	_, err := handler.handleRequest(ctx, req, nil)
	require.NoError(t, err)

	// Verify no session was found
	assert.False(t, sessionFound)
}

// TestServerWithMiddlewareOption tests the WithMiddleware option
func TestServerWithMiddlewareOption(t *testing.T) {
	var middlewareCalled bool

	testMiddleware := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			middlewareCalled = true
			return next(ctx, req)
		}
	}

	server := NewServer("test", "1.0.0",
		WithMiddleware(testMiddleware))

	// Verify middleware was registered
	assert.Len(t, server.mcpHandler.middlewares, 1)

	// Create request
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{
			Method: MethodPing,
		},
	}

	ctx := context.Background()
	session := newSession()
	ctx = withClientSession(ctx, session)

	// Execute request via handler
	_, err := server.mcpHandler.handleRequest(ctx, req, session)
	require.NoError(t, err)

	// Verify middleware was called
	assert.True(t, middlewareCalled)
}

// TestMiddlewareChaining tests multiple middlewares chained together
func TestMiddlewareChaining(t *testing.T) {
	handler := newMCPHandler()

	var executionLog []string

	// Create three middlewares
	for i := 1; i <= 3; i++ {
		i := i // Capture loop variable
		middleware := func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
				executionLog = append(executionLog, string(rune('A'+i-1))+"-before")
				result, err := next(ctx, req)
				executionLog = append(executionLog, string(rune('A'+i-1))+"-after")
				return result, err
			}
		}
		handler.use(middleware)
	}

	// Create request
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{
			Method: MethodPing,
		},
	}

	ctx := context.Background()
	session := newSession()
	ctx = withClientSession(ctx, session)

	// Execute request
	_, err := handler.handleRequest(ctx, req, session)
	require.NoError(t, err)

	// Verify execution order (onion model: A wraps B wraps C)
	expectedLog := []string{
		"A-before",
		"B-before",
		"C-before",
		"C-after",
		"B-after",
		"A-after",
	}
	assert.Equal(t, expectedLog, executionLog)
}

func TestWithMiddlewareOption(t *testing.T) {
	var executionLog []string

	mw1 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionLog = append(executionLog, "mw1")
			return next(ctx, req)
		}
	}

	mw2 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionLog = append(executionLog, "mw2")
			return next(ctx, req)
		}
	}

	mw3 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionLog = append(executionLog, "mw3")
			return next(ctx, req)
		}
	}

	// Test: Configure middlewares via WithMiddleware option
	server := NewServer("test", "1.0.0",
		WithMiddleware(mw1, mw2, mw3))

	// Verify middlewares are registered and execute in correct order
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{Method: MethodPing},
	}

	ctx := withClientSession(context.Background(), newSession())
	session := ClientSessionFromContext(ctx)
	_, err := server.mcpHandler.handleRequest(ctx, req, session)

	require.NoError(t, err)
	assert.Equal(t, []string{"mw1", "mw2", "mw3"}, executionLog)
}

func TestMultipleWithMiddlewareCalls(t *testing.T) {
	var executionLog []string

	mw1 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionLog = append(executionLog, "mw1")
			return next(ctx, req)
		}
	}

	mw2 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionLog = append(executionLog, "mw2")
			return next(ctx, req)
		}
	}

	mw3 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionLog = append(executionLog, "mw3")
			return next(ctx, req)
		}
	}

	// Configure via multiple WithMiddleware calls
	server := NewServer("test", "1.0.0",
		WithMiddleware(mw1, mw2),
		WithMiddleware(mw3))

	// Verify execution order
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{Method: MethodPing},
	}

	ctx := withClientSession(context.Background(), newSession())
	session := ClientSessionFromContext(ctx)
	_, err := server.mcpHandler.handleRequest(ctx, req, session)

	require.NoError(t, err)
	assert.Equal(t, []string{
		"mw1",
		"mw2",
		"mw3",
	}, executionLog)
}

func TestWithSSEMiddlewareOption(t *testing.T) {
	var executionLog []string

	mw1 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionLog = append(executionLog, "sse-mw1")
			return next(ctx, req)
		}
	}

	mw2 := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			executionLog = append(executionLog, "sse-mw2")
			return next(ctx, req)
		}
	}

	// Test: Configure middlewares for SSEServer via WithSSEMiddleware option
	server := NewSSEServer("test", "1.0.0",
		WithSSEMiddleware(mw1, mw2))

	// Verify middlewares are registered and execute in correct order
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Request: Request{Method: MethodPing},
	}

	ctx := withClientSession(context.Background(), newSession())
	session := ClientSessionFromContext(ctx)
	_, err := server.mcpHandler.handleRequest(ctx, req, session)

	require.NoError(t, err)
	assert.Equal(t, []string{"sse-mw1", "sse-mw2"}, executionLog)
}
