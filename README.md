# Trpc MCP Go: Model Context Protocol Implementation with Streaming HTTP Support

A Go implementation of the [Model Context Protocol (MCP)](https://github.com/model-context-protocol/model-context-protocol) with comprehensive streaming support via Server-Sent Events (SSE). This library enables efficient communication between client applications and tools/resources.

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
	"os"
	"os/signal"
	"syscall"

	"trpc.group/trpc-go/trpc-mcp-go/examples/basic/tools" // Assuming tools package is here
	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/server"
)

func main() {
	// Configure logging
	log.SetLevel(log.InfoLevel)
	log.Info("Starting example server...")

	// Create server
	mcpServer := server.NewServer(":3000", mcp.Implementation{
		Name:    "Example-Server",
		Version: "1.0.0",
	}, server.WithPathPrefix("/mcp"))

	// Register a tool (defined elsewhere)
	if err := mcpServer.RegisterTool(tools.NewGreetTool()); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	
	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	
	// Start server in background
	go func() {
		log.Info("Server started on port 3000")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()
	
	// Wait for termination signal
	<-stop
	log.Info("Shutting down server...")
}
```

### Tool Definition Example

```go
package tools // Assuming this is in a 'tools' sub-package

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

// NewGreetTool creates a simple greeting tool
func NewGreetTool() *mcp.Tool {
	return mcp.NewTool("greet",
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				// Continue execution
			}
			
			// Extract name parameter
			name := "World"
			if nameArg, ok := req.Params.Arguments["name"]; ok {
				if nameStr, ok := nameArg.(string); ok && nameStr != "" {
					name = nameStr
				}
			}
			
			// Create greeting message
			greeting := fmt.Sprintf("Hello, %s!", name)
			
			// Return result
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(greeting)}}, nil
		},
		mcp.WithDescription("A simple greeting tool"),
		mcp.WithString("name", 
			mcp.Description("Name to greet"),
		),
	)
}
```

### Client Example

```go
package main

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-mcp-go/client"
	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

func main() {
	// Initialize logging
	log.SetLevel(log.InfoLevel)
	log.Info("Starting client...")
	
	// Create context
	ctx := context.Background()
	
	// Create client
	mcpClient, err := client.NewClient("http://localhost:3000/mcp", mcp.Implementation{
		Name:    "MCP-Go-Client",
		Version: "1.0.0",
	}, client.WithProtocolVersion(mcp.ProtocolVersion_2024_11_05))
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		return
	}
	defer mcpClient.Close()
	
	// Initialize client
	initResp, err := mcpClient.Initialize(ctx)
	if err != nil {
		log.Errorf("Initialization failed: %v", err)
		return
	}
	log.Infof("Server: %s %s, Protocol: %s", 
		initResp.ServerInfo.Name, 
		initResp.ServerInfo.Version,
		initResp.ProtocolVersion)
	
	// Get session ID
	sessionID := mcpClient.GetSessionID()
	if sessionID != "" {
		log.Infof("Session ID: %s", sessionID)
	}
	
	// List available tools
	toolsResult, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Errorf("Failed to list tools: %v", err)
		return
	}
	
	// Call a tool if available
	if len(toolsResult.Tools) > 0 {
		log.Infof("Calling tool: %s", toolsResult.Tools[0].Name)
		callRes, err := mcpClient.CallTool(ctx, toolsResult.Tools[0].Name, map[string]interface{}{
			"name": "MCP User",
		})
		if err != nil {
			log.Errorf("Tool call failed: %v", err)
			return
		}
		
		// Process results
		for _, item := range callRes.Content {
			if textContent, ok := item.(mcp.TextContent); ok {
				log.Infof("Result: %s", textContent.Text)
			}
		}
	}
	
	// Terminate session if active
	if sessionID != "" {
		if err := mcpClient.TerminateSession(ctx); err != nil {
			log.Errorf("Failed to terminate session: %v", err)
		}
	}
}
```

## Configuration

### Server Configuration

The server can be configured using option functions:

```go
server := server.NewServer(
    ":3000",                        // Listen address
    mcp.Implementation{             // Server info
        Name:    "My-MCP-Server",
        Version: "1.0.0",
    },
    server.WithPathPrefix("/mcp"),            // API path prefix
    server.WithSSEEnabled(true),              // Enable SSE
    server.WithGetSSEEnabled(true),           // Allow GET for SSE
    server.WithDefaultResponseMode("sse"),    // Default to SSE mode
    server.WithStatelessMode(false),          // Use stateful mode
)
```

### Available Server Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithPathPrefix` | Set API path prefix | `/mcp` |
| `WithSessionManager` | Use custom session manager | Built-in manager |
| `WithoutSession` | Disable session management | Sessions enabled |
| `WithSSEEnabled` | Enable SSE responses | `true` |
| `WithGetSSEEnabled` | Allow GET for SSE connections | `true` |
| `WithDefaultResponseMode` | Default mode: "json" or "sse" | `"sse"` |
| `WithNotificationBufferSize` | Size of notification buffer | `10` |
| `WithStatelessMode` | Run in stateless mode | `false` |

