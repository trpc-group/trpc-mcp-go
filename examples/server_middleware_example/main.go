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
	"net/http"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// CustomServerLoggingMiddleware 自定义服务端日志中间件
func CustomServerLoggingMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
	start := time.Now()
	
	// 请求前处理
	log.Printf("🚀 [Server] Request started: %T", req)
	
	// 调用下一个处理器
	resp, err := next(ctx, req)
	
	// 请求后处理
	duration := time.Since(start)
	if err != nil {
		log.Printf("❌ [Server] Request failed after %v: %v", duration, err)
	} else {
		log.Printf("✅ [Server] Request completed in %v", duration)
	}
	
	return resp, err
}

// ServerAuthMiddleware 服务端认证中间件
func ServerAuthMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
	// 从上下文中获取认证信息（实际中可能从HTTP头获取）
	log.Printf("🔐 [Server] Authenticating request: %T", req)
	
	// 模拟认证检查
	// 在实际应用中，这里会检查API密钥、JWT令牌等
	
	// 在上下文中添加用户信息
	ctx = context.WithValue(ctx, "server_authenticated", true)
	ctx = context.WithValue(ctx, "server_user_id", "server_user_123")
	
	log.Printf("✅ [Server] Authentication successful")
	return next(ctx, req)
}

// ServerMetricsMiddleware 服务端指标中间件
func ServerMetricsMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
	start := time.Now()
	
	// 调用下一个处理器
	resp, err := next(ctx, req)
	
	// 记录指标
	duration := time.Since(start)
	requestType := fmt.Sprintf("%T", req)
	status := "success"
	if err != nil {
		status = "error"
	}
	
	log.Printf("📊 [Server] Metrics - Type: %s, Status: %s, Duration: %v", 
		requestType, status, duration)
	
	return resp, err
}

// ServerValidationMiddleware 服务端验证中间件
func ServerValidationMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
	log.Printf("🔍 [Server] Validating request: %T", req)
	
	// 根据请求类型进行验证
	switch r := req.(type) {
	case *mcp.CallToolRequest:
		if r.Params.Name == "" {
			return nil, fmt.Errorf("server validation failed: tool name is required")
		}
		log.Printf("✅ [Server] Tool request validation passed: %s", r.Params.Name)
	case *mcp.ReadResourceRequest:
		if r.Params.URI == "" {
			return nil, fmt.Errorf("server validation failed: resource URI is required")
		}
		log.Printf("✅ [Server] Resource request validation passed: %s", r.Params.URI)
	case *mcp.GetPromptRequest:
		if r.Params.Name == "" {
			return nil, fmt.Errorf("server validation failed: prompt name is required")
		}
		log.Printf("✅ [Server] Prompt request validation passed: %s", r.Params.Name)
	default:
		log.Printf("✅ [Server] Generic request validation passed")
	}
	
	return next(ctx, req)
}

func main() {
	log.Println("🚀 Starting MCP Server with Middleware Example")

	// 创建服务器信息
	serverInfo := mcp.Implementation{
		Name:    "ServerMiddlewareExample",
		Version: "1.0.0",
	}

	// 创建带有多个中间件的服务器
	server := mcp.NewServer(
		serverInfo.Name,
		serverInfo.Version,
		// 添加服务端中间件（按执行顺序）
		mcp.WithServerMiddleware(mcp.RecoveryMiddleware),              // 最外层：错误恢复
		mcp.WithServerMiddleware(CustomServerLoggingMiddleware),       // 日志记录
		mcp.WithServerMiddleware(ServerMetricsMiddleware),             // 性能监控
		mcp.WithServerMiddleware(ServerAuthMiddleware),                // 认证鉴权
		mcp.WithServerMiddleware(ServerValidationMiddleware),          // 请求验证
		mcp.WithServerMiddleware(mcp.RateLimitingMiddleware(100, time.Minute)), // 限流：100请求/分钟
		mcp.WithServerMiddleware(mcp.ToolHandlerMiddleware),           // 工具处理
		mcp.WithServerMiddleware(mcp.ResourceMiddleware),              // 资源处理
		mcp.WithServerMiddleware(mcp.PromptMiddleware),                // 提示处理
		mcp.WithServerLogger(mcp.GetDefaultLogger()),
	)

	// 注册一个示例工具
	err := server.RegisterTool("greet", "Greet someone", func(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
		// 从上下文中获取认证信息
		authenticated := ctx.Value("server_authenticated")
		userID := ctx.Value("server_user_id")
		
		log.Printf("🔧 [Tool] Greet tool called by user: %v (authenticated: %v)", userID, authenticated)
		
		name, ok := args["name"].(string)
		if !ok {
			name = "World"
		}
		
		message := fmt.Sprintf("Hello, %s! (from server with middleware)", name)
		
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: message,
				},
			},
		}, nil
	})
	
	if err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}

	// 注册一个示例资源
	err = server.RegisterResource("welcome", "Welcome message resource", "text/plain", func(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
		// 从上下文中获取认证信息
		userID := ctx.Value("server_user_id")
		log.Printf("📄 [Resource] Welcome resource accessed by user: %v", userID)
		
		content := "Welcome to the MCP Server with Middleware!"
		
		return &mcp.ReadResourceResult{
			Contents: []mcp.ResourceContents{
				{
					URI:      uri,
					MimeType: "text/plain",
					Text:     &content,
				},
			},
		}, nil
	})
	
	if err != nil {
		log.Fatalf("Failed to register resource: %v", err)
	}

	// 注册一个示例提示
	err = server.RegisterPrompt("greeting", "A greeting prompt template", func(ctx context.Context, name string, args map[string]interface{}) (*mcp.GetPromptResult, error) {
		// 从上下文中获取认证信息
		userID := ctx.Value("server_user_id")
		log.Printf("💬 [Prompt] Greeting prompt accessed by user: %v", userID)
		
		// 从参数中获取名字
		targetName, ok := args["name"].(string)
		if !ok {
			targetName = "Guest"
		}
		
		promptText := fmt.Sprintf("Generate a warm greeting for %s", targetName)
		
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: "user",
					Content: mcp.Content{
						Type: "text",
						Text: promptText,
					},
				},
			},
		}, nil
	})
	
	if err != nil {
		log.Fatalf("Failed to register prompt: %v", err)
	}

	// 启动服务器
	log.Println("🌐 Server starting on http://localhost:3000/mcp")
	log.Println("📋 Available endpoints:")
	log.Println("   - Tools: greet")
	log.Println("   - Resources: welcome")
	log.Println("   - Prompts: greeting")
	log.Println("")
	log.Println("🔧 Middleware chain active:")
	log.Println("   1. RecoveryMiddleware (Error recovery)")
	log.Println("   2. CustomServerLoggingMiddleware (Request logging)")
	log.Println("   3. ServerMetricsMiddleware (Performance metrics)")
	log.Println("   4. ServerAuthMiddleware (Authentication)")
	log.Println("   5. ServerValidationMiddleware (Request validation)")
	log.Println("   6. RateLimitingMiddleware (Rate limiting)")
	log.Println("   7. ToolHandlerMiddleware (Tool processing)")
	log.Println("   8. ResourceMiddleware (Resource processing)")
	log.Println("   9. PromptMiddleware (Prompt processing)")
	log.Println("")
	log.Println("💡 Test with a client:")
	log.Println("   cd examples/basic/client && go run main.go")

	if err := http.ListenAndServe(":3000", server.Handler()); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
