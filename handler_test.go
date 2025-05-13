package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMCPHandler(t *testing.T) {
	// Create handler
	handler := NewMCPHandler()

	// Verify object created successfully
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.toolManager)
	assert.NotNil(t, handler.lifecycleManager)
}

func TestMCPHandler_WithOptions(t *testing.T) {
	// Create custom components
	toolManager := NewToolManager()
	lifecycleManager := NewLifecycleManager(Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	})

	// Create handler with options
	handler := NewMCPHandler(
		WithToolManager(toolManager),
		WithLifecycleManager(lifecycleManager),
	)

	// Verify options applied
	assert.Equal(t, toolManager, handler.toolManager)
	assert.Equal(t, lifecycleManager, handler.lifecycleManager)
}

func TestMCPHandler_HandleRequest_Initialize(t *testing.T) {
	// Create handler
	toolManager := NewToolManager()
	lifecycleManager := NewLifecycleManager(Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	})

	handler := NewMCPHandler(
		WithToolManager(toolManager),
		WithLifecycleManager(lifecycleManager),
	)

	// Create initialization request
	request := NewInitializeRequest(
		ProtocolVersion_2024_11_05,
		Implementation{
			Name:    "Test-Client",
			Version: "1.0.0",
		},
		ClientCapabilities{
			Roots: &RootsCapability{
				ListChanged: true,
			},
			Sampling: &SamplingCapability{},
		},
	)

	// Create session
	session := NewSession()

	// Process request
	ctx := context.Background()
	resp, err := handler.HandleRequest(ctx, request, session)

	// Verify results
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify protocol version in session
	protocolVersion, ok := session.GetData("protocolVersion")
	require.True(t, ok)
	assert.Equal(t, ProtocolVersion_2024_11_05, protocolVersion)
}

func TestMCPHandler_HandleRequest_UnknownMethod(t *testing.T) {
	// Create handler
	handler := NewMCPHandler()

	// Create request with unknown method
	req := NewJSONRPCRequest(1, "unknown/method", nil)

	// Process request
	ctx := context.Background()
	resp, err := handler.HandleRequest(ctx, req, nil)

	// For unknown method, we should get an error
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestMCPHandler_HandleRequest_ToolsList(t *testing.T) {
	// Create handler
	handler := NewMCPHandler()

	// Register test tool
	tool := NewMockTool("test-tool", "Test Tool", map[string]interface{}{})
	handler.toolManager.RegisterTool(tool)

	// Create session and set protocol version
	session := NewSession()
	session.SetData("protocolVersion", ProtocolVersion_2024_11_05)

	// Create list tools request
	req := NewJSONRPCRequest(1, MethodToolsList, nil)

	// Process request
	ctx := context.Background()
	resp, err := handler.HandleRequest(ctx, req, session)

	// Verify results
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify response content
	result, ok := resp.(JSONRPCResponse).Result.(map[string]interface{})
	assert.True(t, ok)
	tools, ok := result["tools"].([]*Tool)
	assert.True(t, ok)
	assert.Len(t, tools, 1)
	assert.Equal(t, "test-tool", tools[0].Name)
	assert.Equal(t, "Test Tool", tools[0].Description)
}

func TestMCPHandler_HandleRequest_ToolsCall(t *testing.T) {
	// Create handler
	handler := NewMCPHandler()

	// Register test tool
	tool := NewMockTool("test-tool", "Test Tool", map[string]interface{}{})
	handler.toolManager.RegisterTool(tool)

	// Create session
	session := NewSession()

	// Create call tool request
	req := NewJSONRPCRequest(1, MethodToolsCall, map[string]interface{}{
		"name": "test-tool",
		"arguments": map[string]interface{}{
			"param1": "value1",
		},
	})

	// Process request
	ctx := context.Background()
	resp, err := handler.HandleRequest(ctx, req, session)

	// Verify results
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify response content
	result, ok := resp.(JSONRPCResponse).Result.(map[string]interface{})
	assert.True(t, ok)
	content, ok := result["content"].([]Content)
	assert.True(t, ok)
	assert.Len(t, content, 1)

	// Verify first content item
	_, ok = content[0].(Content)
	assert.True(t, ok)
}
