# tRPC MCP Go: Model Context Protocol Implementation with Streaming HTTP Support

A Go implementation of the [Model Context Protocol (MCP)](https://github.com/modelcontextprotocol/modelcontextprotocol) with comprehensive streaming HTTP support. This library enables efficient communication between client applications and tools/resources.

## Features

### Core Features

- **Full MCP Specification Support**: Implements MCP, supporting protocol versions up to 2025-03-26 (defaulting to 2024-11-05 for client compatibility in examples).
- **Streaming Support**: Real-time data streaming with Server-Sent Events (SSE)
- **Tool Framework**: Register and execute tools with structured parameter handling
- **Resource Management**: Serve text and binary resources with RESTful interfaces
- **Prompt Templates**: Create and manage prompt templates for LLM interactions
- **Progress Notifications**: Built-in support for progress updates on long-running operations
- **Logging System**: Integrated logging with configurable levels

### Transport Options

- **Server-Sent Events (SSE)**: Efficient one-way streaming from server to client
- **JSON-RPC**: Standard request-response communication

### Connection Modes

- **Stateless**: Simple request-response pattern without persistent sessions
- **Stateful**: Persistent connections with session management

## Installation

```bash
go get trpc.group/trpc-go/trpc-mcp-go
```

## Quick Start

### Server Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Print startup message.
	log.Printf("Starting basic example server...")

	// Create server using the new API style:
	// - First two required parameters: server name and version
	// - WithServerAddress sets the address to listen on (default: "localhost:3000")
	// - WithServerPath sets the API path prefix
	// - WithServerLogger injects logger at the server level
	mcpServer := mcp.NewServer(
		"Basic-Example-Server",
		"0.1.0",
		mcp.WithServerAddress(":3000"),
		mcp.WithServerPath("/mcp"),
		mcp.WithServerLogger(mcp.GetDefaultLogger()),
	)

	// Register basic greet tool.
	greetTool := mcp.NewTool("greet",
		mcp.WithDescription("A simple greeting tool."),
		mcp.WithString("name", mcp.Description("Name to greet.")))

	greetHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Check if the context is cancelled.
		select {
		case <-ctx.Done():
			return mcp.NewErrorResult("Request cancelled"), ctx.Err()
		default:
			// Continue execution.
		}

		// Extract name parameter.
		name := "World"
		if nameArg, ok := req.Params.Arguments["name"]; ok {
			if nameStr, ok := nameArg.(string); ok && nameStr != "" {
				name = nameStr
			}
		}

		// Create greeting message.
		greeting := fmt.Sprintf("Hello, %s!", name)

		// Create tool result.
		return mcp.NewTextResult(greeting), nil
	}

	mcpServer.RegisterTool(greetTool, greetHandler)
	log.Printf("Registered basic greet tool: greet")

	// Set up a graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server (run in goroutine).
	go func() {
		log.Printf("MCP server started, listening on port 3000, path /mcp")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for termination signal.
	<-stop
	log.Printf("Shutting down server...")
}
```

### Client Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// initializeClient initializes the MCP client with server connection and session setup
func initializeClient(ctx context.Context) (*mcp.Client, error) {
	log.Println("===== Initialize client =====")
	serverURL := "http://localhost:3000/mcp"
	mcpClient, err := mcp.NewClient(
		serverURL,
		mcp.Implementation{
			Name:    "MCP-Go-Client",
			Version: "1.0.0",
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("initialization failed: %v", err)
	}

	log.Printf("Server info: %s %s", initResp.ServerInfo.Name, initResp.ServerInfo.Version)
	log.Printf("Protocol version: %s", initResp.ProtocolVersion)
	if initResp.Instructions != "" {
		log.Printf("Server instructions: %s", initResp.Instructions)
	}

	sessionID := mcpClient.GetSessionID()
	if sessionID != "" {
		log.Printf("Session ID: %s", sessionID)
	}

	return mcpClient, nil
}

// handleTools manages tool-related operations including listing and calling tools
func handleTools(ctx context.Context, client *mcp.Client) error {
	log.Println("===== List available tools =====")
	listToolsResp, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %v", err)
	}

	tools := listToolsResp.Tools
	if len(tools) == 0 {
		log.Printf("No available tools.")
		return nil
	}

	log.Printf("Found %d tools:", len(tools))
	for _, tool := range tools {
		log.Printf("- %s: %s", tool.Name, tool.Description)
	}

	// Call the first tool
	log.Printf("===== Call tool: %s =====", tools[0].Name)
	callToolReq := &mcp.CallToolRequest{}
	callToolReq.Params.Name = tools[0].Name
	callToolReq.Params.Arguments = map[string]interface{}{
		"name": "MCP User",
	}
	callToolResp, err := client.CallTool(ctx, callToolReq)
	if err != nil {
		return fmt.Errorf("failed to call tool: %v", err)
	}

	log.Printf("Tool result:")
	for _, item := range callToolResp.Content {
		if textContent, ok := item.(mcp.TextContent); ok {
			log.Printf("  %s", textContent.Text)
		}
	}

	return nil
}

// terminateSession handles the termination of the current session
func terminateSession(ctx context.Context, client *mcp.Client) error {
	sessionID := client.GetSessionID()
	if sessionID == "" {
		return nil
	}

	log.Printf("===== Terminate session =====")
	if err := client.TerminateSession(ctx); err != nil {
		return fmt.Errorf("failed to terminate session: %v", err)
	}
	log.Printf("Session terminated.")
	return nil
}

func main() {
	// Initialize log.
	log.Println("Starting example client...")

	// Create context.
	ctx := context.Background()

	// Initialize client
	client, err := initializeClient(ctx)
	if err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Handle tools
	if err := handleTools(ctx, client); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Terminate session
	if err := terminateSession(ctx, client); err != nil {
		log.Printf("Error: %v\n", err)
	}

	log.Printf("Example finished.")
}
```

