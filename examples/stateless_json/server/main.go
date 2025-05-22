package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// Simple greet tool handler.
func handleGreet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract name from parameters.
	name := "World"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Create response content.
	content := []mcp.Content{
		mcp.NewTextContent(fmt.Sprintf(
			"Hello, %s! This is a greeting from the stateless JSON server.",
			name,
		)),
	}

	return &mcp.CallToolResult{Content: content}, nil
}

func main() {
	// Print startup message.
	log.Printf("Starting Stateless JSON No GET SSE mode MCP server...")

	// Create MCP server with the following configuration:
	// 1. Stateless mode
	// 2. Only return JSON responses (no SSE)
	// 3. Does not support standalone GET SSE
	mcpServer := mcp.NewServer(
		"Stateless-JSON-No-GETSSE-Server", // Server name
		"1.0.0",                           // Server version
		mcp.WithServerAddress(":3001"),    // Server address and port
		mcp.WithServerPath("/mcp"),        // Set API path
		mcp.WithStatelessMode(true),       // Enable stateless mode
		mcp.WithPostSSEEnabled(false),     // Disable SSE
		mcp.WithGetSSEEnabled(false),      // Disable GET SSE
	)

	// Register a greet tool.
	greetTool := mcp.NewTool("greet", handleGreet,
		mcp.WithDescription("A simple greeting tool."),
		mcp.WithString("name", mcp.Description("Name to greet.")))

	if err := mcpServer.RegisterTool(greetTool); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Printf("Registered greet tool: greet")

	// Set up a simple health check route.
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Server is running normally."))
	})

	// Handle graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, exiting...", sig)
		os.Exit(0)
	}()

	// Start server.
	log.Printf("MCP server started at :3001, path /mcp")
	log.Printf(
		"This is a stateless, pure JSON response server - no session ID will be returned, SSE is not used.",
	)
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
