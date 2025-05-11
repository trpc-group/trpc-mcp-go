package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/client"
	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/transport"
)

// handleNotification processes server notifications.
func handleNotification(notification *mcp.JSONRPCNotification) error {
	// 使用 AdditionalFields 访问参数
	paramsMap := notification.Params.AdditionalFields
	level, _ := paramsMap["level"].(string)

	// Check if data is map[string]interface{}
	if dataMap, ok := paramsMap["data"].(map[string]interface{}); ok {
		notificationType, _ := dataMap["type"].(string)

		log.Infof("Received notification [%s] (Level: %s, Type: %s): %+v", notification.Method, level, notificationType, dataMap)

		// Process based on notificationType
		switch notificationType {
		case "test_notification":
			// Handle test notification
			if message, exists := dataMap["message"].(string); exists {
				log.Infof("  Test notification content: %s", message)
			}
		case "process_started":
			// Handle process start notification
			if message, exists := dataMap["message"].(string); exists {
				log.Infof("  Process started: %s (Steps: %v, Delay: %vms)", message, dataMap["steps"], dataMap["delayMs"])
			}
		case "process_progress":
			// Handle process progress notification
			if message, exists := dataMap["message"].(string); exists {
				log.Infof("  Process progress: %s (Step: %v/%v, Progress: %.2f%%)", message, dataMap["step"], dataMap["total"], dataMap["progress"])
			}
		case "chat_message":
			// Handle chat message notification
			if userName, uOk := dataMap["userName"].(string); uOk {
				if message, mOk := dataMap["message"].(string); mOk {
					log.Infof("  Chat message [%s] %s: %s", dataMap["timestamp"], userName, message)
				}
			}
		case "chat_system_message":
			// Handle chat system message
			if message, exists := dataMap["message"].(string); exists {
				log.Infof("  Chat system message [%s]: %s", dataMap["timestamp"], message)
			}
		case "log_message":
			// Handle log message
			if message, exists := dataMap["message"].(string); exists {
				log.Infof("  Log message: %s", message)
			}
		default:
			log.Infof("  Unknown notification data type: %s", notificationType)
		}
	} else if dataStr, ok := paramsMap["data"].(string); ok {
		// Handle string type data field
		log.Infof("Received notification [%s] (Level: %s): %s", notification.Method, level, dataStr)
	} else {
		// Other data field types
		log.Infof("Received notification [%s] (Level: %s): %+v", notification.Method, level, paramsMap["data"])
	}

	return nil
}

// handleProgressNotification processes progress notifications.
func handleProgressNotification(notification *mcp.JSONRPCNotification) error {
	// 使用 AdditionalFields 访问参数
	paramsMap := notification.Params.AdditionalFields

	// Extract progress value and message
	progress, _ := paramsMap["progress"].(float64)
	message, _ := paramsMap["message"].(string)

	// Check if data is map[string]interface{}
	if dataMap, ok := paramsMap["data"].(map[string]interface{}); ok {
		notificationType, _ := dataMap["type"].(string)

		log.Infof("Received progress notification [%s] (Type: %s): %+v", notification.Method, notificationType, dataMap)

		// Process based on notificationType
		switch notificationType {
		case "process_progress":
			// Parse step information from message
			progressStr := fmt.Sprintf("%.2f%%", progress*100)

			// Extract current step and total steps
			currentStep := "?"
			totalSteps := "?"

			// Parse message in "Step 1/5" format
			if parts := strings.Split(message, " "); len(parts) > 1 {
				if stepParts := strings.Split(parts[1], "/"); len(stepParts) > 1 {
					currentStep = stepParts[0]
					totalSteps = stepParts[1]
				}
			}

			log.Infof("  Process progress: %s (Step: %s/%s, Progress: %s)", message, currentStep, totalSteps, progressStr)
		default:
			log.Infof("  Unknown progress notification data type: %s", notificationType)
		}
	} else {
		// No data field or other cases, display basic info
		log.Infof("Received progress notification [%s]: Progress %.0f%%, %s", notification.Method, progress*100, message)
	}

	return nil
}

