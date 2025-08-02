// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestMiddlewareError 测试中间件错误系统
func TestMiddlewareError(t *testing.T) {
	t.Run("错误创建和堆栈追踪", func(t *testing.T) {
		originalErr := fmt.Errorf("original error")
		middlewareErr := NewMiddlewareError(ErrCodeAuth, "authentication failed", originalErr)
		
		if middlewareErr.Code != ErrCodeAuth {
			t.Errorf("期望错误码 %s，得到 %s", ErrCodeAuth, middlewareErr.Code)
		}
		
		if middlewareErr.Cause != originalErr {
			t.Errorf("期望原始错误 %v，得到 %v", originalErr, middlewareErr.Cause)
		}
		
		if len(middlewareErr.Trace) == 0 {
			t.Error("期望非空的调用堆栈")
		}
		
		// 测试错误链
		if unwrappedErr := middlewareErr.Unwrap(); unwrappedErr != originalErr {
			t.Errorf("期望解包后的错误 %v，得到 %v", originalErr, unwrappedErr)
		}
	})
	
	t.Run("错误上下文", func(t *testing.T) {
		err := NewMiddlewareError(ErrCodeRateLimit, "rate limit exceeded", nil)
		err.AddContext("client_id", "test-client")
		err.AddContext("requests", 100)
		
		if clientID, exists := err.Context["client_id"]; !exists || clientID != "test-client" {
			t.Error("期望上下文包含正确的 client_id")
		}
		
		if requests, exists := err.Context["requests"]; !exists || requests != 100 {
			t.Error("期望上下文包含正确的 requests")
		}
	})
}

// TestAuthMiddlewareError 测试认证中间件的错误处理
func TestAuthMiddlewareError(t *testing.T) {
	middleware := AuthMiddleware("")
	
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}
	
	ctx := context.Background()
	req := &CallToolRequest{}
	
	resp, err := middleware(ctx, req, handler)
	
	if resp != nil {
		t.Error("期望响应为 nil")
	}
	
	if err == nil {
		t.Fatal("期望返回错误")
	}
	
	middlewareErr, ok := err.(*MiddlewareError)
	if !ok {
		t.Fatal("期望返回 MiddlewareError 类型")
	}
	
	if middlewareErr.Code != ErrCodeAuth {
		t.Errorf("期望错误码 %s，得到 %s", ErrCodeAuth, middlewareErr.Code)
	}
	
	if requestType, exists := middlewareErr.Context["request_type"]; !exists {
		t.Error("期望上下文包含 request_type")
	} else if requestType != "*mcp.CallToolRequest" {
		t.Errorf("期望 request_type 为 *mcp.CallToolRequest，得到 %v", requestType)
	}
}

// TestRateLimitingMiddlewareThreadSafety 测试限流中间件的线程安全
func TestRateLimitingMiddlewareThreadSafety(t *testing.T) {
	middleware := RateLimitingMiddleware(10, time.Second)
	
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}
	
	var wg sync.WaitGroup
	var successCount, errorCount int64
	var mu sync.Mutex
	
	// 并发执行 50 个请求
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			
			ctx := context.WithValue(context.Background(), "user_id", fmt.Sprintf("user-%d", i%5))
			req := &CallToolRequest{}
			
			_, err := middleware(ctx, req, handler)
			
			mu.Lock()
			if err != nil {
				errorCount++
			} else {
				successCount++
			}
			mu.Unlock()
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("成功请求: %d, 失败请求: %d", successCount, errorCount)
	
	// 由于有5个不同的用户，每个用户可以有10个请求，所以最多50个请求都可能成功
	// 但由于并发执行，可能会有一些请求被限流
	if successCount == 0 {
		t.Error("期望至少有一些成功的请求")
	}
}