## Configuration

### Server Configuration

The server can be configured using option functions:

```go
server := mcp.NewServer(
    "My-MCP-Server",                      // Server name
    "1.0.0",                              // Server version 
    mcp.WithServerAddress(":3000"),       // Listen address
    mcp.WithServerPath("/mcp"),           // API path prefix
    mcp.WithPostSSEEnabled(true),         // Enable SSE responses
    mcp.WithGetSSEEnabled(true),          // Allow GET for SSE
    mcp.WithStatelessMode(false),         // Use stateful mode
    mcp.WithServerLogger(mcp.GetDefaultLogger()), // Custom logger
)
```

### Available Server Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithServerAddress` | Set server address to listen on | `"localhost:3000"` |
| `WithServerPath` | Set API path prefix | `/mcp` |
| `WithServerLogger` | Custom logger for server | Default logger |
| `WithoutSession` | Disable session management | Sessions enabled |
| `WithPostSSEEnabled` | Enable SSE responses | `true` |
| `WithGetSSEEnabled` | Allow GET for SSE connections | `true` |
| `WithNotificationBufferSize` | Size of notification buffer | `10` |
| `WithStatelessMode` | Run in stateless mode | `false` |

### Client Configuration

The client can be configured using option functions:

```go
client, err := mcp.NewClient(
    "http://localhost:3000/mcp",                        // Server URL
    mcp.Implementation{                                 // Client info
        Name:    "MCP-Client",
        Version: "1.0.0",
    },
    mcp.WithProtocolVersion(mcp.ProtocolVersion_2025_03_26),  // Protocol version
    mcp.WithClientGetSSEEnabled(true),                        // Use GET for SSE
    mcp.WithClientLogger(mcp.GetDefaultLogger()),             // Custom logger
)
```

### Available Client Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithProtocolVersion` | Specify MCP protocol version | `mcp.ProtocolVersion_2025_03_26` |
| `WithClientGetSSEEnabled` | Use GET for SSE instead of POST | `false` |
| `WithClientLogger` | Custom logger for client | Default logger |
| `WithClientPath` | Set custom client path | Server path |
| `WithHTTPReqHandler` | Use custom HTTP request handler | Default handler |

## Advanced Features

### Streaming Progress with SSE

Create tools that provide real-time progress updates:

