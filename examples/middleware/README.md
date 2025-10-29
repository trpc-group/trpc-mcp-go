# Middleware Example

This example demonstrates how to use middleware in trpc-mcp-go to add cross-cutting concerns like logging, metrics, authentication, tracing, and request/response interception.

## Server Configuration

This example uses **Stateless JSON Mode** with the following configuration:
- ✅ **Stateless mode**: No session persistence across requests
- ✅ **Pure JSON responses**: No SSE streaming
- ✅ **Simple request-response**: Clean HTTP JSON-RPC
- ✅ **8 Middleware demonstrations**: Complete middleware showcase

Configuration:
```go
mcp.WithStatelessMode(true),   // Enable stateless mode
mcp.WithPostSSEEnabled(false),  // Disable SSE streaming
mcp.WithGetSSEEnabled(false),   // Disable GET SSE notifications
```

## What is Middleware?

Middleware is a function that wraps request processing to add additional functionality. Each middleware can:
- Inspect the request before it's processed
- Add tracing, logging, metrics
- Intercept and modify requests/responses
- Return custom results without calling the actual handler
- Call the next handler in the chain
- Handle errors and implement graceful degradation

## Middleware Signature

```go
type Middleware func(next HandlerFunc) HandlerFunc
type HandlerFunc func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error)
```

Middleware operates at the **JSON-RPC layer**, giving you full control over all MCP methods (tools, prompts, resources, initialize, ping, etc.).

## Example Middlewares

This example includes **8 middleware implementations**:

### 1. TraceMiddleware
Adds unique tracing IDs to track request flow through the system.

### 2. LoggingMiddleware  
Logs request details and execution time.

### 3. MetricsMiddleware
Collects request counts per method. Demonstrates stateful middleware using closures.

### 4. AuthMiddleware
Checks authorization before processing requests.

### 5. InitializeInterceptorMiddleware
Intercepts and enhances the `initialize` response.

### 6. PingInterceptorMiddleware
Intercepts `ping` requests and adds custom timestamps.

### 7. PromptInterceptorMiddleware
Demonstrates prompt interception:
- **prompts/list**: Dynamically adds extra prompts
- **prompts/get**: Returns cached or dynamically generated prompt content

### 8. ToolInterceptorMiddleware
Demonstrates tool interception with multiple scenarios:
- **Mocking**: Return mock data without calling actual handler
- **Caching**: Simulate cached responses
- **Graceful degradation**: Return fallback for failing tools
- **Access control**: Block specific tools

## Execution Order

Middlewares are registered using `WithMiddleware` option and execute in the order specified:

```go
mcp.WithMiddleware(
    TraceMiddleware,                 // First = outermost layer
    LoggingMiddleware,
    MetricsMiddleware,
    AuthMiddleware,
    InitializeInterceptorMiddleware,
    PingInterceptorMiddleware,
    PromptInterceptorMiddleware,
    ToolInterceptorMiddleware,       // Last = closest to core handler
)
```

Request flow (Onion Model):
```
Request → Trace → Logging → Metrics → Auth → Interceptors → Core Handler
                                                                    ↓
Response ← Trace ← Logging ← Metrics ← Auth ← Interceptors ← Core Handler
```

## Running the Example

### Quick Start (Automated)

Run both server and client automatically:

```bash
cd examples/middleware
./run_demo.sh
```

### Manual Start (Two Terminals)

**Terminal 1 - Start Server:**
```bash
cd examples/middleware/server
go build -o middleware-server
./middleware-server
```

**Terminal 2 - Run Client:**
```bash
cd examples/middleware/client
go build -o client
./client
```

## Example Output

### Server Logs

When the client runs, you'll see middleware in action:

```
Starting MCP Server with Middleware...

Registered middlewares:
  1. TraceMiddleware                 - Adds trace IDs
  2. LoggingMiddleware               - Logs requests and responses
  3. MetricsMiddleware               - Collects request metrics
  4. AuthMiddleware                  - Checks authorization
  5. InitializeInterceptorMiddleware - Enhances initialize response
  6. PingInterceptorMiddleware       - Adds timestamp to ping
  7. PromptInterceptorMiddleware     - Intercepts prompt requests
  8. ToolInterceptorMiddleware       - Intercepts specific tools

MCP server started on :3000, access path /mcp

[Trace] trace-1761037757043 | START | Method: initialize
[Logging] → Session: 505f1a1e..., Method: initialize
[Metrics] Request count for initialize: 1
[InitInterceptor] 🚀 Intercepting initialize request
[Logging] ← Method: initialize, Duration: 16.75µs, Success
[Trace] trace-1761037757043 | END   | Method: initialize

[Trace] trace-1761037758651 | START | Method: tools/call
[Logging] → Session: 89abd868..., Method: tools/call
[Metrics] Request count for tools/call: 1
[Auth] ✓ Authorized: session 89abd868... for method tools/call
[Interceptor] 🛡️ Intercepting 'fail' tool for graceful degradation
[Logging] ← Method: tools/call, Duration: 9.375µs, Success
[Trace] trace-1761037758651 | END   | Method: tools/call

[PromptInterceptor] 📋 Intercepting prompts/list request
[PromptInterceptor] ✅ Added 1 intercepted prompt to the list
```

### Client Output

```
=======================================================
Starting Middleware Example Client (Stateless Mode)...
This client demonstrates middleware in action
=======================================================

📡 Connecting to server at http://localhost:3000/mcp...
✅ Client created successfully

🔧 Initializing connection...
✅ Initialization successful!
   Server: Middleware-Example-Server 1.0.0
   Protocol: 2025-03-26
   Note: Stateless mode - no session ID

📋 Listing available tools...
✅ Server provides 3 tools:
   1. hello - Says hello with optional name
   2. fail - Always fails to test error handling
   3. counter - A session counter to demonstrate middleware with stateful sessions

=======================================================
TEST 1: Calling 'hello' tool
=======================================================
✅ Hello tool result:
   📝 Hello, Middleware Tester!

=======================================================
TEST 2: Calling 'counter' tool (demonstrates stateless mode)
=======================================================
📊 In stateless mode, counter will reset each call

🔢 Counter call #1 (increment=1)...
   📝 Counter value: 1 (Session: 6bb050ae...)
🔢 Counter call #2 (increment=1)...
   📝 Counter value: 1 (Session: 61df3093...)
🔢 Counter call #3 (increment=1)...
   📝 Counter value: 1 (Session: 6dd0a565...)

👉 Note: Each call gets a NEW session ID and counter resets to 1

=======================================================
TEST 3: Calling 'fail' tool (demonstrates error handling)
=======================================================
🔴 Intentionally calling a tool that fails...
   📝 🛡️ [DEGRADED] Service temporarily unavailable. Using fallback response.

👉 Note: Tool interceptor provided graceful degradation!

=======================================================
TEST 5: Prompt Interceptor (list & get prompts)
=======================================================
📋 Listing prompts...
✅ Found 3 prompts:
   1. code-analysis - Analyze code and provide suggestions
   2. cached-prompt - A prompt that will be intercepted and cached
   3. intercepted-prompt - 🎯 This prompt was added by middleware!
      🎯 THIS PROMPT WAS ADDED BY MIDDLEWARE!

📝 Getting 'intercepted-prompt' (middleware generated)...
✅ Got intercepted-prompt:
   Description: 🎯 This is a dynamically generated prompt by middleware

💾 Getting 'cached-prompt' (should be cached)...
✅ Got cached-prompt:
   Content: This is cached content, loaded instantly without calling the actual handler!
   💾 CONFIRMED: Came from cache middleware!

=======================================================
SUMMARY
=======================================================
✅ All tests completed successfully!

📊 Middleware Features Demonstrated:
   1. ✅ TraceMiddleware              - Added unique trace IDs
   2. ✅ LoggingMiddleware            - Logged all requests
   3. ✅ MetricsMiddleware            - Counted requests
   4. ✅ AuthMiddleware               - Checked authorization
   5. ✅ InitializeInterceptor        - Enhanced init response
   6. ✅ PingInterceptor              - Added timestamp
   7. ✅ PromptInterceptor            - Intercepted prompts
   8. ✅ ToolInterceptor              - Intercepted tools

📌 Note: Running in stateless mode - no session persistence
👀 Check the server logs to see detailed middleware execution!
=======================================================
```

