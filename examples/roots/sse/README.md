# MCP Roots Example - SSE Transport

This example demonstrates how to implement MCP Roots functionality using SSE (Server-Sent Events) transport in `trpc-mcp-go`.

## Overview

The Roots capability allows MCP clients to provide filesystem root directories to servers, enabling servers to understand the client's file structure. SSE transport supports real-time bidirectional communication.

## Running Steps

### 1. Start the SSE Server

```bash
cd server
go run main.go
```

Server will start at `http://localhost:3002/mcp/sse`

### 2. Start the Client

In another terminal:

```bash
cd client
go run main.go
```

## Usage

### Client Interactive Commands

After the client starts, it will show an interactive interface:

```
=== MCP SSE Roots Interactive Demo ===
Commands:
  1. Add root directory        # Add root directory
  2. Remove root directory     # Remove root directory  
  3. List current root directories  # List current root directories
  4. Send roots list changed notification  # Send roots list changed notification
  5. Test server tools         # Test server tools
  q. Quit                      # Quit
```

### Server Tools

The server provides the following tools to demonstrate Roots functionality:

1. **`list_client_roots`** - List all client root directories
2. **`explore_root`** - Explore specified root directory (requires root_index parameter)
3. **`root_stats`** - Show root directory statistics

### Operation Examples

1. **Add root directory**: Enter `1`, then input directory path and name
   - Supports absolute paths or `file://` URI format
   - System will validate if directory exists

2. **Send change notification**: Enter `4`
   - Notify server that root directory list has changed
   - Server will automatically request the latest root directory list

3. **Test server tools**: Enter `5`
   - Call all server tools in sequence
   - Observe how server requests and processes root directory information

## Expected Output

### Server Side

```
2025/07/25 14:24:07 Starting MCP SSE Roots Example Server...
2025/07/25 14:24:07 Registered tools: list_client_roots, explore_root, root_stats
2025/07/25 14:24:07 SSE endpoint: /mcp/sse
2025/07/25 14:24:07 Message endpoint: /mcp/message
2025/07/25 14:24:07 SSE server starting on port 3002...
2025/07/25 14:24:27 🔵 Server received 'initialized' notification
2025/07/25 14:24:27 ✅ Client initialized successfully
2025/07/25 14:24:36 🔵 Server received 'roots/list_changed' notification
2025/07/25 14:24:36 ✅ After roots list changed, server received 4 roots
2025/07/25 14:24:36   1. Working Directory (file:///path/to/working/dir)
2025/07/25 14:24:36   2. Parent Directory (file:///path/to/parent/dir)
2025/07/25 14:24:36   3. Temporary Directory (file:///tmp)
2025/07/25 14:24:36   4. Home Directory (file:///Users/username)
```

### Client Side

```
2025/07/25 14:24:27 Starting MCP SSE Roots Example Client...
2025/07/25 14:24:27 Connecting to SSE server: http://localhost:3002/mcp/sse
2025/07/25 14:24:27 Setting up roots provider...
2025/07/25 14:24:27 Configured 4 root directories:
2025/07/25 14:24:27   1. Working Directory (file:///path/to/working/dir)
2025/07/25 14:24:27   2. Parent Directory (file:///path/to/parent/dir)
2025/07/25 14:24:27   3. Temporary Directory (file:///tmp)
2025/07/25 14:24:27   4. Home Directory (file:///Users/username)
2025/07/25 14:24:27 Connected! Server: SSE-Roots-Example-Server 1.0.0
```

## Important Notes

### SSE Transport Features
- Uses Server-Sent Events for real-time server-to-client communication
- Client sends requests and responses via HTTP POST
- Server sends requests to client via SSE event stream

### Security Considerations
- Carefully choose root directories to expose, avoid exposing sensitive areas
- Client has full control over which directories to expose to the server
- Authentication through SSE session

### Path Format
- Supports local paths and `file://` URI format
- System automatically adds `file://` prefix to ensure correct format
- Use complete URI for matching when removing root directories

### Connection Management
- SSE connection needs to be persistent
- Client automatically handles server's `roots/list` requests
- Server sends requests through event queue

## Common Issues

1. **Connection failed**
   - Ensure server is started and port 3002 is not occupied
   - Check firewall settings
   - Confirm SSE connection is properly established

2. **Tool call timeout**
   - Check if SSE connection is working properly
   - Increase timeout settings
   - Check server logs for request processing status

3. **Root directory updates not timely**
   - Ensure `roots/list_changed` notification is sent (command 4)
   - Check if SSE event stream is working properly
   - Verify client correctly responds to server requests

4. **"Directory does not exist" error**
   - Ensure the input path actually exists
   - Use absolute paths instead of relative paths
   - Check directory permissions

## Debug Tips

- Enable verbose logging to observe message flow
- Use browser developer tools to inspect SSE events
- Monitor HTTP POST requests from client to server
- Check server logs for request processing details 