```go
// handleMultiStageGreeting handles the multi-stage greeting tool and sends multiple notifications via SSE.
func handleMultiStageGreeting(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract name from parameters.
	name := "Guest"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	stages := 3
	if stagesArg, ok := req.Params.Arguments["stages"]; ok {
		if stagesFloat, ok := stagesArg.(float64); ok && stagesFloat > 0 {
			stages = int(stagesFloat)
		}
	}

	// Get notification sender from context.
	notificationSender, hasNotificationSender := mcp.GetNotificationSender(ctx)
	if !hasNotificationSender {
		log.Printf("unable to get notification sender from context")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: unable to get notification sender."),
			},
		}, fmt.Errorf("unable to get notification sender from context")
	}

	// Send progress update.
	sendProgress := func(progress float64, message string) {
		err := notificationSender.SendProgress(progress, message)
		if err != nil {
			log.Printf("Failed to send progress notification: %v", err)
		}
	}

	// Send log message.
	sendLogMessage := func(level string, message string) {
		err := notificationSender.SendLogMessage(level, message)
		if err != nil {
			log.Printf("Failed to send log notification: %v", err)
		}
	}

	// Start greeting process.
	sendProgress(0.0, "Start multi-stage greeting")
	sendLogMessage("info", fmt.Sprintf("Start greeting to %s", name))
	time.Sleep(500 * time.Millisecond)

	// Send multiple stage notifications.
	for i := 1; i <= stages; i++ {
		// Check if context is canceled.
		select {
		case <-ctx.Done():
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Greeting canceled at stage %d", i)),
				},
			}, ctx.Err()
		default:
			// Continue sending.
		}

		sendProgress(float64(i)/float64(stages), fmt.Sprintf("Stage %d greeting", i))
		sendLogMessage("info", fmt.Sprintf("Stage %d greeting: Hello %s!", i, name))
		time.Sleep(800 * time.Millisecond)
	}

	// Send final greeting.
	sendProgress(1.0, "Greeting completed")
	sendLogMessage("info", fmt.Sprintf("Completed multi-stage greeting to %s", name))

	// Return final result.
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Completed %d-stage greeting to %s!",
				stages, name,
			)),
		},
	}, nil
}

func main() {
	// Create MCP server
	mcpServer := mcp.NewServer(
		"Multi-Stage-Greeting-Server",
		"1.0.0",
		mcp.WithServerAddress(":3000"),
		mcp.WithServerPath("/mcp"),
		mcp.WithPostSSEEnabled(true),
	)

	// Register a multi-stage greeting tool
	multiStageGreetingTool := mcp.NewTool("multi-stage-greeting",
		mcp.WithDescription("Send multi-stage greeting via SSE."),
		mcp.WithString("name", mcp.Description("Name to greet.")),
		mcp.WithNumber("stages",
			mcp.Description("Number of greeting stages."),
			mcp.Default(3),
		),
	)

	mcpServer.RegisterTool(multiStageGreetingTool, handleMultiStageGreeting)
	log.Printf("Registered multi-stage greeting tool: multi-stage-greeting")

	// Start server
	log.Printf("MCP server started at :3000, path /mcp")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
```

### Client-Side Progress Handling

```go
package main

import (
	"context"
	"fmt"
	"log"

	"trpc.group/trpc-go/trpc-mcp-go"
)

// Example NotificationCollector structure and methods
type NotificationCollector struct{}

func (nc *NotificationCollector) HandleProgress(notification *mcp.JSONRPCNotification) error {
	progress, _ := notification.Params.AdditionalFields["progress"].(float64)
	message, _ := notification.Params.AdditionalFields["message"].(string)
	fmt.Printf("Progress: %.0f%% - %s\n", progress*100, message)
	return nil
}

func (nc *NotificationCollector) HandleLog(notification *mcp.JSONRPCNotification) error {
	level, _ := notification.Params.AdditionalFields["level"].(string)
	data, _ := notification.Params.AdditionalFields["data"].(string)
	fmt.Printf("[%s] %s\n", level, data)
	return nil
}

func main() {
	// Create context
	ctx := context.Background()

	// Initialize client
	client, err := mcp.NewClient(
		"http://localhost:3000/mcp",
		mcp.Implementation{
			Name:    "MCP-Client-Stream-Handler",
			Version: "1.0.0",
		},
		mcp.WithClientGetSSEEnabled(true),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Initialize connection
	_, err = client.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	// Create notification collector
	collector := &NotificationCollector{}

	// Register notification handlers
	client.RegisterNotificationHandler("notifications/progress", collector.HandleProgress)
	client.RegisterNotificationHandler("notifications/message", collector.HandleLog)

	// Call tool with streaming
	log.Printf("Calling multi-stage greeting tool...")
	callRes, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "multi-stage-greeting",
			Arguments: map[string]interface{}{
				"name":   "MCP User",
				"stages": 5,
			},
		},
	})
	if err != nil {
		log.Printf("Tool call failed: %v", err)
		return
	}

	// Process final result
	for _, item := range callRes.Content {
		if textContent, ok := item.(mcp.TextContent); ok {
			log.Printf("Final tool result: %s", textContent.Text)
		}
	}

	log.Printf("Client example finished.")
}
```

