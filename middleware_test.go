// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestMiddlewareChain tests the middleware chain functionality
func TestMiddlewareChain(t *testing.T) {
	// Create a test handler
	finalHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "final response", nil
	}

	// Create middleware that adds a prefix
	middleware1 := func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		resp, err := next(ctx, req)
		if err != nil {
			return nil, err
		}
		return "middleware1-" + resp.(string), nil
	}

	// Create middleware that adds another prefix
	middleware2 := func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		resp, err := next(ctx, req)
		if err != nil {
			return nil, err
		}
		return "middleware2-" + resp.(string), nil
	}

	// Chain the middlewares
	chainedHandler := Chain(finalHandler, middleware1, middleware2)

	// Execute the chain
	result, err := chainedHandler(context.Background(), "test request")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "middleware1-middleware2-final response"
	if result != expected {
		t.Fatalf("Expected %q, got %q", expected, result)
	}
}

// TestLoggingMiddleware tests the logging middleware
func TestLoggingMiddleware(t *testing.T) {
	finalHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	// Execute with logging middleware
	result, err := LoggingMiddleware(context.Background(), "test", finalHandler)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "response" {
		t.Fatalf("Expected 'response', got %v", result)
	}
}

// TestRecoveryMiddleware tests the recovery middleware
func TestRecoveryMiddleware(t *testing.T) {
	// Create a handler that panics
	panicHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("test panic")
	}

	// Execute with recovery middleware (should not panic)
	_, err := RecoveryMiddleware(context.Background(), "test", panicHandler)
	if err != nil {
		t.Fatalf("Recovery middleware should handle panic gracefully, got error: %v", err)
	}
}

// TestToolHandlerMiddleware tests the tool handler middleware
func TestToolHandlerMiddleware(t *testing.T) {
	finalHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return &CallToolResult{
			Content: []Content{NewTextContent("tool response")},
			IsError: false,
		}, nil
	}

	// Create a tool call request
	toolReq := &CallToolRequest{
		Params: CallToolParams{
			Name: "test-tool",
			Arguments: map[string]interface{}{
				"param1": "value1",
			},
		},
	}

	// Execute with tool handler middleware
	result, err := ToolHandlerMiddleware(context.Background(), toolReq, finalHandler)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	toolResult, ok := result.(*CallToolResult)
	if !ok {
		t.Fatalf("Expected CallToolResult, got %T", result)
	}

	if toolResult.IsError {
		t.Fatalf("Expected successful tool result, got error")
	}
}

// TestToolHandlerMiddlewareValidation tests tool handler middleware validation
func TestToolHandlerMiddlewareValidation(t *testing.T) {
	finalHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}

	// Create a tool call request with empty name
	toolReq := &CallToolRequest{
		Params: CallToolParams{
			Name: "", // Empty name should trigger validation error
		},
	}

	// Execute with tool handler middleware
	_, err := ToolHandlerMiddleware(context.Background(), toolReq, finalHandler)
	if err == nil {
		t.Fatalf("Expected validation error for empty tool name")
	}

	expectedError := "tool name is required"
	if err.Error() != expectedError {
		t.Fatalf("Expected error %q, got %q", expectedError, err.Error())
	}
}

// TestMetricsMiddleware tests the metrics middleware
func TestMetricsMiddleware(t *testing.T) {
	finalHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		time.Sleep(10 * time.Millisecond) // Simulate some processing time
		return "response", nil
	}

	// Execute with metrics middleware
	result, err := MetricsMiddleware(context.Background(), "test", finalHandler)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "response" {
		t.Fatalf("Expected 'response', got %v", result)
	}
}

// TestRetryMiddleware tests the retry middleware
func TestRetryMiddleware(t *testing.T) {
	attemptCount := 0
	
	// Create a handler that fails twice then succeeds
	flakyHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		attemptCount++
		if attemptCount < 3 {
			return nil, fmt.Errorf("attempt %d failed", attemptCount)
		}
		return "success", nil
	}

	// Create retry middleware with max 3 retries
	retryMiddleware := RetryMiddleware(3)

	// Execute with retry middleware
	result, err := retryMiddleware(context.Background(), "test", flakyHandler)
	if err != nil {
		t.Fatalf("Expected no error after retries, got %v", err)
	}

	if result != "success" {
		t.Fatalf("Expected 'success', got %v", result)
	}

	if attemptCount != 3 {
		t.Fatalf("Expected 3 attempts, got %d", attemptCount)
	}
}

// TestValidationMiddleware tests the validation middleware
func TestValidationMiddleware(t *testing.T) {
	finalHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	// Test with valid tool request
	validToolReq := &CallToolRequest{
		Params: CallToolParams{
			Name: "valid-tool",
		},
	}

	result, err := ValidationMiddleware(context.Background(), validToolReq, finalHandler)
	if err != nil {
		t.Fatalf("Expected no error for valid request, got %v", err)
	}

	if result != "response" {
		t.Fatalf("Expected 'response', got %v", result)
	}

	// Test with invalid tool request (empty name)
	invalidToolReq := &CallToolRequest{
		Params: CallToolParams{
			Name: "", // Empty name should fail validation
		},
	}

	_, err = ValidationMiddleware(context.Background(), invalidToolReq, finalHandler)
	if err == nil {
		t.Fatalf("Expected validation error for empty tool name")
	}

	expectedError := "validation failed: tool name is required"
	if err.Error() != expectedError {
		t.Fatalf("Expected error %q, got %q", expectedError, err.Error())
	}
}

