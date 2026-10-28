// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestServer_GetServerInfo(t *testing.T) {
	serverName := "Test-Server-GetInfo"
	serverVersion := "2.0.0"

	server := NewServer(
		serverName,
		serverVersion,
		WithServerAddress(":3000"),
	)

	info := server.GetServerInfo()

	assert.Equal(t, serverName, info.Name)
	assert.Equal(t, serverVersion, info.Version)
}

func TestServer_UnregisterTools(t *testing.T) {
	server := NewServer("Test-Server", "1.0.0")

	// Create and register multiple tools
	tool1 := NewTool("test-tool-1", WithDescription("Test Tool 1"))
	tool2 := NewTool("test-tool-2", WithDescription("Test Tool 2"))
	tool3 := NewTool("test-tool-3", WithDescription("Test Tool 3"))

	handler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock result"), nil
	}

	server.RegisterTool(tool1, handler)
	server.RegisterTool(tool2, handler)
	server.RegisterTool(tool3, handler)

	// Verify all tools are registered
	tools := server.toolManager.getTools()
	assert.Len(t, tools, 3)

	// Test unregistering multiple existing tools
	err := server.UnregisterTools("test-tool-1", "test-tool-3")
	assert.NoError(t, err)

	tools = server.toolManager.getTools()
	assert.Len(t, tools, 1)

	// Check that only tool2 remains
	_, exists := server.toolManager.getTool("test-tool-1")
	assert.False(t, exists)
	_, exists = server.toolManager.getTool("test-tool-2")
	assert.True(t, exists)
	_, exists = server.toolManager.getTool("test-tool-3")
	assert.False(t, exists)

	// Test unregistering non-existent tools
	err = server.UnregisterTools("non-existent-1", "non-existent-2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "none of the specified tools were found")

	// Test unregistering with no names provided
	err = server.UnregisterTools()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tool names provided")

	// Test unregistering mix of existing and non-existent tools
	server.RegisterTool(tool1, handler)
	err = server.UnregisterTools("test-tool-1", "non-existent", "test-tool-2")
	assert.NoError(t, err) // Should succeed if at least one tool is unregistered

	tools = server.toolManager.getTools()
	assert.Len(t, tools, 0)
}

func TestServerInstructions(t *testing.T) {
	custom := "数据模型：Team -> Space -> Entry。\nURL：\"/teams/{team}\""

	testCases := []struct {
		name     string
		options  []ServerOption
		protocol string
		want     string
	}{
		{
			name:     "default",
			protocol: ProtocolVersion_2025_03_26,
			want:     "MCP server is ready",
		},
		{
			name:     "custom on 2024-11-05",
			options:  []ServerOption{WithInstructions(custom)},
			protocol: ProtocolVersion_2024_11_05,
			want:     custom,
		},
		{
			name:     "custom on 2025-03-26",
			options:  []ServerOption{WithInstructions(custom)},
			protocol: ProtocolVersion_2025_03_26,
			want:     custom,
		},
		{
			name:     "explicit empty is omitted",
			options:  []ServerOption{WithInstructions("")},
			protocol: ProtocolVersion_2025_03_26,
			want:     "",
		},
		{
			name:     "last option wins",
			options:  []ServerOption{WithInstructions("first"), WithInstructions(custom)},
			protocol: ProtocolVersion_2024_11_05,
			want:     custom,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer("Instructions-Server", "1.0.0", tc.options...)
			result := initializeThroughServer(t, server, tc.protocol)
			assert.Equal(t, tc.want, result.Instructions)
			assert.Equal(t, tc.protocol, result.ProtocolVersion)
			assert.Equal(t, "Instructions-Server", result.ServerInfo.Name)
			assertInstructionsJSON(t, marshalInitializeWireResult(t, result), tc.want)
		})
	}

	t.Run("servers are configured independently", func(t *testing.T) {
		serverA := NewServer("Server-A", "1.0.0", WithInstructions("alpha"))
		serverB := NewServer("Server-B", "1.0.0", WithInstructions("beta"))
		serverDefault := NewServer("Server-Default", "1.0.0")
		serverEmpty := NewServer("Server-Empty", "1.0.0", WithInstructions(""))

		resultA := initializeThroughServer(t, serverA, ProtocolVersion_2025_03_26)
		resultB := initializeThroughServer(t, serverB, ProtocolVersion_2024_11_05)
		resultDefault := initializeThroughServer(t, serverDefault, ProtocolVersion_2025_03_26)
		resultEmpty := initializeThroughServer(t, serverEmpty, ProtocolVersion_2024_11_05)
		resultAAgain := initializeThroughServer(t, serverA, ProtocolVersion_2024_11_05)

		assert.Equal(t, "alpha", resultA.Instructions)
		assert.Equal(t, "beta", resultB.Instructions)
		assert.Equal(t, "MCP server is ready", resultDefault.Instructions)
		assert.Empty(t, resultEmpty.Instructions)
		assert.Equal(t, "alpha", resultAAgain.Instructions)
		assertInstructionsJSON(t, marshalInitializeWireResult(t, resultEmpty), "")
	})
}