### Resource Management

#### Single Content Resources

Register and serve resources with single content:

```go
// Register text resource
textResource := &mcp.Resource{
    URI:         "resource://example/text",
    Name:        "example-text",
    Description: "Example text resource",
    MimeType:    "text/plain",
}

// Define text resource handler
textHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) (mcp.ResourceContents, error) {
    return mcp.TextResourceContents{
        URI:      textResource.URI,
        MIMEType: textResource.MimeType,
        Text:     "This is an example text resource content.",
    }, nil
}

// Register the text resource
server.RegisterResource(textResource, textHandler)
```

#### Multiple Contents Resources (Recommended)

Register resources that can provide multiple content representations:

```go
// Register a resource with multiple content formats
multiResource := &mcp.Resource{
    URI:         "resource://example/document",
    Name:        "example-document",
    Description: "Document available in multiple formats",
    MimeType:    "text/plain",
}

// Define multiple contents handler
multiHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
    return []mcp.ResourceContents{
        // Text representation
        mcp.TextResourceContents{
            URI:      req.Params.URI,
            MIMEType: "text/plain",
            Text:     "Document content in plain text format.",
        },
        // JSON representation
        mcp.BlobResourceContents{
            URI:      req.Params.URI,
            MIMEType: "application/json",
            Blob:     "eyJjb250ZW50IjogIkRvY3VtZW50IGNvbnRlbnQgaW4gSlNPTiBmb3JtYXQuIn0=", // base64 encoded JSON
        },
    }, nil
}

// Register the resource with multiple contents (recommended)
server.RegisterResources(multiResource, multiHandler)
```

The `RegisterResources` method is recommended as it:
- Aligns with the MCP protocol specification
- Allows a single resource to provide multiple content representations
- Supports use cases like serving the same data in different formats (text, JSON, XML, etc.)

### Prompt

Register prompt:

```go
// Register basic prompt
basicPrompt := &mcp.Prompt{
    Name:        "basic-prompt",
    Description: "Basic prompt example",
    Arguments: []mcp.PromptArgument{
        {
            Name:        "name",
            Description: "User name",
            Required:    true,
        },
    },
}

// Define basic prompt handler
basicPromptHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
    name := req.Params.Arguments["name"]
    return &mcp.GetPromptResult{
        Description: basicPrompt.Description,
        Messages: []mcp.PromptMessage{
            {
                Role: "user",
                Content: mcp.TextContent{
                    Type: "text",
                    Text: fmt.Sprintf("Hello, %s! This is a basic prompt example.", name),
                },
            },
        },
    }, nil
}

// Register the basic prompt
server.RegisterPrompt(basicPrompt, basicPromptHandler)
```

## Example Patterns

The project includes several example patterns:

| Pattern | Description |
|---------|-------------|
| `basic` | Simple tool registration and usage |
| `resource_prompt_example` | Resource and prompt template examples |
| `stateful_json` | Stateful connections with JSON-RPC |
| `stateful_sse` | Stateful connections with SSE |
| `stateful_json_getsse` | Stateful JSON with GET SSE support |
| `stateful_sse_getsse` | Stateful SSE with GET SSE support |
| `stateless_json` | Stateless connections with JSON-RPC |
| `stateless_sse` | Stateless connections with SSE |

