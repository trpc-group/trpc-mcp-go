# Stateful SSE Example

Demonstrates stateful Streamable HTTP transport with SSE streaming and session management.

## Configuration

- **Transport**: Streamable HTTP
- **Response Mode**: SSE streaming
- **Session State**: Stateful (with session management)
- **GET SSE**: Disabled

## Features

- Real-time streaming responses
- Session-based state management
- Progressive updates with context
- User-aware streaming

## Usage

```bash
# Start server
cd server && go run main.go

# Run client  
cd client && go run main.go
```

The most common streaming configuration - combines session state with real-time SSE streaming for rich interactive experiences.
