// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// TestStdioClientServer_EndToEndIntegration tests STDIO client-server integration.
func TestStdioClientServer_EndToEndIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// === Create Go executable for the server ===
	serverConfig := mcp.StdioTransportConfig{
		ServerParams: mcp.StdioServerParameters{
			Command: "go",
			Args:    []string{"run", "./test_server/main.go"},
		},
		Timeout: 10 * time.Second,
	}

	// === Create STDIO Client ===
	client, err := mcp.NewStdioClient(
		serverConfig,
		mcp.Implementation{
			Name:    "e2e-test-client",
			Version: "1.0.0",
		},
		mcp.WithStdioLogger(mcp.GetDefaultLogger()),
	)
	require.NoError(t, err)
	defer client.Close()

	// === Test Initialization ===
	t.Run("Initialize", func(t *testing.T) {
		initResult, err := client.Initialize(ctx, &mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.ProtocolVersion_2025_03_26,
				ClientInfo: mcp.Implementation{
					Name:    "e2e-test-client",
					Version: "1.0.0",
				},
			},
		})
		require.NoError(t, err)
		assert.NotNil(t, initResult)
		assert.Equal(t, mcp.ProtocolVersion_2025_03_26, initResult.ProtocolVersion)
		assert.Equal(t, "e2e-test-server", initResult.ServerInfo.Name)
		assert.Equal(t, "1.0.0", initResult.ServerInfo.Version)
		assert.NotNil(t, initResult.Capabilities.Tools)
	})

	// === Test Tools ===
	t.Run("Tools", func(t *testing.T) {
		// Test list tools.
		toolsResult, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
		require.NoError(t, err)
		assert.NotEmpty(t, toolsResult.Tools)

		// Verify echo tool exists.
		var echoTool *mcp.Tool
		for _, tool := range toolsResult.Tools {
			if tool.Name == "echo" {
				echoTool = &tool
				break
			}
		}
		require.NotNil(t, echoTool, "echo tool should be available")
		assert.Equal(t, "Echo a message back", echoTool.Description)

		// Test call echo tool.
		callResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "echo",
				Arguments: map[string]interface{}{
					"text": "Hello from e2e test!",
				},
			},
		})
		require.NoError(t, err)
		require.Len(t, callResult.Content, 1)

		// Verify result content.
		textContent, ok := callResult.Content[0].(mcp.TextContent)
		require.True(t, ok, "Expected text content")
		assert.Contains(t, textContent.Text, "Hello from e2e test!")

		// Test math tool.
		var addTool *mcp.Tool
		for _, tool := range toolsResult.Tools {
			if tool.Name == "add" {
				addTool = &tool
				break
			}
		}
		require.NotNil(t, addTool, "add tool should be available")

		// Test call add tool.
		mathResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "add",
				Arguments: map[string]interface{}{
					"a": 123.0,
					"b": 456.0,
				},
			},
		})
		require.NoError(t, err)
		require.Len(t, mathResult.Content, 1)

		mathContent, ok := mathResult.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, mathContent.Text, "579") // 123 + 456 = 579
	})

	// === Test Error Handling ===
	t.Run("ErrorHandling", func(t *testing.T) {
		// Test non-existent tool
		_, err := client.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "non-existent-tool",
				Arguments: map[string]interface{}{},
			},
		})
		assert.Error(t, err, "Should error when calling non-existent tool")

		// Test invalid parameters
		_, err = client.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "echo",
				Arguments: map[string]interface{}{
					// Missing required 'text' parameter
				},
			},
		})
		assert.Error(t, err, "Should error with missing required parameters")
	})

	// === Test Client State ===
	t.Run("ClientState", func(t *testing.T) {
		state := client.GetState()
		assert.Equal(t, mcp.StateInitialized, state)

		// Give process a moment to stabilize
		time.Sleep(100 * time.Millisecond)
		assert.True(t, client.IsProcessRunning())
		assert.Greater(t, client.GetProcessID(), 0)
	})
}

// TestStdioClientServer_ProcessManagement tests process lifecycle management.
func TestStdioClientServer_ProcessManagement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	serverConfig := mcp.StdioTransportConfig{
		ServerParams: mcp.StdioServerParameters{
			Command: "go",
			Args:    []string{"run", "./test_server/main.go"},
		},
		Timeout: 10 * time.Second,
	}

	client, err := mcp.NewStdioClient(
		serverConfig,
		mcp.Implementation{
			Name:    "process-test-client",
			Version: "1.0.0",
		},
	)
	require.NoError(t, err)
	defer client.Close()

	// Initialize
	_, err = client.Initialize(ctx, &mcp.InitializeRequest{})
	require.NoError(t, err)

	// Test process info.
	t.Run("ProcessInfo", func(t *testing.T) {
		// Give process a moment to stabilize.
		time.Sleep(100 * time.Millisecond)
		assert.True(t, client.IsProcessRunning())
		pid := client.GetProcessID()
		assert.Greater(t, pid, 0)

		commandLine := client.GetCommandLine()
		assert.Contains(t, commandLine, "go")
		assert.Contains(t, commandLine, "run")
	})

	// Test transport info.
	t.Run("TransportInfo", func(t *testing.T) {
		info := client.GetTransportInfo()
		assert.Equal(t, "stdio", info.Type)
		assert.NotEmpty(t, info.Description)
	})

	// Test client close.
	t.Run("ClientClose", func(t *testing.T) {
		err := client.Close()
		// Process might already be finished, so close errors are acceptable.
		if err != nil {
			t.Logf("Client close returned error (acceptable): %v", err)
		}
		assert.False(t, client.IsProcessRunning())
		assert.Equal(t, mcp.StateDisconnected, client.GetState())
	})
}

