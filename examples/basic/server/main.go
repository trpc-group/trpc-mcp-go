package main

import (
	"os"
	"os/signal"
	"syscall"

	"trpc.group/trpc-go/trpc-mcp-go/examples/basic/tools"
	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/server"
)

func main() {
	// Configure log.
	log.SetLevel(log.InfoLevel)
	log.Info("Starting basic example server...")

	// Create server, set API path to "/mcp".
	mcpServer := server.NewServer(":3000", mcp.Implementation{
		Name:    "Basic-Example-Server",
		Version: "0.1.0",
	}, server.WithPathPrefix("/mcp"))

	// Register basic greet tool.
	if err := mcpServer.RegisterTool(tools.NewGreetTool()); err != nil {
		log.Fatalf("Failed to register basic greet tool: %v", err)
	}
	log.Info("Registered basic greet tool: greet")

	// Register advanced greet tool.
	if err := mcpServer.RegisterTool(tools.NewAdvancedGreetTool()); err != nil {
		log.Fatalf("Failed to register advanced greet tool: %v", err)
	}
	log.Info("Registered advanced greet tool: advanced-greet")

	// Set up graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server (run in goroutine).
	go func() {
		log.Info("MCP server started, listening on port 3000, path /mcp")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for termination signal.
	<-stop
	log.Info("Shutting down server...")
}