// TestCacheMiddleware tests the cache middleware
func TestCacheMiddleware(t *testing.T) {
	callCount := 0
	cache := make(map[string]interface{})

	// Create a handler that increments call count
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		callCount++
		return fmt.Sprintf("response-%d", callCount), nil
	}

	cacheMiddleware := CacheMiddleware(cache)

	// First call should execute handler
	result1, err := cacheMiddleware(context.Background(), "test", handler)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Second call should return cached result
	result2, err := cacheMiddleware(context.Background(), "test", handler)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have called handler only once
	if callCount != 1 {
		t.Fatalf("Expected handler to be called once, got %d calls", callCount)
	}

	// Both results should be the same (from cache)
	if result1 != result2 {
		t.Fatalf("Expected same result from cache, got %v and %v", result1, result2)
	}
}

// BenchmarkMiddlewareChain benchmarks the middleware chain performance
func BenchmarkMiddlewareChain(b *testing.B) {
	finalHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	// Create a chain with multiple middlewares
	chainedHandler := Chain(finalHandler, 
		LoggingMiddleware,
		MetricsMiddleware,
		ValidationMiddleware,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chainedHandler(context.Background(), &CallToolRequest{
			Params: CallToolParams{Name: "test-tool"},
		})
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

// TestRateLimitingMiddleware tests the rate limiting middleware
func TestRateLimitingMiddleware(t *testing.T) {
	middleware := RateLimitingMiddleware(2, time.Second*1)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		_, err := middleware(context.Background(), "test", handler)
		if err != nil {
			t.Fatalf("Request %d should succeed, got error: %v", i+1, err)
		}
	}

	// Third request should be rate limited
	_, err := middleware(context.Background(), "test", handler)
	if err == nil {
		t.Fatal("Third request should be rate limited")
	}
}

// TestCircuitBreakerMiddleware tests the circuit breaker middleware
func TestCircuitBreakerMiddleware(t *testing.T) {
	middleware := CircuitBreakerMiddleware(2, time.Millisecond*100)

	failingHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, fmt.Errorf("simulated error")
	}

	// First two requests should fail and trigger circuit breaker
	for i := 0; i < 2; i++ {
		_, err := middleware(context.Background(), "test", failingHandler)
		if err == nil {
			t.Fatalf("Request %d should fail", i+1)
		}
	}

	// Third request should be blocked by circuit breaker
	_, err := middleware(context.Background(), "test", failingHandler)
	if err == nil || err.Error() != "circuit breaker is open" {
		t.Fatalf("Expected circuit breaker to be open, got: %v", err)
	}

	// Wait for timeout and test half-open state
	time.Sleep(time.Millisecond * 150)

	successHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	// This should succeed and close the circuit breaker
	_, err = middleware(context.Background(), "test", successHandler)
	if err != nil {
		t.Fatalf("Request should succeed in half-open state, got: %v", err)
	}
}

// TestTimeoutMiddleware tests the timeout middleware
func TestTimeoutMiddleware(t *testing.T) {
	middleware := TimeoutMiddleware(time.Millisecond * 100)

	slowHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		time.Sleep(time.Millisecond * 200)
		return "slow response", nil
	}

	_, err := middleware(context.Background(), "test", slowHandler)
	if err == nil || !strings.Contains(fmt.Sprintf("%v", err), "timeout") {
		t.Fatalf("Expected timeout error, got: %v", err)
	}

	fastHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "fast response", nil
	}

	resp, err := middleware(context.Background(), "test", fastHandler)
	if err != nil {
		t.Fatalf("Fast handler should not timeout, got: %v", err)
	}
	if resp != "fast response" {
		t.Fatalf("Expected 'fast response', got: %v", resp)
	}
}

// TestCORSMiddleware tests the CORS middleware
func TestCORSMiddleware(t *testing.T) {
	allowOrigins := []string{"http://localhost:3000", "https://example.com"}
	allowMethods := []string{"GET", "POST", "PUT", "DELETE"}
	allowHeaders := []string{"Content-Type", "Authorization"}

	middleware := CORSMiddleware(allowOrigins, allowMethods, allowHeaders)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Check if CORS info is set in context
		origins := ctx.Value("cors_allowed_origins")
		if origins == nil {
			return nil, fmt.Errorf("CORS origins not set in context")
		}
		return "success", nil
	}

	_, err := middleware(context.Background(), "test", handler)
	if err != nil {
		t.Fatalf("CORS middleware should set context values, got error: %v", err)
	}
}

// TestSecurityMiddleware tests the security middleware
func TestSecurityMiddleware(t *testing.T) {
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "secure response", nil
	}

	resp, err := SecurityMiddleware(context.Background(), "test", handler)
	if err != nil {
		t.Fatalf("Security middleware should not fail for normal request, got: %v", err)
	}
	if resp != "secure response" {
		t.Fatalf("Expected 'secure response', got: %v", resp)
	}
}

// TestCompressionMiddleware tests the compression middleware
func TestCompressionMiddleware(t *testing.T) {
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Check if compression is enabled in context
		compressionEnabled := ctx.Value("compression_enabled")
		if compressionEnabled != true {
			return nil, fmt.Errorf("compression not enabled in context")
		}
		return "compressed response", nil
	}

	resp, err := CompressionMiddleware(context.Background(), "test", handler)
	if err != nil {
		t.Fatalf("Compression middleware should set context values, got error: %v", err)
	}
	if resp != "compressed response" {
		t.Fatalf("Expected 'compressed response', got: %v", resp)
	}
}
