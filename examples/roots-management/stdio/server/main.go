// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// STDIO MCP Roots Server Example.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	log.Printf("Starting MCP STDIO Roots Example Server...")

	// Create STDIO server with roots support
	server := mcp.NewStdioServer(
		"STDIO-Roots-Example-Server",
		"1.0.0",
		mcp.WithStdioServerLogger(mcp.GetDefaultLogger()),
	)

	// Register tools to demonstrate roots functionality
	registerRootsTools(server)

	// Register notification handlers
	registerNotificationHandlers(server)

	log.Printf("Server: STDIO-Roots-Example-Server v1.0.0")
	log.Printf("Registered tools: list_client_roots, explore_root, root_stats, count_roots")
	log.Printf("Registered notification handlers: notifications/initialized, notifications/roots/list_changed")
	log.Printf("Ready for STDIO communication...")

	// Start the STDIO server (blocks until stdin is closed)
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// registerRootsTools registers tools that demonstrate roots functionality
func registerRootsTools(server *mcp.StdioServer) {
	// Tool 1: List all client roots
	listRootsTool := mcp.NewTool("list_client_roots",
		mcp.WithDescription("List all root directories provided by the client"),
	)
	server.RegisterTool(listRootsTool, handleListClientRoots)

	// Tool 2: Explore specific root directory
	exploreRootTool := mcp.NewTool("explore_root",
		mcp.WithDescription("Explore a specific client root directory"),
		mcp.WithNumber("root_index",
			mcp.Required(),
			mcp.Description("Index of the root directory to explore (0-based)"),
		),
	)
	server.RegisterTool(exploreRootTool, handleExploreRoot)

	// Tool 3: Root directory statistics
	rootStatsTool := mcp.NewTool("root_stats",
		mcp.WithDescription("Get statistics about client's root directories"),
	)
	server.RegisterTool(rootStatsTool, handleRootStats)

	// Tool 4: Count roots (simple example)
	countRootsTool := mcp.NewTool("count_roots",
		mcp.WithDescription("Count the number of root directories provided by client"),
	)
	server.RegisterTool(countRootsTool, handleCountRoots)
}

// registerNotificationHandlers registers handlers for client notifications
func registerNotificationHandlers(server *mcp.StdioServer) {
	// Handle client initialization notification
	server.RegisterNotificationHandler("notifications/initialized", func(ctx context.Context, notification *mcp.JSONRPCNotification) error {
		fmt.Fprintf(os.Stderr, "🔵 Server received 'initialized' notification\n")
		log.Printf("🔵 Server received 'initialized' notification")

		// We can directly use the provided context to call ListRoots
		roots, err := server.ListRoots(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to get roots after initialization: %v\n", err)
			log.Printf("❌ Failed to get roots after initialization: %v", err)
			return nil
		}

		fmt.Fprintf(os.Stderr, "✅ After initialization, server received %d roots\n", len(roots.Roots))
		log.Printf("✅ After initialization, server received %d roots", len(roots.Roots))
		for i, root := range roots.Roots {
			fmt.Fprintf(os.Stderr, "  %d. %s (%s)\n", i+1, root.Name, root.URI)
			log.Printf("  %d. %s (%s)", i+1, root.Name, root.URI)
		}
		return nil
	})

	// Handle roots list changed notification
	server.RegisterNotificationHandler("notifications/roots/list_changed", func(ctx context.Context, notification *mcp.JSONRPCNotification) error {
		fmt.Fprintf(os.Stderr, "🔵 Server received 'roots/list_changed' notification\n")
		log.Printf("🔵 Server received 'roots/list_changed' notification")

		// We can directly use the provided context to call ListRoots
		roots, err := server.ListRoots(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to get roots after list_changed: %v\n", err)
			log.Printf("❌ Failed to get roots after list_changed: %v", err)
			return nil
		}

		fmt.Fprintf(os.Stderr, "✅ After roots list changed, server received %d roots\n", len(roots.Roots))
		log.Printf("✅ After roots list changed, server received %d roots", len(roots.Roots))
		for i, root := range roots.Roots {
			fmt.Fprintf(os.Stderr, "  %d. %s (%s)\n", i+1, root.Name, root.URI)
			log.Printf("  %d. %s (%s)", i+1, root.Name, root.URI)
		}
		return nil
	})
}

