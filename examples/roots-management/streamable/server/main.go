// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	log.Printf("Starting MCP Roots example server...")

	// Create server with roots support
	mcpServer := mcp.NewServer(
		"Roots-Example-Server",
		"1.0.0",
		mcp.WithServerAddress(":3001"),
		mcp.WithServerPath("/mcp"),
		mcp.WithServerLogger(mcp.GetDefaultLogger()),
	)

	// Register notification handlers
	registerNotificationHandlers(mcpServer)

	// Register a simple tool to demonstrate server functionality
	listFilesTool := mcp.NewTool("list_files",
		mcp.WithDescription("List files in client's root directories"),
		mcp.WithNumber("root_index",
			mcp.Description("Index of the root directory to list (optional)"),
		),
	)

	mcpServer.RegisterTool(listFilesTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Now we can directly get session information from context, just like mcp-go!

		log.Printf("Tool called: list_files - requesting client's root directories")

		// Call ListRoots directly, no need to manually pass sessionID
		rootsResult, err := mcpServer.ListRoots(ctx)
		if err != nil {
			log.Printf("Failed to get roots: %v", err)
			return mcp.NewErrorResult(fmt.Sprintf("Failed to get roots: %v", err)), nil
		}

		log.Printf("Successfully received %d root directories from client", len(rootsResult.Roots))

		// Format response
		if len(rootsResult.Roots) == 0 {
			return mcp.NewTextResult("Client has no configured root directories."), nil
		}

		message := fmt.Sprintf("Client has %d root directories:\n", len(rootsResult.Roots))
		for i, root := range rootsResult.Roots {
			message += fmt.Sprintf("  %d. %s (%s)\n", i+1, root.Name, root.URI)
		}

		return mcp.NewTextResult(message), nil
	})

	log.Printf("Registered tools")
	log.Printf("Registered notification handlers: notifications/initialized, notifications/roots/list_changed")
	log.Printf("Server listening on http://localhost:3001/mcp")
	log.Printf("Connect with a client that supports roots to see the interaction!")

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server (run in goroutine)
	go func() {
		log.Printf("MCP server started, listening on port 3001, path /mcp")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for termination signal
	<-stop
	log.Printf("Shutting down server...")
}

// registerNotificationHandlers registers handlers for client notifications
func registerNotificationHandlers(server *mcp.Server) {
	// Handle client initialization notification
	server.RegisterNotificationHandler("notifications/initialized", func(ctx context.Context, notification *mcp.JSONRPCNotification) error {
		log.Printf("🔵 Server received 'initialized' notification")
		log.Printf("✅ Client initialized successfully")
		return nil
	})

	// Handle roots list changed notification
	server.RegisterNotificationHandler("notifications/roots/list_changed", func(ctx context.Context, notification *mcp.JSONRPCNotification) error {
		log.Printf("🔵 Server received 'roots/list_changed' notification")

		// Call ListRoots to get updated root directories from client
		roots, err := server.ListRoots(ctx)
		if err != nil {
			log.Printf("❌ Failed to get roots after list_changed: %v", err)
			return nil
		}

		log.Printf("✅ After roots list changed, server received %d roots", len(roots.Roots))
		for i, root := range roots.Roots {
			log.Printf("  %d. %s (%s)", i+1, root.Name, root.URI)
		}

		return nil
	})
}