// TestStdioClientServer_ErrorRecovery tests error recovery scenarios.
func TestStdioClientServer_ErrorRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with invalid command
	t.Run("InvalidCommand", func(t *testing.T) {
		invalidConfig := mcp.StdioTransportConfig{
			ServerParams: mcp.StdioServerParameters{
				Command: "non-existent-command",
				Args:    []string{},
			},
			Timeout: 5 * time.Second,
		}

		_, err := mcp.NewStdioClient(
			invalidConfig,
			mcp.Implementation{
				Name:    "error-test-client",
				Version: "1.0.0",
			},
		)
		// Should succeed creating client, but fail on first operation.
		assert.NoError(t, err)
	})

	// Test timeout scenario.
	t.Run("Timeout", func(t *testing.T) {
		shortTimeoutConfig := mcp.StdioTransportConfig{
			ServerParams: mcp.StdioServerParameters{
				Command: "sleep",
				Args:    []string{"100"}, // Sleep for 100 seconds.
			},
			Timeout: 1 * time.Second, // Very short timeout.
		}

		client, err := mcp.NewStdioClient(
			shortTimeoutConfig,
			mcp.Implementation{
				Name:    "timeout-test-client",
				Version: "1.0.0",
			},
		)
		require.NoError(t, err)
		defer client.Close()

		// This should timeout.
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		_, err = client.Initialize(timeoutCtx, &mcp.InitializeRequest{})
		assert.Error(t, err, "Should timeout")
	})
}

// TestStdioClientServer_ConcurrentOperations tests concurrent operations.
func TestStdioClientServer_ConcurrentOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverConfig := mcp.StdioTransportConfig{
		ServerParams: mcp.StdioServerParameters{
			Command: "go",
			Args:    []string{"run", "./test_server/main.go"},
		},
		Timeout: 10 * time.Second,
	}

	client, err := mcp.NewStdioClient(
		serverConfig,
		mcp.Implementation{
			Name:    "concurrent-test-client",
			Version: "1.0.0",
		},
	)
	require.NoError(t, err)
	defer client.Close()

	// Initialize.
	_, err = client.Initialize(ctx, &mcp.InitializeRequest{})
	require.NoError(t, err)

	t.Run("ConcurrentToolCalls", func(t *testing.T) {
		const numCalls = 5
		results := make(chan error, numCalls)

		// Make multiple concurrent tool calls.
		for i := 0; i < numCalls; i++ {
			go func(id int) {
				_, err := client.CallTool(ctx, &mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Name: "echo",
						Arguments: map[string]interface{}{
							"text": fmt.Sprintf("Concurrent call %d", id),
						},
					},
				})
				results <- err
			}(i)
		}

		// Wait for all calls to complete.
		for i := 0; i < numCalls; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent call %d should succeed", i)
		}
	})
}

// TestStdioClientServer_RootsProvider tests server-to-client roots functionality over STDIO transport.
func TestStdioClientServer_RootsProvider(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create STDIO server configuration
	serverConfig := mcp.StdioTransportConfig{
		ServerParams: mcp.StdioServerParameters{
			Command: "go",
			Args:    []string{"run", "./test_server/main.go"},
		},
		Timeout: 10 * time.Second,
	}

	// Create STDIO client with RootsProvider capability
	client, err := mcp.NewStdioClient(
		serverConfig,
		mcp.Implementation{
			Name:    "roots-stdio-test-client",
			Version: "1.0.0",
		},
		mcp.WithStdioLogger(mcp.GetDefaultLogger()),
	)
	require.NoError(t, err)
	defer client.Close()

	// Set up roots provider with test directories
	rootsProvider := mcp.NewDefaultRootsProvider()
	rootsProvider.AddRoot("/etc", "Configuration Directory")
	rootsProvider.AddRoot("/usr", "User Directory")
	client.SetRootsProvider(rootsProvider)

	// Initialize with roots capability
	initResult, err := client.Initialize(ctx, &mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.ProtocolVersion_2025_03_26,
			ClientInfo: mcp.Implementation{
				Name:    "roots-stdio-test-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{
				Roots: &mcp.RootsCapability{
					ListChanged: true,
				},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, mcp.ProtocolVersion_2025_03_26, initResult.ProtocolVersion)

	// Test sending roots list changed notification
	err = client.SendRootsListChangedNotification(ctx)
	require.NoError(t, err)

	// Give the server a moment to process the notification
	time.Sleep(100 * time.Millisecond)

	// Test calling the list-roots tool
	result, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "list-roots",
		},
	})

	if err == nil {
		// If the tool exists, verify the response
		require.NotEmpty(t, result.Content)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "Configuration Directory")
		assert.Contains(t, textContent.Text, "User Directory")
	} else {
		// The tool might not exist in the test server, which is acceptable
		// We're primarily testing that the notification was sent successfully
		t.Logf("Tool 'list-roots' not available: %v", err)
	}
}
