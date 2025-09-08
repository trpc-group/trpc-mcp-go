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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Print server start message.
	log.Printf("Starting Stateful SSE No GET SSE mode MCP server...")

	// Create MCP server, configured as:
	// 1. Stateful mode (using sessionManager)
	// 2. Use SSE response (streaming)
	// 3. Do not support independent GET SSE
	mcpServer := mcp.NewServer(
		"Stateful-SSE-No-GETSSE-Server", // Server name
		"1.0.0",                         // Server version
		mcp.WithServerAddress(":3005"),  // Server address and port
		mcp.WithServerPath("/mcp"),      // Set API path
		mcp.WithPostSSEEnabled(true),    // Enable SSE
		mcp.WithGetSSEEnabled(false),    // Disable GET SSE
	)

	// Register a greeting tool.
	greetTool := mcp.NewTool("greet",
		mcp.WithDescription("A simple greeting tool"),
		mcp.WithString("name", mcp.Description("Name to greet")))

	mcpServer.RegisterTool(greetTool, handleGreet)
	log.Printf("Registered greeting tool: greet")

	// Register counter tool
	counterTool := mcp.NewTool("counter",
		mcp.WithDescription("A session counter tool to demonstrate stateful sessions"),
		mcp.WithNumber("increment",
			mcp.Description("Counter increment"),
			mcp.Default(1)))

	mcpServer.RegisterTool(counterTool, handleCounter)
	log.Printf("Registered counter tool: counter")

	// Register delayed response tool
	delayedTool := mcp.NewTool("delayedResponse",
		mcp.WithDescription("A delayed response tool to demonstrate the advantages of SSE streaming responses"),
		mcp.WithNumber("steps",
			mcp.Description("Number of processing steps"),
			mcp.Default(5)),
		mcp.WithNumber("delayMs",
			mcp.Description("Delay in milliseconds per step"),
			mcp.Default(500)))

	mcpServer.RegisterTool(delayedTool, handleDelayedResponse)
	log.Printf("Registered delayed response tool: delayedResponse")

	// Set up a simple health check route
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Server is running normally"))
	})

	// Register session management route to allow viewing active sessions
	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// We cannot directly get all active sessions here because sessionManager does not provide such a method
			// But we can provide a session monitoring page
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "Session manager status: Active\n")
			fmt.Fprintf(w, "Session expiration time: %d seconds\n", 3600)
			fmt.Fprintf(w, "SSE mode: Enabled\n")
			fmt.Fprintf(w, "GET SSE support: Disabled\n")
			fmt.Fprintf(w,
				"Note: The session manager does not provide the function to list all active sessions.\n",
			)
			fmt.Fprintf(w,
				"In a real server, it is recommended to implement session monitoring functionality.\n",
			)
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
	log.Printf("MCP server started on :3005, access path /mcp")
	log.Printf(
		"This is a stateful, SSE streaming response server - " +
			"it assigns session IDs, uses SSE, and does not support GET SSE",
	)
	log.Printf("You can view the session manager status at http://localhost:3005/sessions")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
}

// Callback function for handling the greet tool.
func handleGreet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session.
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Warning: Session info not found, but you may continue."),
			},
		}, nil
	}

	// Extract name parameter.
	name := "Client user"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Return greeting message.
	log.Printf(
		"Hello, %s! (Session ID: %s)",
		name, session.GetID()[:8]+"...",
	)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Hello, %s! (Session ID: %s)",
				name, session.GetID()[:8]+"...",
			)),
		},
	}, nil
}

// Counter tool, used to demonstrate session state keeping.
func handleCounter(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session.
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Could not get session info. This tool requires a stateful session."),
			},
		}, fmt.Errorf("failed to get session from context")
	}

	// Get current count from session.
	count := 0
	if countVal, exists := session.GetData("counter"); exists && countVal != nil {
		count, _ = countVal.(int)
	}

	// Extract increment parameter.
	increment := 1
	if incArg, ok := req.Params.Arguments["increment"]; ok {
		if incFloat, ok := incArg.(float64); ok {
			increment = int(incFloat)
		}
	}

	count += increment

	// Save back to session.
	session.SetData("counter", count)

	// Return result.
	log.Printf(
		"Counter current value: %d (Session ID: %s)",
		count, session.GetID()[:8]+"...",
	)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Counter current value: %d (Session ID: %s)",
				count, session.GetID()[:8]+"...",
			)),
		},
	}, nil
}

// Delayed response tool, demonstrates the advantage of SSE streaming response.
func handleDelayedResponse(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session.
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Could not get session info. This tool requires a stateful session."),
			},
		}, fmt.Errorf("failed to get session from context")
	}

	// Get notification sender.
	notifSender, ok := mcp.GetNotificationSender(ctx)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(
					"Error: Could not get notification sender. " +
						"This feature requires SSE streaming response support.",
				),
			},
		}, fmt.Errorf("failed to get notification sender from context")
	}

	// Get steps and delay per step from parameters.
	steps := 5
	if s, ok := req.Params.Arguments["steps"].(float64); ok && s > 0 {
		steps = int(s)
	}

	delayMs := 500
	if d, ok := req.Params.Arguments["delayMs"].(float64); ok && d > 0 {
		delayMs = int(d)
	}

	// Send initial response.
	paramsMap := map[string]interface{}{
		"level": "info",
		"data": map[string]interface{}{
			"type":    "process_started",
			"message": "Start processing request...",
			"steps":   steps,
			"delayMs": delayMs,
		},
	}
	//initialNotification := mcp.NewJSONRPCNotificationFromMap("notifications/message", paramsMap)
	initialNotification := mcp.NewNotification("notifications/message", paramsMap)

	if err := notifSender.SendCustomNotification(initialNotification.Method, paramsMap); err != nil {
		log.Printf("Failed to send initial notification: %v", err)
	}

	// Send progress notifications.
	for i := 1; i <= steps; i++ {
		// Delay for a while.
		time.Sleep(time.Duration(delayMs) * time.Millisecond)

		// Send progress notification.
		progressParamsMap := map[string]interface{}{
			"level": "info",
			"data": map[string]interface{}{
				"type":     "process_progress",
				"step":     i,
				"total":    steps,
				"progress": float64(i) / float64(steps) * 100,
				"message":  fmt.Sprintf("Processing step %d/%d...", i, steps),
			},
		}

		progressNotification := mcp.NewNotification("notifications/message", progressParamsMap)
		if err := notifSender.SendNotification(progressNotification); err != nil {
			log.Printf("Failed to send progress notification: %v", err)
		}
	}

	// Final return result.
	log.Printf(
		"Processing complete! %d steps executed, %d ms delay per step. (Session ID: %s)",
		steps, delayMs, session.GetID()[:8]+"...",
	)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Processing complete! %d steps executed, %d ms delay per step. (Session ID: %s)",
				steps, delayMs, session.GetID()[:8]+"...",
			)),
		},
	}, nil
}
