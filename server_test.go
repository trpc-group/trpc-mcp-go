// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Create test server
func createTestServer() (*Server, *httptest.Server) {
	// Create MCP server
	mcpServer := NewServer(
		"Test-Server",              // Server name
		"1.0.0",                    // Server version
		WithServerAddress(":3000"), // Server address
		WithServerPath("/mcp"),     // Set API path
	)

	// Create HTTP test server
	httpServer := httptest.NewServer(mcpServer.HTTPHandler())

	return mcpServer, httpServer
}

func TestNewServer(t *testing.T) {
	// Create server
	server := NewServer(
		"Test-Server",              // Server name
		"1.0.0",                    // Server version
		WithServerAddress(":3000"), // Server address
	)

	// Verify object creation is successful
	assert.NotNil(t, server)
	assert.Equal(t, ":3000", server.config.addr)
	assert.Equal(t, "/mcp", server.config.path) // Default prefix
	assert.NotNil(t, server.httpHandler)
	assert.NotNil(t, server.mcpHandler)
	assert.NotNil(t, server.toolManager)
}

func TestServer_WithPathPrefix(t *testing.T) {
	// Create server with custom path prefix
	server := NewServer(
		"Test-Server",                 // Server name
		"1.0.0",                       // Server version
		WithServerAddress(":3000"),    // Server address
		WithServerPath("/custom-api"), // Custom path prefix
	)

	// Verify path prefix
	assert.Equal(t, "/custom-api", server.config.path)
}

func TestServer_WithoutSession(t *testing.T) {
	// Create server with sessions disabled
	server := NewServer(
		"Test-Server",              // Server name
		"1.0.0",                    // Server version
		WithServerAddress(":3000"), // Server address
		WithoutSession(),           // Disable sessions
	)

	// Verify server created successfully
	assert.NotNil(t, server)
	assert.NotNil(t, server.httpHandler)
}

func TestServer_RegisterTool(t *testing.T) {
	// Create server
	server := NewServer(
		"Test-Server",              // Server name
		"1.0.0",                    // Server version
		WithServerAddress(":3000"), // Server address
	)

	// Register tool
	tool := NewTool("mock-tool",
		WithDescription("Mock Tool"),
	)
	server.RegisterTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock Response"), nil
	})

	// Verify tool was registered
	registeredTool, exists := server.toolManager.getTool("mock-tool")
	assert.True(t, exists)
	assert.NotNil(t, registeredTool)
	assert.Equal(t, "mock-tool", registeredTool.Name)
	assert.Equal(t, "Mock Tool", registeredTool.Description)
}

func TestServer_HTTPHandler(t *testing.T) {
	// Create server
	server, httpServer := createTestServer()
	defer httpServer.Close()

	// Verify HTTP handler
	assert.NotNil(t, server.HTTPHandler())
	assert.Equal(t, server.httpHandler, server.HTTPHandler())

	// Send HTTP request
	resp, err := http.Get(httpServer.URL + "/mcp")

	// Verify response
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode) // Server now returns 400 Bad Request instead of 405 Method Not Allowed
}

func TestServer_MCPHandler(t *testing.T) {
	// Create server
	server := NewServer(
		"Test-Server",              // Server name
		"1.0.0",                    // Server version
		WithServerAddress(":3000"), // Server address
	)

	// Verify MCP handler
	assert.NotNil(t, server.MCPHandler())
	assert.Equal(t, server.mcpHandler, server.MCPHandler())
}

// Test HTTP context function registration
func TestServer_WithHTTPContextFunc(t *testing.T) {
	// Define context keys
	type contextKey string
	const authTokenKey contextKey = "auth_token"

	// Define HTTP context function
	extractAuthToken := func(ctx context.Context, r *http.Request) context.Context {
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			return context.WithValue(ctx, authTokenKey, authHeader)
		}
		return ctx
	}

	// Create server with HTTP context function
	server := NewServer(
		"Test-Server",
		"1.0.0",
		WithServerAddress(":3000"),
		WithHTTPContextFunc(extractAuthToken),
	)

	// Verify server created successfully
	assert.NotNil(t, server)
	assert.NotNil(t, server.config.httpContextFuncs)
	assert.Len(t, server.config.httpContextFuncs, 1)
}

