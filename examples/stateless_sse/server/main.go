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

// handleMultiStageGreeting handles the multi-stage greeting tool and sends multiple notifications via SSE.
func handleMultiStageGreeting(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract name from parameters.
	name := "Guest"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	stages := 3
	if stagesArg, ok := req.Params.Arguments["stages"]; ok {
		if stagesFloat, ok := stagesArg.(float64); ok && stagesFloat > 0 {
			stages = int(stagesFloat)
		}
	}

	// Get notification sender from context.
	notificationSender, hasNotificationSender := mcp.GetNotificationSender(ctx)
	if !hasNotificationSender {
		log.Printf("unable to get notification sender from context")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: unable to get notification sender."),
			},
		}, fmt.Errorf("unable to get notification sender from context")
	}

	// Send progress update.
	sendProgress := func(progress float64, message string) {
		err := notificationSender.SendProgress(progress, message)
		if err != nil {
			log.Printf("Failed to send progress notification: %v", err)
		}
	}

	// Send log message.
	sendLogMessage := func(level string, message string) {
		err := notificationSender.SendLogMessage(level, message)
		if err != nil {
			log.Printf("Failed to send log notification: %v", err)
		}
	}

	// Start greeting process.
	sendProgress(0.0, "Start multi-stage greeting")
	sendLogMessage("info", fmt.Sprintf("Start greeting to %s", name))
	time.Sleep(500 * time.Millisecond)

	// Send multiple stage notifications.
	for i := 1; i <= stages; i++ {
		// Check if context is canceled.
		select {
		case <-ctx.Done():
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Greeting canceled at stage %d", i)),
				},
			}, ctx.Err()
		default:
			// Continue sending.
		}

		sendProgress(float64(i)/float64(stages), fmt.Sprintf("Stage %d greeting", i))
		sendLogMessage("info", fmt.Sprintf("Stage %d greeting: Hello %s!", i, name))
		time.Sleep(800 * time.Millisecond)
	}

	// Send final greeting.
	sendProgress(1.0, "Greeting completed")
	sendLogMessage("info", fmt.Sprintf("Completed multi-stage greeting to %s", name))

	// Return final result.
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Completed %d-stage greeting to %s!", stages, name)),
		},
	}, nil
}

func main() {
	log.Printf("Starting Stateless SSE No GET SSE mode MCP server...")

	// Create MCP server with the following configuration:
	// 1. Stateless mode
	// 2. SSE enabled
	// 3. GET SSE not supported
	mcpServer := mcp.NewServer(
		"Stateless-SSE-No-GETSSE-Server", // Server name
		"1.0.0",                          // Server version
		mcp.WithServerAddress(":3002"),   // Server address and port
		mcp.WithPathPrefix("/mcp"),       // Set API path
		mcp.WithStatelessMode(true),      // Enable stateless mode
		mcp.WithPostSSEEnabled(true),     // Enable SSE
		mcp.WithGetSSEEnabled(false),     // Disable GET SSE
	)

	// Register a simple greeting tool.
	greetTool := mcp.NewTool("greet",
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name := "World"
			if nameArg, ok := req.Params.Arguments["name"]; ok {
				if nameStr, ok := nameArg.(string); ok && nameStr != "" {
					name = nameStr
				}
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Hello, %s! This is a greeting from the stateless SSE server.", name)),
				},
			}, nil
		},
		mcp.WithDescription("A simple greeting tool."),
		mcp.WithString("name", mcp.Description("Name to greet.")),
	)

	if err := mcpServer.RegisterTool(greetTool); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Printf("Registered greet tool: greet")

	// Register a multi-stage greeting tool (sends multiple notifications via SSE).
	multiStageGreetingTool := mcp.NewTool("multi-stage-greeting",
		handleMultiStageGreeting,
		mcp.WithDescription("Send multi-stage greeting via SSE."),
		mcp.WithString("name", mcp.Description("Name to greet.")),
		mcp.WithNumber("stages",
			mcp.Description("Number of greeting stages."),
			mcp.Default(3),
		),
	)

	if err := mcpServer.RegisterTool(multiStageGreetingTool); err != nil {
		log.Fatalf("Failed to register multi-stage greeting tool: %v", err)
	}
	log.Printf("Registered multi-stage greeting tool: multi-stage-greeting")

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
	log.Printf("MCP server started at :3002, path /mcp")
	log.Printf("This is a stateless, SSE response server - no session ID will be returned, SSE is used, GET SSE is not supported.")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
