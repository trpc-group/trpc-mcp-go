# Stateful JSON Example

Demonstrates stateful Streamable HTTP transport with JSON responses and session management.

## Configuration

- **Transport**: Streamable HTTP
- **Response Mode**: JSON responses only
- **Session State**: Stateful (with session management)
- **GET SSE**: Disabled

## Features

- Session-based communication
- User context preservation
- State management across requests
- Session-aware tool handlers

## Usage

```bash
# Start server
cd server && go run main.go

# Run client  
cd client && go run main.go
```

Adds session management to JSON responses - useful when you need to maintain state between tool calls.
