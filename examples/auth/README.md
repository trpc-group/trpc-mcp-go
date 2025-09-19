# OAuth 2.1 Authentication Example

This example demonstrates how to use **trpc-mcp-go** to implement an OAuth 2.1 authentication flow between a client and a server.

## Features

- **OAuth 2.1 Authorization Code Flow**
    - Client registration and redirect handling
    - Token exchange (authorization code & refresh token)
    - Token verification with HMAC-signed JWTs

- **MCP Integration**
    - MCP server with OAuth-protected routes
    - MCP client with integrated authorization flow
    - Example of authenticated tool invocation

- **Infrastructure**
    - Mock OAuth 2.1 server for local testing
    - Support for access & refresh tokens
    - Token introspection and metadata endpoints

## Quick Start

### 1. Start the OAuth Authentication Server
```bash
cd server
go run main.go
```

- Runs a mock OAuth 2.1 server on `http://localhost:3030`
- Starts an MCP server with OAuth-protected endpoints on `http://localhost:3000/mcp`

### 2. Start the OAuth Client
```bash
cd client
go run main.go
```
- Launches an MCP client with OAuth support
- Opens a browser redirect for user authorization
- Completes the authorization code exchange automatically

## What it demonstrates

1. **Client Authorization Flow**: How an MCP client performs OAuth 2.1 authorization using redirect URIs.
2. **Server Protection**: How an MCP server integrates OAuth 2.1 for protecting tools and resources.
3. **JWT Token Handling**: Issuing and verifying HMAC-signed JWT access and refresh tokens.
4. **Metadata & Introspection**: Provides `.well-known` OAuth server metadata and introspection endpoints for compatibility.  
5. **Audit Logging**: Examples of secure server-side logging with sensitive data hashing and reduced verbosity.