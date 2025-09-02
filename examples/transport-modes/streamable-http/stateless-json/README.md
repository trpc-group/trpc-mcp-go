# Stateless JSON Example

Demonstrates stateless Streamable HTTP transport with JSON responses.

## Configuration

- **Transport**: Streamable HTTP
- **Response Mode**: JSON responses only
- **Session State**: Stateless (no session management)
- **GET SSE**: Disabled

## Features

- Simple request-response pattern
- No persistent connections
- Lightweight and fast
- Perfect for stateless APIs

## Usage

```bash
# Start server
cd server && go run main.go

# Run client  
cd client && go run main.go
```

This is the simplest HTTP configuration - ideal for basic tool calls without session requirements.
