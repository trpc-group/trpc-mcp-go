package mcp

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NewMockTool creates a new mock tool
func NewMockTool(name, description string, toolSchema map[string]interface{}) *Tool {
	opts := []ToolOption{
		WithDescription(description),
	}

	// If schema is provided, add parameters
	if toolSchema != nil {
		if props, ok := toolSchema["properties"].(map[string]interface{}); ok {
			for paramName, paramSchema := range props {
				if paramMap, ok := paramSchema.(map[string]interface{}); ok {
					if paramType, ok := paramMap["type"].(string); ok {
						switch paramType {
						case "string":
							opts = append(opts, WithString(paramName))
						case "number":
							opts = append(opts, WithNumber(paramName))
						case "boolean":
							opts = append(opts, WithBoolean(paramName))
						}
					}
				}
			}
		}
	}

	return NewTool(name,
		func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return NewTextResult("Mock tool execution result"), nil
		},
		opts...,
	)
}

func TestNewToolManager(t *testing.T) {
	// Create tool manager
	manager := newToolManager()

	// Verify object created successfully
	assert.NotNil(t, manager)
	assert.Empty(t, manager.tools)
}

func TestToolManager_RegisterTool(t *testing.T) {
	// Create tool manager
	manager := newToolManager()

	// Create mock tools
	tool1 := NewMockTool("test-tool-1", "Test Tool 1", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"param1": map[string]interface{}{
				"type": "string",
			},
		},
	})

	tool2 := NewMockTool("test-tool-2", "Test Tool 2", nil)

	// Test registering a tool
	err := manager.registerTool(tool1)
	assert.NoError(t, err)
	assert.Len(t, manager.tools, 1)
	assert.Contains(t, manager.tools, "test-tool-1")

	// Test registering another tool
	err = manager.registerTool(tool2)
	assert.NoError(t, err)
	assert.Len(t, manager.tools, 2)
	assert.Contains(t, manager.tools, "test-tool-2")

	// Test duplicate registration
	err = manager.registerTool(tool1)
	assert.Error(t, err)
	assert.Len(t, manager.tools, 2)
}

func TestToolManager_GetTool(t *testing.T) {
	// Create tool manager
	manager := newToolManager()

	// Create and register tool
	tool := NewMockTool("test-tool", "Test Tool", map[string]interface{}{})

	err := manager.registerTool(tool)
	require.NoError(t, err)

	// Test getting an existing tool
	result, exists := manager.getTool("test-tool")
	assert.True(t, exists)
	assert.Equal(t, tool, result)

	// Test getting a non-existent tool
	result, exists = manager.getTool("non-existent-tool")
	assert.False(t, exists)
	assert.Nil(t, result)
}

func TestToolManager_GetTools(t *testing.T) {
	// Create tool manager
	manager := newToolManager()

	// Test empty list
	tools := manager.getTools("")
	assert.Empty(t, tools)

	// Register multiple tools
	tool1 := NewMockTool("tool1", "Tool 1", map[string]interface{}{
		"type": "object",
	})

	tool2 := NewMockTool("tool2", "Tool 2", nil)

	manager.registerTool(tool1)
	manager.registerTool(tool2)

	// Test getting tool list
	tools = manager.getTools("")
	assert.Len(t, tools, 2)

	// Verify tool information
	var tool1Info, tool2Info *Tool
	for _, tool := range tools {
		if tool.Name == "tool1" {
			tool1Info = tool
		} else if tool.Name == "tool2" {
			tool2Info = tool
		}
	}

	assert.Equal(t, "tool1", tool1Info.Name)
	assert.Equal(t, "Tool 1", tool1Info.Description)
	assert.NotNil(t, tool1Info.InputSchema)
	assert.Equal(t, openapi3.Types{openapi3.TypeObject}, *tool1Info.InputSchema.Type)

	assert.Equal(t, "tool2", tool2Info.Name)
	assert.Equal(t, "Tool 2", tool2Info.Description)
	assert.NotNil(t, tool2Info.InputSchema)
	assert.Equal(t, openapi3.Types{openapi3.TypeObject}, *tool2Info.InputSchema.Type)
	assert.Empty(t, tool2Info.InputSchema.Properties)
}

func TestToolManager_HandleCallTool(t *testing.T) {
	// Create tool manager
	manager := newToolManager()

	// Create and register tool
	tool := NewMockTool("test-exec-tool", "Test Execution Tool", map[string]interface{}{})

	err := manager.registerTool(tool)
	require.NoError(t, err)

	// Test executing the tool
	ctx := context.Background()

	// Create request
	req := newJSONRPCRequest("call-1", MethodToolsCall, map[string]interface{}{
		"name": "test-exec-tool",
		"arguments": map[string]interface{}{
			"param1": "value1",
		},
	})

	// Process request
	result, err := manager.handleCallTool(ctx, req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Type assert to CallToolResult
	callResult, ok := result.(*CallToolResult)
	assert.True(t, ok, "Expected *CallToolResult but got %T", result)

	// Verify content
	assert.NotNil(t, callResult.Content)
	assert.Len(t, callResult.Content, 1)

	// Verify first content item
	content, ok := callResult.Content[0].(TextContent)
	assert.True(t, ok, "Expected TextContent but got %T", callResult.Content[0])
	assert.Equal(t, "text", content.Type)
	assert.Equal(t, "Mock tool execution result", content.Text)

	// Test executing a non-existent tool
	req = newJSONRPCRequest("call-2", MethodToolsCall, map[string]interface{}{
		"name": "non-existent-tool",
	})

	result, err = manager.handleCallTool(ctx, req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Type assert to JSONRPCError
	errorResp, ok := result.(*JSONRPCError)
	assert.True(t, ok, "Expected *JSONRPCError but got %T", result)
	assert.Equal(t, ErrCodeMethodNotFound, errorResp.Error.Code)
}