func main() {
	// Set up logging
	log.Info("Starting Stateful SSE Yes GET SSE mode client (default configuration)...")

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create client info
	clientInfo := mcp.Implementation{
		Name:    "Stateful-SSE-GETSSE-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server
	mcpClient, err := client.NewClient(
		"http://localhost:3006/mcp",
		clientInfo,
		client.WithGetSSEEnabled(true), // Enable GET SSE
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// Register notification handlers
	mcpClient.RegisterNotificationHandler("notifications/message", handleNotification)
	mcpClient.RegisterNotificationHandler("notifications/progress", handleProgressNotification)

	// Initialize client
	log.Info("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx)
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	log.Infof("Initialization successful: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Infof("Server capabilities: %+v", initResp.Capabilities)

	// Get session information
	sessionID := mcpClient.GetSessionID()
	if sessionID == "" {
		log.Info("Warning: No session ID received. Server may not be properly configured for stateful mode.")
	} else {
		log.Infof("Session established, ID: %s", sessionID)
	}

	// Get available tools list
	log.Info("Listing tools...")
	toolsResult, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
	}

	log.Infof("Server provides %d tools", len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		log.Infof("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call greeting tool
	log.Info("Calling greet tool...")
	callResult, err := mcpClient.CallTool(ctx, "greet", map[string]interface{}{
		"name": "SSE+GETSSE Client User",
	})
	if err != nil {
		log.Fatalf("Tool call failed: %v", err)
	}

	// Display call result
	log.Info("Greeting tool call result:")
	for _, content := range callResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		} else {
			log.Infof("- Other content type: %+v", content)
		}
	}

	// Call counter tool, demonstrating session state retention.
	log.Info("Calling counter tool for the first time...")
	counterResult1, err := mcpClient.CallTool(ctx, "counter", map[string]interface{}{
		"increment": 1,
	})
	if err != nil {
		log.Fatalf("Counter tool call failed: %v", err)
	}

	// Display counter result
	log.Info("Counter result (first call):")
	for _, content := range counterResult1.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Join chat room
	log.Info("Calling chatJoin tool to join chat room...")
	userName := fmt.Sprintf("User_%d", time.Now().Unix()%1000)
	chatJoinResult, err := mcpClient.CallTool(ctx, "chatJoin", map[string]interface{}{
		"userName": userName,
	})
	if err != nil {
		log.Fatalf("Failed to join chat room: %v", err)
	}

	// Display chat room join result
	log.Info("Chat room join result:")
	for _, content := range chatJoinResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Send chat message
	log.Info("Calling chatSend tool to send chat message...")
	chatSendResult, err := mcpClient.CallTool(ctx, "chatSend", map[string]interface{}{
		"message": "Hello everyone! This is a test message from the complete SSE client.",
	})
	if err != nil {
		log.Fatalf("Failed to send chat message: %v", err)
	}

	// Display chat message send result
	log.Info("Chat message send result:")
	for _, content := range chatSendResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Call delayed response tool, demonstrating SSE streaming response advantage
	log.Info("Calling delayedResponse tool to experience streaming response...")

	// Create stream options, set callback function for processing incremental content
	streamOpts := &transport.StreamOptions{
		NotificationHandlers: map[string]transport.NotificationHandler{
			"*": handleNotification,
		},
	}

	// Use streaming API to call the tool
	_, err = mcpClient.CallToolWithStream(ctx, "delayedResponse", map[string]interface{}{
		"steps":   5,
		"delayMs": 500,
	}, streamOpts)

	if err != nil {
		log.Fatalf("Delayed response tool call failed: %v", err)
	}

	log.Info("Delayed response tool streaming call completed")

	// Call notification tool, demonstrating server push notification functionality
	log.Info("Calling sendNotification tool, will receive notification after delay...")
	notifyResult, err := mcpClient.CallTool(ctx, "sendNotification", map[string]interface{}{
		"message": "This is a test notification message from the complete SSE client",
		"delay":   2,
	})
	if err != nil {
		log.Fatalf("Notification tool call failed: %v", err)
	}

	// Display call result
	log.Info("Notification tool call result:")
	for _, content := range notifyResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Print session information
	log.Infof("\nSession information: ID=%s", mcpClient.GetSessionID())
	log.Info("Client has full SSE functionality enabled, including GET SSE connection, waiting for notifications and messages...")
	log.Info("Tip: You can check server session status via curl http://localhost:3006/sessions")
	log.Info("Press Ctrl+C to exit...")

	// Set up signal handling to keep client running until user presses Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("Received termination signal, client example complete, exiting...")
}
