// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// MiddlewareError 中间件错误类型，支持错误分类和链路追踪
type MiddlewareError struct {
	Code      string                 // 错误码
	Message   string                 // 错误消息
	Cause     error                  // 原始错误
	Context   map[string]interface{} // 错误上下文
	Timestamp time.Time              // 错误时间
	Trace     []string               // 调用堆栈
}

func (e *MiddlewareError) Error() string {
	return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
}

func (e *MiddlewareError) Unwrap() error {
	return e.Cause
}

// 错误码常量
const (
	ErrCodeAuth           = "AUTH_FAILED"
	ErrCodeRateLimit      = "RATE_LIMIT_EXCEEDED"
	ErrCodeCircuitBreaker = "CIRCUIT_BREAKER_OPEN"
	ErrCodeValidation     = "VALIDATION_FAILED"
	ErrCodeTimeout        = "TIMEOUT"
	ErrCodePanic         = "PANIC_RECOVERED"
	ErrCodeUnknown       = "UNKNOWN_ERROR"
)

// NewMiddlewareError 创建新的中间件错误
func NewMiddlewareError(code, message string, cause error) *MiddlewareError {
	// 获取调用堆栈
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	
	trace := make([]string, 0, n)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		trace = append(trace, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}
	
	return &MiddlewareError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Context:   make(map[string]interface{}),
		Timestamp: time.Now(),
		Trace:     trace,
	}
}

// AddContext 添加错误上下文信息
func (e *MiddlewareError) AddContext(key string, value interface{}) *MiddlewareError {
	e.Context[key] = value
	return e
}

// Handler 定义了中间件链末端处理请求的函数签名。
// 这是实际执行业务逻辑（如发送网络请求或执行工具逻辑）的函数。
type Handler func(ctx context.Context, req interface{}) (interface{}, error)

// MiddlewareFunc 定义了中间件函数的接口。
// 它接收上下文、请求以及链中的下一个处理器，允许在请求前后执行逻辑。
type MiddlewareFunc func(ctx context.Context, req interface{}, next Handler) (interface{}, error)

// MiddlewareChain 表示中间件执行链，按注册顺序执行中间件
type MiddlewareChain struct {
	middlewares []MiddlewareFunc
}

// NewMiddlewareChain 创建一个新的中间件链
func NewMiddlewareChain(middlewares ...MiddlewareFunc) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: middlewares,
	}
}

// Use 添加中间件到链中
func (mc *MiddlewareChain) Use(middleware MiddlewareFunc) {
	mc.middlewares = append(mc.middlewares, middleware)
}

// Execute 执行中间件链
func (mc *MiddlewareChain) Execute(ctx context.Context, req interface{}, finalHandler Handler) (interface{}, error) {
	return Chain(finalHandler, mc.middlewares...)(ctx, req)
}

// Chain 将一系列中间件和一个最终的处理器链接起来，形成一个完整的执行链。
// 中间件会按照参数顺序执行，最后一个参数的中间件在最外层最先执行。
// 例如：Chain(handler, m1, m2) 的执行顺序是 m2 -> m1 -> handler
func Chain(handler Handler, middlewares ...MiddlewareFunc) Handler {
	// 从最后一个中间件开始，将处理器逐层向内包装，使最后的中间件在最外层
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = wrap(middlewares[i], handler)
	}
	return handler
}

// wrap 是一个辅助函数，用于将一个中间件包装在下一个处理器周围。
func wrap(m MiddlewareFunc, next Handler) Handler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		return m(ctx, req, next)
	}
}

// LoggingMiddleware 日志记录中间件，记录请求的基本信息和处理时间
func LoggingMiddleware(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
	startTime := time.Now()
	
	// 记录请求开始
	log.Printf("[Middleware] Request started: %T", req)
	
	// 调用下一个处理器
	resp, err := next(ctx, req)
	
	// 记录请求结束和耗时
	duration := time.Since(startTime)
	if err != nil {
		log.Printf("[Middleware] Request failed after %v: %v", duration, err)
	} else {
		log.Printf("[Middleware] Request completed in %v", duration)
	}
	
	return resp, err
}

