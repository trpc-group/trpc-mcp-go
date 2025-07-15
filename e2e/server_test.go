// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// TestServerInfo_EndToEndIntegration tests server info functionality end-to-end
func TestServerInfo_EndToEndIntegration(t *testing.T) {
	// Test parameters
	testServerName := "Integration-Test-Server"
	testServerVersion := "4.0.0"

	// Create server
	server := mcp.NewServer(
		testServerName,
		testServerVersion,
		mcp.WithServerPath("/mcp"),
	)

	// Register a tool that extracts server info from context
	tool := mcp.NewTool("server-info-tool",
		mcp.WithDescription("A tool that demonstrates getting server info from context"),
		mcp.WithString("prefix", mcp.Description("Prefix for the response message")))

	toolHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Get server instance from context
		serverInstance := mcp.GetServerFromContext(ctx)
		if serverInstance == nil {
			return mcp.NewTextResult("Error: No server instance in context"), nil
		}

		// Cast to *Server and get server info
		mcpServer, ok := serverInstance.(*mcp.Server)
		if !ok {
			return mcp.NewTextResult("Error: Failed to cast to *Server"), nil
		}

		// Get server info using our new method
		info := mcpServer.GetServerInfo()

		// Get prefix from parameters
		prefix := "Server Info"
		if prefixArg, ok := req.Params.Arguments["prefix"]; ok {
			if prefixStr, ok := prefixArg.(string); ok && prefixStr != "" {
				prefix = prefixStr
			}
		}

		response := strings.Join([]string{
			prefix + ":",
			"Name: " + info.Name,
			"Version: " + info.Version,
		}, " ")

		return mcp.NewTextResult(response), nil
	}

	server.RegisterTool(tool, toolHandler)

	// Create HTTP test server
	httpServer := httptest.NewServer(server.HTTPHandler())
	defer httpServer.Close()

	// Create client
	client, err := mcp.NewClient(
		httpServer.URL+"/mcp",
		mcp.Implementation{
			Name:    "Test-Client",
			Version: "1.0.0",
		},
	)
	require.NoError(t, err)
	defer client.Close()

	// Initialize connection
	initResp, err := client.Initialize(context.Background(), &mcp.InitializeRequest{})
	require.NoError(t, err)

	// Verify server info in initialization response
	assert.Equal(t, testServerName, initResp.ServerInfo.Name)
	assert.Equal(t, testServerVersion, initResp.ServerInfo.Version)

	// Test tool call that extracts server info from context
	toolResult, err := client.CallTool(context.Background(), &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "server-info-tool",
			Arguments: map[string]interface{}{
				"prefix": "Test Prefix",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, toolResult.Content, 1)

	// Verify the result contains server info
	var textContent *mcp.TextContent
	switch content := toolResult.Content[0].(type) {
	case *mcp.TextContent:
		textContent = content
	case mcp.TextContent:
		textContent = &content
	default:
		t.Fatalf("Expected text content, got %T", content)
	}

	expectedResponse := "Test Prefix: Name: " + testServerName + " Version: " + testServerVersion
	assert.Equal(t, expectedResponse, textContent.Text)

	// Also test without prefix parameter
	toolResult2, err := client.CallTool(context.Background(), &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "server-info-tool",
			Arguments: map[string]interface{}{},
		},
	})
	require.NoError(t, err)
	require.Len(t, toolResult2.Content, 1)

	// Verify the result with default prefix
	switch content := toolResult2.Content[0].(type) {
	case *mcp.TextContent:
		textContent = content
	case mcp.TextContent:
		textContent = &content
	default:
		t.Fatalf("Expected text content, got %T", content)
	}

	expectedDefaultResponse := "Server Info: Name: " + testServerName + " Version: " + testServerVersion
	assert.Equal(t, expectedDefaultResponse, textContent.Text)
}

// TestServerInfo_StatelessMode tests server info functionality in stateless mode
func TestServerInfo_StatelessMode(t *testing.T) {
	testServerName := "Stateless-Test-Server"
	testServerVersion := "5.0.0"

	// Create server in stateless mode
	server := mcp.NewServer(
		testServerName,
		testServerVersion,
		mcp.WithServerPath("/mcp"),
		mcp.WithStatelessMode(true), // Enable stateless mode
	)

	// Register the same server info tool
	tool := mcp.NewTool("stateless-info-tool", mcp.WithDescription("Server info tool for stateless mode"))

	toolHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Get server instance from context (should work even in stateless mode)
		serverInstance := mcp.GetServerFromContext(ctx)
		if serverInstance == nil {
			return mcp.NewTextResult("Error: No server instance in context"), nil
		}

		mcpServer, ok := serverInstance.(*mcp.Server)
		if !ok {
			return mcp.NewTextResult("Error: Failed to cast to *Server"), nil
		}

		info := mcpServer.GetServerInfo()
		response := "Stateless Mode - Name: " + info.Name + " Version: " + info.Version

		return mcp.NewTextResult(response), nil
	}

	server.RegisterTool(tool, toolHandler)

	// Create HTTP test server
	httpServer := httptest.NewServer(server.HTTPHandler())
	defer httpServer.Close()

	// Create client
	client, err := mcp.NewClient(
		httpServer.URL+"/mcp",
		mcp.Implementation{
			Name:    "Stateless-Test-Client",
			Version: "1.0.0",
		},
	)
	require.NoError(t, err)
	defer client.Close()

	// Initialize connection
	_, err = client.Initialize(context.Background(), &mcp.InitializeRequest{})
	require.NoError(t, err)

	// Test tool call in stateless mode
	toolResult, err := client.CallTool(context.Background(), &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "stateless-info-tool",
			Arguments: map[string]interface{}{},
		},
	})
	require.NoError(t, err)
	require.Len(t, toolResult.Content, 1)

	// Verify the result
	var textContent *mcp.TextContent
	switch content := toolResult.Content[0].(type) {
	case *mcp.TextContent:
		textContent = content
	case mcp.TextContent:
		textContent = &content
	default:
		t.Fatalf("Expected text content, got %T", content)
	}

	expectedResponse := "Stateless Mode - Name: " + testServerName + " Version: " + testServerVersion
	assert.Equal(t, expectedResponse, textContent.Text)
}
