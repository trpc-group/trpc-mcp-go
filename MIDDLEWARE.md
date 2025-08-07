# tRPC-MCP-Go 中间件系统

tRPC-MCP-Go 提供了一套灵活、易用的中间件系统，支持请求前/后处理逻辑，助力业务构建可扩展、可观测的 MCP 客户端和服务器。

## 特性

- 🔗 **灵活的中间件链**：支持按注册顺序执行多个中间件
- 🛡️ **异常处理**：内置错误恢复、重试机制
- 📊 **可观测性**：内置日志记录、性能监控中间件
- 🔐 **安全性**：支持认证鉴权、验证中间件
- 🚀 **高性能**：优化的中间件执行链，最小化性能开销
- 🧩 **可扩展**：易于编写自定义中间件
- 🌐 **客户端/服务端支持**：同时支持客户端和服务端中间件
- 🎯 **专用中间件**：针对不同请求类型的专门处理中间件

## 核心组件

### 1. MiddlewareFunc - 中间件函数接口

```go
type MiddlewareFunc func(ctx context.Context, req interface{}, next Handler) (interface{}, error)
```

中间件函数接口支持请求前/后处理逻辑，可以：
- 在请求前执行预处理逻辑（如验证、日志记录）
- 在请求后执行后处理逻辑（如指标收集、响应转换）
- 控制是否继续执行后续中间件

### 2. MiddlewareChain - 中间件执行链管理

```go
type MiddlewareChain struct {
    middlewares []MiddlewareFunc
}
```

按注册顺序执行中间件，提供链式调用管理。

### 3. 专用中间件

- **ToolHandlerMiddleware**：专门处理 CallTool 请求
- **ResourceMiddleware**：处理 ReadResource 请求  
- **PromptMiddleware**：处理 GetPrompt 请求

## 内置中间件

### 基础中间件

1. **LoggingMiddleware** - 日志记录中间件
   ```go
   mcp.WithMiddleware(mcp.LoggingMiddleware)
   mcp.WithServerMiddleware(mcp.LoggingMiddleware)
   ```

2. **RecoveryMiddleware** - 错误恢复中间件
   ```go
   mcp.WithMiddleware(mcp.RecoveryMiddleware)
   mcp.WithServerMiddleware(mcp.RecoveryMiddleware)
   ```

3. **ValidationMiddleware** - 验证中间件
   ```go
   mcp.WithMiddleware(mcp.ValidationMiddleware)
   mcp.WithServerMiddleware(mcp.ValidationMiddleware)
   ```

4. **MetricsMiddleware** - 性能监控中间件
   ```go
   mcp.WithMiddleware(mcp.MetricsMiddleware)
   mcp.WithServerMiddleware(mcp.MetricsMiddleware)
   ```

### 高级中间件

1. **AuthMiddleware** - 认证鉴权中间件
   ```go
   mcp.WithMiddleware(mcp.AuthMiddleware("your-api-key"))
   mcp.WithServerMiddleware(mcp.AuthMiddleware("your-api-key"))
   ```

2. **RetryMiddleware** - 重试中间件
   ```go
   mcp.WithMiddleware(mcp.RetryMiddleware(3)) // 最多重试3次
   mcp.WithServerMiddleware(mcp.RetryMiddleware(3))
   ```

3. **CacheMiddleware** - 缓存中间件
   ```go
   cache := make(map[string]interface{})
   mcp.WithMiddleware(mcp.CacheMiddleware(cache))
   mcp.WithServerMiddleware(mcp.CacheMiddleware(cache))
   ```

4. **RateLimitingMiddleware** - 限流中间件
   ```go
   mcp.WithMiddleware(mcp.RateLimitingMiddleware(100, time.Minute))
   mcp.WithServerMiddleware(mcp.RateLimitingMiddleware(100, time.Minute))
   ```

5. **CircuitBreakerMiddleware** - 熔断器中间件
   ```go
   mcp.WithMiddleware(mcp.CircuitBreakerMiddleware(5, 30*time.Second))
   mcp.WithServerMiddleware(mcp.CircuitBreakerMiddleware(5, 30*time.Second))
   ```

