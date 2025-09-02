// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// Simple STDIO server for integration testing.
package main

import (
	"context"
	"fmt"
	"log"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Create STDIO server with simple tools for testing.
	server := mcp.NewStdioServer("e2e-test-server", "1.0.0")

	// Register echo tool.
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

	// Register add tool.
	addTool := mcp.NewTool("add",
		mcp.WithDescription("Add two numbers"),
		mcp.WithNumber("a", mcp.Required(), mcp.Description("First number")),
		mcp.WithNumber("b", mcp.Required(), mcp.Description("Second number")),
	)

	addHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a, aOk := req.Params.Arguments["a"].(float64)
		b, bOk := req.Params.Arguments["b"].(float64)

		if !aOk || !bOk {
			return nil, fmt.Errorf("invalid number parameters")
		}

		result := a + b
		return mcp.NewTextResult(fmt.Sprintf("Result: %g + %g = %g", a, b, result)), nil
	}

	server.RegisterTool(addTool, addHandler)

	// Register a tool to test roots functionality
	listRootsTool := mcp.NewTool("list-roots",
		mcp.WithDescription("List client's root directories"),
	)

	listRootsHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Call ListRoots to get client roots
		roots, err := server.ListRoots(ctx)
		if err != nil {
			return mcp.NewErrorResult(fmt.Sprintf("Failed to list roots: %v", err)), nil
		}

		// Format response
		message := fmt.Sprintf("Client has %d root directories:\n", len(roots.Roots))
		for i, root := range roots.Roots {
			message += fmt.Sprintf("%d. %s (%s)\n", i+1, root.Name, root.URI)
		}

		return mcp.NewTextResult(message), nil
	}

	server.RegisterTool(listRootsTool, listRootsHandler)

	// Register notification handlers
	server.RegisterNotificationHandler("notifications/initialized", func(ctx context.Context, notification *mcp.JSONRPCNotification) error {
		log.Printf("Received initialized notification")
		return nil
	})

	server.RegisterNotificationHandler("notifications/roots/list_changed", func(ctx context.Context, notification *mcp.JSONRPCNotification) error {
		log.Printf("Received roots/list_changed notification")

		// Call ListRoots when notification is received
		roots, err := server.ListRoots(ctx)
		if err != nil {
			log.Printf("Failed to list roots after notification: %v", err)
			return nil
		}

		log.Printf("Received %d roots after notification:", len(roots.Roots))
		for i, root := range roots.Roots {
			log.Printf("  %d. %s (%s)", i+1, root.Name, root.URI)
		}
		return nil
	})

	log.Printf("Registered tools: echo, add, list-roots")
	log.Printf("Registered notification handlers: notifications/initialized, notifications/roots/list_changed")
	log.Printf("Starting E2E Test STDIO MCP Server...")
	log.Printf("Server: e2e-test-server v1.0.0")

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
