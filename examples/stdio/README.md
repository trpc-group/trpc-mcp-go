# STDIO Multi-Language Compatibility Demo

This example demonstrates that `trpc-mcp-go` STDIO client can successfully connect to MCP servers written in different languages:

- **TypeScript servers** (via `npx`)
- **Python servers** (via `uvx`) 
- **Go servers** (via `go run`)

## 🎯 Overview

The demo validates cross-language MCP compatibility by connecting to:

✅ **TypeScript filesystem server** - File operations via npx  
✅ **Python time server** - Time queries via uvx  
✅ **Go echo server** - Local server with math and echo tools  

## 🚀 Quick Start

```bash
# Test TypeScript server
go run main.go typescript

# Test Python server
go run main.go python

# Test Go server
go run main.go go

# Test all servers
go run main.go all
```

## 📋 Available Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `typescript` | `ts` | Connect to TypeScript filesystem server via npx |
| `python` | `py` | Connect to Python time server via uvx |
| `go` | `golang` | Connect to local Go server via go run |
| `all` | - | Test all three server types sequentially |
| `help` | `-h`, `--help` | Show usage information |

## 🧪 Server Details

### TypeScript Server
- **Package**: `@modelcontextprotocol/server-filesystem`
- **Command**: `npx -y @modelcontextprotocol/server-filesystem /tmp`
- **Capabilities**: File system operations
- **Timeout**: 30 seconds

### Python Server  
- **Package**: `mcp-server-time`
- **Command**: `uvx mcp-server-time --local-timezone=America/New_York`
- **Capabilities**: Time queries and timezone handling
- **Timeout**: 30 seconds
- **Test**: Calls `get_current_time` or `current_time` tool if available

### Go Server
- **Location**: `./server/main.go`
- **Command**: `go run ./server/main.go`
- **Capabilities**: Echo and math operations
- **Timeout**: 30 seconds
- **Test**: Calls `echo` tool with test message

## 🔧 Prerequisites

### Required Tools

```bash
# Install Node.js package runner (for TypeScript servers)
npm install -g npx

# Install Python package runner (for Python servers)  
pip install uvx
# or via pipx: pipx install uvx

# Go toolchain (for Go servers)
go version  # Should be 1.21 or later
```

### Server Installation

The servers are automatically installed when first run:

```bash
# TypeScript servers are auto-installed via npx
npx -y @modelcontextprotocol/server-filesystem /tmp

# Python servers are auto-installed via uvx
uvx mcp-server-time --local-timezone=America/New_York

# Go server runs from source
cd server && go run main.go
```

## 📊 Code Examples

### Basic Client Usage

```go
// TypeScript server via npx
client, err := mcp.NewNpxStdioClient(
    "@modelcontextprotocol/server-filesystem",
    []string{"/tmp"},
    mcp.Implementation{Name: "compatibility-test", Version: "1.0.0"},
    mcp.WithStdioLogger(mcp.GetDefaultLogger()),
)

// Python server via uvx
config := mcp.StdioTransportConfig{
    ServerParams: mcp.StdioServerParameters{
        Command: "uvx",
        Args:    []string{"mcp-server-time", "--local-timezone=America/New_York"},
    },
    Timeout: 30 * time.Second,
}
client, err := mcp.NewStdioClient(config, impl, opts...)

// Go server via go run
config := mcp.StdioTransportConfig{
    ServerParams: mcp.StdioServerParameters{
        Command: "go",
        Args:    []string{"run", "./server/main.go"},
    },
    Timeout: 30 * time.Second,
}
```

### Testing Tools

```go
// Initialize connection
initResp, err := client.Initialize(ctx, &mcp.InitializeRequest{})

// List available tools
toolsResp, err := client.ListTools(ctx, &mcp.ListToolsRequest{})

// Call a tool
callReq := &mcp.CallToolRequest{}
callReq.Params.Name = "echo"
callReq.Params.Arguments = map[string]interface{}{
    "text": "Hello from compatibility test!",
}
callResp, err := client.CallTool(ctx, callReq)
```