### Client Configuration

The client can be configured using option functions:

```go
client, err := client.NewClient(
    "http://localhost:3000/mcp",             // Server URL
    mcp.Implementation{                      // Client info
        Name:    "MCP-Client",
        Version: "1.0.0",
    },
    client.WithProtocolVersion(mcp.ProtocolVersion_2024_11_05),  // Protocol version
    client.WithGetSSEEnabled(true),                                // Use GET for SSE
)
```

### Available Client Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithProtocolVersion` | Specify MCP protocol version | `mcp.ProtocolVersion_2024_11_05` |
| `WithGetSSEEnabled` | Use GET for SSE instead of POST | `false` |
| `WithTransport`     | Use a custom HTTP transport   | Default `http.DefaultTransport` |


## Advanced Features

### Streaming Progress with SSE

Create tools that provide real-time progress updates:

```go
package tools // Assuming this is in a 'tools' sub-package

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)


func NewStreamingTool() *mcp.Tool {
    return mcp.NewTool("sse-progress",
        func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            // Extract parameters
            steps := 5 // Default
            if stepsArg, ok := req.Params.Arguments["steps"]; ok {
                if stepsFloat, ok := stepsArg.(float64); ok {
                    steps = int(stepsFloat)
                }
            }
            
            // Get notification sender from context
            notifier, ok := mcp.GetNotificationSender(ctx)
            if !ok {
                return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("No notification sender available")}}, nil
            }
            
            // Send initial progress
            notifier.SendProgress(0.0, "Starting process")
            notifier.SendLogMessage("info", "Starting process") // Assuming data is a simple string here
            
            // Process steps and send progress updates
            for i := 1; i <= steps; i++ {
                select {
                case <-ctx.Done():
                    return nil, ctx.Err()
                default:
                    // Calculate progress
                    progress := float64(i) / float64(steps)
                    message := fmt.Sprintf("Step %d/%d", i, steps)
                    
                    // Send notifications
                    notifier.SendProgress(progress, message)
                    // For SendLogMessage, the 'data' part might be a structured map or a simple string.
                    // Example with simple string:
                    notifier.SendLogMessage("info", fmt.Sprintf("Completed %s", message))
                    
                    // Simulate work
                    time.Sleep(500 * time.Millisecond)
                }
            }
            
            // Final progress update
            notifier.SendProgress(1.0, "Complete")
            
            return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Process completed successfully")}}, nil
        },
        mcp.WithDescription("Tool with progress notifications"),
        mcp.WithNumber("steps",
            mcp.Description("Number of steps to process"),
            mcp.Default(5),
        ),
    )
}
```

### Client-Side Progress Handling

