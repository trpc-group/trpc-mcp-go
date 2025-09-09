// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
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

// TestSSEFilterOptions tests that SSE server filter options compile and are accepted
func TestSSEFilterOptions(t *testing.T) {
	// Test that all SSE filter options can be created and used without compilation errors
	toolFilter := func(ctx context.Context, tools []*mcp.Tool) []*mcp.Tool {
		if len(tools) > 0 {
			return tools[:1] // Return only first tool if available
		}
		return tools
	}

	promptFilter := func(ctx context.Context, prompts []*mcp.Prompt) []*mcp.Prompt {
		if len(prompts) > 0 {
			return prompts[:1] // Return only first prompt if available
		}
		return prompts
	}

	resourceFilter := func(ctx context.Context, resources []*mcp.Resource) []*mcp.Resource {
		if len(resources) > 0 {
			return resources[:1] // Return only first resource if available
		}
		return resources
	}

	// This test verifies that the SSE server can be created with all filter options
	// without runtime errors
	server := mcp.NewSSEServer(
		"Test-SSE-Server-With-Filters",
		"1.0.0",
		mcp.WithSSEToolListFilter(toolFilter),
		mcp.WithSSEPromptListFilter(promptFilter),
		mcp.WithSSEResourceListFilter(resourceFilter),
	)

	// Verify server was created successfully
	assert.NotNil(t, server, "SSE server with filters should be created successfully")

	// Verify we can register tools/prompts/resources (this tests that filters don't break registration)
	tool := mcp.NewTool("test_tool", mcp.WithDescription("Test tool"))
	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewTextResult("Test result"), nil
	}

	// This should not panic
	assert.NotPanics(t, func() {
		server.RegisterTool(tool, handler)
	}, "RegisterTool should not panic with filters enabled")

	prompt := &mcp.Prompt{
		Name:        "test_prompt",
		Description: "Test prompt",
	}
	promptHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "Test prompt result",
			Messages: []mcp.PromptMessage{
				{
					Role:    "user",
					Content: mcp.NewTextContent("Test message"),
				},
			},
		}, nil
	}

	assert.NotPanics(t, func() {
		server.RegisterPrompt(prompt, promptHandler)
	}, "RegisterPrompt should not panic with filters enabled")

	resource := &mcp.Resource{
		Name: "test_resource",
		URI:  "test://resource",
	}
	resourceHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) (mcp.ResourceContents, error) {
		return mcp.TextResourceContents{
			URI:  req.Params.URI,
			Text: "Test resource content",
		}, nil
	}

	assert.NotPanics(t, func() {
		server.RegisterResource(resource, resourceHandler)
	}, "RegisterResource should not panic with filters enabled")
}