// Test multiple HTTP context functions
func TestServer_WithMultipleHTTPContextFuncs(t *testing.T) {
	// Define context keys
	type contextKey string
	const (
		authTokenKey contextKey = "auth_token"
		userAgentKey contextKey = "user_agent"
	)

	// Define HTTP context functions
	extractAuthToken := func(ctx context.Context, r *http.Request) context.Context {
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			return context.WithValue(ctx, authTokenKey, authHeader)
		}
		return ctx
	}

	extractUserAgent := func(ctx context.Context, r *http.Request) context.Context {
		if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
			return context.WithValue(ctx, userAgentKey, userAgent)
		}
		return ctx
	}

	// Create server with multiple HTTP context functions
	server := NewServer(
		"Test-Server",
		"1.0.0",
		WithServerAddress(":3000"),
		WithHTTPContextFunc(extractAuthToken),
		WithHTTPContextFunc(extractUserAgent),
	)

	// Verify server created successfully
	assert.NotNil(t, server)
	assert.NotNil(t, server.config.httpContextFuncs)
	assert.Len(t, server.config.httpContextFuncs, 2)
}

// Test tool handler accessing headers via context
func TestServer_ToolHandlerWithHeaders(t *testing.T) {
	// Define context keys
	type contextKey string
	const authTokenKey contextKey = "auth_token"

	// Define HTTP context function
	extractAuthToken := func(ctx context.Context, r *http.Request) context.Context {
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			return context.WithValue(ctx, authTokenKey, authHeader)
		}
		return ctx
	}

	// Create server with HTTP context function
	server := NewServer(
		"Test-Server",
		"1.0.0",
		WithServerPath("/mcp"),
		WithHTTPContextFunc(extractAuthToken),
	)

	// Define tool handler that accesses headers
	headerTool := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		authToken, _ := ctx.Value(authTokenKey).(string)
		return NewTextResult("Auth: " + authToken), nil
	}

	// Register tool
	tool := NewTool("header-tool", WithDescription("Tool that uses headers"))
	server.RegisterTool(tool, headerTool)

	// Create HTTP test server
	httpServer := httptest.NewServer(server.HTTPHandler())
	defer httpServer.Close()

	// Create client
	client, err := NewClient(httpServer.URL+"/mcp", Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	})
	require.NoError(t, err)
	defer client.Close()

	// Initialize client
	ctx := context.Background()
	_, err = client.Initialize(ctx, &InitializeRequest{})
	require.NoError(t, err)

	// Call tool without headers
	result, err := client.CallTool(ctx, &CallToolRequest{
		Params: CallToolParams{
			Name:      "header-tool",
			Arguments: map[string]interface{}{},
		},
	})
	require.NoError(t, err)
	textContent, ok := result.Content[0].(TextContent)
	require.True(t, ok)
	assert.Equal(t, "Auth: ", textContent.Text) // Empty auth token

	// Create client with headers
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer test-token")

	clientWithHeaders, err := NewClient(httpServer.URL+"/mcp", Implementation{
		Name:    "Test-Client-With-Headers",
		Version: "1.0.0",
	}, WithHTTPHeaders(headers))
	require.NoError(t, err)
	defer clientWithHeaders.Close()

	// Initialize client with headers
	_, err = clientWithHeaders.Initialize(ctx, &InitializeRequest{})
	require.NoError(t, err)

	// Call tool with headers
	result, err = clientWithHeaders.CallTool(ctx, &CallToolRequest{
		Params: CallToolParams{
			Name:      "header-tool",
			Arguments: map[string]interface{}{},
		},
	})
	require.NoError(t, err)
	textContent, ok = result.Content[0].(TextContent)
	require.True(t, ok)
	assert.Equal(t, "Auth: Bearer test-token", textContent.Text) // Header extracted successfully
}
