# Stateless SSE Example

Demonstrates stateless Streamable HTTP transport with SSE streaming responses.

## Configuration

- **Transport**: Streamable HTTP
- **Response Mode**: SSE streaming
- **Session State**: Stateless (no session management)
- **GET SSE**: Disabled

## Features

- Real-time streaming responses
- Server-Sent Events for progressive updates
- No persistent session state
- Ideal for streaming operations

## Usage

```bash
# Start server
cd server && go run main.go

# Run client  
cd client && go run main.go
```

Combines the simplicity of stateless operation with the power of SSE streaming - perfect for real-time data without session complexity.