// TestCircuitBreakerMiddlewareThreadSafety 测试熔断器中间件的线程安全
func TestCircuitBreakerMiddlewareThreadSafety(t *testing.T) {
	middleware := CircuitBreakerMiddleware(3, time.Second)
	
	// 创建一个会失败的处理器
	failingHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, fmt.Errorf("simulated failure")
	}
	
	var wg sync.WaitGroup
	var circuitBreakerErrors int64
	var otherErrors int64
	var mu sync.Mutex
	
	// 并发执行请求直到熔断器打开
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			ctx := context.Background()
			req := &CallToolRequest{}
			
			_, err := middleware(ctx, req, failingHandler)
			
			mu.Lock()
			if err != nil {
				if middlewareErr, ok := err.(*MiddlewareError); ok && middlewareErr.Code == ErrCodeCircuitBreaker {
					circuitBreakerErrors++
				} else {
					otherErrors++
				}
			}
			mu.Unlock()
		}()
	}
	
	wg.Wait()
	
	t.Logf("熔断器错误: %d, 其他错误: %d", circuitBreakerErrors, otherErrors)
	
	if circuitBreakerErrors == 0 {
		t.Error("期望有一些熔断器错误")
	}
	
	if otherErrors == 0 {
		t.Error("期望有一些原始错误")
	}
}

// TestRecoveryMiddlewareError 测试恢复中间件的错误处理
func TestRecoveryMiddlewareError(t *testing.T) {
	panicHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("test panic")
	}
	
	ctx := context.Background()
	req := &CallToolRequest{}
	
	resp, err := RecoveryMiddleware(ctx, req, panicHandler)
	
	if resp != nil {
		t.Error("期望响应为 nil")
	}
	
	if err == nil {
		t.Fatal("期望返回错误")
	}
	
	middlewareErr, ok := err.(*MiddlewareError)
	if !ok {
		t.Fatal("期望返回 MiddlewareError 类型")
	}
	
	if middlewareErr.Code != ErrCodePanic {
		t.Errorf("期望错误码 %s，得到 %s", ErrCodePanic, middlewareErr.Code)
	}
	
	if panicValue, exists := middlewareErr.Context["panic_value"]; !exists {
		t.Error("期望上下文包含 panic_value")
	} else if panicValue != "test panic" {
		t.Errorf("期望 panic_value 为 'test panic'，得到 %v", panicValue)
	}
}

// TestMiddlewareChainErrorPropagation 测试中间件链中的错误传播
func TestMiddlewareChainErrorPropagation(t *testing.T) {
	// 创建一个会产生错误的中间件
	errorMiddleware := func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		// 不调用下一个中间件，直接返回错误
		return nil, NewMiddlewareError(ErrCodeValidation, "validation failed", nil)
	}
	
	// 创建一个记录中间件
	var executed bool
	loggingMiddleware := func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		executed = true
		return next(ctx, req)
	}
	
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Error("不应该执行到最终处理器")
		return "success", nil
	}
	
	// 创建中间件链
	chain := NewMiddlewareChain()
	// 调整中间件顺序，确保 loggingMiddleware 先于 errorMiddleware 执行
	chain.Use(loggingMiddleware)
	chain.Use(errorMiddleware)
	
	ctx := context.Background()
	req := &CallToolRequest{}
	
	resp, err := chain.Execute(ctx, req, handler)
	
	if resp != nil {
		t.Error("期望响应为 nil")
	}
	
	if err == nil {
		t.Fatal("期望返回错误")
	}
	
	if !executed {
		t.Error("期望日志中间件被执行")
	}
	
	middlewareErr, ok := err.(*MiddlewareError)
	if !ok {
		t.Fatal("期望返回 MiddlewareError 类型")
	}
	
	if middlewareErr.Code != ErrCodeValidation {
		t.Errorf("期望错误码 %s，得到 %s", ErrCodeValidation, middlewareErr.Code)
	}
}

// BenchmarkMiddlewareErrorCreation 基准测试错误创建性能
func BenchmarkMiddlewareErrorCreation(b *testing.B) {
	originalErr := fmt.Errorf("original error")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := NewMiddlewareError(ErrCodeAuth, "test error", originalErr)
		err.AddContext("test", "value")
	}
}

// BenchmarkRateLimitingMiddleware 基准测试限流中间件性能
func BenchmarkRateLimitingMiddleware(b *testing.B) {
	middleware := RateLimitingMiddleware(1000, time.Second)
	
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}
	
	ctx := context.Background()
	req := &CallToolRequest{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		middleware(ctx, req, handler)
	}
}

// BenchmarkCircuitBreakerMiddleware 基准测试熔断器中间件性能
func BenchmarkCircuitBreakerMiddleware(b *testing.B) {
	middleware := CircuitBreakerMiddleware(100, time.Second)
	
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}
	
	ctx := context.Background()
	req := &CallToolRequest{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		middleware(ctx, req, handler)
	}
}