func TestServerInstructionsHTTP(t *testing.T) {
	custom := "数据模型：Team -> Space -> Entry。\nURL：\"/teams/{team}\""
	testCases := []struct {
		name    string
		options []ServerOption
		want    string
	}{
		{
			name:    "stateful custom",
			options: []ServerOption{WithInstructions(custom)},
			want:    custom,
		},
		{
			name:    "stateful empty",
			options: []ServerOption{WithInstructions("")},
			want:    "",
		},
		{
			name:    "stateless custom",
			options: []ServerOption{WithStatelessMode(true), WithInstructions(custom)},
			want:    custom,
		},
		{
			name:    "stateless empty",
			options: []ServerOption{WithStatelessMode(true), WithInstructions("")},
			want:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer("http-server", "1.0.0", tc.options...)
			body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"c","version":"1.0.0"},"capabilities":{}}}`)
			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			assertInstructionsJSON(t, decodeInitializeResult(t, rec.Body.Bytes()), tc.want)
		})
	}
}

func TestSSEAndStdioInstructions(t *testing.T) {
	custom := "数据模型：Team -> Space -> Entry。\nURL：\"/teams/{team}\""

	t.Run("sse", func(t *testing.T) {
		cases := []struct {
			name    string
			options []SSEOption
			want    string
		}{
			{name: "default", want: "MCP server is ready"},
			{name: "custom", options: []SSEOption{WithSSEInstructions(custom)}, want: custom},
			{name: "empty", options: []SSEOption{WithSSEInstructions("")}, want: ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				server := NewSSEServer("sse-server", "1.0.0", tc.options...)
				request := NewInitializeRequest(
					ProtocolVersion_2025_03_26,
					Implementation{Name: "Test-Client", Version: "1.0.0"},
					ClientCapabilities{},
				)
				resp, err := server.mcpHandler.handleRequest(context.Background(), request, newSession())
				require.NoError(t, err)
				result, ok := resp.(InitializeResult)
				require.True(t, ok, "sse initialize response type = %T", resp)
				assert.Equal(t, tc.want, result.Instructions)
				assert.Equal(t, "sse-server", result.ServerInfo.Name)
				assertInstructionsJSON(t, marshalInitializeWireResult(t, result), tc.want)
			})
		}
	})

	t.Run("stdio", func(t *testing.T) {
		cases := []struct {
			name    string
			options []StdioServerOption
			want    string
		}{
			{name: "default", want: "MCP server is ready"},
			{name: "custom", options: []StdioServerOption{WithStdioInstructions(custom)}, want: custom},
			{name: "empty", options: []StdioServerOption{WithStdioInstructions("")}, want: ""},
		}
		raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"c","version":"1.0.0"},"capabilities":{}}}`)
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				server := NewStdioServer("stdio-server", "1.0.0", tc.options...)
				session := &stdioSession{id: "stdio", data: make(map[string]interface{})}
				ctx := setSessionToContext(context.Background(), session)
				resp, err := server.internal.HandleRequest(ctx, raw)
				require.NoError(t, err)
				jsonResp, ok := resp.(*JSONRPCResponse)
				require.True(t, ok, "stdio initialize response type = %T", resp)
				result, ok := jsonResp.Result.(InitializeResult)
				require.True(t, ok, "stdio initialize result type = %T", jsonResp.Result)
				assert.Equal(t, tc.want, result.Instructions)
				assert.Equal(t, ProtocolVersion_2024_11_05, result.ProtocolVersion)
				payload, err := json.Marshal(jsonResp)
				require.NoError(t, err)
				assertInstructionsJSON(t, decodeInitializeResult(t, payload), tc.want)
			})
		}
	})
}

// initializeThroughServer runs initialize on the server's shared MCP handler.
func initializeThroughServer(t *testing.T, server *Server, protocolVersion string) InitializeResult {
	t.Helper()
	request := NewInitializeRequest(
		protocolVersion,
		Implementation{Name: "Test-Client", Version: "1.0.0"},
		ClientCapabilities{},
	)
	response, err := server.mcpHandler.handleRequest(context.Background(), request, newSession())
	require.NoError(t, err)
	result, ok := response.(InitializeResult)
	require.True(t, ok, "initialize response type = %T", response)
	return result
}

// marshalInitializeWireResult JSON-encodes an initialize result the same way transports do.
func marshalInitializeWireResult(t *testing.T, result InitializeResult) map[string]interface{} {
	t.Helper()
	payload, err := json.Marshal(JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Result:  result,
	})
	require.NoError(t, err)
	return decodeInitializeResult(t, payload)
}

func decodeInitializeResult(t *testing.T, payload []byte) map[string]interface{} {
	t.Helper()
	var wire map[string]interface{}
	require.NoError(t, json.Unmarshal(payload, &wire))
	result, ok := wire["result"].(map[string]interface{})
	require.True(t, ok)
	return result
}

func assertInstructionsJSON(t *testing.T, result map[string]interface{}, want string) {
	t.Helper()
	got, present := result["instructions"]
	if want == "" {
		assert.False(t, present)
		return
	}
	require.True(t, present)
	assert.Equal(t, want, got)
}
