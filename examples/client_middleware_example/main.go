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

// ClientLoggingMiddleware 客户端日志中间件
func ClientLoggingMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
	start := time.Now()
	
	log.Printf("🚀 [Client] Request started: %T", req)
	
	resp, err := next(ctx, req)
	
	duration := time.Since(start)
	if err != nil {
		log.Printf("❌ [Client] Request failed after %v: %v", duration, err)
	} else {
		log.Printf("✅ [Client] Request completed in %v", duration)
	}
	
	return resp, err
}

// ClientMetricsMiddleware 客户端指标中间件
func ClientMetricsMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
	start := time.Now()
	
	resp, err := next(ctx, req)
	
	duration := time.Since(start)
	requestType := fmt.Sprintf("%T", req)
	status := "success"
	if err != nil {
		status = "error"
	}
	
	log.Printf("📊 [Client] Metrics - Type: %s, Status: %s, Duration: %v", 
		requestType, status, duration)
	
	return resp, err
}

// ClientValidationMiddleware 客户端验证中间件
func ClientValidationMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
	log.Printf("🔍 [Client] Validating request: %T", req)
	
	// 根据请求类型进行验证
	switch r := req.(type) {
	case *mcp.CallToolRequest:
		if r.Params.Name == "" {
			return nil, fmt.Errorf("client validation failed: tool name is required")
		}
		log.Printf("✅ [Client] Tool request validation passed: %s", r.Params.Name)
	case *mcp.ReadResourceRequest:
		if r.Params.URI == "" {
			return nil, fmt.Errorf("client validation failed: resource URI is required")
		}
		log.Printf("✅ [Client] Resource request validation passed: %s", r.Params.URI)
	case *mcp.GetPromptRequest:
		if r.Params.Name == "" {
			return nil, fmt.Errorf("client validation failed: prompt name is required")
		}
		log.Printf("✅ [Client] Prompt request validation passed: %s", r.Params.Name)
	default:
		log.Printf("✅ [Client] Generic request validation passed")
	}
	
	return next(ctx, req)
}

// demonstrateClientMiddleware 演示客户端中间件功能
func demonstrateClientMiddleware() {
	log.Println("=== 客户端中间件演示 ===")

	// 创建客户端信息
	clientInfo := mcp.Implementation{
		Name:    "ClientMiddlewareDemo",
		Version: "1.0.0",
	}

	// 创建带有中间件的客户端
	client, err := mcp.NewClient(
		"http://localhost:3000/mcp",
		clientInfo,
		// 添加客户端中间件（按执行顺序）
		mcp.WithMiddleware(mcp.RecoveryMiddleware),           // 错误恢复
		mcp.WithMiddleware(ClientLoggingMiddleware),          // 日志记录
		mcp.WithMiddleware(ClientMetricsMiddleware),          // 性能监控
		mcp.WithMiddleware(ClientValidationMiddleware),       // 请求验证
		mcp.WithMiddleware(mcp.ToolHandlerMiddleware),        // 工具处理
		mcp.WithMiddleware(mcp.ResourceMiddleware),           // 资源处理
		mcp.WithMiddleware(mcp.PromptMiddleware),             // 提示处理
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	)
	
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// 初始化客户端
	ctx := context.Background()
	initReq := &mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.ProtocolVersion_2025_03_26,
			ClientInfo:      clientInfo,
			Capabilities:    mcp.ClientCapabilities{},
		},
	}

	log.Println("🔄 Initializing client...")
	_, err = client.Initialize(ctx, initReq)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	log.Println("✅ Client initialized successfully")

	// 测试工具调用（会经过所有中间件）
	log.Println("\n📞 Testing tool call with middleware...")
	toolResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "greet",
			Arguments: map[string]interface{}{
				"name": "Middleware World",
			},
		},
	})

	if err != nil {
		log.Printf("❌ Tool call failed: %v", err)
	} else {
		log.Printf("✅ Tool call successful: %+v", toolResult)
	}

	// 测试资源读取（会经过所有中间件）
	log.Println("\n📄 Testing resource read with middleware...")
	resourceResult, err := client.ReadResource(ctx, &mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "welcome",
		},
	})

	if err != nil {
		log.Printf("❌ Resource read failed: %v", err)
	} else {
		log.Printf("✅ Resource read successful: %+v", resourceResult)
	}

	// 测试提示获取（会经过所有中间件）
	log.Println("\n💬 Testing prompt get with middleware...")
	promptResult, err := client.GetPrompt(ctx, &mcp.GetPromptRequest{
		Params: mcp.GetPromptParams{
			Name: "greeting",
			Arguments: map[string]interface{}{
				"name": "Middleware User",
			},
		},
	})

	if err != nil {
		log.Printf("❌ Prompt get failed: %v", err)
	} else {
		log.Printf("✅ Prompt get successful: %+v", promptResult)
	}

	// 测试工具列表（会经过所有中间件）
	log.Println("\n🛠️ Testing list tools with middleware...")
	toolsResult, err := client.ListTools(ctx, &mcp.ListToolsRequest{})

	if err != nil {
		log.Printf("❌ List tools failed: %v", err)
	} else {
		log.Printf("✅ List tools successful, found %d tools", len(toolsResult.Tools))
	}

	log.Println("\n🎉 Client middleware demonstration completed!")
}

// checkServerAvailability 检查服务器是否可用
func checkServerAvailability() bool {
	resp, err := http.Get("http://localhost:3000/mcp")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode != 404
}

func main() {
	log.Println("🚀 Starting Client Middleware Example")
	
	// 检查服务器是否运行
	if !checkServerAvailability() {
		log.Println("⚠️ Warning: Server not available at http://localhost:3000/mcp")
		log.Println("Please start the server middleware example first:")
		log.Println("  cd examples/server_middleware_example && go run main.go")
		log.Println("Then run this client example in another terminal.")
		return
	}

	// 等待一下让服务器完全启动
	time.Sleep(time.Second)

	// 演示客户端中间件
	demonstrateClientMiddleware()
}