```go
package main

import (
	"context"
	"fmt"
	// ... other necessary imports like client, log, mcp, transport
	"trpc.group/trpc-go/trpc-mcp-go/client"
	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/transport"
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
	// The 'data' field for log messages can be a string or a structured map.
	// This example assumes it's a simple string for brevity.
	// In a real application, you might need to check its type.
	logMessageText := "Could not extract log message data"
	if dataStr, ok := notification.Params.AdditionalFields["data"].(string); ok {
		logMessageText = dataStr
	} else if dataMap, ok := notification.Params.AdditionalFields["data"].(map[string]interface{}); ok {
		// If it's a map, you might want to format it or extract a specific field.
		// For this example, we'll just try to get a "message" field if it exists.
		if msg, found := dataMap["message"].(string); found {
			logMessageText = msg
		} else {
			logMessageText = fmt.Sprintf("%+v", dataMap) // Or some other formatting
		}
	}
	fmt.Printf("[%s] %s\n", level, logMessageText)
	return nil
}

func main() {
	// ... (client setup as in previous example)
	log.SetLevel(log.InfoLevel)
	ctx := context.Background()
	mcpClient, err := client.NewClient("http://localhost:3000/mcp", mcp.Implementation{
		Name:    "MCP-Go-Client-Stream-Handler",
		Version: "1.0.0",
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()
	// ... (client initialization)
	_, err = mcpClient.Initialize(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}


	// Create notification collector
	collector := &NotificationCollector{}

	// Set up stream options
	streamOpts := &transport.StreamOptions{
		NotificationHandlers: map[string]transport.NotificationHandler{
			"notifications/progress": collector.HandleProgress,
			"notifications/message":  collector.HandleLog,
		},
	}

	// Call tool with streaming
	log.Info("Calling tool with streaming...")
	callRes, err := mcpClient.CallToolWithStream(ctx, "sse-progress", map[string]interface{}{ // Ensure "sse-progress" tool is registered on server
		"steps": 5,
	}, streamOpts)
	if err != nil {
		log.Errorf("Tool call with stream failed: %v", err)
		return
	}

	// Process final result from CallToolWithStream (if any)
	if callRes != nil {
		for _, item := range callRes.Content {
			if textContent, ok := item.(mcp.TextContent); ok {
				log.Infof("Final tool result: %s", textContent.Text)
			}
		}
	}
	// Keep the main function running for a bit to receive notifications if the tool runs asynchronously
	// time.Sleep(10 * time.Second) // Uncomment if needed for testing async notifications
	log.Info("Client example finished.")
}
```

### Resource Management

Register and serve resources:

```go
// Presuming 'server' is an initialized *server.Server instance
// Register a text resource
textResource := &mcp.Resource{
    URI:         "resource://example/text", // Ensure URI scheme is meaningful
    Name:        "example-text",
    Description: "Example text resource",
    MimeType:    "text/plain",
	// Content can be set via a handler or direct data if supported by your server setup.
	// This example focuses on registration. Serving actual content requires a handler.
}
// The actual registration mechanism might vary. If RegisterResource takes a handler:
// server.RegisterResource(textResource, myTextResourceHandler)
// If it's just metadata for now:
err := mcpServer.RegisterResource(textResource, func(ctx context.Context, uri string) (io.ReadCloser, error) {
    // Example handler: return a string reader
    return io.NopCloser(strings.NewReader("This is the content of the text resource.")), nil
})
if err != nil {
    log.Fatalf("Failed to register resource: %v", err)
}


// Resource handler is automatically set up through the HTTP handler
```

### Prompt Templates

Register prompt templates:

```go
// Presuming 'mcpServer' is an initialized *server.Server instance
// Register a prompt template
promptTemplate := &mcp.Prompt{
    Name:        "example-prompt",
    Description: "Example prompt template",
    Arguments: []mcp.PromptArgument{
        {
            Name:        "topic",
            Description: "Topic to discuss",
            Required:    true,
        },
    },
	// Template string itself would be part of how it's handled or stored.
	// For example:
	// Template: "Please tell me more about {{topic}}.",
}
// The registration of the template string itself might be part of the server's prompt handling logic.
// This example focuses on registering the metadata.
err := mcpServer.RegisterPrompt(promptTemplate, func(ctx context.Context, args map[string]interface{}) (string, error) {
    // Example handler:
    topic, ok := args["topic"].(string)
    if !ok {
        return "", fmt.Errorf("topic argument is missing or not a string")
    }
    return fmt.Sprintf("Please tell me more about %s.", topic), nil
})
if err != nil {
    log.Fatalf("Failed to register prompt: %v", err)
}
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

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. 