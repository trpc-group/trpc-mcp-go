# Server-Sent Events (SSE) Example

This example demonstrates how to use the `trpc-mcp-go` package to implement a Server-Sent Events (SSE) based MCP server and client.

## 🎯 Overview

The SSE example showcases:

- **HTTP-based SSE server** - Implements MCP over Server-Sent Events
- **SSE client** - Connects to the server and receives real-time updates
- **Tool handling** - Shows how to implement and call MCP tools

## 🚀 Quick Start

```bash
# Start the server in one terminal
cd server
go run main.go

# Start the client in another terminal
cd client
go run main.go
```

## 📋 Components

### SSE Server

The server implements a simple MCP server over SSE with:

- **HTTP server** on port 4000
- **SSE endpoint** at `/sse` for event streaming
- **Message endpoint** at `/message` for client-to-server messages
- **Two tools**: `greet` and `weather`
- **Notification support** for real-time updates

### SSE Client

The client connects to the SSE server and:

- **Establishes SSE connection** to the server
- **Registers notification handlers** for server push events
- **Calls server tools** and processes responses
- **Handles notifications** from the server

## 🔧 Tools and Features

### Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `greet` | Greets a user by name | `name`: Name of the person to greet |
| `weather` | Gets weather for a city | `city`: City name (Beijing, Shanghai, etc.) |

### Notifications

The server sends notifications for:

- **Greeting events** - When a user is greeted
- **Process updates** - Progress of simulated long-running tasks

## 📊 Code Examples

### Server Implementation

```go
// Create SSE server
server := mcp.NewSSEServer(
    "SSE Compatibility Server",          // Server name
    "1.0.0",                             // Server version
    mcp.WithSSEServerLogger(logger),     // Use logger
    mcp.WithSSEEndpoint("/sse"),         // Set SSE endpoint
    mcp.WithMessageEndpoint("/message"), // Set message endpoint
)

// Register tools
greetTool := mcp.NewTool("greet",
    mcp.WithDescription("Greet a user by name"),
    mcp.WithString("name", mcp.Description("Name of the person to greet")),
)
server.RegisterTool(greetTool, handleGreet)

// Start server
if err := server.Start(":4000"); err != nil {
    log.Fatalf("Server failed to start: %v", err)
}
```

### Client Implementation

```go
// Create client
mcpClient, err := mcp.NewSSEClient(
    "http://localhost:4000/sse",
    clientInfo,
    mcp.WithClientLogger(logger),
    mcp.WithProtocolVersion(mcp.ProtocolVersion_2024_11_05),
)

// Register notification handler
mcpClient.RegisterNotificationHandler("notifications/message", handleNotification)

// Call a tool
result, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
    Params: mcp.CallToolParams{
        Name: "greet",
        Arguments: map[string]interface{}{
            "name": "SSE compatibility client user",
        },
    },
})
```

## 🏗️ Project Structure

```
sse/
├── client/             # SSE client implementation
│   └── main.go         # Client code
├── server/             # SSE server implementation
│   └── main.go         # Server code
└── README.md           # This documentation
```
