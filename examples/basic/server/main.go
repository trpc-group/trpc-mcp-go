package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/examples/basic/tools"
)

func main() {
	// Print startup message.
	log.Printf("Starting basic example server...")

	// Create server, set an API path to "/mcp".
	// Inject custom logger using WithServerTransportLogger.
	// Use WithServerLogger to inject logger at the server level (not to be confused with WithServerTransportLogger for HTTPServerHandler).
	mcpServer := mcp.NewServer(":3000", mcp.Implementation{
		Name:    "Basic-Example-Server",
		Version: "0.1.0",
	}, mcp.WithPathPrefix("/mcp"), mcp.WithServerLogger(mcp.GetDefaultLogger()))

	// Register basic greet tool.
	if err := mcpServer.RegisterTool(tools.NewGreetTool()); err != nil {
		log.Fatalf("Failed to register basic greet tool: %v", err)
	}
	log.Printf("Registered basic greet tool: greet")

	// Register advanced greet tool.
	if err := mcpServer.RegisterTool(tools.NewAdvancedGreetTool()); err != nil {
		log.Fatalf("Failed to register advanced greet tool: %v", err)
	}
	log.Printf("Registered advanced greet tool: advanced-greet")

	// Set up a graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server (run in goroutine).
	go func() {
		log.Printf("MCP server started, listening on port 3000, path /mcp")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for termination signal.
	<-stop
	log.Printf("Shutting down server...")
}