## 🏗️ Project Structure

```
stdio/
├── main.go              # Multi-language compatibility demo
├── server/              # Local Go MCP server
│   ├── main.go          # Server implementation
│   ├── go.mod           # Go module file
│   └── go.sum           # Go dependencies
└── README.md            # This documentation
```

## 🔍 Local Go Server

The included Go server (`./server/main.go`) demonstrates a complete MCP server implementation with:

- **Echo tool**: Returns the input text with optional formatting
- **Add tool**: Performs mathematical addition  
- **JSON-RPC 2.0**: Full protocol compliance
- **Proper error handling**: Standard MCP error responses

### Running the Server Standalone

```bash
cd server
go run main.go

# In another terminal, test with client
cd ..
go run main.go go
```

## ✅ Expected Results

When running `go run main.go all`, you should see:

```
Testing All Server Types
============================
Demonstrating cross-language MCP compatibility

Test 1/3: TypeScript Server
---
Testing TypeScript MCP Server
Server: @modelcontextprotocol/server-filesystem
Command: npx -y @modelcontextprotocol/server-filesystem /tmp
Connected! Server: filesystem 0.5.0
Protocol: 2025-03-26
🔧 Found 8 tools
   Example: read_file - Read the complete contents of a file
✅ TypeScript server test completed successfully!

Test 2/3: Python Server
---
🔧 Testing Python MCP Server
🕐 Server: mcp-server-time
🚀 Command: uvx mcp-server-time --local-timezone=America/New_York
✅ Connected! Server: time-server 0.1.0
📞 Protocol: 2025-03-26
🔧 Found 3 tools
   Example: get_current_time - Get the current time
   🕐 Time result: TextContent{type="text", text="2025-01-XX XX:XX:XX EST"}
✅ Python server test completed successfully!

Test 3/3: Go Server
---
Testing Go MCP Server
Server: Local Go server with high-level API
Command: go run ./server/main.go
Connected! Server: stdio-server 1.0.0
Protocol: 2025-03-26
Found 2 tools
   Testing: echo - Echo back any message
Echo result: TextContent{type="text", text="Hello from compatibility test!"}
Go server test completed successfully!

Results: 3/3 servers connected successfully
Perfect! trpc-mcp-go STDIO client is compatible with all tested languages!
```

## 🚨 Troubleshooting

### Common Issues

1. **Command not found**: Ensure `npx`, `uvx`, and `go` are installed and in PATH
2. **Timeout errors**: Servers may take time to start on first run (package installation)
3. **Permission errors**: Check file permissions for server commands

### Debug Mode

```bash
# Enable detailed logging
export MCP_LOG_LEVEL=debug
go run main.go typescript
```

### Manual Testing

```bash
# Test each server type individually
go run main.go typescript
go run main.go python  
go run main.go go

# Check tool availability
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | npx @modelcontextprotocol/server-filesystem /tmp
```

## 🎉 Conclusion

This example proves that `trpc-mcp-go` provides **excellent cross-language compatibility** for STDIO-based MCP servers. The same Go client code successfully connects to and interacts with servers written in:

- **TypeScript** (Node.js ecosystem)
- **Python** (Python ecosystem)  
- **Go** (native Go implementation)

This demonstrates the power and flexibility of the MCP (Model Context Protocol) standard and the `trpc-mcp-go` implementation.

## 📚 Related Examples

- [Basic Examples](../basic/) - Simple MCP client usage
- [HTTP Examples](../http_*/) - HTTP transport examples  
- [SSE Examples](../stateful_sse/) - Server-sent events transport

## 🤝 Contributing

When adding new server compatibility tests:

1. Add test function following the pattern: `testXxxServer(ctx)`
2. Update the `testAllServers()` function to include the new test
3. Add command handling in `main()` function
4. Update this README with server details
5. Test with both individual and `all` commands 