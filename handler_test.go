// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMCPHandler(t *testing.T) {
	// Create handler
	handler := newMCPHandler()

	// Verify object created successfully
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.toolManager)
	assert.NotNil(t, handler.lifecycleManager)
}

func TestMCPHandler_WithOptions(t *testing.T) {
	// Create custom components
	toolManager := newToolManager()
	lifecycleManager := newLifecycleManager(Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	})

	// Create handler with options
	handler := newMCPHandler(
		withToolManager(toolManager),
		withLifecycleManager(lifecycleManager),
	)

	// Verify options applied
	assert.Equal(t, toolManager, handler.toolManager)
	assert.Equal(t, lifecycleManager, handler.lifecycleManager)
}

func TestMCPHandler_Initialize(t *testing.T) {
	// Create handler
	toolManager := newToolManager()
	lifecycleManager := newLifecycleManager(Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	})

	handler := newMCPHandler(
		withToolManager(toolManager),
		withLifecycleManager(lifecycleManager),
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
	session := newSession()

	// Process request
	ctx := context.Background()
	resp, err := handler.handleRequest(ctx, request, session)

	// Verify results
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify protocol version in session
	protocolVersion, ok := session.GetData("protocolVersion")
	require.True(t, ok)
	assert.Equal(t, ProtocolVersion_2024_11_05, protocolVersion)
}

func TestMCPHandler_UnknownMethod(t *testing.T) {
	// Create handler
	handler := newMCPHandler()

	// Create request with unknown method
	req := newJSONRPCRequest(1, "unknown/method", nil)

	// Create session
	session := newSession()

	// Process request
	ctx := context.Background()
	resp, err := handler.handleRequest(ctx, req, session)

	// Updated test expectation: for unknown methods, the handler now may return a JSONRPCError response instead of an error
	// This might be due to internal implementation changes
	assert.Nil(t, err)
	assert.NotNil(t, resp)

	// Check if a JSONRPCError was returned
	errorResp, ok := resp.(*JSONRPCError)
	assert.True(t, ok, "Expected JSONRPCError response")
	assert.Equal(t, -32601, errorResp.Error.Code)
	assert.Equal(t, "method not found", errorResp.Error.Message)
}

// handleMockTool handles the mock tool
func handleMockTool(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
	return &CallToolResult{
		Content: []Content{
			NewTextContent("Mock tool response"),
		},
	}, nil
}

func TestMCPHandler_ToolsList(t *testing.T) {
	// Create handler
	handler := newMCPHandler()

	// Register test tool
	tool := NewMockTool("test-tool", "Test Tool", map[string]interface{}{})
	handler.toolManager.registerTool(tool, handleMockTool)

	// Create session and set protocol version
	session := newSession()
	session.SetData("protocolVersion", ProtocolVersion_2025_03_26)

	// Create list tools request
	req := newJSONRPCRequest(1, MethodToolsList, nil)

	// Process request
	ctx := context.Background()
	resp, err := handler.handleRequest(ctx, req, session)

	// Verify results
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Print actual response type for debugging
	t.Logf("Response type: %T", resp)

	// Check if it's a JSONRPCResponse type
	if jsonRPCResp, ok := resp.(*JSONRPCResponse); ok {
		t.Logf("It's a JSONRPCResponse with JSONRPC: %s, ID: %d", jsonRPCResp.JSONRPC, jsonRPCResp.ID)

		// Check the Result field
		if result, ok := jsonRPCResp.Result.(*ListToolsResult); ok {
			assert.NotNil(t, result.Tools)
			assert.Len(t, result.Tools, 1)
			assert.Equal(t, "test-tool", result.Tools[0].Name)
			assert.Equal(t, "Test Tool", result.Tools[0].Description)
			return
		}

		// Check if Result is of another type
		t.Logf("Result type: %T", jsonRPCResp.Result)
	}

	// Check if it's a ListToolsResult type
	if result, ok := resp.(ListToolsResult); ok {
		assert.NotNil(t, result.Tools)
		assert.Len(t, result.Tools, 1)
		assert.Equal(t, "test-tool", result.Tools[0].Name)
		assert.Equal(t, "Test Tool", result.Tools[0].Description)
		return
	}

	// If it's neither, print detailed type information for debugging
	t.Errorf("Unexpected response type: %T", resp)
}

func TestMCPHandler_ToolsCall(t *testing.T) {
	// Create handler
	handler := newMCPHandler()

	// Register test tool
	tool := NewMockTool("test-tool", "Test Tool", map[string]interface{}{})
	handler.toolManager.registerTool(tool, handleMockTool)

	// Create session
	session := newSession()
	session.SetData("protocolVersion", ProtocolVersion_2024_11_05)

	// Create call tool request
	req := newJSONRPCRequest(1, MethodToolsCall, map[string]interface{}{
		"name": "test-tool",
		"arguments": map[string]interface{}{
			"param1": "value1",
		},
	})

	// Process request
	ctx := context.Background()
	resp, err := handler.handleRequest(ctx, req, session)

	// Verify results
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Updated test expectation: the response might now be a CallToolResult or another direct response type
	result, ok := resp.(*CallToolResult)
	assert.True(t, ok, "Expected CallToolResult response")
	assert.NotNil(t, result.Content)
	assert.Len(t, result.Content, 1)
}
