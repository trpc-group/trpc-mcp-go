// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

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

	return NewTool(name, opts...)
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
	manager.registerTool(tool1, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})
	assert.Len(t, manager.tools, 1)
	assert.Contains(t, manager.tools, "test-tool-1")

	// Test registering another tool
	manager.registerTool(tool2, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})
	assert.Len(t, manager.tools, 2)
	assert.Contains(t, manager.tools, "test-tool-2")

	// Test duplicate registration
	manager.registerTool(tool1, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})
	assert.Len(t, manager.tools, 2)
}

func TestToolManager_GetTool(t *testing.T) {
	// Create tool manager
	manager := newToolManager()

	// Create and register tool
	tool := NewMockTool("test-tool", "Test Tool", map[string]interface{}{})

	manager.registerTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})

	// Test getting an existing tool
	result, exists := manager.getTool("test-tool")
	assert.True(t, exists)
	assert.Equal(t, tool, result)

	// Test getting a non-existent tool
	result, exists = manager.getTool("non-existent-tool")
	assert.False(t, exists)
	assert.Nil(t, result)
}

func TestToolManager_UnregisterTools(t *testing.T) {
	// Create tool manager
	manager := newToolManager()

	// Create and register multiple tools
	tool1 := NewMockTool("test-tool-1", "Test Tool 1", map[string]interface{}{})
	tool2 := NewMockTool("test-tool-2", "Test Tool 2", map[string]interface{}{})
	tool3 := NewMockTool("test-tool-3", "Test Tool 3", map[string]interface{}{})

	manager.registerTool(tool1, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})
	manager.registerTool(tool2, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})
	manager.registerTool(tool3, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})

	// Verify all tools are registered
	assert.Len(t, manager.tools, 3)
	assert.Len(t, manager.toolsOrder, 3)

	// Test unregistering multiple existing tools
	unregisteredCount := manager.unregisterTools("test-tool-1", "test-tool-3")
	assert.Equal(t, 2, unregisteredCount)
	assert.Len(t, manager.tools, 1)
	assert.NotContains(t, manager.tools, "test-tool-1")
	assert.Contains(t, manager.tools, "test-tool-2")
	assert.NotContains(t, manager.tools, "test-tool-3")
	assert.Len(t, manager.toolsOrder, 1)
	assert.Equal(t, "test-tool-2", manager.toolsOrder[0])

	// Test unregistering non-existent tools
	unregisteredCount = manager.unregisterTools("non-existent-1", "non-existent-2")
	assert.Equal(t, 0, unregisteredCount)
	assert.Len(t, manager.tools, 1)

	// Test unregistering mix of existing and non-existent tools
	manager.registerTool(tool1, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})
	unregisteredCount = manager.unregisterTools("test-tool-1", "non-existent", "test-tool-2")
	assert.Equal(t, 2, unregisteredCount)
	assert.Len(t, manager.tools, 0)
	assert.Len(t, manager.toolsOrder, 0)

	// Test unregistering with empty names
	manager.registerTool(tool1, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})
	unregisteredCount = manager.unregisterTools("", "test-tool-1", "")
	assert.Equal(t, 1, unregisteredCount)
	assert.Len(t, manager.tools, 0)

	// Test unregistering with no names provided
	unregisteredCount = manager.unregisterTools()
	assert.Equal(t, 0, unregisteredCount)
}

// TestToolManager_ConcurrentRegisterUnregister tests concurrent registration and unregistration
func TestToolManager_ConcurrentRegisterUnregister(t *testing.T) {
	manager := newToolManager()

	const numWorkers = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Worker function that registers and unregisters tools concurrently
	worker := func(workerID int) {
		defer wg.Done()

		for i := 0; i < numOperations; i++ {
			toolName := fmt.Sprintf("worker-%d-tool-%d", workerID, i)

			// Register tool
			tool := NewMockTool(toolName, "Test Tool", map[string]interface{}{})
			handler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
				return NewTextResult("Mock result"), nil
			}

			manager.registerTool(tool, handler)

			// Small delay to increase chance of race conditions
			time.Sleep(time.Microsecond)

			// Unregister tool
			manager.unregisterTools(toolName)
		}
	}

	// Start workers
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go worker(i)
	}

	// Wait for all workers to complete
	wg.Wait()

	// Verify final state
	tools := manager.getTools()
	assert.Len(t, tools, 0, "All tools should be unregistered")
}

