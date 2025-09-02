# Quick Start Example

A simple introduction to MCP tool registration and basic usage with Streamable HTTP transport.

## Features

- **Basic Tool Registration**: Two simple tools (`greet` and `advanced-greet`)
- **Parameter Handling**: String and boolean parameters with validation
- **HTTP Transport**: Standard Streamable HTTP with JSON responses

## Quick Start

**Start the server:**
```bash
cd server
go run main.go
```
Server will start on `localhost:3000/mcp`

**Run the client:**
```bash
cd client
go run main.go  
```

## What it demonstrates

1. **Server Setup**: Creating an MCP server with basic configuration
2. **Tool Registration**: How to define and register tools with parameters
3. **Tool Handlers**: Implementing tool logic with context and error handling
4. **Client Usage**: Connecting to server, listing tools, and calling them
5. **Clean Shutdown**: Proper server shutdown with signal handling

Perfect starting point for understanding MCP concepts and API patterns!