6. **TimeoutMiddleware** - 超时中间件
   ```go
   mcp.WithMiddleware(mcp.TimeoutMiddleware(5*time.Second))
   mcp.WithServerMiddleware(mcp.TimeoutMiddleware(5*time.Second))
   ```

7. **CORSMiddleware** - CORS 中间件
   ```go
   origins := []string{"http://localhost:3000", "https://example.com"}
   methods := []string{"GET", "POST", "PUT", "DELETE"}
   headers := []string{"Content-Type", "Authorization"}
   mcp.WithServerMiddleware(mcp.CORSMiddleware(origins, methods, headers))
   ```

8. **SecurityMiddleware** - 安全中间件
   ```go
   mcp.WithMiddleware(mcp.SecurityMiddleware)
   mcp.WithServerMiddleware(mcp.SecurityMiddleware)
   ```

9. **CompressionMiddleware** - 压缩中间件
   ```go
   mcp.WithMiddleware(mcp.CompressionMiddleware)
   mcp.WithServerMiddleware(mcp.CompressionMiddleware)
   ```

### 专用中间件

1. **ToolHandlerMiddleware** - 工具处理中间件
   ```go
   mcp.WithMiddleware(mcp.ToolHandlerMiddleware)
   ```

2. **ResourceMiddleware** - 资源访问中间件
   ```go
   mcp.WithMiddleware(mcp.ResourceMiddleware)
   ```

3. **PromptMiddleware** - 提示模板中间件
   ```go
   mcp.WithMiddleware(mcp.PromptMiddleware)
   ```

## 使用示例

### 客户端中间件

```go
package main

import (
    "context"
    "log"
    mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
    // 创建带有中间件的客户端
    client, err := mcp.NewClient(
        "http://localhost:3000",
        mcp.Implementation{
            Name:    "MyClient",
            Version: "1.0.0",
        },
        // 添加多个中间件
        mcp.WithMiddleware(mcp.RecoveryMiddleware),
        mcp.WithMiddleware(mcp.LoggingMiddleware),
        mcp.WithMiddleware(mcp.ValidationMiddleware),
        mcp.WithMiddleware(mcp.ToolHandlerMiddleware),
        mcp.WithMiddleware(mcp.ResourceMiddleware),
        mcp.WithMiddleware(mcp.PromptMiddleware),
    )
    
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 正常使用客户端，中间件会自动执行
    result, err := client.CallTool(ctx, &mcp.CallToolRequest{
        Params: mcp.CallToolParams{
            Name: "greet",
            Arguments: map[string]interface{}{
                "name": "World",
            },
        },
    })
}
```

### 服务端中间件

```go
package main

import (
    "log"
    "net/http"
    mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
    // 创建带有中间件的服务器
    server := mcp.NewServer(
        "MyServer",
        "1.0.0",
        // 添加服务端中间件（按执行顺序）
        mcp.WithServerMiddleware(mcp.RecoveryMiddleware),
        mcp.WithServerMiddleware(mcp.LoggingMiddleware),
        mcp.WithServerMiddleware(mcp.MetricsMiddleware),
        mcp.WithServerMiddleware(mcp.ValidationMiddleware),
        mcp.WithServerMiddleware(mcp.RateLimitingMiddleware(100, time.Minute)),
        mcp.WithServerMiddleware(mcp.ToolHandlerMiddleware),
        mcp.WithServerMiddleware(mcp.ResourceMiddleware),
        mcp.WithServerMiddleware(mcp.PromptMiddleware),
    )

    // 注册工具、资源、提示等
    server.RegisterTool("greet", "Greet someone", func(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
        // 工具实现
        return &mcp.CallToolResult{...}, nil
    })

    // 启动服务器
    log.Fatal(http.ListenAndServe(":3000", server.Handler()))
}
```
```

### 自定义中间件

```go
// 自定义日志中间件
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

// 使用自定义中间件
client, err := mcp.NewClient(
    serverURL,
    clientInfo,
    mcp.WithMiddleware(CustomLoggingMiddleware),
)
```

### 带参数的中间件

```go
// 限流中间件
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
            return nil, fmt.Errorf("rate limit exceeded")
        }
        
        requestCount++
        return next(ctx, req)
    }
}