// handleListClientRoots lists all root directories from the client
func handleListClientRoots(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Tool called: list_client_roots - requesting client's root directories")

	// 🎯 Use the unified API - just like mcp-go!
	// Get server instance from context to call ListRoots
	server := mcp.GetServerFromContext(ctx)
	if server == nil {
		return mcp.NewErrorResult("Server context not available"), nil
	}

	stdioServer, ok := server.(*mcp.StdioServer)
	if !ok {
		return mcp.NewErrorResult("Invalid server type"), nil
	}

	// Call ListRoots using the unified API
	roots, err := stdioServer.ListRoots(ctx)
	if err != nil {
		log.Printf("Failed to get roots: %v", err)
		return mcp.NewErrorResult(fmt.Sprintf("Failed to get roots: %v", err)), nil
	}

	log.Printf("Successfully received %d root directories from client", len(roots.Roots))

	// Format response
	if len(roots.Roots) == 0 {
		return mcp.NewTextResult("Client has no configured root directories."), nil
	}

	message := fmt.Sprintf("📁 Client has %d root directories:\n\n", len(roots.Roots))
	for i, root := range roots.Roots {
		message += fmt.Sprintf("  %d. %s\n", i+1, root.Name)
		message += fmt.Sprintf("     URI: %s\n\n", root.URI)
	}

	return mcp.NewTextResult(message), nil
}

// handleExploreRoot explores a specific root directory
func handleExploreRoot(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Tool called: explore_root - exploring client root directory")

	// Get root index parameter
	indexArg, ok := req.Params.Arguments["root_index"]
	if !ok {
		return mcp.NewErrorResult("Missing required parameter: root_index"), nil
	}

	rootIndex, ok := indexArg.(float64)
	if !ok {
		return mcp.NewErrorResult("Parameter root_index must be a number"), nil
	}

	rootIndexInt := int(rootIndex)

	// Get server and call ListRoots
	server := mcp.GetServerFromContext(ctx)
	if server == nil {
		return mcp.NewErrorResult("Server context not available"), nil
	}

	stdioServer, ok := server.(*mcp.StdioServer)
	if !ok {
		return mcp.NewErrorResult("Invalid server type"), nil
	}

	roots, err := stdioServer.ListRoots(ctx)
	if err != nil {
		return mcp.NewErrorResult(fmt.Sprintf("Failed to get roots: %v", err)), nil
	}

	if len(roots.Roots) == 0 {
		return mcp.NewTextResult("No root directories available to explore."), nil
	}

	if rootIndexInt < 0 || rootIndexInt >= len(roots.Roots) {
		return mcp.NewErrorResult(fmt.Sprintf("Invalid root index %d. Available indices: 0-%d", rootIndexInt, len(roots.Roots)-1)), nil
	}

	selectedRoot := roots.Roots[rootIndexInt]
	message := fmt.Sprintf("🔍 Exploring root directory: %s\n\n", selectedRoot.Name)
	message += fmt.Sprintf("URI: %s\n\n", selectedRoot.URI)
	message += "ℹ️ This is a demonstration. In a real implementation, you would:\n"
	message += "- Parse the URI to get the local file path\n"
	message += "- List directory contents using filepath.Walk or os.ReadDir\n"
	message += "- Show file/folder structure\n"
	message += "- Provide file access capabilities\n"
	message += "- Implement security checks to stay within root boundaries\n"

	return mcp.NewTextResult(message), nil
}

