# tRPC-MCP-Go 日志中间件

## 概述
日志中间件为 tRPC-MCP-Go 服务器提供请求/响应日志记录功能，支持结构化日志、自定义过滤和多种输出格式。该中间件设计用于帮助开发者监控服务器请求、调试问题以及分析性能。

## 功能特性
- **多级日志支持**: 支持 DEBUG, INFO, WARN, ERROR, FATAL 五个日志级别
- **结构化字段记录**: 支持键值对格式的结构化日志记录
- **请求/响应负载记录**: 可选择记录完整的请求和响应数据
- **自定义日志过滤**: 基于日志级别、执行时间和错误状态的智能过滤
- **终端颜色支持**: 自动检测终端并启用彩色输出
- **上下文字段提取**: 从请求上下文中提取自定义字段
- **线程安全设计**: 多 goroutine 环境下的安全使用
- **可扩展性**: 支持自定义 Logger 实现

## 快速开始

### 基本使用
```go
package main

import (
    "context"
    "log"
    
    "trpc.group/trpc-go/trpc-mcp-go"
    "trpc.group/trpc-go/trpc-mcp-go/examples/middlewares/logging"
)

// SimpleLogger 实现基本的日志接口
type SimpleLogger struct{}

func (s *SimpleLogger) Log(ctx context.Context, level logging.Level, msg string, fields ...any) {
    log.Printf("[%s] %s %v", level, msg, fields)
}

func main() {
    // 创建简单的日志中间件
    logger := &SimpleLogger{}
    loggingmiddleware := logging.NewLoggingMiddleware(logger)
    
    // 创建服务器并应用中间件
    server := mcp.NewServer("example-server", "1.0.0")
    server.Use(loggingmiddleware)
    
    // 注册工具和启动服务器...
}
```

### 高级配置
```go
package main

import (
    "context"
    "time"
    
    "trpc.group/trpc-go/trpc-mcp-go"
    "trpc.group/trpc-go/trpc-mcp-go/examples/middlewares/logging"
)

func main() {
    // 创建高级日志中间件
    loggingmiddleware := logging.NewLoggingMiddleware(
        logger,
        logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
            // 记录所有错误请求和执行时间超过100ms的请求
            return err != nil || duration > 100*time.Millisecond
        }),
        logging.WithPayloadLogging(true),        // 启用负载记录
        logging.WithColor(true),                 // 启用颜色输出
        logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
            // 从上下文提取用户信息
            if userID := getUserID(ctx); userID != "" {
                return logging.Fields{"user_id", userID}
            }
            return nil
        }),
    )
    
    server := mcp.NewServer("advanced-server", "1.0.0")
    server.Use(loggingmiddleware)
}
```

## API 文档

### 接口定义

#### Logger 接口
```go
type Logger interface {
    Log(ctx context.Context, level Level, msg string, fields ...any)
}
```

#### LoggerFunc 类型
```go
type LoggerFunc func(ctx context.Context, level Level, msg string, fields ...any)

func (f LoggerFunc) Log(ctx context.Context, level Level, msg string, fields ...any) {
    f(ctx, level, msg, fields...)
}
```

#### Fields 类型
```go
type Fields []interface{}
```

### 配置选项

#### WithShouldLog
自定义日志过滤逻辑，基于日志级别、执行时间和错误状态决定是否记录日志。

```go
func WithShouldLog(f func(level Level, duration time.Duration, err error) bool) Option
```

**示例**:
```go
// 只记录错误和警告
logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
    return level >= logging.LevelWarn || err != nil
})

// 记录所有请求
logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
    return true
})
```

#### WithPayloadLogging
启用或禁用请求和响应负载的记录。

```go
func WithPayloadLogging(enabled bool) Option
```

**注意**: 启用负载记录可能会影响性能，建议在生产环境中谨慎使用。

#### WithColor
启用或禁用彩色日志输出。中间件会自动检测终端支持情况。

```go
func WithColor(enabled bool) Option
```

#### WithFieldsFromContext
从请求上下文中提取自定义字段添加到日志中。

```go
func WithFieldsFromContext(f func(ctx context.Context) Fields) Option
```

**示例**:
```go
logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
    return logging.Fields{
        "request_id", getRequestID(ctx),
        "user_agent", getUserAgent(ctx),
        "ip_address", getClientIP(ctx),
    }
})
```

### 日志级别

```go
const (
    LevelDebug Level = -4
    LevelInfo  Level = 0
    LevelWarn  Level = 4
    LevelError Level = 8
    LevelFatal Level = 12
)
```

#### 级别说明
- **LevelDebug**: 调试信息，用于开发阶段
- **LevelInfo**: 一般信息，记录正常操作
- **LevelWarn**: 警告信息，潜在问题
- **LevelError**: 错误信息，操作失败
- **LevelFatal**: 致命错误，需要立即处理

#### 级别比较
```go
// 检查级别是否启用
if LevelInfo.Enabled(LevelDebug) {
    // Debug 级别被启用
}
```

