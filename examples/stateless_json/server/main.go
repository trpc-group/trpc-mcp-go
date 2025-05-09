package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/server"
)

// Simple greet tool handler.
func handleGreet(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
	// Extract name from parameters.
	name := "World"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Create response content.
	content := []schema.ToolContent{
		schema.NewTextContent(fmt.Sprintf("Hello, %s! This is a greeting from the stateless JSON server.", name)),
	}

	return &schema.CallToolResult{Content: content}, nil
}

func main() {
	// Set log level.
	log.SetLevel(log.InfoLevel)
	log.Info("Starting Stateless JSON No GET SSE mode MCP server...")

	// Create server info.
	serverInfo := schema.Implementation{
		Name:    "Stateless-JSON-No-GETSSE-Server",
		Version: "1.0.0",
	}

	// Create MCP server with the following configuration:
	// 1. Stateless mode
	// 2. Only return JSON responses (no SSE)
	// 3. Does not support standalone GET SSE
	mcpServer := server.NewServer(
		":3001", // Server address and port.
		serverInfo,
		server.WithPathPrefix("/mcp"),          // Set API path.
		server.WithStatelessMode(true),         // Enable stateless mode.
		server.WithSSEEnabled(false),           // Disable SSE.
		server.WithGetSSEEnabled(false),        // Disable GET SSE.
		server.WithDefaultResponseMode("json"), // Set default response mode to JSON.
	)

	// Register a greet tool.
	greetTool := schema.NewTool("greet", handleGreet,
		schema.WithDescription("A simple greeting tool."),
		schema.WithString("name", schema.Description("Name to greet.")))

	if err := mcpServer.RegisterTool(greetTool); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Info("Registered greet tool: greet")

	// Set up a simple health check route.
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Server is running normally."))
	})

	// Handle graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Infof("Received signal %v, exiting...", sig)
		os.Exit(0)
	}()

	// Start server.
	log.Infof("MCP server started at :3001, path /mcp")
	log.Infof("This is a stateless, pure JSON response server - no session ID will be returned, SSE is not used.")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