// 使用带参数的中间件
client, err := mcp.NewClient(
    serverURL,
    clientInfo,
    mcp.WithMiddleware(RateLimitingMiddleware(10, time.Minute)),
)
```

### 中间件链的直接使用

```go
// 创建中间件链
chain := mcp.NewMiddlewareChain(
    mcp.LoggingMiddleware,
    mcp.ValidationMiddleware,
    mcp.MetricsMiddleware,
)

// 定义最终处理器
handler := func(ctx context.Context, req interface{}) (interface{}, error) {
    return "response", nil
}

// 执行中间件链
result, err := chain.Execute(ctx, request, handler)
```

## 中间件执行顺序

中间件按照注册顺序执行，形成一个洋葱模型：

```
Request → Middleware1 → Middleware2 → Handler → Middleware2 → Middleware1 → Response
```

例如：
```go
mcp.WithMiddleware(LoggingMiddleware),    // 1. 最外层
mcp.WithMiddleware(AuthMiddleware),      // 2. 中间层  
mcp.WithMiddleware(ValidationMiddleware) // 3. 最内层
```

执行流程：
1. LoggingMiddleware (请求前)
2. AuthMiddleware (请求前)
3. ValidationMiddleware (请求前)
4. 实际处理器
5. ValidationMiddleware (请求后)
6. AuthMiddleware (请求后)
7. LoggingMiddleware (请求后)

## 最佳实践

### 1. 中间件顺序

建议的中间件注册顺序：
```go
mcp.WithMiddleware(mcp.RecoveryMiddleware),    // 最外层：错误恢复
mcp.WithMiddleware(mcp.LoggingMiddleware),     // 日志记录
mcp.WithMiddleware(mcp.MetricsMiddleware),     // 性能监控
mcp.WithMiddleware(AuthMiddleware),            // 认证鉴权
mcp.WithMiddleware(mcp.ValidationMiddleware),  // 请求验证
mcp.WithMiddleware(mcp.RetryMiddleware(3)),    // 重试机制
mcp.WithMiddleware(mcp.ToolHandlerMiddleware), // 专用处理
```

### 2. 错误处理

中间件应当正确处理和传播错误：
```go
func MyMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
    // 请求前处理
    if err := validateRequest(req); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    // 调用下一个处理器
    resp, err := next(ctx, req)
    if err != nil {
        // 记录错误但继续传播
        log.Printf("Request failed: %v", err)
        return nil, err
    }
    
    // 请求后处理
    return transformResponse(resp), nil
}
```

### 3. 上下文使用

利用 context 传递跨中间件的信息：
```go
func AuthMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
    // 验证并在上下文中添加用户信息
    ctx = context.WithValue(ctx, "user_id", "123")
    ctx = context.WithValue(ctx, "authenticated", true)
    
    return next(ctx, req)
}

func LoggingMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
    // 从上下文中获取用户信息
    userID := ctx.Value("user_id")
    log.Printf("Request from user: %v", userID)
    
    return next(ctx, req)
}
```

## 测试

运行中间件测试：
```bash
go test ./... -v -run TestMiddleware
```

运行性能测试：
```bash
go test ./... -bench=BenchmarkMiddleware
```

## 扩展开发

### 创建自定义中间件

1. 实现 `MiddlewareFunc` 接口
2. 处理请求前逻辑
3. 调用 `next(ctx, req)`
4. 处理请求后逻辑
5. 返回结果

### 集成第三方监控

可以轻松集成 Prometheus、Jaeger 等监控系统：

```go
func PrometheusMiddleware(ctx context.Context, req interface{}, next mcp.Handler) (interface{}, error) {
    start := time.Now()
    
    resp, err := next(ctx, req)
    
    // 记录到 Prometheus
    duration := time.Since(start)
    requestDuration.WithLabelValues(fmt.Sprintf("%T", req)).Observe(duration.Seconds())
    
    if err != nil {
        requestErrors.WithLabelValues(fmt.Sprintf("%T", req)).Inc()
    }
    
    return resp, err
}
```

## 贡献

欢迎提交 Issue 和 Pull Request 来改进中间件系统！
