package main

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/client"
	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

// Handler function for server notifications.
func handleNotification(notification *schema.Notification) error {
	paramsMap := notification.Params // Use directly, since Params is already map[string]interface{}.

	level, _ := paramsMap["level"].(string)
	dataMap, ok := paramsMap["data"].(map[string]interface{})
	if !ok {
		log.Infof("Received notification [%s] (Level: %s), but 'data' field is invalid or missing: %+v", notification.Method, level, paramsMap)
		return fmt.Errorf("'data' field is invalid or missing")
	}

	notificationType, _ := dataMap["type"].(string)

	log.Infof("Received notification [%s] (Level: %s, Type: %s): %+v", notification.Method, level, notificationType, dataMap)

	// Handle specific notification types.
	switch notificationType {
	case "test_notification":
		// Handle test notification.
		if message, exists := dataMap["message"].(string); exists {
			log.Infof("  Test notification content: %s", message)
		}
	case "system_status":
		// Handle system status notification.
		if cpu, exists := dataMap["cpu"].(string); exists {
			log.Infof("  System status - CPU: %s", cpu)
		}
		if memory, exists := dataMap["memory"].(string); exists {
			log.Infof("  System status - Memory: %s", memory)
		}
	default:
		log.Infof("  Unknown notification data type: %s", notificationType)
	}
	return nil
}

func main() {
	// Set log level.
	log.Info("Starting Stateful JSON Yes GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := schema.Implementation{
		Name:    "Stateful-JSON-GETSSE-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	mcpClient, err := client.NewClient(
		"http://localhost:3004/mcp",
		clientInfo,
		client.WithGetSSEEnabled(true), // Enable GET SSE
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// Register notification handler.
	mcpClient.RegisterNotificationHandler("notifications/message", handleNotification)

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
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
	}

	log.Infof("Server provides %d tools", len(tools))
	for _, tool := range tools {
		log.Infof("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call greet tool.
	log.Info("Calling greet tool...")
	callResult, err := mcpClient.CallTool(ctx, "greet", map[string]interface{}{
		"name": "JSON+GETSSE client user",
	})
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
	}

	// Show call result.
	log.Info("Call result:")
	for _, content := range callResult {
		if textContent, ok := content.(schema.TextContent); ok {
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
	for _, content := range counterResult1 {
		if textContent, ok := content.(schema.TextContent); ok {
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
	for _, content := range counterResult2 {
		if textContent, ok := content.(schema.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Call notification tool, demonstrate server push notification feature.
	log.Info("Calling sendNotification tool, you will receive a notification after a delay...")
	notifyResult, err := mcpClient.CallTool(ctx, "sendNotification", map[string]interface{}{
		"message": "This is a test notification message sent from JSON+GETSSE client",
		"delay":   2,
	})
	if err != nil {
		log.Fatalf("Failed to call notification tool: %v", err)
	}

	// Show call result.
	log.Info("Notification tool call result:")
	for _, content := range notifyResult {
		if textContent, ok := content.(schema.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Print session info.
	log.Infof("\nSession info: ID=%s", mcpClient.GetSessionID())
	log.Info("Client has enabled GET SSE connection, waiting for notifications...")
	log.Info("Tip: you can use curl http://localhost:3004/sessions to check server session state")

	// Wait a while to receive possible notifications.
	log.Info("Waiting to receive notifications (15 seconds)...")
	time.Sleep(120 * time.Second)

	log.Info("Client example finished, exiting soon...")
}