// TestUnifiedFiltering tests that filtering works consistently across tools, prompts, and resources
func TestUnifiedFiltering(t *testing.T) {
	// Create role-based filters for all three types
	toolFilter := func(ctx context.Context, tools []*mcp.Tool) []*mcp.Tool {
		userRole := getUserRole(ctx)
		var filtered []*mcp.Tool
		for _, tool := range tools {
			switch userRole {
			case "admin":
				filtered = append(filtered, tool)
			case "user":
				if tool.Name == "calculator" || tool.Name == "weather" {
					filtered = append(filtered, tool)
				}
			case "guest":
				if tool.Name == "calculator" {
					filtered = append(filtered, tool)
				}
			}
		}
		return filtered
	}

	promptFilter := func(ctx context.Context, prompts []*mcp.Prompt) []*mcp.Prompt {
		userRole := getUserRole(ctx)
		var filtered []*mcp.Prompt
		for _, prompt := range prompts {
			switch userRole {
			case "admin":
				filtered = append(filtered, prompt)
			case "user":
				if prompt.Name == "greeting" || prompt.Name == "summary" {
					filtered = append(filtered, prompt)
				}
			case "guest":
				if prompt.Name == "greeting" {
					filtered = append(filtered, prompt)
				}
			}
		}
		return filtered
	}

	resourceFilter := func(ctx context.Context, resources []*mcp.Resource) []*mcp.Resource {
		userRole := getUserRole(ctx)
		var filtered []*mcp.Resource
		for _, resource := range resources {
			switch userRole {
			case "admin":
				filtered = append(filtered, resource)
			case "user":
				if resource.Name == "public_data" || resource.Name == "user_data" {
					filtered = append(filtered, resource)
				}
			case "guest":
				if resource.Name == "public_data" {
					filtered = append(filtered, resource)
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

	// Create server with all filters
	server := mcp.NewServer(
		"Unified-Filtering-Test-Server",
		"1.0.0",
		mcp.WithServerPath("/mcp"),
		mcp.WithHTTPContextFunc(headerExtractor),
		mcp.WithToolListFilter(toolFilter),
		mcp.WithPromptListFilter(promptFilter),
		mcp.WithResourceListFilter(resourceFilter),
	)

	// Register test tools
	registerUnifiedTestTools(server)
	// Register test prompts
	registerUnifiedTestPrompts(server)
	// Register test resources
	registerUnifiedTestResources(server)

	// Create HTTP test server
	httpServer := httptest.NewServer(server.HTTPHandler())
	defer httpServer.Close()

	// Test different user roles
	testCases := []struct {
		name              string
		userRole          string
		expectedTools     []string
		expectedPrompts   []string
		expectedResources []string
	}{
		{
			name:              "Admin sees all items",
			userRole:          "admin",
			expectedTools:     []string{"calculator", "weather", "admin_panel"},
			expectedPrompts:   []string{"greeting", "summary", "admin_report"},
			expectedResources: []string{"public_data", "user_data", "admin_config"},
		},
		{
			name:              "User sees filtered items",
			userRole:          "user",
			expectedTools:     []string{"calculator", "weather"},
			expectedPrompts:   []string{"greeting", "summary"},
			expectedResources: []string{"public_data", "user_data"},
		},
		{
			name:              "Guest sees limited items",
			userRole:          "guest",
			expectedTools:     []string{"calculator"},
			expectedPrompts:   []string{"greeting"},
			expectedResources: []string{"public_data"},
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

			// Test tools filtering
			toolsResp, err := client.ListTools(context.Background(), &mcp.ListToolsRequest{})
			require.NoError(t, err)
			assert.Len(t, toolsResp.Tools, len(tc.expectedTools), "Expected %d tools for role %s", len(tc.expectedTools), tc.userRole)

			toolNames := make([]string, len(toolsResp.Tools))
			for i, tool := range toolsResp.Tools {
				toolNames[i] = tool.Name
			}
			for _, expectedTool := range tc.expectedTools {
				assert.Contains(t, toolNames, expectedTool, "Expected tool %s to be visible for role %s", expectedTool, tc.userRole)
			}

			// Test prompts filtering
			promptsResp, err := client.ListPrompts(context.Background(), &mcp.ListPromptsRequest{})
			require.NoError(t, err)
			assert.Len(t, promptsResp.Prompts, len(tc.expectedPrompts), "Expected %d prompts for role %s", len(tc.expectedPrompts), tc.userRole)

			promptNames := make([]string, len(promptsResp.Prompts))
			for i, prompt := range promptsResp.Prompts {
				promptNames[i] = prompt.Name
			}
			for _, expectedPrompt := range tc.expectedPrompts {
				assert.Contains(t, promptNames, expectedPrompt, "Expected prompt %s to be visible for role %s", expectedPrompt, tc.userRole)
			}

			// Test resources filtering
			resourcesResp, err := client.ListResources(context.Background(), &mcp.ListResourcesRequest{})
			require.NoError(t, err)
			assert.Len(t, resourcesResp.Resources, len(tc.expectedResources), "Expected %d resources for role %s", len(tc.expectedResources), tc.userRole)

			resourceNames := make([]string, len(resourcesResp.Resources))
			for i, resource := range resourcesResp.Resources {
				resourceNames[i] = resource.Name
			}
			for _, expectedResource := range tc.expectedResources {
				assert.Contains(t, resourceNames, expectedResource, "Expected resource %s to be visible for role %s", expectedResource, tc.userRole)
			}
		})
	}
}

// getUserRole extracts user role from context
func getUserRole(ctx context.Context) string {
	if role, ok := ctx.Value("user_role").(string); ok && role != "" {
		return role
	}
	return "guest"
}

// registerUnifiedTestTools registers tools for unified filtering test
func registerUnifiedTestTools(server *mcp.Server) {
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

// registerUnifiedTestPrompts registers prompts for unified filtering test
func registerUnifiedTestPrompts(server *mcp.Server) {
	// Greeting prompt - basic prompt available to most users
	greetingPrompt := &mcp.Prompt{
		Name:        "greeting",
		Description: "A simple greeting prompt.",
		Arguments: []mcp.PromptArgument{
			{Name: "name", Description: "Name to greet", Required: true},
		},
	}
	greetingHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "Greeting prompt result",
			Messages: []mcp.PromptMessage{
				{
					Role:    "user",
					Content: mcp.NewTextContent("Hello!"),
				},
			},
		}, nil
	}
	server.RegisterPrompt(greetingPrompt, greetingHandler)

	// Summary prompt - available to users and admins
	summaryPrompt := &mcp.Prompt{
		Name:        "summary",
		Description: "Generate a summary.",
		Arguments: []mcp.PromptArgument{
			{Name: "text", Description: "Text to summarize", Required: true},
		},
	}
	summaryHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "Summary prompt result",
			Messages: []mcp.PromptMessage{
				{
					Role:    "user",
					Content: mcp.NewTextContent("Summary generated"),
				},
			},
		}, nil
	}
	server.RegisterPrompt(summaryPrompt, summaryHandler)

	// Admin report prompt - only available to admins
	adminPrompt := &mcp.Prompt{
		Name:        "admin_report",
		Description: "Generate administrative reports.",
		Arguments: []mcp.PromptArgument{
			{Name: "type", Description: "Report type", Required: true},
		},
	}
	adminPromptHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "Admin report result",
			Messages: []mcp.PromptMessage{
				{
					Role:    "user",
					Content: mcp.NewTextContent("Admin report generated"),
				},
			},
		}, nil
	}
	server.RegisterPrompt(adminPrompt, adminPromptHandler)
}

