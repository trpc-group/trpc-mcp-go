// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// STDIO MCP Server Example.
package main

import (
	"context"
	"fmt"
	"log"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	server := mcp.NewStdioServer("clean-stdio-server", "1.0.0")

	registerTools(server)
	log.Printf("Server: clean-stdio-server v1.0.0")

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func registerTools(server *mcp.StdioServer) {
	echoTool := mcp.NewTool("echo",
		mcp.WithDescription("Echo a message back"),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to echo")),
	)

	echoHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, ok := req.Params.Arguments["text"].(string)
		if !ok {
			return nil, fmt.Errorf("missing 'text' parameter")
		}
		return mcp.NewTextResult(fmt.Sprintf("Echo: %s", text)), nil
	}

	server.RegisterTool(echoTool, echoHandler)

	mathTool := mcp.NewTool("add",
		mcp.WithDescription("Add two numbers"),
		mcp.WithNumber("a", mcp.Required(), mcp.Description("First number")),
		mcp.WithNumber("b", mcp.Required(), mcp.Description("Second number")),
	)

	mathHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a, aOk := req.Params.Arguments["a"].(float64)
		b, bOk := req.Params.Arguments["b"].(float64)

		if !aOk || !bOk {
			return nil, fmt.Errorf("invalid number parameters")
		}

		result := a + b
		return mcp.NewTextResult(fmt.Sprintf("Result: %g + %g = %g", a, b, result)), nil
	}

	server.RegisterTool(mathTool, mathHandler)

	log.Printf("Registered tools: echo, add")
}
