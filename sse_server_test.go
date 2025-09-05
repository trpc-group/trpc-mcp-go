// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"testing"
)

func TestSSEServer_UnregisterTools(t *testing.T) {
	// Create an SSE server
	server := NewSSEServer("test-server", "1.0.0")

	// Create test tools
	tool1 := &Tool{
		Name:        "test-tool-1",
		Description: "Test tool 1",
	}
	tool2 := &Tool{
		Name:        "test-tool-2",
		Description: "Test tool 2",
	}
	tool3 := &Tool{
		Name:        "test-tool-3",
		Description: "Test tool 3",
	}

	// Register tools
	server.RegisterTool(tool1, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("result1"), nil
	})
	server.RegisterTool(tool2, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("result2"), nil
	})
	server.RegisterTool(tool3, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("result3"), nil
	})

	// Verify all tools are registered
	tools := server.toolManager.getTools()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}

	// Test unregistering multiple tools
	err := server.UnregisterTools("test-tool-1", "test-tool-2")
	if err != nil {
		t.Errorf("UnregisterTools failed: %v", err)
	}

	// Verify tools were unregistered
	tools = server.toolManager.getTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after unregistering 2, got %d", len(tools))
	}
	if tools[0].Name != "test-tool-3" {
		t.Errorf("Expected remaining tool to be test-tool-3, got %s", tools[0].Name)
	}

	// Test unregistering single tool
	err = server.UnregisterTools("test-tool-3")
	if err != nil {
		t.Errorf("UnregisterTools failed for single tool: %v", err)
	}

	// Verify all tools are unregistered
	tools = server.toolManager.getTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools after unregistering all, got %d", len(tools))
	}

	// Test error cases
	t.Run("No tool names provided", func(t *testing.T) {
		err := server.UnregisterTools()
		if err == nil {
			t.Error("Expected error when no tool names provided")
		}
		if err.Error() != "no tool names provided" {
			t.Errorf("Expected 'no tool names provided' error, got: %v", err)
		}
	})

	t.Run("None of the specified tools found", func(t *testing.T) {
		err := server.UnregisterTools("non-existent-tool")
		if err == nil {
			t.Error("Expected error when trying to unregister non-existent tool")
		}
		if err.Error() != "none of the specified tools were found" {
			t.Errorf("Expected 'none of the specified tools were found' error, got: %v", err)
		}
	})
}

func TestSSEServer_RegisterAndUnregisterTool(t *testing.T) {
	// Create an SSE server
	server := NewSSEServer("test-server", "1.0.0")

	// Create a test tool
	tool := &Tool{
		Name:        "dynamic-tool",
		Description: "A dynamically managed tool",
	}

	// Register the tool
	server.RegisterTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("dynamic result"), nil
	})

	// Verify tool is registered
	tools := server.toolManager.getTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after registration, got %d", len(tools))
	}
	if tools[0].Name != "dynamic-tool" {
		t.Errorf("Expected tool name to be dynamic-tool, got %s", tools[0].Name)
	}

	// Unregister the tool
	err := server.UnregisterTools("dynamic-tool")
	if err != nil {
		t.Errorf("UnregisterTools failed: %v", err)
	}

	// Verify tool is unregistered
	tools = server.toolManager.getTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools after unregistration, got %d", len(tools))
	}

	// Register the tool again to verify it can be re-registered
	server.RegisterTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("dynamic result again"), nil
	})

	// Verify tool is registered again
	tools = server.toolManager.getTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after re-registration, got %d", len(tools))
	}
}
