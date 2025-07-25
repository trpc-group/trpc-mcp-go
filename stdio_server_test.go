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
)

func TestStdioServer_UnregisterTools(t *testing.T) {
	server := NewStdioServer("Test-Stdio-Server", "1.0.0")

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
	tools := server.toolManager.getTools("")
	assert.Len(t, tools, 3)

	// Test unregistering multiple existing tools
	err := server.UnregisterTools("test-tool-1", "test-tool-3")
	assert.NoError(t, err)

	tools = server.toolManager.getTools("")
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

	tools = server.toolManager.getTools("")
	assert.Len(t, tools, 0)
}

func TestStdioServer_RegisterAndUnregisterTool(t *testing.T) {
	server := NewStdioServer("Test-Stdio-Server", "1.0.0")

	// Test registering and unregistering tools in sequence
	tool := NewTool("dynamic-tool", WithDescription("Dynamic Tool"))
	handler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Dynamic result"), nil
	}

	// Initially no tools
	tools := server.toolManager.getTools("")
	assert.Len(t, tools, 0)

	// Register tool
	server.RegisterTool(tool, handler)
	tools = server.toolManager.getTools("")
	assert.Len(t, tools, 1)

	// Unregister tool
	err := server.UnregisterTools("dynamic-tool")
	assert.NoError(t, err)
	tools = server.toolManager.getTools("")
	assert.Len(t, tools, 0)

	// Re-register same tool
	server.RegisterTool(tool, handler)
	tools = server.toolManager.getTools("")
	assert.Len(t, tools, 1)

	// Unregister again
	err = server.UnregisterTools("dynamic-tool")
	assert.NoError(t, err)
	tools = server.toolManager.getTools("")
	assert.Len(t, tools, 0)
}