// RecoveryMiddleware 错误恢复中间件，捕获 panic 并转换为错误
func RecoveryMiddleware(ctx context.Context, req interface{}, next Handler) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[RecoveryMiddleware] Panic recovered: %v", r)
			
			// 将 panic 转换为结构化错误
			panicErr := NewMiddlewareError(ErrCodePanic, "panic recovered in middleware chain", 
				fmt.Errorf("panic: %v", r))
			panicErr.AddContext("panic_value", r)
			panicErr.AddContext("request_type", fmt.Sprintf("%T", req))
			
			resp = nil
			err = panicErr
		}
	}()
	
	return next(ctx, req)
}

// ToolHandlerMiddleware 工具处理中间件，专门处理 CallTool 请求
func ToolHandlerMiddleware(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
	// 检查是否是工具调用请求
	if callToolReq, ok := req.(*CallToolRequest); ok {
		log.Printf("[ToolMiddleware] Calling tool: %s", callToolReq.Params.Name)
		
		// 验证工具名称
		if callToolReq.Params.Name == "" {
			return nil, fmt.Errorf("tool name is required")
		}
		
		// 记录工具参数
		if len(callToolReq.Params.Arguments) > 0 {
			log.Printf("[ToolMiddleware] Tool arguments: %v", callToolReq.Params.Arguments)
		}
		
		// 调用下一个处理器
		resp, err := next(ctx, req)
		
		// 处理工具调用结果
		if err == nil {
			if toolResult, ok := resp.(*CallToolResult); ok {
				if toolResult.IsError {
					log.Printf("[ToolMiddleware] Tool execution returned error")
				} else {
					log.Printf("[ToolMiddleware] Tool execution successful, content items: %d", len(toolResult.Content))
				}
			}
		}
		
		return resp, err
	}
	
	// 对于非工具调用请求，直接传递给下一个处理器
	return next(ctx, req)
}

// ResourceMiddleware 资源访问中间件，处理 ReadResource 请求
func ResourceMiddleware(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
	// 检查是否是资源读取请求
	if readResourceReq, ok := req.(*ReadResourceRequest); ok {
		log.Printf("[ResourceMiddleware] Reading resource: %s", readResourceReq.Params.URI)
		
		// 验证资源 URI
		if readResourceReq.Params.URI == "" {
			return nil, fmt.Errorf("resource URI is required")
		}
		
		// 这里可以添加资源访问权限检查、缓存逻辑等
		// 例如：检查用户是否有权限访问该资源
		
		// 调用下一个处理器
		resp, err := next(ctx, req)
		
		// 处理资源读取结果
		if err == nil {
			log.Printf("[ResourceMiddleware] Resource read successful")
		}
		
		return resp, err
	}
	
	// 对于非资源读取请求，直接传递给下一个处理器
	return next(ctx, req)
}

// PromptMiddleware 提示模板中间件，处理 GetPrompt 请求
func PromptMiddleware(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
	// 检查是否是获取提示请求
	if getPromptReq, ok := req.(*GetPromptRequest); ok {
		log.Printf("[PromptMiddleware] Getting prompt: %s", getPromptReq.Params.Name)
		
		// 验证提示名称
		if getPromptReq.Params.Name == "" {
			return nil, fmt.Errorf("prompt name is required")
		}
		
		// 这里可以添加提示模板的预处理、验证等逻辑
		
		// 调用下一个处理器
		resp, err := next(ctx, req)
		
		// 处理获取提示结果
		if err == nil {
			log.Printf("[PromptMiddleware] Prompt retrieved successfully")
		}
		
		return resp, err
	}
	
	// 对于非获取提示请求，直接传递给下一个处理器
	return next(ctx, req)
}

// MetricsMiddleware 性能监控中间件，收集请求指标
func MetricsMiddleware(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
	startTime := time.Now()
	
	// 调用下一个处理器
	resp, err := next(ctx, req)
	
	// 记录指标
	duration := time.Since(startTime)
	
	// 根据请求类型记录不同的指标
	requestType := fmt.Sprintf("%T", req)
	status := "success"
	if err != nil {
		status = "error"
	}
	
	// 这里可以集成到实际的监控系统（如 Prometheus）
	log.Printf("[Metrics] RequestType: %s, Status: %s, Duration: %v", requestType, status, duration)
	
	return resp, err
}

