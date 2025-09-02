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
	log.Printf("Starting MCP SSE Roots Example Server...")

	// Create SSE server with roots support
	server := mcp.NewSSEServer(
		"SSE-Roots-Example-Server",
		"1.0.0",
		mcp.WithSSEEndpoint("/sse"),         // SSE endpoint for client connections
		mcp.WithMessageEndpoint("/message"), // Message endpoint for JSON-RPC requests
		mcp.WithBasePath("/mcp"),            // Base path for all endpoints
	)

	// Register notification handlers
	registerNotificationHandlers(server)

	// Register a tool to demonstrate roots functionality
	listRootsTool := mcp.NewTool("list_client_roots",
		mcp.WithDescription("List all root directories provided by the client"),
	)
	server.RegisterTool(listRootsTool, handleListRoots)

	// Register a tool to explore specific root directory
	exploreRootTool := mcp.NewTool("explore_root",
		mcp.WithDescription("Explore files in a specific client root directory"),
		mcp.WithNumber("root_index",
			mcp.Description("Index of the root directory to explore (0-based)"),
		),
	)
	server.RegisterTool(exploreRootTool, handleExploreRoot)

	// Register a tool for root statistics
	rootStatsTool := mcp.NewTool("root_stats",
		mcp.WithDescription("Get statistics about client's root directories"),
	)
	server.RegisterTool(rootStatsTool, handleRootStats)

	// Register the list_files tool that client is trying to call
	listFilesTool := mcp.NewTool("list_files",
		mcp.WithDescription("List files in client's root directories"),
	)
	server.RegisterTool(listFilesTool, handleListFiles)

	log.Printf("Registered tools: list_client_roots, explore_root, root_stats, list_files")
	log.Printf("Registered notification handlers: notifications/initialized, notifications/roots/list_changed")
	log.Printf("SSE endpoint: /mcp/sse")
	log.Printf("Message endpoint: /mcp/message")
	log.Printf("Connect your MCP client to: http://localhost:3002/mcp/sse")

	// Start server in background
	go func() {
		log.Printf("SSE server starting on port 3002...")
		if err := server.Start(":3002"); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for termination signal
	<-stop
	log.Printf("Shutting down server...")
}

