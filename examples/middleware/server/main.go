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
	"os"
	"os/signal"
	"syscall"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// LoggingMiddleware logs request method and execution time
func LoggingMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
	return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
		// Get session from context if available
		var sessionID string
		if session, ok := mcp.GetSessionFromContext(ctx); ok && session != nil {
			sessionID = session.GetID()
			if len(sessionID) > 8 {
				sessionID = sessionID[:8] + "..."
			}
		} else {
			sessionID = "no-session"
		}

		log.Printf("[Logging] → Session: %s, Method: %s", sessionID, req.Method)

		// Record start time
		start := time.Now()

		// Call next handler
		result, err := next(ctx, req)

		// Log duration and error status
		duration := time.Since(start)
		if err != nil {
			log.Printf("[Logging] ← Method: %s, Duration: %v, Error: %v", req.Method, duration, err)
		} else {
			log.Printf("[Logging] ← Method: %s, Duration: %v, Success", req.Method, duration)
		}

		return result, err
	}
}

// MetricsMiddleware collects metrics for each request
func MetricsMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
	requestCount := make(map[string]int)
	errorCount := make(map[string]int)

	return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
		// Increment request counter
		requestCount[req.Method]++
		log.Printf("[Metrics] Request count for %s: %d", req.Method, requestCount[req.Method])

		// Call next handler
		result, err := next(ctx, req)

		// Track errors
		if err != nil {
			errorCount[req.Method]++
			log.Printf("[Metrics] Error count for %s: %d", req.Method, errorCount[req.Method])
		}

		return result, err
	}
}

// AuthMiddleware checks if requests are authorized (simple example)
func AuthMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
	return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
		// Skip auth for initialize, ping, and list methods
		if req.Method == mcp.MethodInitialize ||
			req.Method == mcp.MethodPing ||
			req.Method == mcp.MethodToolsList {
			return next(ctx, req)
		}

		// Check if session exists
		session, ok := mcp.GetSessionFromContext(ctx)
		if !ok || session == nil {
			log.Printf("[Auth] ✗ Unauthorized: no session for method %s", req.Method)
			// In a real implementation, you might want to return an error
			// For demo purposes, we just log and continue
			log.Printf("[Auth] ! Continuing without session (demo mode)")
			return next(ctx, req)
		}

		log.Printf("[Auth] ✓ Authorized: session %s for method %s", session.GetID()[:8]+"...", req.Method)
		return next(ctx, req)
	}
}

// TraceMiddleware adds tracing information
func TraceMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
	return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
		// Generate trace ID (simplified)
		traceID := fmt.Sprintf("trace-%d", time.Now().UnixNano())
		log.Printf("[Trace] %s | START | Method: %s", traceID, req.Method)

		result, err := next(ctx, req)

		log.Printf("[Trace] %s | END   | Method: %s", traceID, req.Method)
		return result, err
	}
}

// InitializeInterceptorMiddleware intercepts initialize requests and enhances the response
// This demonstrates how to modify initialization results
func InitializeInterceptorMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
	return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
		// Only intercept initialize requests
		if req.Method != mcp.MethodInitialize {
			return next(ctx, req)
		}

		log.Printf("[InitInterceptor] 🚀 Intercepting initialize request")

		// Call the original handler to get the standard response
		result, err := next(ctx, req)
		if err != nil {
			return nil, err
		}

		// Enhance the initialize result
		if initResult, ok := result.(*mcp.InitializeResult); ok {
			// Add custom instructions
			originalInstructions := initResult.Instructions
			initResult.Instructions = "🎯 [ENHANCED BY MIDDLEWARE]\n\n" +
				"This server has been enhanced with middleware capabilities:\n" +
				"- Tool interception for mocking/caching/degradation\n" +
				"- Initialize request enhancement\n" +
				"- Ping request timestamping\n\n" +
				"Original instructions:\n" + originalInstructions

			// Modify server info
			initResult.ServerInfo.Name = "Enhanced-" + initResult.ServerInfo.Name

			log.Printf("[InitInterceptor] ✅ Enhanced initialize response")
		}

		return result, err
	}
}

