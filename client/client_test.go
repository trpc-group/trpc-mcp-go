package client

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTool is a mock tool for testing
type MockTool struct {
	*schema.Tool
}

// NewMockTool creates a new mock tool
func NewMockTool() *schema.Tool {
	return schema.NewTool("test-tool",
		func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
			name := "World"
			if nameArg, ok := req.Params.Arguments["name"]; ok {
				if nameStr, ok := nameArg.(string); ok && nameStr != "" {
					name = nameStr
				}
			}

			return schema.NewTextResult("Hello, " + name + "!"), nil
		},
		schema.WithDescription("Test Tool"),
		schema.WithString("name",
			schema.Description("Name to greet"),
		),
	)
}

// Create test environment including server and client
func setupTestEnvironment(t *testing.T) (*Client, *httptest.Server, func()) {
	// Create MCP server
	mcpServer := server.NewServer("", schema.Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	}, server.WithPathPrefix("/mcp"))

	// Register test tool
	tool := NewMockTool()
	err := mcpServer.RegisterTool(tool)
	require.NoError(t, err)

	// Create HTTP test server
	httpServer := httptest.NewServer(mcpServer.HTTPHandler())

	// Create client
	client, err := NewClient(httpServer.URL+"/mcp", schema.Implementation{
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
	client, err := NewClient("http://localhost:3000/mcp", schema.Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	})

	// Verify successful object creation
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "Test-Client", client.clientInfo.Name)
	assert.Equal(t, "1.0.0", client.clientInfo.Version)
	assert.Equal(t, schema.ProtocolVersion_2024_11_05, client.protocolVersion) // Default version
	assert.False(t, client.initialized)
}

func TestClient_WithProtocolVersion(t *testing.T) {
	// Test creating client with custom protocol version
	client, err := NewClient("http://localhost:3000/mcp", schema.Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	}, WithProtocolVersion(schema.ProtocolVersion_2024_11_05))

	// Verify protocol version
	assert.NoError(t, err)
	assert.Equal(t, schema.ProtocolVersion_2024_11_05, client.protocolVersion)
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
	assert.Equal(t, schema.ProtocolVersion_2024_11_05, resp.ProtocolVersion)
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
	tools, err := client.ListTools(ctx)

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "test-tool", tools[0].Name)
	assert.Equal(t, "Test Tool", tools[0].Description)
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
	content, err := client.CallTool(ctx, "test-tool", map[string]interface{}{
		"name": "Test User",
	})

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, content, 1)

	// Use type assertion to convert ToolContent interface to TextContent type
	textContent, ok := content[0].(schema.TextContent)
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
