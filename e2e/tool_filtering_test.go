// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// TestToolFiltering_BasicFiltering tests basic tool filtering functionality end-to-end
func TestToolFiltering_BasicFiltering(t *testing.T) {
	// Create a simple context-based filter (using role as an example)
	roleBasedFilter := func(ctx context.Context, tools []*mcp.Tool) []*mcp.Tool {
		// Simple logic: filter based on X-User-Role header stored in context
		userRole := "guest" // default

		// Try to extract role from context (simplified approach)
		if role, ok := ctx.Value("user_role").(string); ok && role != "" {
			userRole = role
		}

		var filtered []*mcp.Tool
		for _, tool := range tools {
			switch userRole {
			case "admin":
				// Admin sees all tools
				filtered = append(filtered, tool)
			case "user":
				// User sees calculator and weather
				if tool.Name == "calculator" || tool.Name == "weather" {
					filtered = append(filtered, tool)
				}
			case "guest":
				// Guest sees only calculator
				if tool.Name == "calculator" {
					filtered = append(filtered, tool)
				}
			}
		}
		return filtered
	}

	// Create context extractor for headers
	headerExtractor := func(ctx context.Context, r *http.Request) context.Context {
		if userRole := r.Header.Get("X-User-Role"); userRole != "" {
			ctx = context.WithValue(ctx, "user_role", userRole)
		}
		return ctx
	}

	// Create server with tool filtering
	server := mcp.NewServer(
		"Tool-Filtering-Test-Server",
		"1.0.0",
		mcp.WithServerPath("/mcp"),
		mcp.WithHTTPContextFunc(headerExtractor),
		mcp.WithToolListFilter(roleBasedFilter),
	)

	// Register test tools
	registerBasicTestTools(server)

	// Create HTTP test server
	httpServer := httptest.NewServer(server.HTTPHandler())
	defer httpServer.Close()

	// Test different user roles
	testCases := []struct {
		name          string
		userRole      string
		expectedTools []string
	}{
		{
			name:          "Admin sees all tools",
			userRole:      "admin",
			expectedTools: []string{"calculator", "weather", "admin_panel"},
		},
		{
			name:          "User sees filtered tools",
			userRole:      "user",
			expectedTools: []string{"calculator", "weather"},
		},
		{
			name:          "Guest sees limited tools",
			userRole:      "guest",
			expectedTools: []string{"calculator"},
		},
		{
			name:          "No role header defaults to guest level",
			userRole:      "",
			expectedTools: []string{"calculator"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create client with role-specific headers
			headers := make(http.Header)
			if tc.userRole != "" {
				headers.Set("X-User-Role", tc.userRole)
			}

			client, err := mcp.NewClient(
				httpServer.URL+"/mcp",
				mcp.Implementation{
					Name:    "Test-Client",
					Version: "1.0.0",
				},
				mcp.WithHTTPHeaders(headers),
			)
			require.NoError(t, err)
			defer client.Close()

			// Initialize connection
			_, err = client.Initialize(context.Background(), &mcp.InitializeRequest{})
			require.NoError(t, err)

			// List tools
			toolsResp, err := client.ListTools(context.Background(), &mcp.ListToolsRequest{})
			require.NoError(t, err)

			// Verify filtered tools
			assert.Len(t, toolsResp.Tools, len(tc.expectedTools), "Expected %d tools for role %s", len(tc.expectedTools), tc.userRole)

			toolNames := make([]string, len(toolsResp.Tools))
			for i, tool := range toolsResp.Tools {
				toolNames[i] = tool.Name
			}

			for _, expectedTool := range tc.expectedTools {
				assert.Contains(t, toolNames, expectedTool, "Expected tool %s to be visible for role %s", expectedTool, tc.userRole)
			}
		})
	}
}

// TestToolFiltering_StatelessMode tests tool filtering in stateless mode
func TestToolFiltering_StatelessMode(t *testing.T) {
	// Simple filter for stateless mode testing
	simpleFilter := func(ctx context.Context, tools []*mcp.Tool) []*mcp.Tool {
		// In stateless mode, we can still filter based on context values
		userRole := "guest"
		if role, ok := ctx.Value("user_role").(string); ok && role != "" {
			userRole = role
		}

		var filtered []*mcp.Tool
		for _, tool := range tools {
			if userRole == "user" && (tool.Name == "calculator" || tool.Name == "weather") {
				filtered = append(filtered, tool)
			} else if userRole == "guest" && tool.Name == "calculator" {
				filtered = append(filtered, tool)
			}
		}
		return filtered
	}

	server := mcp.NewServer(
		"Stateless-Filtering-Test-Server",
		"1.0.0",
		mcp.WithServerPath("/mcp"),
		mcp.WithStatelessMode(true), // Enable stateless mode
		mcp.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			if userRole := r.Header.Get("X-User-Role"); userRole != "" {
				ctx = context.WithValue(ctx, "user_role", userRole)
			}
			return ctx
		}),
		mcp.WithToolListFilter(simpleFilter),
	)

	registerBasicTestTools(server)

	httpServer := httptest.NewServer(server.HTTPHandler())
	defer httpServer.Close()

	// Test that filtering works in stateless mode
	headers := make(http.Header)
	headers.Set("X-User-Role", "user")

	client, err := mcp.NewClient(
		httpServer.URL+"/mcp",
		mcp.Implementation{
			Name:    "Test-Client",
			Version: "1.0.0",
		},
		mcp.WithHTTPHeaders(headers),
	)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.Initialize(context.Background(), &mcp.InitializeRequest{})
	require.NoError(t, err)

	toolsResp, err := client.ListTools(context.Background(), &mcp.ListToolsRequest{})
	require.NoError(t, err)

	// User should see calculator and weather
	assert.Len(t, toolsResp.Tools, 2)

	toolNames := make([]string, len(toolsResp.Tools))
	for i, tool := range toolsResp.Tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "calculator")
	assert.Contains(t, toolNames, "weather")
}

// registerBasicTestTools registers a minimal set of tools for testing
func registerBasicTestTools(server *mcp.Server) {
	// Calculator tool - basic tool available to most users
	calculatorTool := mcp.NewTool("calculator",
		mcp.WithDescription("A simple calculator tool."))

	calculatorHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewTextResult("Calculator result"), nil
	}

	server.RegisterTool(calculatorTool, calculatorHandler)

	// Weather tool - available to users and admins
	weatherTool := mcp.NewTool("weather",
		mcp.WithDescription("Get weather information."))

	weatherHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewTextResult("Weather result"), nil
	}

	server.RegisterTool(weatherTool, weatherHandler)

	// Admin tool - only available to admins
	adminTool := mcp.NewTool("admin_panel",
		mcp.WithDescription("Administrative functions."))

	adminHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewTextResult("Admin panel result"), nil
	}

	server.RegisterTool(adminTool, adminHandler)
}
