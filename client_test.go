package mcp

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTool is a test tool for testing
type TestTool struct {
	*Tool
}

// NewTestTool creates a new test tool
func NewTestTool() *Tool {
	return NewTool("test-tool",
		func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			name := "World"
			if nameArg, ok := req.Params.Arguments["name"]; ok {
				if nameStr, ok := nameArg.(string); ok && nameStr != "" {
					name = nameStr
				}
			}

			return NewTextResult("Hello, " + name + "!"), nil
		},
		WithDescription("Test Tool"),
		WithString("name",
			Description("Name to greet"),
		),
	)
}

// Create test environment including server and client
func setupTestEnvironment(t *testing.T) (*Client, *httptest.Server, func()) {
	// Create MCP server
	mcpServer := NewServer(
		"Test-Server",          // Server name
		"1.0.0",                // Server version
		WithPathPrefix("/mcp"), // Set API path
	)

	// Register test tool
	tool := NewTestTool()
	err := mcpServer.RegisterTool(tool)
	require.NoError(t, err)

	// Create HTTP test server
	httpServer := httptest.NewServer(mcpServer.HTTPHandler())

	// Create client
	client, err := NewClient(httpServer.URL+"/mcp", Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	})
	require.NoError(t, err)

	// Return cleanup function
	cleanup := func() {
		client.Close()
		httpServer.Close()
	}

	return client, httpServer, cleanup
}

func TestNewClient(t *testing.T) {
	// Test client creation
	client, err := NewClient("http://localhost:3000/mcp", Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	})

	// Verify successful object creation
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "Test-Client", client.clientInfo.Name)
	assert.Equal(t, "1.0.0", client.clientInfo.Version)
	assert.Equal(t, ProtocolVersion_2025_03_26, client.protocolVersion) // Update to current default version.
	assert.False(t, client.initialized)
}

func TestClient_WithProtocolVersion(t *testing.T) {
	// Test creating client with custom protocol version
	client, err := NewClient("http://localhost:3000/mcp", Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	}, WithProtocolVersion(ProtocolVersion_2024_11_05))

	// Verify protocol version
	assert.NoError(t, err)
	assert.Equal(t, ProtocolVersion_2024_11_05, client.protocolVersion)
}

func TestClient_Initialize(t *testing.T) {
	// Create test environment
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test initialization
	ctx := context.Background()
	resp, err := client.Initialize(ctx)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Test-Server", resp.ServerInfo.Name)
	assert.Equal(t, "1.0.0", resp.ServerInfo.Version)
	assert.Equal(t, ProtocolVersion_2024_11_05, resp.ProtocolVersion)
	assert.NotNil(t, resp.Capabilities)

	// Verify client state
	assert.True(t, client.initialized)
	assert.NotEmpty(t, client.GetSessionID())
}

func TestClient_ListTools(t *testing.T) {
	// Create test environment
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Initialize client
	ctx := context.Background()
	_, err := client.Initialize(ctx)
	require.NoError(t, err)

	// Test listing tools
	toolsResult, err := client.ListTools(ctx)

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, toolsResult.Tools, 1)
	assert.Equal(t, "test-tool", toolsResult.Tools[0].Name)
	assert.Equal(t, "Test Tool", toolsResult.Tools[0].Description)
}

func TestClient_CallTool(t *testing.T) {
	// Create test environment
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Initialize client
	ctx := context.Background()
	_, err := client.Initialize(ctx)
	require.NoError(t, err)

	// Test calling tool
	toolResult, err := client.CallTool(ctx, "test-tool", map[string]interface{}{
		"name": "Test User",
	})

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, toolResult.Content, 1)

	// Use type assertion to convert ToolContent interface to TextContent type
	textContent, ok := toolResult.Content[0].(TextContent)
	assert.True(t, ok, "Content should be of TextContent type")
	assert.Equal(t, "text", textContent.Type)
	assert.Equal(t, "Hello, Test User!", textContent.Text)
}

func TestClient_GetSessionID(t *testing.T) {
	// Create test environment
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Session ID should be empty in initial state
	assert.Empty(t, client.GetSessionID())

	// Initialize client
	ctx := context.Background()
	_, err := client.Initialize(ctx)
	require.NoError(t, err)

	// Session ID should not be empty after initialization
	assert.NotEmpty(t, client.GetSessionID())
}