### 构造函数

#### NewLoggingMiddleware
创建新的日志中间件实例。

```go
func NewLoggingMiddleware(logger Logger, opts ...Option) mcp.MiddlewareFunc
```

**参数**:
- `logger`: 实现 Logger 接口的日志记录器
- `opts`: 可选配置参数

**返回值**:
- `mcp.MiddlewareFunc`: 可用于服务器的中间件函数

## 日志格式

### 基本格式
日志采用结构化格式，包含时间戳、日志级别、消息和键值对字段：

```
[timestamp] [level] message
  key1: value1
  key2: value2
  key3: value3
```

### 示例输出

#### 请求开始日志
```
[2024-01-01 10:00:00.123] [INFO] Request started
  event: request_started
  system: mcp
  span.kind: server
  method: tools/call
  start_time: 2024-01-01T10:00:00.123Z
  session_id: abc123
  request: {
    params: {
      name: "test_tool",
      arguments: {
        message: "hello"
      }
    }
  }
```

#### 请求完成日志
```
[2024-01-01 10:00:00.168] [INFO] Request completed
  event: request_completed
  method: tools/call
  duration_ms: 45
  response: {
    result: {
      content: [
        {
          type: "text",
          text: "TestTool executed at: 2024-01-01T10:00:00.123Z"
        }
      ]
    }
  }
```

#### 错误日志
```
[2024-01-01 10:00:01.234] [ERROR] Request failed
  event: request_failed
  method: tools/call
  duration_ms: 12
  error: {
    message: "something went wrong",
    type: "*errors.errorString"
  }
```

### 颜色输出
中间件支持终端颜色输出，不同级别使用不同颜色：

- **DEBUG**: 蓝色
- **INFO**: 绿色
- **WARN**: 黄色
- **ERROR**: 红色
- **FATAL**: 红色

## 自定义实现

### 实现自定义 Logger
```go
type CustomLogger struct {
    // 自定义字段
}

func (c *CustomLogger) Log(ctx context.Context, level logging.Level, msg string, fields ...any) {
    // 解析字段
    fieldMap := make(map[string]interface{})
    for i := 0; i < len(fields); i += 2 {
        if i+1 < len(fields) {
            key := fmt.Sprintf("%v", fields[i])
            fieldMap[key] = fields[i+1]
        }
    }
    
    // 自定义日志逻辑
    logEntry := map[string]interface{}{
        "timestamp": time.Now(),
        "level":     level.String(),
        "message":   msg,
        "fields":    fieldMap,
    }
    
    // 输出到自定义目标（文件、数据库、远程服务等）
    c.output(logEntry)
}

func (c *CustomLogger) output(entry map[string]interface{}) {
    // 实现自定义输出逻辑
}
```

### 集成第三方日志库

#### 集成 Zap
```go
type ZapLogger struct {
    logger *zap.Logger
}

func NewZapLogger(logger *zap.Logger) *ZapLogger {
    return &ZapLogger{logger: logger}
}

func (z *ZapLogger) Log(ctx context.Context, level logging.Level, msg string, fields ...any) {
    // 将字段转换为 zap.Field
    zapFields := make([]zap.Field, 0, len(fields)/2)
    for i := 0; i < len(fields); i += 2 {
        if i+1 < len(fields) {
            key := fmt.Sprintf("%v", fields[i])
            zapFields = append(zapFields, zap.Any(key, fields[i+1]))
        }
    }
    
    // 根据级别调用对应的 zap 方法
    switch level {
    case logging.LevelDebug:
        z.logger.Debug(msg, zapFields...)
    case logging.LevelInfo:
        z.logger.Info(msg, zapFields...)
    case logging.LevelWarn:
        z.logger.Warn(msg, zapFields...)
    case logging.LevelError:
        z.logger.Error(msg, zapFields...)
    case logging.LevelFatal:
        z.logger.Fatal(msg, zapFields...)
    default:
        z.logger.Info(msg, zapFields...)
    }
}
```

#### 集成 Logrus
```go
type LogrusLogger struct {
    logger *logrus.Logger
}

func NewLogrusLogger(logger *logrus.Logger) *LogrusLogger {
    return &LogrusLogger{logger: logger}
}

func (l *LogrusLogger) Log(ctx context.Context, level logging.Level, msg string, fields ...any) {
    // 创建 logrus.Entry 并添加字段
    entry := l.logger.WithFields(logrus.Fields{})
    for i := 0; i < len(fields); i += 2 {
        if i+1 < len(fields) {
            key := fmt.Sprintf("%v", fields[i])
            entry = entry.WithField(key, fields[i+1])
        }
    }
    
    // 根据级别调用对应的 logrus 方法
    switch level {
    case logging.LevelDebug:
        entry.Debug(msg)
    case logging.LevelInfo:
        entry.Info(msg)
    case logging.LevelWarn:
        entry.Warn(msg)
    case logging.LevelError:
        entry.Error(msg)
    case logging.LevelFatal:
        entry.Fatal(msg)
    default:
        entry.Info(msg)
    }
}
```