// PingInterceptorMiddleware adds timestamp to ping responses
// This demonstrates intercepting simple requests
func PingInterceptorMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
	return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
		// Only intercept ping requests
		if req.Method != mcp.MethodPing {
			return next(ctx, req)
		}

		log.Printf("[PingInterceptor] 🏓 Intercepting ping request")

		// Return custom ping response with timestamp
		return map[string]interface{}{
			"_intercepted": true,
			"timestamp":    time.Now().Unix(),
			"message":      "Pong from intercepted middleware!",
		}, nil
	}
}

// PromptInterceptorMiddleware intercepts prompt requests
// This demonstrates how to intercept prompts/list and prompts/get
func PromptInterceptorMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
	return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
		// Intercept prompts/list
		if req.Method == mcp.MethodPromptsList {
			log.Printf("[PromptInterceptor] 📋 Intercepting prompts/list request")

			// Option 1: Call original handler and enhance the result
			result, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Add an extra intercepted prompt to the list
			if promptList, ok := result.(*mcp.ListPromptsResult); ok {
				promptList.Prompts = append(promptList.Prompts, mcp.Prompt{
					Name:        "intercepted-prompt",
					Description: "🎯 This prompt was added by middleware!",
					Arguments: []mcp.PromptArgument{
						{
							Name:        "dynamic_topic",
							Description: "A dynamically added parameter",
							Required:    false,
						},
					},
				})
				log.Printf("[PromptInterceptor] ✅ Added 1 intercepted prompt to the list")
			}

			return result, err
		}

		// Intercept prompts/get
		if req.Method == mcp.MethodPromptsGet {
			// Parse the request to get prompt name
			var getPromptReq mcp.GetPromptRequest
			if params, ok := req.Params.(map[string]interface{}); ok {
				if name, ok := params["name"].(string); ok {
					getPromptReq.Params.Name = name
				}
			}

			log.Printf("[PromptInterceptor] 📝 Intercepting prompts/get for: %s", getPromptReq.Params.Name)

			// Intercept specific prompt
			if getPromptReq.Params.Name == "intercepted-prompt" {
				// Return completely custom prompt content
				customPrompt := &mcp.GetPromptResult{
					Description: "🎯 This is a dynamically generated prompt by middleware",
					Messages: []mcp.PromptMessage{
						{
							Role: mcp.RoleUser,
							Content: mcp.TextContent{
								Type: "text",
								Text: "This prompt content was generated by middleware, not from a registered handler!\n\nYou can use this to:\n- Generate prompts dynamically\n- Load prompts from external sources\n- Apply A/B testing on prompts\n- Cache prompt content",
							},
						},
						{
							Role: mcp.RoleAssistant,
							Content: mcp.TextContent{
								Type: "text",
								Text: "I understand! This prompt came from the middleware layer.",
							},
						},
					},
				}
				log.Printf("[PromptInterceptor] ✅ Returned intercepted prompt content")
				return customPrompt, nil
			}

			// For "cached-prompt", simulate cached response
			if getPromptReq.Params.Name == "cached-prompt" {
				log.Printf("[PromptInterceptor] 💾 Returning cached prompt content")
				cachedPrompt := &mcp.GetPromptResult{
					Description: "💾 This prompt was served from cache",
					Messages: []mcp.PromptMessage{
						{
							Role: mcp.RoleUser,
							Content: mcp.TextContent{
								Type: "text",
								Text: "This is cached content, loaded instantly without calling the actual handler!",
							},
						},
					},
				}
				return cachedPrompt, nil
			}
		}

		// For all other cases, continue to next handler
		return next(ctx, req)
	}
}