// TestToolManager_ConcurrentCallAndUnregister tests calling tools while unregistering them
func TestToolManager_ConcurrentCallAndUnregister(t *testing.T) {
	manager := newToolManager()

	// Register initial tools
	for i := 0; i < 10; i++ {
		toolName := fmt.Sprintf("test-tool-%d", i)
		tool := NewMockTool(toolName, "Test Tool", map[string]interface{}{})
		handler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			// Simulate some work
			time.Sleep(time.Millisecond)
			return NewTextResult("Mock result"), nil
		}
		manager.registerTool(tool, handler)
	}

	var wg sync.WaitGroup
	ctx := context.Background()

	// Worker that calls tools
	callWorker := func() {
		defer wg.Done()

		for i := 0; i < 50; i++ {
			toolName := fmt.Sprintf("test-tool-%d", i%10)

			// Create a mock request
			req := &JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      i,
				Request: Request{
					Method: MethodToolsCall,
				},
				Params: map[string]interface{}{
					"name":      toolName,
					"arguments": map[string]interface{}{},
				},
			}

			// Call tool (this should not panic even if tool is unregistered concurrently)
			_, err := manager.handleCallTool(ctx, req, nil)

			// Error is acceptable (tool might have been unregistered), but no panic
			_ = err
		}
	}

	// Worker that unregisters tools
	unregisterWorker := func() {
		defer wg.Done()

		for i := 0; i < 50; i++ {
			toolName := fmt.Sprintf("test-tool-%d", i%10)
			manager.unregisterTools(toolName)

			// Re-register for continued testing
			tool := NewMockTool(toolName, "Test Tool", map[string]interface{}{})
			handler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
				return NewTextResult("Mock result"), nil
			}
			manager.registerTool(tool, handler)
		}
	}

	// Start workers
	wg.Add(4) // 2 call workers + 2 unregister workers
	go callWorker()
	go callWorker()
	go unregisterWorker()
	go unregisterWorker()

	// Wait for all workers to complete
	wg.Wait()

	// Test passes if no panic occurred
	t.Log("Concurrent call and unregister test completed successfully")
}

// TestToolManager_ConcurrentGetTools tests concurrent getTools calls
func TestToolManager_ConcurrentGetTools(t *testing.T) {
	manager := newToolManager()

	// Register some initial tools
	for i := 0; i < 5; i++ {
		toolName := fmt.Sprintf("initial-tool-%d", i)
		tool := NewMockTool(toolName, "Test Tool", map[string]interface{}{})
		handler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return NewTextResult("Mock result"), nil
		}
		manager.registerTool(tool, handler)
	}

	var wg sync.WaitGroup
	results := make([][]string, 20)

	// Worker that gets tools list
	getWorker := func(workerID int) {
		defer wg.Done()

		tools := manager.getTools()
		toolNames := make([]string, len(tools))
		for i, tool := range tools {
			toolNames[i] = tool.Name
		}
		results[workerID] = toolNames
	}

	// Worker that modifies tools
	modifyWorker := func(workerID int) {
		defer wg.Done()

		for i := 0; i < 10; i++ {
			toolName := fmt.Sprintf("dynamic-tool-%d-%d", workerID, i)

			// Register tool
			tool := NewMockTool(toolName, "Dynamic Tool", map[string]interface{}{})
			handler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
				return NewTextResult("Mock result"), nil
			}
			manager.registerTool(tool, handler)

			// Unregister tool
			manager.unregisterTools(toolName)
		}
	}

	// Start workers
	wg.Add(20) // 15 get workers + 5 modify workers
	for i := 0; i < 15; i++ {
		go getWorker(i)
	}
	for i := 15; i < 20; i++ {
		go modifyWorker(i)
	}

	// Wait for all workers to complete
	wg.Wait()

	// Verify that all get operations completed without panic
	for i := 0; i < 15; i++ {
		assert.NotNil(t, results[i], "Worker %d should have returned a result", i)
	}

	t.Log("Concurrent getTools test completed successfully")
}

func TestToolManager_GetTools(t *testing.T) {
	// Create tool manager
	manager := newToolManager()

	// Test empty list
	tools := manager.getTools()
	assert.Empty(t, tools)

	// Register multiple tools
	tool1 := NewMockTool("tool1", "Tool 1", map[string]interface{}{
		"type": "object",
	})

	tool2 := NewMockTool("tool2", "Tool 2", nil)

	manager.registerTool(tool1, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})
	manager.registerTool(tool2, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})

	// Test getting tool list
	tools = manager.getTools()
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
	assert.Equal(t, "object", tool1Info.InputSchema.Type)

	assert.Equal(t, "tool2", tool2Info.Name)
	assert.Equal(t, "Tool 2", tool2Info.Description)
	assert.NotNil(t, tool2Info.InputSchema)
	assert.Equal(t, "object", tool2Info.InputSchema.Type)
	assert.Empty(t, tool2Info.InputSchema.Properties)
}