## 最佳实践

### 生产环境配置
```go
middleware := logging.NewLoggingMiddleware(
    logger,
    logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
        // 只记录错误和执行时间超过500ms的请求
        return err != nil || duration > 500*time.Millisecond
    }),
    logging.WithPayloadLogging(false),       // 禁用负载记录以提高性能
    logging.WithColor(false),                // 生产环境通常禁用颜色
    logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
        // 只记录关键字段
        return logging.Fields{
            "request_id", getRequestID(ctx),
            "user_id",    getUserID(ctx),
        }
    }),
)
```

### 开发环境配置
```go
middleware := logging.NewLoggingMiddleware(
    logger,
    logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
        // 记录所有请求用于调试
        return true
    }),
    logging.WithPayloadLogging(true),        // 启用负载记录
    logging.WithColor(true),                 // 启用颜色输出
    logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
        // 记录详细的上下文信息
        return logging.Fields{
            "request_id",  getRequestID(ctx),
            "user_id",     getUserID(ctx),
            "user_agent",  getUserAgent(ctx),
            "ip_address",  getClientIP(ctx),
            "trace_id",    getTraceID(ctx),
        }
    }),
)
```

### 性能优化建议

1. **合理设置日志级别**: 生产环境建议只记录 ERROR 和 WARN 级别
2. **谨慎使用负载记录**: 负载记录可能包含敏感信息且影响性能
3. **使用异步日志**: 考虑使用异步日志库避免阻塞请求处理
4. **字段提取优化**: 避免在字段提取函数中执行耗时操作
5. **日志轮转**: 实现日志文件的轮转和归档机制

### 安全考虑

1. **敏感信息过滤**: 避免在日志中记录密码、token等敏感信息
2. **访问控制**: 确保日志文件的访问权限设置正确
3. **数据脱敏**: 对记录的用户数据进行脱敏处理
4. **合规性**: 遵循相关数据保护法规的要求

## 故障排除

### 常见问题

#### 1. 颜色输出不工作
**问题**: 终端中日志没有颜色显示

**解决方案**:
- 检查终端是否支持 ANSI 颜色代码
- 验证环境变量设置:
  ```bash
  export CLICOLOR=1
  export FORCE_COLOR=1
  ```
- 确保输出目标是终端而非文件

#### 2. 日志未记录
**问题**: 某些请求的日志没有出现

**解决方案**:
- 检查 `WithShouldLog` 配置是否过滤掉了这些请求
- 验证日志级别设置是否正确
- 确认 Logger 实现是否正常工作

#### 3. 性能问题
**问题**: 启用日志中间件后服务器性能下降

**解决方案**:
- 禁用负载记录: `WithPayloadLogging(false)`
- 优化日志过滤逻辑
- 考虑使用异步日志库
- 减少字段提取的复杂度

#### 4. 字段格式错误
**问题**: 日志中的字段格式不正确

**解决方案**:
- 确保字段数量为偶数（键值对）
- 检查字段值是否支持 JSON 序列化
- 验证自定义字段提取函数的正确性

### 调试技巧

#### 1. 使用 MockLogger 进行测试
```go
type MockLogger struct {
    logs []string
    mu   sync.Mutex
}

func (m *MockLogger) Log(ctx context.Context, level logging.Level, msg string, fields ...any) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    logEntry := fmt.Sprintf("[%s] %s %v", level, msg, fields)
    m.logs = append(m.logs, logEntry)
}

func (m *MockLogger) GetLogs() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.logs
}

func (m *MockLogger) Clear() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.logs = nil
}
```

#### 2. 启用调试模式
```go
middleware := logging.NewLoggingMiddleware(
    logger,
    logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
        return true // 记录所有请求
    }),
    logging.WithPayloadLogging(true),
    logging.WithColor(true),
)
```

#### 3. 添加上下文信息
```go
logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
    return logging.Fields{
        "debug.request_id", getRequestID(ctx),
        "debug.timestamp", time.Now().Unix(),
        "debug.goroutine", runtime.NumGoroutine(),
    }
})
```

## 示例

### 完整示例
参考 `examples/middleware_usage/logging_middleware_test/main.go` 中的完整实现示例。

### 第三方集成示例
参考 `examples/middlewares/logging/examples/zap_adaptor.go` 中的 Zap 集成示例。

### 测试示例
参考 `examples/middlewares/logging/new_logging_test.go` 中的测试用例。

## 版本历史

### v1.0.0
- 初始版本
- 支持基本日志记录功能
- 支持配置选项
- 支持颜色输出

## 贡献指南

欢迎提交 Issue 和 Pull Request 来改进日志中间件。在提交之前，请确保：

1. 代码符合 Go 语言规范
2. 添加相应的测试用例
3. 更新相关文档
4. 确保所有测试通过

## 许可证

本项目采用 MIT 许可证，详见 LICENSE 文件。