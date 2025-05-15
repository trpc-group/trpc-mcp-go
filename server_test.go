package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
		func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return NewTextResult("Mock Response"), nil
		},
		WithDescription("Mock Tool"),
	)
	err := server.RegisterTool(tool)

	// Verify registration result
	assert.NoError(t, err)
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
