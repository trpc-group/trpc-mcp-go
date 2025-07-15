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
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// handleNotification processes server notifications.
func handleNotification(notification *mcp.JSONRPCNotification) error {
	// Access parameters using AdditionalFields
	paramsMap := notification.Params.AdditionalFields
	level, _ := paramsMap["level"].(string)

	// Check if data is map[string]interface{}
	if dataMap, ok := paramsMap["data"].(map[string]interface{}); ok {
		notificationType, _ := dataMap["type"].(string)

		log.Printf(
			"Received notification [%s] (Level: %s, Type: %s): %+v",
			notification.Method, level, notificationType, dataMap,
		)

		// Process based on notificationType
		switch notificationType {
		case "test_notification":
			// Handle test notification
			if message, exists := dataMap["message"].(string); exists {
				log.Printf("  Test notification content: %s", message)
			}
		case "process_started":
			// Handle process start notification
			if message, exists := dataMap["message"].(string); exists {
				log.Printf("  Process started: %s (Steps: %v, Delay: %vms)", message, dataMap["steps"], dataMap["delayMs"])
			}
		case "process_progress":
			// Handle process progress notification
			if message, exists := dataMap["message"].(string); exists {
				log.Printf(
					"  Process progress: %s (Step: %v/%v, Progress: %.2f%%)",
					message, dataMap["step"], dataMap["total"], dataMap["progress"],
				)
			}
		case "chat_message":
			// Handle chat message notification
			if userName, uOk := dataMap["userName"].(string); uOk {
				if message, mOk := dataMap["message"].(string); mOk {
					log.Printf("  Chat message [%s] %s: %s", dataMap["timestamp"], userName, message)
				}
			}
		case "chat_system_message":
			// Handle chat system message
			if message, exists := dataMap["message"].(string); exists {
				log.Printf("  Chat system message [%s]: %s", dataMap["timestamp"], message)
			}
		case "log_message":
			// Handle log message
			if message, exists := dataMap["message"].(string); exists {
				log.Printf("  Log message: %s", message)
			}
		default:
			log.Printf("  Unknown notification data type: %s", notificationType)
		}
	} else if dataStr, ok := paramsMap["data"].(string); ok {
		// Handle string type data field
		log.Printf("Received notification [%s] (Level: %s): %s", notification.Method, level, dataStr)
	} else {
		// Other data field types
		log.Printf("Received notification [%s] (Level: %s): %+v", notification.Method, level, paramsMap["data"])
	}

	return nil
}

// handleProgressNotification processes progress notifications.
func handleProgressNotification(notification *mcp.JSONRPCNotification) error {
	// Access parameters using AdditionalFields
	paramsMap := notification.Params.AdditionalFields

	// Extract progress value and message
	progress, _ := paramsMap["progress"].(float64)
	message, _ := paramsMap["message"].(string)

	// Check if data is map[string]interface{}
	if dataMap, ok := paramsMap["data"].(map[string]interface{}); ok {
		notificationType, _ := dataMap["type"].(string)

		log.Printf("Received progress notification [%s] (Type: %s): %+v", notification.Method, notificationType, dataMap)

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

			log.Printf("  Process progress: %s (Step: %s/%s, Progress: %s)", message, currentStep, totalSteps, progressStr)
		default:
			log.Printf("  Unknown progress notification data type: %s", notificationType)
		}
	} else {
		// No data field or other cases, display basic info
		log.Printf("Received progress notification [%s]: Progress %.0f%%, %s", notification.Method, progress*100, message)
	}

	return nil
}

