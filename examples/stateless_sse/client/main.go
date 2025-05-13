package main

import (
	"context"
	"log"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go"
)

// handleNotifications is a simple notification handler example.
func handleNotifications(notification *mcp.JSONRPCNotification) error {
	log.Printf("Client received notification: Method=%s", notification.Method)

	// Handle different types of notifications.
	switch notification.Method {
	case "notifications/message":
		level, _ := notification.Params.AdditionalFields["level"].(string)
		data, _ := notification.Params.AdditionalFields["data"].(string)
		log.Printf("Received log message: [%s] %s", level, data)
	case "notifications/progress":
		progress, _ := notification.Params.AdditionalFields["progress"].(float64)
		message, _ := notification.Params.AdditionalFields["message"].(string)
		log.Printf("Received progress update: %.0f%% - %s", progress*100, message)
	default:
		log.Printf("Received other type of notification: %+v", notification.Params.AdditionalFields)
	}

	return nil
}

func main() {
	// Print startup message.
	log.Printf("Starting Stateless SSE No GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := mcp.Implementation{
		Name:    "Stateless-SSE-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	mcpClient, err := mcp.NewClient("http://localhost:3002/mcp", clientInfo)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		return
	}
	defer mcpClient.Close()

	// Initialize client.
	log.Printf("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx)
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
		return
	}

	log.Printf("Initialization successful: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Printf("Server capabilities: %+v", initResp.Capabilities)

	// Register notification handlers.
	log.Printf("Registering notification handlers...")
	mcpClient.RegisterNotificationHandler("notifications/message", handleNotifications)
	mcpClient.RegisterNotificationHandler("notifications/progress", handleNotifications)

	// Get available tools list.
	log.Printf("Listing tools...")
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
		return
	}

	log.Printf("Server provides %d tools", len(tools.Tools))
	for _, tool := range tools.Tools {
		log.Printf("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call simple greet tool first.
	log.Printf("\nCalling greet tool...")
	callResult, err := mcpClient.CallTool(ctx, "greet", map[string]interface{}{
		"name": "Client user",
	})
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
		return
	}

	// Show call result.
	log.Printf("Call result:")
	for _, content := range callResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s", textContent.Text)
		} else {
			log.Printf("- Other type content: %+v", content)
		}
	}

	// Call multi-stage greeting tool (this will send notifications via SSE).
	log.Printf("\nCalling multi-stage-greeting tool...")
	multiStageResult, err := mcpClient.CallTool(ctx, "multi-stage-greeting", map[string]interface{}{
		"name":   "SSE client user",
		"stages": 5,
	})
	if err != nil {
		log.Fatalf("Failed to call multi-stage greeting tool: %v", err)
		return
	}

	// Show multi-stage call result.
	log.Printf("Multi-stage greeting result:")
	for _, content := range multiStageResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s", textContent.Text)
		} else {
			log.Printf("- Other type content: %+v", content)
		}
	}

	// Unregister notification handlers.
	mcpClient.UnregisterNotificationHandler("notifications/message")
	mcpClient.UnregisterNotificationHandler("notifications/progress")

	log.Printf("\nClient example finished, exiting in 3 seconds...")
	time.Sleep(3 * time.Second)
}