// handleRootStats provides statistics about root directories
func handleRootStats(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Tool called: root_stats - analyzing client root directories")

	server := mcp.GetServerFromContext(ctx)
	if server == nil {
		return mcp.NewErrorResult("Server context not available"), nil
	}

	stdioServer, ok := server.(*mcp.StdioServer)
	if !ok {
		return mcp.NewErrorResult("Invalid server type"), nil
	}

	roots, err := stdioServer.ListRoots(ctx)
	if err != nil {
		return mcp.NewErrorResult(fmt.Sprintf("Failed to get roots: %v", err)), nil
	}

	// Analyze roots
	totalRoots := len(roots.Roots)
	var namedRoots, unnamedRoots int
	var fileURIs, otherURIs int

	for _, root := range roots.Roots {
		if root.Name != "" {
			namedRoots++
		} else {
			unnamedRoots++
		}

		// Check URI scheme
		if len(root.URI) >= 7 && root.URI[:7] == "file://" {
			fileURIs++
		} else {
			otherURIs++
		}
	}

	// Format statistics
	message := fmt.Sprintf("📊 Root Directory Statistics\n\n")
	message += fmt.Sprintf("Total roots: %d\n", totalRoots)
	message += fmt.Sprintf("Named roots: %d\n", namedRoots)
	message += fmt.Sprintf("Unnamed roots: %d\n", unnamedRoots)
	message += fmt.Sprintf("File URIs: %d\n", fileURIs)
	message += fmt.Sprintf("Other URIs: %d\n", otherURIs)

	if totalRoots > 0 {
		message += "\n💡 Usage Tips:\n"
		message += "- Use 'list_client_roots' to see all root directories\n"
		message += "- Use 'explore_root' with root_index to explore specific directories\n"
		message += "- Use 'count_roots' for a quick count\n"
		message += "- Root directories provide controlled filesystem access\n"
	}

	return mcp.NewTextResult(message), nil
}

// handleCountRoots provides a simple count of root directories
func handleCountRoots(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Output directly to stderr for debugging
	fmt.Fprintf(os.Stderr, "🔵 handleCountRoots: Starting tool execution\n")
	log.Printf("🔵 handleCountRoots: Starting tool execution")

	// Get server instance from context to call ListRoots
	server := mcp.GetServerFromContext(ctx)
	if server == nil {
		fmt.Fprintf(os.Stderr, "❌ handleCountRoots: Server context not available\n")
		log.Printf("❌ handleCountRoots: Server context not available")
		return mcp.NewErrorResult("Server context not available"), nil
	}
	fmt.Fprintf(os.Stderr, "✅ handleCountRoots: Got server from context\n")
	log.Printf("✅ handleCountRoots: Got server from context")

	stdioServer, ok := server.(*mcp.StdioServer)
	if !ok {
		fmt.Fprintf(os.Stderr, "❌ handleCountRoots: Invalid server type\n")
		log.Printf("❌ handleCountRoots: Invalid server type")
		return mcp.NewErrorResult("Invalid server type"), nil
	}
	fmt.Fprintf(os.Stderr, "✅ handleCountRoots: Got STDIO server\n")
	log.Printf("✅ handleCountRoots: Got STDIO server")

	fmt.Fprintf(os.Stderr, "🔵 handleCountRoots: About to call ListRoots\n")
	log.Printf("🔵 handleCountRoots: About to call ListRoots")
	// Call ListRoots using the unified API
	roots, err := stdioServer.ListRoots(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ handleCountRoots: ListRoots failed: %v\n", err)
		log.Printf("❌ handleCountRoots: ListRoots failed: %v", err)
		return mcp.NewErrorResult(fmt.Sprintf("Failed to get roots: %v", err)), nil
	}
	fmt.Fprintf(os.Stderr, "✅ handleCountRoots: ListRoots succeeded, got %d roots\n", len(roots.Roots))
	log.Printf("✅ handleCountRoots: ListRoots succeeded, got %d roots", len(roots.Roots))

	count := len(roots.Roots)
	message := fmt.Sprintf("📊 Client has %d root directories configured.", count)

	return mcp.NewTextResult(message), nil
}
