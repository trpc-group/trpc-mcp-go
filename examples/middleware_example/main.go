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

// CustomLoggingMiddleware 自定义日志中间件示例
func CustomLoggingMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
	start := time.Now()
	
	// 请求前处理
	log.Printf("🚀 Request started: %T", req)
	
	// 调用下一个处理器
	resp, err := next(ctx, req)
	
	// 请求后处理
	duration := time.Since(start)
	if err != nil {
		log.Printf("❌ Request failed after %v: %v", duration, err)
	} else {
		log.Printf("✅ Request completed in %v", duration)
	}
	
	return resp, err
}

// AuthenticationMiddleware 认证中间件示例
func AuthenticationMiddleware(apiKey string) mcp.MiddlewareFunc {
	return func(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
		// 验证 API Key
		if apiKey == "" {
			log.Printf("🔒 Authentication failed: API key is required")
			return nil, fmt.Errorf("authentication failed: API key is required")
		}
		
		log.Printf("🔑 Authentication successful")
		
		// 在上下文中添加认证信息
		ctx = context.WithValue(ctx, "authenticated", true)
		ctx = context.WithValue(ctx, "api_key", apiKey)
		
		return next(ctx, req)
	}
}

// RateLimitingMiddleware 限流中间件示例
func RateLimitingMiddleware(maxRequests int, window time.Duration) mcp.MiddlewareFunc {
	requestCount := 0
	lastReset := time.Now()
	
	return func(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
		now := time.Now()
		
		// 重置计数器
		if now.Sub(lastReset) > window {
			requestCount = 0
			lastReset = now
		}
		
		// 检查限流
		if requestCount >= maxRequests {
			log.Printf("🚫 Rate limit exceeded: %d requests in %v", requestCount, window)
			return nil, fmt.Errorf("rate limit exceeded: too many requests")
		}
		
		requestCount++
		log.Printf("📊 Request %d/%d in current window", requestCount, maxRequests)
		
		return next(ctx, req)
	}
}

// CircuitBreakerMiddleware 熔断器中间件示例
func CircuitBreakerMiddleware(threshold int, timeout time.Duration) mcp.MiddlewareFunc {
	failureCount := 0
	lastFailure := time.Time{}
	isOpen := false
	
	return func(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
		now := time.Now()
		
		// 检查熔断器状态
		if isOpen {
			if now.Sub(lastFailure) > timeout {
				// 尝试半开状态
				isOpen = false
				failureCount = 0
				log.Printf("🔄 Circuit breaker: attempting to close")
			} else {
				log.Printf("🔴 Circuit breaker is open")
				return nil, fmt.Errorf("circuit breaker is open")
			}
		}
		
		// 执行请求
		resp, err := next(ctx, req)
		
		if err != nil {
			failureCount++
			lastFailure = now
			
			if failureCount >= threshold {
				isOpen = true
				log.Printf("🔴 Circuit breaker opened after %d failures", failureCount)
			}
		} else {
			// 重置失败计数
			failureCount = 0
		}
		
		return resp, err
	}
}

func main() {
	// 创建客户端信息
	clientInfo := mcp.Implementation{
		Name:    "MiddlewareExample",
		Version: "1.0.0",
	}

	// 创建带有多个中间件的客户端
	client, err := mcp.NewClient(
		"http://localhost:3000",
		clientInfo,
		// 添加多个中间件（按顺序执行）
		mcp.WithMiddleware(CustomLoggingMiddleware),
		mcp.WithMiddleware(AuthenticationMiddleware("your-secret-api-key")),
		mcp.WithMiddleware(RateLimitingMiddleware(10, time.Minute)), // 每分钟最多10个请求
		mcp.WithMiddleware(CircuitBreakerMiddleware(3, 30*time.Second)), // 3次失败后熔断30秒
		mcp.WithMiddleware(mcp.ValidationMiddleware),
		mcp.WithMiddleware(mcp.MetricsMiddleware),
		mcp.WithMiddleware(mcp.RetryMiddleware(2)), // 最多重试2次
		mcp.WithMiddleware(mcp.RecoveryMiddleware),
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

	_, err = client.Initialize(ctx, initReq)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	log.Println("🎉 Client initialized successfully")

	// 调用工具（会经过所有中间件）
	toolResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "greet",
			Arguments: map[string]interface{}{
				"name": "World",
			},
		},
	})

	if err != nil {
		log.Printf("Tool call failed: %v", err)
	} else {
		log.Printf("Tool call succeeded: %+v", toolResult)
	}

	// 演示中间件链的直接使用
	log.Println("\n--- 演示中间件链的直接使用 ---")
	
	// 创建一个简单的处理器
	simpleHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "Hello from handler!", nil
	}
	
	// 创建中间件链
	chain := mcp.NewMiddlewareChain(
		CustomLoggingMiddleware,
		mcp.ValidationMiddleware,
		mcp.MetricsMiddleware,
	)
	
	// 执行中间件链
	result, err := chain.Execute(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "example"},
	}, simpleHandler)
	
	if err != nil {
		log.Printf("Chain execution failed: %v", err)
	} else {
		log.Printf("Chain execution result: %v", result)
	}
}
