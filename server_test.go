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
	mcpServer := NewServer(":3000", Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	}, WithPathPrefix("/mcp"))

	// Create HTTP test server
	httpServer := httptest.NewServer(mcpServer.HTTPHandler())

	return mcpServer, httpServer
}

func TestNewServer(t *testing.T) {
	// Create server
	server := NewServer(":3000", Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	})

	// Verify object creation is successful
	assert.NotNil(t, server)
	assert.Equal(t, ":3000", server.config.Addr)
	assert.Equal(t, "/mcp", server.config.PathPrefix) // Default prefix
	assert.NotNil(t, server.httpHandler)
	assert.NotNil(t, server.mcpHandler)
	assert.NotNil(t, server.toolManager)
}

func TestServer_WithPathPrefix(t *testing.T) {
	// Create server with custom path prefix
	server := NewServer(":3000", Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	}, WithPathPrefix("/custom-api"))

	// Verify path prefix
	assert.Equal(t, "/custom-api", server.config.PathPrefix)
}

func TestServer_WithoutSession(t *testing.T) {
	// Create server with sessions disabled
	server := NewServer(":3000", Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	}, WithoutSession())

	// Verify server created successfully
	assert.NotNil(t, server)
	assert.NotNil(t, server.httpHandler)
}

func TestServer_RegisterTool(t *testing.T) {
	// Create server
	server := NewServer(":3000", Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	})

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
	server := NewServer(":3000", Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	})

	// Verify MCP handler
	assert.NotNil(t, server.MCPHandler())
	assert.Equal(t, server.mcpHandler, server.MCPHandler())
}
