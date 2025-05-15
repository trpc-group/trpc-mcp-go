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

// Simple greeting tool handler function.
func handleGreet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session from context (if any).
	session, ok := mcp.GetSessionFromContext(ctx)

	// Extract name from parameters.
	name, _ := req.Params.Arguments["name"].(string)
	if name == "" {
		name = "World"
	}

	// Create response content, customize message if session exists.
	var content []mcp.Content

	if ok && session != nil {
		content = append(content, mcp.NewTextContent(fmt.Sprintf(
			"Hello, %s! This is a greeting from the stateful JSON server. Your session ID is: %s",
			name, session.GetID()[:8]+"...")))
	} else {
		content = append(content, mcp.NewTextContent(fmt.Sprintf(
			"Hello, %s! This is a greeting from the stateful JSON server, but session info could not be obtained.",
			name)))
	}

	return &mcp.CallToolResult{Content: content}, nil
}

// Counter-tool, used to demonstrate session state management.
func handleCounter(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session.
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return mcp.NewErrorResult("Error: Could not get session info. This tool requires a stateful session."),
			fmt.Errorf("failed to get session from context")
	}

	// Get counter from session data.
	var count int
	if data, exists := session.GetData("counter"); exists {
		count, _ = data.(int)
	}

	// Increase counter.
	increment := 1
	if inc, ok := req.Params.Arguments["increment"].(float64); ok {
		increment = int(inc)
	}

	count += increment

	// Save back to session.
	session.SetData("counter", count)

	// Return result.
	return mcp.NewTextResult(fmt.Sprintf("Counter current value: %d (Session ID: %s)",
		count, session.GetID()[:8]+"...")), nil
}

func main() {
	// Print server start message.
	log.Printf("Starting Stateful JSON No GET SSE mode MCP server...")

	// Create session manager (valid for 1 hour).
	sessionManager := mcp.NewSessionManager(3600)

	// Create MCP server, configured as:
	// 1. Stateful mode (using sessionManager)
	// 2. Only return JSON responses (do not use SSE)
	// 3. GET SSE is not supported
	mcpServer := mcp.NewServer(
		"Stateful-JSON-Server",                 // Server name
		"1.0.0",                                // Server version
		mcp.WithServerAddress(":3003"),         // Server address and port
		mcp.WithServerPath("/mcp"),             // Set API path
		mcp.WithSessionManager(sessionManager), // Use session manager (stateful)
		mcp.WithPostSSEEnabled(false),          // Disable SSE
		mcp.WithGetSSEEnabled(false),           // Disable GET SSE
	)

	// Register a greeting tool.
	greetTool := mcp.NewTool("greet", handleGreet,
		mcp.WithDescription("A simple greeting tool"),
		mcp.WithString("name", mcp.Description("Name to greet")))

	if err := mcpServer.RegisterTool(greetTool); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Printf("Registered greeting tool: greet")

	// Register counter-tool.
	counterTool := mcp.NewTool("counter", handleCounter,
		mcp.WithDescription("A session counter tool to demonstrate stateful sessions"),
		mcp.WithNumber("increment",
			mcp.Description("Counter increment"),
			mcp.Default(1)))

	if err := mcpServer.RegisterTool(counterTool); err != nil {
		log.Fatalf("Failed to register counter tool: %v", err)
	}
	log.Printf("Registered counter tool: counter")

	// Set a simple health check route.
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Server is running normally."))
	})

	// Register session management route, allow viewing active sessions.
	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// We cannot directly get all active sessions here because sessionManager does not provide such a method.
			// But we can provide a session monitor page.
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "Session manager status: active\n")
			fmt.Fprintf(w, "Session expiration time: %d seconds\n", 3600)
			fmt.Fprintf(w, "Note: Session manager does not provide a way to list all active sessions.\n")
			fmt.Fprintf(w, "In a real server, it is recommended to implement session monitoring functionality.\n")
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "Method not supported: %s", r.Method)
		}
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
	log.Printf("MCP server started on :3003, path /mcp")
	log.Printf("This is a stateful, pure JSON response server - session ID will be assigned, SSE not used")
	log.Printf("You can check session manager status at http://localhost:3003/sessions")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
