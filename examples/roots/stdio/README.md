# MCP Roots Example - STDIO Transport

This example demonstrates how to implement MCP Roots functionality using STDIO transport in `trpc-mcp-go`.

## Overview

The Roots capability allows MCP clients to provide filesystem root directories to servers, enabling servers to understand the client's file structure. STDIO transport uses standard input/output for inter-process communication.

## Running Steps

### Run STDIO Client (automatically starts server)

```bash
cd /Users/nanjianyang/MCP-FRAMEWORKS/trpc-mcp-go/examples/roots_stdio
go run main.go
```

The client will automatically start the server process and connect to it.

## Usage

### Client Interactive Commands

After the client starts, it will show an interactive interface:

```
=== MCP STDIO Roots Interactive Demo ===
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
4. **`count_roots`** - Simple count of root directories

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

```
Starting MCP STDIO Roots Example Client...
Creating STDIO client...
Started stdio process: go [run ./server/main.go] (PID: 12345)
Setting up roots provider...
Configured 6 root directories:
  1. Working Directory (file:///path/to/roots_stdio)
  2. Parent Directory (file:///path/to/examples)
  3. Temporary Directory (file:///tmp)
  4. Home Directory (file:///Users/username)
  5. Server Directory (file:///path/to/roots_stdio/server)
  6. Examples Directory (file:///path/to/trpc-mcp-go)
Initializing connection...
Connected! Server: STDIO-Roots-Example-Server 1.0.0, Protocol: 2025-03-26

=== Testing STDIO Roots Functionality ===

--- Test 1: Counting roots ---
Calling tool: count_roots
✅ Tool count_roots completed successfully
Result:
📁 Client has 6 root directories configured.

--- Test 2: Listing client roots ---
Calling tool: list_client_roots
✅ Tool list_client_roots completed successfully
Result:
📁 Client has 6 root directories:
  1. Working Directory
     URI: file:///path/to/roots_stdio
  2. Parent Directory
     URI: file:///path/to/examples
  [... more root directory information]

🎉 STDIO Roots example completed successfully!
```

## Important Notes

### STDIO Transport Features
- Uses standard input/output for inter-process communication
- Server runs as a child process
- Bidirectional communication via JSON-RPC protocol
- Server logs output to stderr, data communication uses stdout

### Security Considerations
- Carefully choose root directories to expose, avoid exposing sensitive areas
- Process isolation provides natural security boundaries
- Server process inherits client's environment and permissions
- Validate file paths to ensure they stay within root boundaries

### Path Format
- Supports local paths and `file://` URI format
- System automatically adds `file://` prefix to ensure correct format
- Use complete URI for matching when removing root directories

### Process Management
- Client manages server process lifecycle
- Server process communicates with client via stdin/stdout
- Client automatically handles server's `roots/list` requests

## Comparison with Other Transports

| Feature | STDIO | HTTP | SSE |
|---------|-------|------|-----|
| **Connection** | Inter-process | HTTP requests | HTTP + SSE |
| **Deployment** | Single binary | Web server | Web server |
| **Session Management** | Process lifetime | HTTP session | HTTP session |
| **Debugging** | Process monitoring | HTTP tools | HTTP + SSE tools |
| **Scalability** | One process per client | Multi-client shared | Multi-client shared |

## Common Issues

1. **Process startup failed**
   - Ensure Go is installed and `./server/main.go` file exists
   - Check if current working directory is correct
   - Confirm no permission issues

2. **Permission denied error**
   - Check if client has access to configured root directories
   - Confirm directory paths are correct and exist

3. **Timeout error**
   - Check if server process is running properly
   - Increase timeout in `StdioTransportConfig`
   - Check server's stderr output

4. **Root directory updates not effective**
   - Ensure `roots/list_changed` notification is sent (command 4)
   - Check if server process communication is working properly
   - Verify client correctly responds to server requests

5. **"Directory does not exist" error**
   - Ensure the input path actually exists
   - Use absolute paths instead of relative paths
   - Check directory permissions

## Debug Tips

### Basic Debugging
- Enable verbose logging to observe message flow
- Monitor server process stderr for error messages
- Check file permissions

### Advanced Debugging
- Log all JSON-RPC messages in transport layer
- Use `strace` (Linux) or `dtruss` (macOS) to monitor process communication
- Redirect server stderr to file for detailed error analysis
- Add timing logs to identify performance bottlenecks

### Common Debug Commands
```bash
# Monitor processes
ps aux | grep "go run"

# View process communication (macOS)
sudo dtruss -p <PID>

# View process communication (Linux)
strace -p <PID>
``` 