## Key Features

### 1. Request Interception

Middleware can intercept any MCP method and return custom results:

```go
func ToolInterceptorMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
    return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
        if req.Method == mcp.MethodToolsCall {
            // Parse tool name
            var callReq mcp.CallToolRequest
            if params, ok := req.Params.(map[string]interface{}); ok {
                if name, ok := params["name"].(string); ok {
                    callReq.Params.Name = name
                }
            }
            
            // Intercept specific tool
            if callReq.Params.Name == "expensive-api" {
                // Return mock result, bypassing actual handler
                return mcp.NewTextResult("Mock response!"), nil
            }
        }
        return next(ctx, req)
    }
}
```

### 2. Response Enhancement

Call the handler first, then enhance the result:

```go
func PromptInterceptorMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
    return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
        if req.Method == mcp.MethodPromptsList {
            // Call original handler
            result, err := next(ctx, req)
            if err != nil {
                return nil, err
            }
            
            // Enhance the result
            if promptList, ok := result.(*mcp.ListPromptsResult); ok {
                promptList.Prompts = append(promptList.Prompts, mcp.Prompt{
                    Name:        "dynamic-prompt",
                    Description: "Added by middleware!",
                })
            }
            return result, err
        }
        return next(ctx, req)
    }
}
```

### 3. Stateful Middleware with Closures

```go
func MetricsMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
    requestCounts := make(map[string]int)
    
    return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
        requestCounts[req.Method]++
        log.Printf("Request count for %s: %d", req.Method, requestCounts[req.Method])
        return next(ctx, req)
    }
}
```

### 4. Session Access in Stateless Mode

Even in stateless mode, each request gets a temporary session:

```go
func MyMiddleware(next mcp.HandlerFunc) mcp.HandlerFunc {
    return func(ctx context.Context, req *mcp.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
        // Get session from context (temporary session in stateless mode)
        session := mcp.ClientSessionFromContext(ctx)
        if session != nil {
            log.Printf("Session ID: %s", session.GetID())
        }
        return next(ctx, req)
    }
}
```

## Files Structure

```
examples/middleware/
├── server/
│   ├── main.go              # Server with 8 middlewares
│   └── middleware-server    # Compiled binary
├── client/
│   ├── main.go              # Test client
│   └── client               # Compiled binary
├── README.md                # This file
├── IMPLEMENTATION_NOTES.md  # Developer notes
├── TOOL_INTERCEPTOR.md      # Tool interception guide
├── INTERCEPTOR_EXAMPLES.md  # All interception examples
└── run_demo.sh              # Automated demo script
```

Note: This example shares the parent module's `go.mod` for consistency.

## Use Cases

### 1. Tool Mocking/Testing
Intercept specific tools and return mock data without calling external APIs.

### 2. Caching
Cache tool/prompt results to improve performance and reduce load.

### 3. Graceful Degradation
Return fallback responses when services are unavailable.

### 4. Access Control
Block or filter tools/prompts based on authorization rules.

### 5. Dynamic Content
Load prompts from databases or external sources on-the-fly.

### 6. Logging & Monitoring
Track request patterns, execution times, and error rates.

### 7. Rate Limiting
Control request frequency per client/session.

### 8. Request Transformation
Modify requests before they reach handlers (e.g., parameter validation, normalization).

## Benefits

1. **Clean separation of concerns**: Keep tool handlers focused on business logic
2. **Reusable**: Write once, use across all requests
3. **Composable**: Chain multiple middlewares together
4. **Testable**: Easy to test middleware independently
5. **Powerful**: Can intercept, modify, or replace any MCP operation
6. **Flexible**: Works with all MCP methods (tools, prompts, resources, etc.)
7. **Performance**: Can short-circuit expensive operations (caching, mocking)

## Additional Resources

- See [`TOOL_INTERCEPTOR.md`](TOOL_INTERCEPTOR.md) for detailed tool interception examples
- Check [`INTERCEPTOR_EXAMPLES.md`](INTERCEPTOR_EXAMPLES.md) for comprehensive interception guide
- Look at [`../../README.md#7-how-to-use-middleware`](../../README.md) for FAQ
- Explore test cases in [`../../middleware_test.go`](../../middleware_test.go)