// initializeClient initializes the client
func initializeClient(ctx context.Context) (*mcp.Client, error) {
	clientInfo := mcp.Implementation{
		Name:    "Stateful-SSE-GETSSE-Client",
		Version: "1.0.0",
	}

	mcpClient, err := mcp.NewClient(
		"http://localhost:3006/mcp",
		clientInfo,
		mcp.WithClientGetSSEEnabled(true),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	// Register notification handlers
	mcpClient.RegisterNotificationHandler("notifications/message", handleNotification)
	mcpClient.RegisterNotificationHandler("notifications/progress", handleProgressNotification)

	// Initialize client
	log.Printf("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("initialization failed: %v", err)
	}

	log.Printf("Initialization successful: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Printf("Server capabilities: %+v", initResp.Capabilities)

	// Get session information
	sessionID := mcpClient.GetSessionID()
	if sessionID == "" {
		log.Printf("Warning: No session ID received. Server may not be properly configured for stateful mode.")
	} else {
		log.Printf("Session established, ID: %s", sessionID)
	}

	return mcpClient, nil
}

// printContent prints the content
func printContent(content interface{}) {
	if textContent, ok := content.(mcp.TextContent); ok {
		log.Printf("- Text: %s", textContent.Text)
	} else {
		log.Printf("- Other content type: %+v", content)
	}
}

// handleToolCall handles tool calls
func handleToolCall(ctx context.Context, client *mcp.Client) error {
	// Get tool list
	log.Printf("Listing tools...")
	toolsResult, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to get tools list: %v", err)
	}

	log.Printf("Server provides %d tools", len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		log.Printf("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call greet tool
	log.Printf("Calling greet tool...")
	callToolReq := &mcp.CallToolRequest{}
	callToolReq.Params.Name = "greet"
	callToolReq.Params.Arguments = map[string]interface{}{
		"name": "SSE+GETSSE Client User",
	}
	callResult, err := client.CallTool(ctx, callToolReq)
	if err != nil {
		return fmt.Errorf("tool call failed: %v", err)
	}

	// Display call result
	log.Printf("Greeting tool call result:")
	for _, content := range callResult.Content {
		printContent(content)
	}

	// Call counter tool
	log.Printf("Calling counter tool for the first time...")
	counterResult1, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "counter",
			Arguments: map[string]interface{}{
				"increment": 1,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("counter tool call failed: %v", err)
	}

	// Display counter result
	log.Printf("Counter result (first call):")
	for _, content := range counterResult1.Content {
		printContent(content)
	}

	return nil
}

// handleChatOperations handles chat room operations
func handleChatOperations(ctx context.Context, client *mcp.Client) error {
	// Join chat room
	log.Printf("Calling chatJoin tool to join chat room...")
	userName := fmt.Sprintf("User_%d", time.Now().Unix()%1000)
	chatJoinResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "chatJoin",
			Arguments: map[string]interface{}{
				"userName": userName,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to join chat room: %v", err)
	}

	// Display join result
	log.Printf("Chat room join result:")
	for _, content := range chatJoinResult.Content {
		printContent(content)
	}

	// Send chat message
	log.Printf("Calling chatSend tool to send chat message...")
	chatSendResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "chatSend",
			Arguments: map[string]interface{}{
				"message": "Hello everyone! This is a test message from the complete SSE client.",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send chat message: %v", err)
	}

	// Display send result
	log.Printf("Chat message send result:")
	for _, content := range chatSendResult.Content {
		printContent(content)
	}

	return nil
}

// handleDelayedOperations handles delayed operations
func handleDelayedOperations(ctx context.Context, client *mcp.Client) error {
	// Call delayed response tool
	log.Printf("Calling delayedResponse tool to experience streaming response...")
	_, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "delayedResponse",
			Arguments: map[string]interface{}{
				"steps":   5,
				"delayMs": 500,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("delayed response tool call failed: %v", err)
	}
	log.Printf("Delayed response tool streaming call completed")

	// Call notification tool
	log.Printf("Calling sendNotification tool, will receive notification after delay...")
	notifyResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "sendNotification",
			Arguments: map[string]interface{}{
				"message": "This is a test notification message from the complete SSE client",
				"delay":   2,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("notification tool call failed: %v", err)
	}

	// Display notification result
	log.Printf("Notification tool call result:")
	for _, content := range notifyResult.Content {
		printContent(content)
	}

	return nil
}

func main() {
	// Print client startup message
	log.Printf("Starting Stateful SSE+GET SSE mode client...")

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Initialize client
	client, err := initializeClient(ctx)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer client.Close()

	// Handle tool calls
	if err := handleToolCall(ctx, client); err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Handle chat room operations
	if err := handleChatOperations(ctx, client); err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Handle delayed operations
	if err := handleDelayedOperations(ctx, client); err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Print session information
	log.Printf("\nSession information: ID=%s", client.GetSessionID())
	log.Printf(
		"Client has full SSE functionality enabled, including GET SSE connection, " +
			"waiting for notifications and messages...",
	)
	log.Printf("Tip: You can check server session status via curl http://localhost:3006/sessions")
	log.Printf("Press Ctrl+C to exit...")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Printf("Received termination signal, client example complete, exiting...")
}