// ToolInterceptorMiddleware intercepts specific tool calls and returns custom results
// This is useful for mocking, caching, graceful degradation, or access control
func ToolInterceptorMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
	return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
		// Only intercept tool call requests
		if req.Method != mcp.MethodToolsCall {
			return next(ctx, req)
		}

		// Parse the tool call request to get tool name
		var callReq mcp.CallToolRequest
		if params, ok := req.Params.(map[string]interface{}); ok {
			if name, ok := params["name"].(string); ok {
				callReq.Params.Name = name
				if args, ok := params["arguments"].(map[string]interface{}); ok {
					callReq.Params.Arguments = args
				}
			}
		}

		// Intercept specific tools
		switch callReq.Params.Name {
		case "hello":
			// Example 1: Mock response for testing
			if args, ok := callReq.Params.Arguments["name"].(string); ok && args == "mock" {
				log.Printf("[Interceptor] 🔄 Intercepting 'hello' tool with name='mock', returning mocked response")
				mockResult := mcp.NewTextResult("🤖 [MOCKED] This is a mocked response, actual handler was not called!")
				// ✅ Return CallToolResult directly, not wrapped in JSONRPCResponse
				return mockResult, nil
			}

		case "counter":
			// Example 2: Cached response to avoid computation
			if session, ok := mcp.GetSessionFromContext(ctx); ok {
				// Check if we should use cached response (simulate cache logic)
				sessionID := session.GetID()
				if len(sessionID) > 0 && sessionID[0] == '1' { // Simple cache condition
					log.Printf("[Interceptor] 💾 Returning cached result for 'counter' tool")
					cachedResult := mcp.NewTextResult("💾 [CACHED] Counter: 42 (from cache, handler not called)")
					// ✅ Return CallToolResult directly
					return cachedResult, nil
				}
			}

		case "fail":
			// Example 3: Graceful degradation - intercept error-prone tool
			log.Printf("[Interceptor] 🛡️ Intercepting 'fail' tool for graceful degradation")
			fallbackResult := mcp.NewTextResult("🛡️ [DEGRADED] Service temporarily unavailable. Using fallback response instead of calling actual handler.")
			// ✅ Return CallToolResult directly
			return fallbackResult, nil

		case "blocked_tool":
			// Example 4: Access control - block certain tools
			log.Printf("[Interceptor] ⛔ Blocking 'blocked_tool' - access denied")
			blockResult := mcp.NewTextResult("⛔ [BLOCKED] Access to this tool is denied.")
			blockResult.IsError = true
			// ✅ Return CallToolResult directly
			return blockResult, nil
		}

		// For all other cases, call the actual handler
		log.Printf("[Interceptor] ✓ Allowing '%s' to proceed to actual handler", callReq.Params.Name)
		return next(ctx, req)
	}
}