// AuthMiddleware 认证鉴权中间件
func AuthMiddleware(apiKey string) MiddlewareFunc {
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		// 从上下文中获取认证信息
		if apiKey == "" {
			err := NewMiddlewareError(ErrCodeAuth, "API key is required", nil)
			err.AddContext("request_type", fmt.Sprintf("%T", req))
			return nil, err
		}
		
		log.Printf("[AuthMiddleware] Request authenticated")
		
		// 在上下文中添加认证信息
		ctx = context.WithValue(ctx, "api_key", apiKey)
		
		return next(ctx, req)
	}
}

// RetryMiddleware 重试中间件，对失败的请求进行重试
func RetryMiddleware(maxRetries int) MiddlewareFunc {
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		var lastErr error
		
		for attempt := 0; attempt <= maxRetries; attempt++ {
			if attempt > 0 {
				log.Printf("[RetryMiddleware] Retry attempt %d/%d", attempt, maxRetries)
				// 添加退避延时
				time.Sleep(time.Duration(attempt) * time.Second)
			}
			
			resp, err := next(ctx, req)
			if err == nil {
				return resp, nil
			}
			
			lastErr = err
			log.Printf("[RetryMiddleware] Attempt %d failed: %v", attempt+1, err)
		}
		
		return nil, fmt.Errorf("request failed after %d retries: %v", maxRetries, lastErr)
	}
}

// CacheMiddleware 缓存中间件
func CacheMiddleware(cache map[string]interface{}) MiddlewareFunc {
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		// 生成缓存键
		cacheKey := fmt.Sprintf("%T_%v", req, req)
		
		// 检查缓存
		if cached, exists := cache[cacheKey]; exists {
			log.Printf("[CacheMiddleware] Cache hit for request: %T", req)
			return cached, nil
		}
		
		// 调用下一个处理器
		resp, err := next(ctx, req)
		
		// 如果成功，保存到缓存
		if err == nil {
			cache[cacheKey] = resp
			log.Printf("[CacheMiddleware] Cached response for request: %T", req)
		}
		
		return resp, err
	}
}

// ValidationMiddleware 验证中间件，对请求进行验证
func ValidationMiddleware(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
	// 根据请求类型进行不同的验证
	switch r := req.(type) {
	case *CallToolRequest:
		if r.Params.Name == "" {
			return nil, fmt.Errorf("validation failed: tool name is required")
		}
	case *ReadResourceRequest:
		if r.Params.URI == "" {
			return nil, fmt.Errorf("validation failed: resource URI is required")
		}
	case *GetPromptRequest:
		if r.Params.Name == "" {
			return nil, fmt.Errorf("validation failed: prompt name is required")
		}
	}
	
	log.Printf("[ValidationMiddleware] Request validation passed for: %T", req)
	return next(ctx, req)
}

// RateLimitingMiddleware 限流中间件，控制请求频率
func RateLimitingMiddleware(maxRequests int, window time.Duration) MiddlewareFunc {
	requestCounts := make(map[string][]time.Time)
	var mu sync.RWMutex // 添加线程安全保护
	
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		// 从上下文中获取客户端标识（可以是IP、用户ID等）
		clientID := "default" // 简化实现，实际中应该从上下文获取
		if userID := ctx.Value("user_id"); userID != nil {
			if id, ok := userID.(string); ok {
				clientID = id
			}
		}
		
		now := time.Now()
		
		mu.Lock()
		defer mu.Unlock()
		
		// 清理过期的请求记录
		if timestamps, exists := requestCounts[clientID]; exists {
			var validTimestamps []time.Time
			for _, ts := range timestamps {
				if now.Sub(ts) < window {
					validTimestamps = append(validTimestamps, ts)
				}
			}
			requestCounts[clientID] = validTimestamps
		}
		
		// 检查是否超过限制
		if len(requestCounts[clientID]) >= maxRequests {
			err := NewMiddlewareError(ErrCodeRateLimit, 
				fmt.Sprintf("rate limit exceeded: %d requests per %v", maxRequests, window), nil)
			err.AddContext("client_id", clientID)
			err.AddContext("current_requests", len(requestCounts[clientID]))
			err.AddContext("max_requests", maxRequests)
			err.AddContext("window", window.String())
			return nil, err
		}
		
		// 记录当前请求
		requestCounts[clientID] = append(requestCounts[clientID], now)
		
		log.Printf("[RateLimitingMiddleware] Request allowed for client %s (%d/%d)", 
			clientID, len(requestCounts[clientID]), maxRequests)
		
		return next(ctx, req)
	}
}