// registerNotificationHandlers registers handlers for client notifications
func registerNotificationHandlers(server *mcp.SSEServer) {
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

// handleListRoots demonstrates requesting and displaying client's root directories
func handleListRoots(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Tool called: list_client_roots - requesting client's root directories")

	// 🎯 Use the unified API - just like mcp-go!
	rootsResult, err := getSSEServerFromContext(ctx, req)
	if err != nil {
		return mcp.NewErrorResult(fmt.Sprintf("Failed to get server from context: %v", err)), nil
	}

	// Call ListRoots using the unified API
	if sseServer, ok := rootsResult.(*mcp.SSEServer); ok {
		roots, err := sseServer.ListRoots(ctx)
		if err != nil {
			log.Printf("Failed to get roots: %v", err)
			return mcp.NewErrorResult(fmt.Sprintf("Failed to get roots: %v", err)), nil
		}

		log.Printf("Successfully received %d root directories from client", len(roots.Roots))

		// Format response
		if len(roots.Roots) == 0 {
			return mcp.NewTextResult("Client has no configured root directories."), nil
		}

		message := fmt.Sprintf("🗂️ Client has %d root directories:\n\n", len(roots.Roots))
		for i, root := range roots.Roots {
			message += fmt.Sprintf("  %d. **%s**\n", i+1, root.Name)
			message += fmt.Sprintf("     URI: `%s`\n\n", root.URI)
		}

		return mcp.NewTextResult(message), nil
	}

	return mcp.NewErrorResult("Server context not available"), nil
}

// handleExploreRoot demonstrates exploring a specific root directory
func handleExploreRoot(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Tool called: explore_root - exploring client root directory")

	// Get root index parameter
	rootIndex := 0
	if indexArg, ok := req.Params.Arguments["root_index"]; ok {
		if indexFloat, ok := indexArg.(float64); ok {
			rootIndex = int(indexFloat)
		}
	}

	// Get roots from client
	if server, err := getSSEServerFromContext(ctx, req); err == nil {
		if sseServer, ok := server.(*mcp.SSEServer); ok {
			roots, err := sseServer.ListRoots(ctx)
			if err != nil {
				return mcp.NewErrorResult(fmt.Sprintf("Failed to get roots: %v", err)), nil
			}

			if len(roots.Roots) == 0 {
				return mcp.NewTextResult("No root directories available to explore."), nil
			}

			if rootIndex < 0 || rootIndex >= len(roots.Roots) {
				return mcp.NewErrorResult(fmt.Sprintf("Invalid root index %d. Available indices: 0-%d", rootIndex, len(roots.Roots)-1)), nil
			}

			selectedRoot := roots.Roots[rootIndex]
			message := fmt.Sprintf("📁 Exploring root directory: **%s**\n\n", selectedRoot.Name)
			message += fmt.Sprintf("URI: `%s`\n\n", selectedRoot.URI)
			message += "ℹ️ This is a demonstration. In a real implementation, you would:\n"
			message += "- Parse the URI to get the local file path\n"
			message += "- List directory contents\n"
			message += "- Show file/folder structure\n"
			message += "- Provide file access capabilities\n"

			return mcp.NewTextResult(message), nil
		}
	}

	return mcp.NewErrorResult("Failed to access server context"), nil
}

// handleRootStats demonstrates analyzing root directory information
func handleRootStats(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Tool called: root_stats - analyzing client root directories")

	if server, err := getSSEServerFromContext(ctx, req); err == nil {
		if sseServer, ok := server.(*mcp.SSEServer); ok {
			roots, err := sseServer.ListRoots(ctx)
			if err != nil {
				return mcp.NewErrorResult(fmt.Sprintf("Failed to get roots: %v", err)), nil
			}

			// Analyze roots
			totalRoots := len(roots.Roots)
			var namedRoots, unnamedRoots int
			var uriSchemes = make(map[string]int)

			for _, root := range roots.Roots {
				if root.Name != "" {
					namedRoots++
				} else {
					unnamedRoots++
				}

				// Extract URI scheme
				uri := root.URI
				if len(uri) > 0 {
					// Simple scheme extraction
					if len(uri) > 7 && uri[:7] == "file://" {
						uriSchemes["file"]++
					} else if len(uri) > 7 && uri[:7] == "http://" {
						uriSchemes["http"]++
					} else if len(uri) > 8 && uri[:8] == "https://" {
						uriSchemes["https"]++
					} else {
						uriSchemes["other"]++
					}
				}
			}

			// Format statistics
			message := "📊 **Root Directory Statistics**\n\n"
			message += fmt.Sprintf("Total roots: **%d**\n", totalRoots)
			message += fmt.Sprintf("Named roots: **%d**\n", namedRoots)
			message += fmt.Sprintf("Unnamed roots: **%d**\n\n", unnamedRoots)

			if len(uriSchemes) > 0 {
				message += "**URI Schemes:**\n"
				for scheme, count := range uriSchemes {
					message += fmt.Sprintf("- %s://: %d\n", scheme, count)
				}
			}

			if totalRoots > 0 {
				message += "\n💡 **Usage Tips:**\n"
				message += "- Use `list_client_roots` to see all root directories\n"
				message += "- Use `explore_root` with root_index to explore specific directories\n"
				message += "- Root directories provide controlled access to client's filesystem\n"
			}

			return mcp.NewTextResult(message), nil
		}
	}

	return mcp.NewErrorResult("Failed to access server context"), nil
}

// handleListFiles implements the list_files tool that the client is trying to call
func handleListFiles(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Tool called: list_files - listing files in client's root directories")

	// Get server from context
	server, err := getSSEServerFromContext(ctx, req)
	if err != nil {
		return mcp.NewErrorResult(fmt.Sprintf("Failed to get server from context: %v", err)), nil
	}

	// Call ListRoots using the unified API
	if sseServer, ok := server.(*mcp.SSEServer); ok {
		roots, err := sseServer.ListRoots(ctx)
		if err != nil {
			log.Printf("Failed to get roots: %v", err)
			return mcp.NewErrorResult(fmt.Sprintf("Failed to get roots: %v", err)), nil
		}

		log.Printf("Successfully received %d root directories from client", len(roots.Roots))

		// Format response
		if len(roots.Roots) == 0 {
			return mcp.NewTextResult("Client has no configured root directories to list files from."), nil
		}

		message := fmt.Sprintf("📁 Client has %d root directories with files:\n\n", len(roots.Roots))
		for i, root := range roots.Roots {
			message += fmt.Sprintf("  %d. **%s**\n", i+1, root.Name)
			message += fmt.Sprintf("     URI: `%s`\n", root.URI)
			message += "     Files: (This is a demonstration - would list actual files here)\n\n"
		}

		return mcp.NewTextResult(message), nil
	}

	return mcp.NewErrorResult("Server context not available"), nil
}

// Helper function to get SSE server from context
// Note: In a real implementation, you might want to store the server reference differently
func getSSEServerFromContext(ctx context.Context, req *mcp.CallToolRequest) (interface{}, error) {
	// Try to get server from context
	if server := mcp.GetServerFromContext(ctx); server != nil {
		return server, nil
	}
	return nil, fmt.Errorf("server not available in context")
}