func main() {
	log.Println("Starting MCP Server with Middleware...")

	// Create server with middleware in stateless mode
	// All middlewares must be configured at server creation time using WithMiddleware option.
	// Execution order: Trace → Logging → Metrics → Auth → InitInterceptor → PingInterceptor → ToolInterceptor → Core Handler
	server := mcp.NewServer(
		"Middleware-Example-Server",
		"1.0.0",
		mcp.WithServerAddress(":3000"),
		mcp.WithServerPath("/mcp"),
		// Stateless mode configuration
		mcp.WithStatelessMode(true),   // Enable stateless mode (no session management)
		mcp.WithPostSSEEnabled(false), // Disable SSE streaming
		mcp.WithGetSSEEnabled(false),  // Disable GET SSE for notifications
		// Register middleware in order (first registered = outer layer)
		mcp.WithMiddleware(
			TraceMiddleware,                 // Trace all requests
			LoggingMiddleware,               // Log all requests
			MetricsMiddleware,               // Count all requests
			AuthMiddleware,                  // Check authorization
			InitializeInterceptorMiddleware, // Enhance initialize response
			PingInterceptorMiddleware,       // Add timestamp to ping
			PromptInterceptorMiddleware,     // Intercept prompt requests
			ToolInterceptorMiddleware,       // Intercept specific tools (mock/cache/degrade/block)
		),
	)

	// Register a simple hello tool
	helloTool := mcp.NewTool("hello",
		mcp.WithDescription("Says hello with optional name"),
		mcp.WithString("name",
			mcp.Description("Name to greet"),
			mcp.Default("World")))

	server.RegisterTool(helloTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := "World"
		if nameVal, ok := req.Params.Arguments["name"].(string); ok && nameVal != "" {
			name = nameVal
		}

		// Simulate some processing time
		time.Sleep(100 * time.Millisecond)

		return mcp.NewTextResult(fmt.Sprintf("Hello, %s!", name)), nil
	})

	// Register a fail tool to test error handling
	failTool := mcp.NewTool("fail",
		mcp.WithDescription("Always fails to test error handling"))

	server.RegisterTool(failTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("intentional error for testing")
	})

	// Register a counter tool to demonstrate session state
	counterTool := mcp.NewTool("counter",
		mcp.WithDescription("A session counter to demonstrate middleware with stateful sessions"),
		mcp.WithNumber("increment",
			mcp.Description("Counter increment value"),
			mcp.Default(1)))

	server.RegisterTool(counterTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Get session
		session, ok := mcp.GetSessionFromContext(ctx)
		if !ok || session == nil {
			return mcp.NewErrorResult("Error: Could not get session info"), fmt.Errorf("no session")
		}

		// Get current count from session
		count := 0
		if countVal, exists := session.GetData("counter"); exists && countVal != nil {
			count, _ = countVal.(int)
		}

		// Get increment parameter
		increment := 1
		if incArg, ok := req.Params.Arguments["increment"]; ok {
			if incFloat, ok := incArg.(float64); ok {
				increment = int(incFloat)
			}
		}

		count += increment
		session.SetData("counter", count)

		return mcp.NewTextResult(
			fmt.Sprintf("Counter value: %d (Session: %s)", count, session.GetID()[:8]+"..."),
		), nil
	})

	log.Println("Server configuration:")
	log.Println("  Address: :3000")
	log.Println("  Path: /mcp")
	log.Println("")
	log.Println("Registered middlewares:")
	log.Println("  1. TraceMiddleware                 - Adds trace IDs")
	log.Println("  2. LoggingMiddleware               - Logs requests and responses")
	log.Println("  3. MetricsMiddleware               - Collects request metrics")
	log.Println("  4. AuthMiddleware                  - Checks authorization")
	log.Println("  5. InitializeInterceptorMiddleware - Enhances initialize response")
	log.Println("  6. PingInterceptorMiddleware       - Adds timestamp to ping")
	log.Println("  7. PromptInterceptorMiddleware     - Intercepts prompt requests")
	log.Println("  8. ToolInterceptorMiddleware       - Intercepts specific tools (mock/cache/degrade/block)")
	log.Println("")
	// Register prompts
	analysisPrompt := &mcp.Prompt{
		Name:        "code-analysis",
		Description: "Analyze code and provide suggestions",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "code",
				Description: "The code to analyze",
				Required:    true,
			},
			{
				Name:        "language",
				Description: "Programming language",
				Required:    false,
			},
		},
	}
	server.RegisterPrompt(analysisPrompt, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		code := req.Params.Arguments["code"]
		language := req.Params.Arguments["language"]
		if language == "" {
			language = "unknown"
		}

		return &mcp.GetPromptResult{
			Description: "Code analysis prompt",
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Please analyze this %s code and provide suggestions:\n\n%s", language, code),
					},
				},
			},
		}, nil
	})

	cachedPrompt := &mcp.Prompt{
		Name:        "cached-prompt",
		Description: "A prompt that will be intercepted and cached",
	}
	server.RegisterPrompt(cachedPrompt, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// This handler will be bypassed by middleware
		return &mcp.GetPromptResult{
			Description: "This should not be seen (intercepted by middleware)",
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: "If you see this, the interceptor didn't work!",
					},
				},
			},
		}, nil
	})

	log.Println("Registered tools:")
	log.Println("  - hello   : Says hello with optional name")
	log.Println("  - fail    : Always fails (for testing)")
	log.Println("  - counter : Session-based counter")
	log.Println("")
	log.Println("Registered prompts:")
	log.Println("  - code-analysis    : Analyze code (normal execution)")
	log.Println("  - cached-prompt    : Will be intercepted by middleware")
	log.Println("  - intercepted-prompt : Added dynamically by middleware")
	log.Println("")
	log.Println("Try calling the tools to see middleware in action!")

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("MCP server started on :3000, access path /mcp")
		if err := server.Start(); err != nil {
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	// Wait for termination signal
	sig := <-sigCh
	log.Printf("Received signal %v, shutting down...", sig)
}
