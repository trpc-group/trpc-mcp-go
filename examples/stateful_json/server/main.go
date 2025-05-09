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
	"github.com/modelcontextprotocol/streamable-mcp/transport"
)

// Simple greeting tool handler function.
func handleGreet(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
	// Get session from context (if any).
	session, ok := transport.GetSessionFromContext(ctx)

	// Extract name from parameters.
	name, _ := req.Params.Arguments["name"].(string)
	if name == "" {
		name = "World"
	}

	// Create response content, customize message if session exists.
	var content []schema.ToolContent

	if ok && session != nil {
		content = append(content, schema.NewTextContent(fmt.Sprintf(
			"Hello, %s! This is a greeting from the stateful JSON server. Your session ID is: %s",
			name, session.ID[:8]+"...")))
	} else {
		content = append(content, schema.NewTextContent(fmt.Sprintf(
			"Hello, %s! This is a greeting from the stateful JSON server, but session info could not be obtained.",
			name)))
	}

	return &schema.CallToolResult{Content: content}, nil
}

// Counter tool, used to demonstrate session state management.
func handleCounter(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
	// Get session.
	session, ok := transport.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return schema.NewErrorResult("Error: Could not get session info. This tool requires a stateful session."),
			fmt.Errorf("Failed to get session from context")
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
	return schema.NewTextResult(fmt.Sprintf("Counter current value: %d (Session ID: %s)",
		count, session.ID[:8]+"...")), nil
}

func main() {
	// Set log level.
	log.SetLevel(log.InfoLevel)
	log.Info("Starting Stateful JSON No GET SSE mode MCP server...")

	// Create server info.
	serverInfo := schema.Implementation{
		Name:    "Stateful-JSON-Server",
		Version: "1.0.0",
	}

	// Create session manager (valid for 1 hour).
	sessionManager := transport.NewSessionManager(3600)

	// Create MCP server, configured as:
	// 1. Stateful mode (using SessionManager)
	// 2. Only return JSON responses (do not use SSE)
	// 3. GET SSE is not supported
	mcpServer := server.NewServer(
		":3003", // Server address and port
		serverInfo,
		server.WithPathPrefix("/mcp"), // Set API path
		server.WithSessionManager(sessionManager), // Use session manager (stateful)
		server.WithSSEEnabled(false),              // Disable SSE
		server.WithGetSSEEnabled(false),           // Disable GET SSE
		server.WithDefaultResponseMode("json"),    // Set default response mode to JSON
	)

	// Register a greeting tool.
	greetTool := schema.NewTool("greet", handleGreet,
		schema.WithDescription("A simple greeting tool"),
		schema.WithString("name", schema.Description("Name to greet")))

	if err := mcpServer.RegisterTool(greetTool); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Info("Registered greeting tool: greet")

	// Register counter tool.
	counterTool := schema.NewTool("counter", handleCounter,
		schema.WithDescription("A session counter tool to demonstrate stateful sessions"),
		schema.WithNumber("increment",
			schema.Description("Counter increment"),
			schema.Default(1)))

	if err := mcpServer.RegisterTool(counterTool); err != nil {
		log.Fatalf("Failed to register counter tool: %v", err)
	}
	log.Info("Registered counter tool: counter")

	// Set a simple health check route.
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Server is running normally."))
	})

	// Register session management route, allow viewing active sessions.
	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// We cannot directly get all active sessions here because SessionManager does not provide such a method.
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
		log.Infof("Received signal %v, exiting...", sig)
		os.Exit(0)
	}()
	// Start server.
	log.Infof("MCP server started on :3003, path /mcp")
	log.Infof("This is a stateful, pure JSON response server - session ID will be assigned, SSE not used")
	log.Infof("You can check session manager status at http://localhost:3003/sessions")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
