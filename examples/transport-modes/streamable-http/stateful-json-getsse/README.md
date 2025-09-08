# Stateful JSON + GET SSE Example

Demonstrates stateful JSON responses with GET SSE endpoint for server notifications.

## Configuration

- **Transport**: Streamable HTTP
- **Response Mode**: JSON responses
- **Session State**: Stateful (with session management)
- **GET SSE**: Enabled (server-initiated notifications)

## Features

- JSON responses for tool calls
- Session management and user context
- Independent GET SSE endpoint for notifications
- Server-initiated communication

## Usage

```bash
# Start server
cd server && go run main.go

# Run client  
cd client && go run main.go
```

Best for applications that need structured JSON responses plus the ability for servers to push notifications to clients.
