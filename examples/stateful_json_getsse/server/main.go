package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// Simple greeting tool handler function.
func handleGreet(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session from context (if any).
	session, ok := mcp.GetSessionFromContext(ctx)

	// Extract name from parameters.
	name := "World"
	if nameArg, ok := request.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Create response content, customize message if session exists.
	content := []mcp.Content{}

	if ok && session != nil {
		content = append(content, mcp.NewTextContent(fmt.Sprintf(
			"Hello, %s! This is a greeting from the stateful JSON+GET SSE server. Your session ID is: %s",
			name, session.GetID()[:8]+"...")))
	} else {
		content = append(content, mcp.NewTextContent(fmt.Sprintf(
			"Hello, %s! This is a greeting from the stateful JSON+GET SSE server, but session info could not be obtained.",
			name)))
	}

	return &mcp.CallToolResult{Content: content}, nil
}

// Counter tool, used to demonstrate session state management.
func handleCounter(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session.
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Could not get session info. This tool requires a stateful session."),
			},
		}, fmt.Errorf("Failed to get session from context")
	}

	// Get counter from session data.
	var count int
	if data, exists := session.GetData("counter"); exists {
		count, _ = data.(int)
	}

	// Increase counter.
	increment := 1
	if inc, ok := request.Params.Arguments["increment"].(float64); ok {
		increment = int(inc)
	}

	count += increment

	// Save back to session.
	session.SetData("counter", count)

	// Return result.
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Counter current value: %d (Session ID: %s)",
				count, session.GetID()[:8]+"...")),
		},
	}, nil
}

// Notification demo tool, used to send asynchronous notifications.
func handleNotification(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session.
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Could not get session info. This tool requires a stateful session."),
			},
		}, fmt.Errorf("Failed to get session from context")
	}

	// Get message and delay from parameters.
	message := "This is a test notification message"
	if msgArg, ok := request.Params.Arguments["message"]; ok {
		if msgStr, ok := msgArg.(string); ok && msgStr != "" {
			message = msgStr
		}
	}

	delaySeconds := 2
	if delayArg, ok := request.Params.Arguments["delay"]; ok {
		if delay, ok := delayArg.(float64); ok {
			delaySeconds = int(delay)
		}
	}

	// Immediately return confirmation message.
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Notification will be sent after %d seconds. Please make sure to subscribe to notifications with GET SSE connection. (Session ID: %s)",
				delaySeconds, session.GetID()[:8]+"...")),
		},
	}

	serverInstance := mcp.GetServerFromContext(ctx)

	// Start a goroutine in the background to send delayed notification.
	go func() {
		time.Sleep(time.Duration(delaySeconds) * time.Second)

		serverInstance := serverInstance
		session := session

		if serverInstance == nil {
			log.Printf("Failed to send notification: could not get server instance from context.")
			return
		}

		// Type assertion.
		mcpServer, ok := serverInstance.(*mcp.Server)
		if !ok {
			log.Printf("Failed to send notification: server instance type error in context.")
			return
		}

		// Use server's notification API to send notification directly.
		err := mcpServer.SendNotification(session.GetID(), "notifications/message", map[string]interface{}{
			"level": "info",
			"data": map[string]interface{}{
				"type":      "test_notification",
				"message":   message,
				"timestamp": time.Now().Format(time.RFC3339),
				"sessionId": session.GetID(),
			},
		})

		if err != nil {
			log.Printf("Failed to send notification: %v.", err)
		} else {
			log.Printf("Notification sent to session %s.", session.GetID())
		}
	}()

	return result, nil
}