// registerUnifiedTestResources registers resources for unified filtering test
func registerUnifiedTestResources(server *mcp.Server) {
	// Public data resource - available to all users
	publicResource := &mcp.Resource{
		Name:        "public_data",
		URI:         "file://public/data.txt",
		Description: "Public data accessible to everyone.",
		MimeType:    "text/plain",
	}
	publicHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) (mcp.ResourceContents, error) {
		return mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "text/plain",
			Text:     "Public data content",
		}, nil
	}
	server.RegisterResource(publicResource, publicHandler)

	// User data resource - available to users and admins
	userResource := &mcp.Resource{
		Name:        "user_data",
		URI:         "file://user/data.txt",
		Description: "User data accessible to authenticated users.",
		MimeType:    "text/plain",
	}
	userHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) (mcp.ResourceContents, error) {
		return mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "text/plain",
			Text:     "User data content",
		}, nil
	}
	server.RegisterResource(userResource, userHandler)

	// Admin config resource - only available to admins
	adminResource := &mcp.Resource{
		Name:        "admin_config",
		URI:         "file://admin/config.json",
		Description: "Administrative configuration.",
		MimeType:    "application/json",
	}
	adminResourceHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) (mcp.ResourceContents, error) {
		return mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     `{"admin": "config"}`,
		}, nil
	}
	server.RegisterResource(adminResource, adminResourceHandler)
}