func TestToolManager_HandleCallTool(t *testing.T) {
	// Create tool manager
	manager := newToolManager()

	// Create and register tool
	tool := NewMockTool("test-exec-tool", "Test Execution Tool", map[string]interface{}{})

	manager.registerTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})

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

func TestToolManager_ServerInfoInContext(t *testing.T) {
	// Create a tool manager
	manager := newToolManager()

	// Create a test server
	testServerName := "Test-Server-Context"
	testServerVersion := "3.0.0"
	server := NewServer(testServerName, testServerVersion, WithServerAddress(":0"))

	// Set the server provider
	manager.withServerProvider(server)

	// Create a tool that checks for server info in context
	tool := NewTool("server-info-tool", WithDescription("A tool that extracts server info from context"))

	var capturedServerInfo Implementation
	toolHandler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		// Try to get server instance from context
		serverInstance := GetServerFromContext(ctx)
		require.NotNil(t, serverInstance, "Server instance should be available in context")

		// Cast to *Server and get server info
		mcpServer, ok := serverInstance.(*Server)
		require.True(t, ok, "Should be able to cast to *Server")

		// Get server info using the new method
		capturedServerInfo = mcpServer.GetServerInfo()

		return NewTextResult(fmt.Sprintf("Server: %s v%s", capturedServerInfo.Name, capturedServerInfo.Version)), nil
	}

	// Register the tool
	manager.registerTool(tool, toolHandler)

	// Create a mock session
	session := newSession()

	// Create a request
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "test-id",
		Request: Request{
			Method: MethodToolsCall,
		},
		Params: map[string]interface{}{
			"name":      "server-info-tool",
			"arguments": map[string]interface{}{},
		},
	}

	// Handle the tool call
	ctx := context.Background()
	result, err := manager.handleCallTool(ctx, req, session)

	// Verify no error occurred
	require.NoError(t, err)

	// Verify the result
	toolResult, ok := result.(*CallToolResult)
	require.True(t, ok, "Result should be a CallToolResult")
	require.Len(t, toolResult.Content, 1)

	// Check content type - it might be TextContent or *TextContent
	var textContent *TextContent
	switch content := toolResult.Content[0].(type) {
	case *TextContent:
		textContent = content
	case TextContent:
		textContent = &content
	default:
		t.Fatalf("Content should be text, got %T", content)
	}
	require.Equal(t, fmt.Sprintf("Server: %s v%s", testServerName, testServerVersion), textContent.Text)

	// Verify the captured server info
	assert.Equal(t, testServerName, capturedServerInfo.Name)
	assert.Equal(t, testServerVersion, capturedServerInfo.Version)
}

// TestToolManager_MethodNameModifier tests the method name modifier functionality.
func TestToolManager_MethodNameModifier(t *testing.T) {
	// Create tool manager.
	manager := newToolManager()

	// Track method modification calls.
	var modificationCalls []struct {
		method   string
		toolName string
	}

	// Create a test modifier.
	testModifier := func(ctx context.Context, method, toolName string) {
		modificationCalls = append(modificationCalls, struct {
			method   string
			toolName string
		}{method: method, toolName: toolName})
	}

	// Set the modifier.
	manager.withMethodNameModifier(testModifier)

	// Create and register tool.
	tool := NewMockTool("test-modifier-tool", "Test Method Modifier Tool", map[string]interface{}{})

	manager.registerTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})

	// Test executing the tool.
	ctx := context.Background()

	// Create request
	req := newJSONRPCRequest("modifier-1", MethodToolsCall, map[string]interface{}{
		"name": "test-modifier-tool",
		"arguments": map[string]interface{}{
			"param1": "value1",
		},
	})

	// Process request.
	result, err := manager.handleCallTool(ctx, req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify modifier was called.
	assert.Len(t, modificationCalls, 1)
	assert.Equal(t, MethodToolsCall, modificationCalls[0].method)
	assert.Equal(t, "test-modifier-tool", modificationCalls[0].toolName)

	// Test without modifier (should not panic).
	managerWithoutModifier := newToolManager()
	managerWithoutModifier.registerTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("Mock tool execution result"), nil
	})

	result2, err2 := managerWithoutModifier.handleCallTool(ctx, req, nil)
	assert.NoError(t, err2)
	assert.NotNil(t, result2)
}
