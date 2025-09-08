# Stateful SSE + GET SSE Example

Full-featured Streamable HTTP configuration with SSE streaming and GET SSE notifications.

## Configuration

- **Transport**: Streamable HTTP
- **Response Mode**: SSE streaming
- **Session State**: Stateful (with session management)
- **GET SSE**: Enabled (server-initiated notifications)

## Features

- SSE streaming for tool responses
- Session management and user context  
- Independent GET SSE for server notifications
- Full bidirectional real-time communication
- Most comprehensive configuration

## Usage

```bash
# Start server
cd server && go run main.go

# Run client  
cd client && go run main.go
```

The most advanced configuration - provides all Streamable HTTP capabilities including streaming responses and server-initiated communication.
