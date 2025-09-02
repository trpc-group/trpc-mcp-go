// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// TestSSEClientServer_EndToEndIntegration tests SSE client-server integration.
func TestSSEClientServer_EndToEndIntegration(t *testing.T) {
	_, sseEndpoint, cleanup := startSSEServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create SSE Client.
	client, err := mcp.NewSSEClient(
		sseEndpoint,
		mcp.Implementation{
			Name:    "e2e-test-client",
			Version: "1.0.0",
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	)
	require.NoError(t, err)
	defer client.Close()

	// Test Initialization.
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
		assert.Equal(t, "E2E-SSE-Test-Server", initResult.ServerInfo.Name)
		assert.Equal(t, "1.0.0", initResult.ServerInfo.Version)
		assert.NotNil(t, initResult.Capabilities.Tools)
	})

	// Test Tools.
	t.Run("Tools", func(t *testing.T) {
		// Test list tools.
		toolsResult, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
		require.NoError(t, err)
		assert.NotEmpty(t, toolsResult.Tools)

		// Verify greet tool exists.
		var greetTool *mcp.Tool
		for _, tool := range toolsResult.Tools {
			if tool.Name == "greet" {
				greetTool = &tool
				break
			}
		}
		require.NotNil(t, greetTool, "greet tool should be available")
		assert.Equal(t, "Greet a user by name", greetTool.Description)

		// Test call greet tool.
		callResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "greet",
				Arguments: map[string]interface{}{
					"name": "Hello from e2e test!",
				},
			},
		})
		require.NoError(t, err)
		require.NotEmpty(t, callResult.Content)

		// Verify result content.
		textContent, ok := callResult.Content[0].(mcp.TextContent)
		require.True(t, ok, "Expected text content")
		assert.Contains(t, textContent.Text, "Hello, Hello from e2e test!")

		// Test weather tool.
		var weatherTool *mcp.Tool
		for _, tool := range toolsResult.Tools {
			if tool.Name == "weather" {
				weatherTool = &tool
				break
			}
		}
		require.NotNil(t, weatherTool, "weather tool should be available")

		// Test call weather tool.
		weatherResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "weather",
				Arguments: map[string]interface{}{
					"city": "Beijing",
				},
			},
		})
		require.NoError(t, err)
		require.NotEmpty(t, weatherResult.Content)

		weatherContent, ok := weatherResult.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, weatherContent.Text, "Weather in Beijing")
	})

	// Test Error Handling.
	t.Run("ErrorHandling", func(t *testing.T) {
		// Test non-existent tool.
		_, err := client.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "non-existent-tool",
				Arguments: map[string]interface{}{},
			},
		})
		assert.Error(t, err, "Should error when calling non-existent tool")
	})

	// Test Client State.
	t.Run("ClientState", func(t *testing.T) {
		state := client.GetState()
		assert.Equal(t, mcp.StateInitialized, state)
	})
}

// TestSSEClientServer_ConnectionManagement tests connection lifecycle.
func TestSSEClientServer_ConnectionManagement(t *testing.T) {
	_, sseEndpoint, cleanup := startSSEServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := mcp.NewSSEClient(
		sseEndpoint,
		mcp.Implementation{
			Name:    "connection-test-client",
			Version: "1.0.0",
		},
	)
	require.NoError(t, err)
	defer client.Close()

	// Initialize.
	_, err = client.Initialize(ctx, &mcp.InitializeRequest{})
	require.NoError(t, err)

	// Test connection info - SSE client doesn't have GetTransportInfo.
	t.Run("ConnectionState", func(t *testing.T) {
		// Check client state instead.
		state := client.GetState()
		assert.Equal(t, mcp.StateInitialized, state)
	})

	// Test client close.
	t.Run("ClientClose", func(t *testing.T) {
		err := client.Close()
		assert.NoError(t, err)
		assert.Equal(t, mcp.StateDisconnected, client.GetState())
	})
}

