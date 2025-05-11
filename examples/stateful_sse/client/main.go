package main

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/client"
	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/transport"
)

// Callback function for handling SSE incremental updates.
func handleNotification(notification *mcp.JSONRPCNotification) error {
	paramsMap := notification.Params.AdditionalFields
	level, _ := paramsMap["level"].(string)
	dataMap, ok := paramsMap["data"].(map[string]interface{})
	if !ok {
		log.Infof("Received notification [%s] (Level: %s), but 'data' field is invalid or missing: %+v", notification.Method, level, paramsMap)
		return fmt.Errorf("'data' field is invalid or missing")
	}

	notificationType, _ := dataMap["type"].(string)
	log.Infof("Received notification [%s] (Level: %s, Type: %s): %+v", notification.Method, level, notificationType, dataMap)

	switch notificationType {
	case "process_started":
		if message, exists := dataMap["message"].(string); exists {
			log.Infof("  Stream processing started: %s (Steps: %v, Delay: %vms)", message, dataMap["steps"], dataMap["delayMs"])
		}
	case "process_progress":
		if message, exists := dataMap["message"].(string); exists {
			log.Infof("  Stream processing progress: %s (Step: %v/%v, Progress: %.2f%%)", message, dataMap["step"], dataMap["total"], dataMap["progress"])
		}
	default:
		log.Infof("  Received other type of notification '%s': %+v", notificationType, dataMap)
	}
	return nil
}

func main() {
	// Set log level.
	log.SetLevel(log.InfoLevel)
	log.Info("Starting Stateful SSE No GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := mcp.Implementation{
		Name:    "Stateful-SSE-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	mcpClient, err := client.NewClient(
		"http://localhost:3005/mcp",
		clientInfo,
		client.WithGetSSEEnabled(false), // Disable GET SSE
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// Initialize client.
	log.Info("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx)
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	log.Infof("Initialization succeeded: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Infof("Server capabilities: %+v", initResp.Capabilities)

	// Get session info.
	sessionID := mcpClient.GetSessionID()
	if sessionID == "" {
		log.Info("Warning: No session ID received. Server may not be properly configured for stateful mode.")
	} else {
		log.Infof("Session established, ID: %s", sessionID)
	}

	// Get available tools list.
	log.Info("Listing tools...")
	toolsResult, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
	}

	log.Infof("Server provides %d tools", len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		log.Infof("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call greet tool.
	log.Info("Calling greet tool...")
	callResult, err := mcpClient.CallTool(ctx, "greet", map[string]interface{}{
		"name": "SSE client user",
	})
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
	}

	// Show call result.
	log.Info("Call result:")
	for _, content := range callResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		} else {
			log.Infof("- Other type content: %+v", content)
		}
	}

	// Call counter tool, demonstrate session state keeping.
	log.Info("Calling counter tool first time...")
	counterResult1, err := mcpClient.CallTool(ctx, "counter", map[string]interface{}{
		"increment": 1,
	})
	if err != nil {
		log.Fatalf("Failed to call counter tool: %v", err)
	}

	// Show counter result.
	log.Info("Counter result (first time):")
	for _, content := range counterResult1.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Call counter tool again, verify state keeping.
	log.Info("Calling counter tool second time...")
	counterResult2, err := mcpClient.CallTool(ctx, "counter", map[string]interface{}{
		"increment": 2,
	})
	if err != nil {
		log.Fatalf("Failed to call counter tool: %v", err)
	}

	// Show counter result.
	log.Info("Counter result (second time):")
	for _, content := range counterResult2.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Call delayedResponse tool, demonstrate the advantage of SSE streaming response.
	log.Info("Calling delayedResponse tool, experience streaming response...")

	// Create stream options, set callback for handling incremental content.
	streamOpts := &transport.StreamOptions{
		NotificationHandlers: map[string]transport.NotificationHandler{
			"notifications/message": handleNotification,
		},
	}

	// Use streaming API to call tool.
	_, err = mcpClient.CallToolWithStream(ctx, "delayedResponse", map[string]interface{}{
		"steps":   5,
		"delayMs": 500,
	}, streamOpts)

	if err != nil {
		log.Fatalf("Failed to call delayedResponse tool: %v", err)
	}

	log.Info("delayedResponse tool streaming call finished")

	// Print session info.
	log.Infof("\nSession info: ID=%s", mcpClient.GetSessionID())
	log.Info("Client has not enabled GET SSE connection, cannot receive independent notifications")
	log.Info("Tip: you can use curl http://localhost:3005/sessions to check server session state")

	// Wait a while for user to see output.
	log.Info("Client example finished, exiting in 5 seconds...")
	time.Sleep(5 * time.Second)
}
