package main

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/client"
	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

// handleNotifications is a simple notification handler example.
func handleNotifications(notification *schema.Notification) error {
	log.Infof("Client received notification: Method=%s", notification.Method)

	// Handle different types of notifications.
	switch notification.Method {
	case "notifications/message":
		level, _ := notification.Params["level"].(string)
		data, _ := notification.Params["data"].(string)
		log.Infof("Received log message: [%s] %s", level, data)
	case "notifications/progress":
		progress, _ := notification.Params["progress"].(float64)
		message, _ := notification.Params["message"].(string)
		log.Infof("Received progress update: %.0f%% - %s", progress*100, message)
	default:
		log.Infof("Received other type of notification: %+v", notification.Params)
	}

	return nil
}

func main() {
	// Set log level.
	log.SetLevel(log.DebugLevel)
	log.Info("Starting Stateless SSE No GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := schema.Implementation{
		Name:    "Stateless-SSE-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	mcpClient, err := client.NewClient("http://localhost:3002/mcp", clientInfo)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		return
	}
	defer mcpClient.Close()

	// Initialize client.
	log.Info("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx)
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
		return
	}

	log.Infof("Initialization successful: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Infof("Server capabilities: %+v", initResp.Capabilities)

	// Register notification handlers.
	log.Info("Registering notification handlers...")
	mcpClient.RegisterNotificationHandler("notifications/message", handleNotifications)
	mcpClient.RegisterNotificationHandler("notifications/progress", handleNotifications)

	// Get available tools list.
	log.Info("Listing tools...")
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
		return
	}

	log.Infof("Server provides %d tools", len(tools))
	for _, tool := range tools {
		log.Infof("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call simple greet tool first.
	log.Info("\nCalling greet tool...")
	callResult, err := mcpClient.CallTool(ctx, "greet", map[string]interface{}{
		"name": "Client user",
	})
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
		return
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

	// Call multi-stage greeting tool (this will send notifications via SSE).
	log.Info("\nCalling multi-stage-greeting tool...")
	multiStageResult, err := mcpClient.CallTool(ctx, "multi-stage-greeting", map[string]interface{}{
		"name":   "SSE client user",
		"stages": 5,
	})
	if err != nil {
		log.Fatalf("Failed to call multi-stage greeting tool: %v", err)
		return
	}

	// Show multi-stage call result.
	log.Info("Multi-stage greeting result:")
	for _, content := range multiStageResult {
		if textContent, ok := content.(schema.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		} else {
			log.Infof("- Other type content: %+v", content)
		}
	}

	// Unregister notification handlers.
	mcpClient.UnregisterNotificationHandler("notifications/message")
	mcpClient.UnregisterNotificationHandler("notifications/progress")

	log.Info("\nClient example finished, exiting in 3 seconds...")
	time.Sleep(3 * time.Second)
}