// TestSSEClientServer_RootsProvider tests server-to-client roots functionality over SSE transport.
func TestSSEClientServer_RootsProvider(t *testing.T) {
	server, sseEndpoint, cleanup := startSSEServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create SSE Client with RootsProvider capability
	client, err := mcp.NewSSEClient(
		sseEndpoint,
		mcp.Implementation{
			Name:    "roots-sse-test-client",
			Version: "1.0.0",
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	)
	require.NoError(t, err)
	defer client.Close()

	// Set up roots provider with test directories
	rootsProvider := mcp.NewDefaultRootsProvider()
	rootsProvider.AddRoot("/tmp", "Temporary Directory")
	rootsProvider.AddRoot("/var", "Variable Directory")
	client.SetRootsProvider(rootsProvider)

	// Initialize with roots capability
	initResult, err := client.Initialize(ctx, &mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.ProtocolVersion_2025_03_26,
			ClientInfo: mcp.Implementation{
				Name:    "roots-sse-test-client",
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

	// Register a tool that uses ListRoots
	listRootsTool := mcp.NewTool("list-sse-roots",
		mcp.WithDescription("List client's root directories via SSE"),
	)

	server.RegisterTool(listRootsTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Call ListRoots to get client roots
		roots, err := server.ListRoots(ctx)
		if err != nil {
			return mcp.NewErrorResult(fmt.Sprintf("Failed to list roots: %v", err)), nil
		}

		// Format response
		message := fmt.Sprintf("SSE client has %d root directories:\n", len(roots.Roots))
		for i, root := range roots.Roots {
			message += fmt.Sprintf("%d. %s (%s)\n", i+1, root.Name, root.URI)
		}

		return mcp.NewTextResult(message), nil
	})

	// Call the tool to test ListRoots functionality
	result, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "list-sse-roots",
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)

	// Verify the response contains our root directories
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "Temporary Directory")
	assert.Contains(t, textContent.Text, "Variable Directory")
}

// startSSEServer starts an SSE server for testing.
func startSSEServer(t *testing.T) (*mcp.SSEServer, string, func()) {
	t.Helper()

	// Find an available port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	serverAddr := fmt.Sprintf(":%d", port)
	sseEndpoint := fmt.Sprintf("http://localhost:%d/sse", port)

	// Create SSE server.
	server := mcp.NewSSEServer(
		"E2E-SSE-Test-Server", // Server name
		"1.0.0",               // Server version
		mcp.WithSSEServerLogger(mcp.GetDefaultLogger()),
		mcp.WithSSEEndpoint("/sse"),
		mcp.WithMessageEndpoint("/message"),
		mcp.WithBasePath(""),
	)

	// Register test tools.
	registerTestTools(server)

	// Start the server in a goroutine.
	go func() {
		if err := server.Start(serverAddr); err != nil {
			if err.Error() != "http: Server closed" {
				t.Logf("Server error: %v", err)
			}
		}
	}()

	// Wait for server to start.
	waitForServerStart(t, fmt.Sprintf("http://localhost:%d", port))

	// Return cleanup function.
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}

	return server, sseEndpoint, cleanup
}

// waitForServerStart waits for the server to start accepting connections.
func waitForServerStart(t *testing.T, url string) {
	t.Helper()

	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		conn, err := net.DialTimeout("tcp", url[7:], 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Server did not start in time")
}

// registerTestTools registers test tools with the server.
func registerTestTools(server *mcp.SSEServer) {
	// Register greet tool.
	greetTool := mcp.NewTool("greet",
		mcp.WithDescription("Greet a user by name"),
		mcp.WithString("name", mcp.Description("Name of the person to greet")),
	)
	server.RegisterTool(greetTool, handleGreet)

	// Register weather tool.
	weatherTool := mcp.NewTool("weather",
		mcp.WithDescription("Get weather information for a city"),
		mcp.WithString("city", mcp.Description("City name (Beijing, Shanghai, Shenzhen, Guangzhou)")),
	)
	server.RegisterTool(weatherTool, handleWeather)
}

// handleGreet handles greet tool callback.
func handleGreet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract name parameter.
	name := "Client user"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Get session information.
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		// Even if session is not found, still return the greeting.
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Hello, %s!", name)),
			},
		}, nil
	}

	// Try to send notification.
	if server, ok := mcp.GetServerFromContext(ctx).(interface {
		SendNotification(string, string, map[string]interface{}) error
	}); ok {
		// Send notification to current session.
		err := server.SendNotification(
			session.GetID(),
			"greeting.echo",
			map[string]interface{}{
				"message": fmt.Sprintf("Server received greeting for: %s", name),
				"time":    time.Now().Format(time.RFC3339),
			},
		)
		if err != nil {
			fmt.Printf("Failed to send notification: %v\n", err)
		}
	}

	// Return greeting message.
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Hello, %s! (Session ID: %s)",
				name, session.GetID()[:8]+"...",
			)),
		},
	}, nil
}

// handleWeather handles weather tool callback.
func handleWeather(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract city parameter.
	city := "Beijing"
	if cityArg, ok := req.Params.Arguments["city"]; ok {
		if cityStr, ok := cityArg.(string); ok && cityStr != "" {
			city = cityStr
		}
	}

	// Simulate weather information.
	weatherInfo := map[string]string{
		"Beijing":   "Sunny, 25°C",
		"Shanghai":  "Cloudy, 22°C",
		"Shenzhen":  "Rainy, 28°C",
		"Guangzhou": "Partly cloudy, 30°C",
	}

	weather, ok := weatherInfo[city]
	if !ok {
		weather = "Unknown, please check a supported city"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Weather in %s: %s", city, weather)),
		},
	}, nil
}