func main() {
	log.Printf("Starting Stateful JSON Yes GET SSE mode MCP server...")

	// Create session manager (valid for 1 hour).
	sessionManager := mcp.NewSessionManager(3600)

	// Create MCP server, configured as:
	// 1. Stateful mode (using sessionManager)
	// 2. Only return JSON responses (do not use SSE)
	// 3. Support GET SSE
	mcpServer := mcp.NewServer(
		"Stateful-JSON-Yes-GETSSE-Server",      // Server name
		"1.0.0",                                // Server version
		mcp.WithServerAddress(":3004"),         // Server address and port
		mcp.WithServerPath("/mcp"),             // Set API path
		mcp.WithSessionManager(sessionManager), // Use session manager (stateful)
		mcp.WithPostSSEEnabled(false),          // Disable SSE
		mcp.WithGetSSEEnabled(true),            // Enable GET SSE
	)

	// Register a greeting tool.
	greetTool := mcp.NewTool("greet", handleGreet,
		mcp.WithDescription("A simple greeting tool"),
		mcp.WithString("name", mcp.Description("Name to greet")))

	if err := mcpServer.RegisterTool(greetTool); err != nil {
		log.Fatalf("Failed to register tool: %v.", err)
	}
	log.Printf("Registered greeting tool: greet.")

	// Register counter tool.
	counterTool := mcp.NewTool("counter", handleCounter,
		mcp.WithDescription("A session counter tool to demonstrate stateful sessions"),
		mcp.WithNumber("increment",
			mcp.Description("Counter increment"),
			mcp.Default(1)))

	if err := mcpServer.RegisterTool(counterTool); err != nil {
		log.Fatalf("Failed to register counter tool: %v.", err)
	}
	log.Printf("Registered counter tool: counter.")

	// Register notification demo tool
	notifyTool := mcp.NewTool("sendNotification", handleNotification,
		mcp.WithDescription("A notification demo tool that sends asynchronous notification messages"),
		mcp.WithString("message",
			mcp.Description("Notification message to send"),
			mcp.Default("This is a test notification message")),
		mcp.WithNumber("delay",
			mcp.Description("Delay in seconds before sending notification"),
			mcp.Default(2)))

	if err := mcpServer.RegisterTool(notifyTool); err != nil {
		log.Fatalf("Failed to register notification tool: %v.", err)
	}
	log.Printf("Registered notification tool: sendNotification.")

	// Example: Periodically broadcast system status notifications
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Broadcast system status notification to all sessions
				failedCount, err := mcpServer.BroadcastNotification(
					"notifications/message",
					map[string]interface{}{
						"level": "info",
						"data": map[string]interface{}{
							"type":      "system_status",
							"memory":    fmt.Sprintf("%.1f%%", float64(50+time.Now().Second()%30)),
							"cpu":       fmt.Sprintf("%.1f%%", float64(30+time.Now().Second()%40)),
							"timestamp": time.Now().Format(time.RFC3339),
							"message":   "System is running normally",
						},
					},
				)

				if err != nil {
					log.Printf("Failed to broadcast system status notification: %v (failed sessions: %d)", err, failedCount)
				} else {
					log.Printf("System status notification broadcast to all sessions")
				}
			}
		}
	}()

	// Set up a simple health check route
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Server is running normally"))
	})

	// Register session management route to allow viewing active sessions
	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// Use the new public API to get the list of active sessions
			sessions, err := mcpServer.GetActiveSessions()
			if err != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				fmt.Fprintf(w, "Error getting active sessions: %v\n", err)
				return
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "Session manager status: Active\n")
			fmt.Fprintf(w, "Session expiration time: %d seconds\n", 3600)
			fmt.Fprintf(w, "GET SSE support: Enabled\n")
			fmt.Fprintf(w, "Number of active sessions: %d\n\n", len(sessions))

			// Display all active sessions
			for i, sessionID := range sessions {
				fmt.Fprintf(w, "%d) %s\n", i+1, sessionID)
			}
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "Unsupported method: %s", r.Method)
		}
	})

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, exiting...", sig)
		os.Exit(0)
	}()

	// Start the server
	log.Printf("MCP server started on :3004, access path /mcp")
	log.Printf("This is a stateful, JSON-only response server - it assigns session IDs, does not use SSE responses, but supports GET SSE")
	log.Printf("You can view the session manager status at http://localhost:3004/sessions")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
}