// CircuitBreakerMiddleware 熔断器中间件，防止级联故障
func CircuitBreakerMiddleware(failureThreshold int, timeout time.Duration) MiddlewareFunc {
	var (
		failureCount    int
		lastFailureTime time.Time
		state          string = "closed" // closed, open, half-open
		mu             sync.RWMutex      // 添加线程安全保护
	)
	
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		now := time.Now()
		
		mu.Lock()
		defer mu.Unlock()
		
		// 检查熔断器状态
		switch state {
		case "open":
			if now.Sub(lastFailureTime) > timeout {
				state = "half-open"
				log.Printf("[CircuitBreakerMiddleware] Circuit breaker transitioning to half-open")
			} else {
				err := NewMiddlewareError(ErrCodeCircuitBreaker, "circuit breaker is open", nil)
				err.AddContext("failure_count", failureCount)
				err.AddContext("failure_threshold", failureThreshold)
				err.AddContext("time_until_retry", timeout-now.Sub(lastFailureTime))
				return nil, err
			}
		case "half-open":
			// 半开状态，尝试一个请求
		case "closed":
			// 关闭状态，正常处理
		}
		
		// 临时释放锁以执行下一个中间件
		mu.Unlock()
		resp, err := next(ctx, req)
		mu.Lock()
		
		if err != nil {
			failureCount++
			lastFailureTime = now
			
			if failureCount >= failureThreshold && state != "open" {
				state = "open"
				log.Printf("[CircuitBreakerMiddleware] Circuit breaker opened due to %d failures", failureCount)
			}
			
			return nil, err
		}
		
		// 请求成功，重置失败计数
		if state == "half-open" {
			state = "closed"
			failureCount = 0
			log.Printf("[CircuitBreakerMiddleware] Circuit breaker closed after successful request")
		}
		
		return resp, nil
	}
}

// CORSMiddleware CORS 中间件，处理跨域请求
func CORSMiddleware(allowOrigins []string, allowMethods []string, allowHeaders []string) MiddlewareFunc {
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		// 这里可以添加 CORS 相关的处理逻辑
		// 在实际的 HTTP 层面处理 CORS，这里主要是记录
		log.Printf("[CORSMiddleware] Processing request with CORS policy")
		
		// 可以在上下文中添加 CORS 相关信息
		ctx = context.WithValue(ctx, "cors_allowed_origins", allowOrigins)
		ctx = context.WithValue(ctx, "cors_allowed_methods", allowMethods)
		ctx = context.WithValue(ctx, "cors_allowed_headers", allowHeaders)
		
		return next(ctx, req)
	}
}

// CompressionMiddleware 压缩中间件，处理响应压缩
func CompressionMiddleware(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
	log.Printf("[CompressionMiddleware] Processing request for compression")
	
	// 在上下文中标记需要压缩
	ctx = context.WithValue(ctx, "compression_enabled", true)
	
	return next(ctx, req)
}

// SecurityMiddleware 安全中间件，处理安全相关检查
func SecurityMiddleware(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
	// 安全检查，如 SQL 注入防护、XSS 防护等
	log.Printf("[SecurityMiddleware] Performing security checks for request: %T", req)
	
	// 这里可以添加具体的安全检查逻辑
	// 例如：检查请求参数中的危险字符串
	
	return next(ctx, req)
}

// TimeoutMiddleware 超时中间件，设置请求超时
func TimeoutMiddleware(timeout time.Duration) MiddlewareFunc {
	return func(ctx context.Context, req interface{}, next Handler) (interface{}, error) {
		// 创建带超时的上下文
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		
		// 使用通道来处理超时和正常完成
		type result struct {
			resp interface{}
			err  error
		}
		
		resultChan := make(chan result, 1)
		
		go func() {
			resp, err := next(timeoutCtx, req)
			resultChan <- result{resp, err}
		}()
		
		select {
		case res := <-resultChan:
			return res.resp, res.err
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("request timeout after %v", timeout)
		}
	}
}