## FAQ

### 1. How to handle HTTP Headers?

**Q: How can I extract HTTP headers on the server side and send custom headers from the client?**

**A:** The library provides comprehensive HTTP header support for both server and client sides while maintaining transport layer independence.

#### Server Side: Extracting HTTP Headers

Use `WithHTTPContextFunc` to extract HTTP headers and make them available in tool handlers through context:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// Define context keys for type safety
type contextKey string
const (
	AuthTokenKey contextKey = "auth_token"
	UserAgentKey contextKey = "user_agent"
)

// HTTP context functions to extract headers
func extractAuthToken(ctx context.Context, r *http.Request) context.Context {
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		return context.WithValue(ctx, AuthTokenKey, authHeader)
	}
	return ctx
}

func extractUserAgent(ctx context.Context, r *http.Request) context.Context {
	if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
		return context.WithValue(ctx, UserAgentKey, userAgent)
	}
	return ctx
}

// Tool handler that accesses headers via context
func myTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract headers from context
	authToken, _ := ctx.Value(AuthTokenKey).(string)
	userAgent, _ := ctx.Value(UserAgentKey).(string)
	
	// Use header information in your tool logic
	response := fmt.Sprintf("Received auth: %s, user-agent: %s", authToken, userAgent)
	return mcp.NewTextResult(response), nil
}

func main() {
	// Create server with HTTP context functions
	server := mcp.NewServer(
		"header-server", "1.0.0",
		// Register multiple HTTP context functions
		mcp.WithHTTPContextFunc(extractAuthToken),
		mcp.WithHTTPContextFunc(extractUserAgent),
	)
	
	// Register tool
	tool := mcp.NewTool("my-tool", mcp.WithDescription("Tool that uses headers"))
	server.RegisterTool(tool, myTool)
	
	// Start server
	server.Start()
}
```

#### Client Side: Sending Custom HTTP Headers

Use `WithHTTPHeaders` to send custom HTTP headers with all requests:

```go
package main

import (
	"context"
	"log"
	"net/http"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Create custom headers
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer your-token-here")
	headers.Set("User-Agent", "MyMCPClient/1.0")
	headers.Set("X-Custom-Header", "custom-value")
	
	// Create client with custom headers
	client, err := mcp.NewClient(
		"http://localhost:3000/mcp",
		mcp.Implementation{
			Name:    "my-client",
			Version: "1.0.0",
		},
		mcp.WithHTTPHeaders(headers), // All requests will include these headers
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	// Initialize and use client normally
	// Headers will be automatically included in all HTTP requests
	_, err = client.Initialize(context.Background(), &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	
	// Call tools - headers are automatically included
	result, err := client.CallTool(context.Background(), &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "my-tool",
			Arguments: map[string]interface{}{
				"message": "Hello with headers!",
			},
		},
	})
	// Handle result...
}
```

#### Key Features

- **Transport Layer Independence**: Tool handlers access headers through context, not directly from HTTP requests
- **Multiple Context Functions**: Use multiple `WithHTTPContextFunc` calls to extract different headers
- **Type Safety**: Use strongly-typed context keys to avoid conflicts
- **Automatic Application**: Client headers are applied to all HTTP requests (initialize, tool calls, notifications, etc.)
- **Backward Compatibility**: Optional feature that doesn't break existing APIs

#### Complete Example

See `examples/headers/` for a complete working example demonstrating both server-side header extraction and client-side header sending.

### 2. How to get server name and version in a tool handler?

You can retrieve the `mcp.Server` instance from the `context.Context` within your tool handler and then call its `GetServerInfo()` method:

```go
func handleMyTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    serverInstance := mcp.GetServerFromContext(ctx)
    if mcpServer, ok := serverInstance.(*mcp.Server); ok {
        serverInfo := mcpServer.GetServerInfo()
        // Now you have serverInfo.Name and serverInfo.Version
        return mcp.NewTextResult(fmt.Sprintf("Server Name: %s, Version: %s", serverInfo.Name, serverInfo.Version)), nil
    }
    return mcp.NewTextResult("Could not retrieve server information."), nil
}
```

### 3. How to dynamically filter tools based on user context?

The library provides a powerful tool filtering system that works with HTTP header extraction to enable dynamic access control. You can filter tools based on user context such as roles, permissions, client version, or any other criteria.

#### Basic Context-Based Filtering

Use `WithToolListFilter` combined with `WithHTTPContextFunc` to filter tools based on user context (e.g., roles, permissions, version, etc.):

```go
package main

