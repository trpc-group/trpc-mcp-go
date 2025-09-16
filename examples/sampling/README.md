# Sampling Example

A demonstration of MCP server-to-client sampling functionality with bidirectional communication.

## Features

- **Server-to-Client Sampling**: Server requests AI model inference from connected clients
- **Bidirectional Communication**: Full duplex via HTTP sessions and SSE connections
- **Mock AI Handler**: Client-side sampling handler with intelligent prompt processing
- **Session Management**: Proper session routing for multi-client scenarios

## Quick Start

**Start the server:**
```bash
cd server
go run main.go
```
Server will start on `localhost:3002/mcp`

**Run the client:**
```bash
cd client
go run main.go  
```

## What it demonstrates

1. **Sampling-Enabled Server**: Creating an MCP server with sampling capability
2. **Tool Registration**: Tools that trigger server-to-client sampling requests
3. **Client Sampling Handler**: Processing sampling requests and returning AI responses
4. **Bidirectional Flow**: Complete request cycle from client tool call to AI response
5. **Content Parsing**: Handling JSON-serialized message content across protocol boundaries
6. **Session Context**: Using session information to route requests correctly