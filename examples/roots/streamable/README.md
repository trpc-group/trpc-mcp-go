# MCP Roots Example - HTTP Transport

This example demonstrates how to implement MCP Roots functionality using HTTP transport in `trpc-mcp-go`.

## Overview

The Roots capability allows MCP clients to provide filesystem root directories to servers, enabling servers to understand the client's file structure.

## Running Steps

### 1. Start the Server

```bash
cd server
go run main.go
```

Server will start at `http://localhost:3001/mcp`

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
=== MCP HTTP Roots Interactive Demo ===
Commands:
  1. Add root directory        # Add root directory
  2. Remove root directory     # Remove root directory  
  3. List current root directories  # List current root directories
  4. Send roots list changed notification  # Send roots list changed notification
  5. Test server tools         # Test server tools
  q. Quit                      # Quit
```

### Operation Examples

1. **Add root directory**: Enter `1`, then input directory path and name
   - Supports absolute paths or `file://` URI format
   - System will validate if directory exists

2. **Send change notification**: Enter `4`
   - Notify server that root directory list has changed
   - Server will automatically request the latest root directory list

3. **Test server tools**: Enter `5`
   - Call server's `list_files` tool
   - Server will request client's root directory information

## Expected Output

### Server Side

```
2025/07/25 14:08:24 Starting MCP Roots example server...
2025/07/25 14:08:24 Registered tools
2025/07/25 14:08:24 Server listening on http://localhost:3001/mcp
2025/07/25 14:08:33 🔵 Server received 'roots/list_changed' notification
2025/07/25 14:08:33 ✅ After roots list changed, server received 4 roots
2025/07/25 14:08:33   1. Working Directory (file:///path/to/working/dir)
2025/07/25 14:08:33   2. Parent Directory (file:///path/to/parent/dir)
2025/07/25 14:08:33   3. Temporary Directory (file:///tmp)
2025/07/25 14:08:33   4. Home Directory (file:///Users/username)
```

### Client Side

```
2025/07/25 14:08:26 Starting MCP HTTP Roots Example Client...
2025/07/25 14:08:26 Connected! Server: Roots-Example-Server 1.0.0
2025/07/25 14:08:26 Configured 4 root directories:
2025/07/25 14:08:26   1. Working Directory (file:///path/to/working/dir)
2025/07/25 14:08:26   2. Parent Directory (file:///path/to/parent/dir)  
2025/07/25 14:08:26   3. Temporary Directory (file:///tmp)
2025/07/25 14:08:26   4. Home Directory (file:///Users/username)
```

## Important Notes

### Security Considerations
- Carefully choose root directories to expose, avoid exposing sensitive areas
- Client has full control over which directories to expose to the server

### Path Format
- Supports local paths and `file://` URI format
- System automatically adds `file://` prefix to ensure correct format
- Use complete URI for matching when removing root directories

### Timing Requirements
- Server only requests root directories after receiving `notifications/roots/list_changed` notification
- Ensure complete connection is established before sending notifications

## Common Issues

1. **"Directory does not exist" error**
   - Ensure the input path actually exists
   - Use absolute paths instead of relative paths

2. **Root directories not showing after addition**
   - Ensure `roots/list_changed` notification is sent (command 4)
   - Check if server is running properly

3. **Connection failed**
   - Ensure server is started and port 3001 is not occupied
   - Check firewall settings 