import (
	"context"
	"net/http"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// Extract user role from HTTP headers
func extractUserRole(ctx context.Context, r *http.Request) context.Context {
	if userRole := r.Header.Get("X-User-Role"); userRole != "" {
		ctx = context.WithValue(ctx, "user_role", userRole)
	}
	return ctx
}

// Create role-based filter
func createRoleBasedFilter() mcp.ToolListFilter {
	return func(ctx context.Context, tools []*mcp.Tool) []*mcp.Tool {
		userRole := "guest" // default
		if role, ok := ctx.Value("user_role").(string); ok && role != "" {
			userRole = role
		}

		var filtered []*mcp.Tool
		for _, tool := range tools {
			switch userRole {
			case "admin":
				filtered = append(filtered, tool) // Admin sees all tools
			case "user":
				if tool.Name == "calculator" || tool.Name == "weather" {
					filtered = append(filtered, tool)
				}
			case "guest":
				if tool.Name == "calculator" {
					filtered = append(filtered, tool)
				}
			}
		}
		return filtered
	}
}

func main() {
	// Create server with context-based tool filtering
	server := mcp.NewServer(
		"filtered-server", "1.0.0",
		mcp.WithHTTPContextFunc(extractUserRole),
		mcp.WithToolListFilter(createRoleBasedFilter()),
	)

	// Register tools as usual
	server.RegisterTool(calculatorTool, calculatorHandler)
	server.RegisterTool(weatherTool, weatherHandler)
	server.RegisterTool(adminTool, adminHandler)
	
	server.Start()
}
```

#### Advanced Custom Filter Functions

You can create more complex filtering logic:

```go
func createAdvancedFilter() mcp.ToolListFilter {
	return func(ctx context.Context, tools []*mcp.Tool) []*mcp.Tool {
		// Extract user information from context
		userRole := "guest"
		if role, ok := ctx.Value("user_role").(string); ok && role != "" {
			userRole = role
		}
		
		clientVersion := ""
		if version, ok := ctx.Value("client_version").(string); ok {
			clientVersion = version
		}
		
		// Complex business logic
		if userRole == "premium" && clientVersion == "2.0.0" {
			return tools // Premium users with v2.0.0 see all tools
		}
		
		var filtered []*mcp.Tool
		for _, tool := range tools {
			if shouldShowTool(tool, userRole, clientVersion) {
				filtered = append(filtered, tool)
			}
		}
		return filtered
	}
}

// Helper function for business logic
func shouldShowTool(tool *mcp.Tool, userRole, clientVersion string) bool {
	// Implement your business logic here
	switch tool.Name {
	case "calculator":
		return true // Available to everyone
	case "weather":
		return userRole == "user" || userRole == "admin"
	case "admin_panel":
		return userRole == "admin"
	default:
		return false
	}
}
```

#### Testing Tool Filtering

You can test different user contexts by sending HTTP headers:

```bash
# Admin sees all tools
curl -X POST http://localhost:3000/mcp \
  -H "Content-Type: application/json" \
  -H "X-User-Role: admin" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}'

# User sees filtered tools
curl -X POST http://localhost:3000/mcp \
  -H "Content-Type: application/json" \
  -H "X-User-Role: user" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}'
```

#### Key Benefits

- **Flexible Filtering**: Create custom filters based on any context information
- **Transport Independence**: Works with both stateful and stateless modes
- **Performance**: Filters run only when tools are listed, not on every request
- **Security**: Tools are completely hidden from unauthorized users

## Copyright

The copyright notice pertaining to the Tencent code in this repo was previously in the name of “THL A29 Limited.”  That entity has now been de-registered.  You should treat all previously distributed copies of the code as if the copyright notice was in the name of “Tencent.”
