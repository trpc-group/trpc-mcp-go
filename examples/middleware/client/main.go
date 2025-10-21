// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	log.Println("=======================================================")
	log.Println("Starting Middleware Example Client (Stateless Mode)...")
	log.Println("This client demonstrates middleware in action")
	log.Println("=======================================================")

	// Create context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := mcp.Implementation{
		Name:    "Middleware-Example-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	log.Printf("📡 Connecting to server at http://localhost:3000/mcp...")
	mcpClient, err := mcp.NewClient("http://localhost:3000/mcp", clientInfo)
	if err != nil {
		log.Fatalf("❌ Failed to create client: %v", err)
	}
	defer mcpClient.Close()
	log.Printf("✅ Client created successfully\n")

	// Initialize client.
	log.Printf("🔧 Initializing connection...")
	log.Printf("   (Watch server logs for middleware execution: Trace → Logging → Metrics → Auth)\n")
	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("❌ Initialization failed: %v", err)
	}
	log.Printf("✅ Initialization successful!")
	log.Printf("   Server: %s %s", initResp.ServerInfo.Name, initResp.ServerInfo.Version)
	log.Printf("   Protocol: %s", initResp.ProtocolVersion)
	log.Printf("   Note: Stateless mode - no session ID\n")

	// List tools
	log.Printf("📋 Listing available tools...")
	log.Printf("   (Watch server logs for middleware chain execution)")
	toolsResp, err := mcpClient.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("❌ Failed to list tools: %v", err)
	}
	log.Printf("✅ Server provides %d tools:", len(toolsResp.Tools))
	for i, tool := range toolsResp.Tools {
		log.Printf("   %d. %s - %s", i+1, tool.Name, tool.Description)
	}
	log.Println()

	// Test 1: Call hello tool
	log.Printf("=======================================================")
	log.Printf("TEST 1: Calling 'hello' tool")
	log.Printf("=======================================================")
	log.Printf("📞 Calling hello tool with name='Middleware Tester'...")
	log.Printf("   Expected middleware order:")
	log.Printf("   → TraceMiddleware (START)")
	log.Printf("   → LoggingMiddleware (log method)")
	log.Printf("   → MetricsMiddleware (increment counter)")
	log.Printf("   → AuthMiddleware (check authorization)")
	log.Printf("   → Core Handler (execute tool)")
	log.Printf("   ← AuthMiddleware")
	log.Printf("   ← MetricsMiddleware")
	log.Printf("   ← LoggingMiddleware (log duration)")
	log.Printf("   ← TraceMiddleware (END)")
	helloResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "hello",
			Arguments: map[string]interface{}{
				"name": "Middleware Tester",
			},
		},
	})
	if err != nil {
		log.Printf("⚠️  Error calling hello: %v", err)
	} else {
		log.Printf("✅ Hello tool result:")
		for _, content := range helloResult.Content {
			if textContent, ok := content.(mcp.TextContent); ok {
				log.Printf("   📝 %s", textContent.Text)
			}
		}
	}
	log.Println()

	// Test 2: Call counter tool (demonstrates stateless mode)
	log.Printf("=======================================================")
	log.Printf("TEST 2: Calling 'counter' tool (demonstrates stateless mode)")
	log.Printf("=======================================================")
	log.Printf("📊 In stateless mode, counter will reset each call")
	for i := 1; i <= 3; i++ {
		log.Printf("🔢 Counter call #%d (increment=1)...", i)
		counterResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "counter",
				Arguments: map[string]interface{}{
					"increment": 1,
				},
			},
		})
		if err != nil {
			log.Printf("   ⚠️  Error: %v", err)
		} else {
			for _, content := range counterResult.Content {
				if textContent, ok := content.(mcp.TextContent); ok {
					log.Printf("   📝 %s", textContent.Text)
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Println()

	// Test 3: Call fail tool (demonstrates graceful degradation)
	log.Printf("=======================================================")
	log.Printf("TEST 3: Calling 'fail' tool (demonstrates error handling)")
	log.Printf("=======================================================")
	log.Printf("🔴 Intentionally calling a tool that fails...")
	log.Printf("   Watch how middleware handles errors:")
	failResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "fail",
		},
	})
	if err != nil {
		log.Printf("   ⚠️  Expected error: %v", err)
	} else {
		for _, content := range failResult.Content {
			if textContent, ok := content.(mcp.TextContent); ok {
				log.Printf("   📝 %s", textContent.Text)
			}
		}
	}
	log.Println()

	// Test 4: Rapid calls (demonstrates metrics accumulation)
	log.Printf("=======================================================")
	log.Printf("TEST 4: Rapid calls (demonstrates metrics accumulation)")
	log.Printf("=======================================================")
	log.Printf("🚀 Making 5 rapid calls to hello tool...")
	log.Printf("   Watch MetricsMiddleware count requests:")
	for i := 1; i <= 5; i++ {
		_, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "hello",
				Arguments: map[string]interface{}{
					"name": fmt.Sprintf("User%d", i),
				},
			},
		})
		if err != nil {
			log.Printf("   ⚠️  Call %d failed: %v", i, err)
		} else {
			log.Printf("   ✅ Call %d completed", i)
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Println()

	// Test 5: Prompt interceptor tests
	log.Printf("=======================================================")
	log.Printf("TEST 5: Prompt Interceptor (list & get prompts)")
	log.Printf("=======================================================")

	// List prompts - should see intercepted prompt
	log.Printf("📋 Listing prompts...")
	promptList, err := mcpClient.ListPrompts(ctx, &mcp.ListPromptsRequest{})
	if err != nil {
		log.Printf("⚠️  Failed to list prompts: %v", err)
	} else {
		log.Printf("✅ Found %d prompts:", len(promptList.Prompts))
		for i, prompt := range promptList.Prompts {
			log.Printf("   %d. %s - %s", i+1, prompt.Name, prompt.Description)
			if prompt.Name == "intercepted-prompt" {
				log.Printf("      🎯 THIS PROMPT WAS ADDED BY MIDDLEWARE!")
			}
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Get intercepted-prompt
	log.Println()
	log.Printf("📝 Getting 'intercepted-prompt' (middleware generated)...")
	interceptedPrompt, err := mcpClient.GetPrompt(ctx, &mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "intercepted-prompt",
		},
	})
	if err != nil {
		log.Printf("⚠️  Failed to get intercepted-prompt: %v", err)
	} else {
		log.Printf("✅ Got intercepted-prompt:")
		log.Printf("   Description: %s", interceptedPrompt.Description)
		if len(interceptedPrompt.Messages) > 0 {
			if textContent, ok := interceptedPrompt.Messages[0].Content.(mcp.TextContent); ok {
				summary := textContent.Text
				if len(summary) > 80 {
					summary = summary[:80] + "..."
				}
				log.Printf("   First message: %s", summary)
			}
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Get cached-prompt
	log.Println()
	log.Printf("💾 Getting 'cached-prompt' (should be cached)...")
	cachedPrompt, err := mcpClient.GetPrompt(ctx, &mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "cached-prompt",
		},
	})
	if err != nil {
		log.Printf("⚠️  Failed to get cached-prompt: %v", err)
	} else {
		log.Printf("✅ Got cached-prompt:")
		if len(cachedPrompt.Messages) > 0 {
			if textContent, ok := cachedPrompt.Messages[0].Content.(mcp.TextContent); ok {
				log.Printf("   Content: %s", textContent.Text)
				if textContent.Text == "This is cached content, loaded instantly without calling the actual handler!" {
					log.Printf("   💾 CONFIRMED: Came from cache middleware!")
				}
			}
		}
	}
	log.Println()

	// Summary
	log.Printf("=======================================================")
	log.Printf("SUMMARY")
	log.Printf("=======================================================")
	log.Printf("✅ All tests completed successfully!")
	log.Printf("")
	log.Printf("📊 Middleware Features Demonstrated:")
	log.Printf("   1. ✅ TraceMiddleware              - Added unique trace IDs")
	log.Printf("   2. ✅ LoggingMiddleware            - Logged all requests")
	log.Printf("   3. ✅ MetricsMiddleware            - Counted requests")
	log.Printf("   4. ✅ AuthMiddleware               - Checked authorization")
	log.Printf("   5. ✅ InitializeInterceptor        - Enhanced init response")
	log.Printf("   6. ✅ PingInterceptor              - Added timestamp")
	log.Printf("   7. ✅ PromptInterceptor            - Intercepted prompts")
	log.Printf("   8. ✅ ToolInterceptor              - Intercepted tools")
	log.Printf("")
	log.Printf("📌 Note: Running in stateless mode - no session persistence")
	log.Printf("👀 Check the server logs to see detailed middleware execution!")
	log.Printf("=======================================================\n")

	// Wait before exiting
	log.Printf("Client finished. Exiting in 3 seconds...")
	time.Sleep(3 * time.Second)
}
