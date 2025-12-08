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
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Set log level.
	log.Printf("Starting Stateful JSON Yes GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := mcp.Implementation{
		Name:    "Stateful-JSON-GETSSE-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	mcpClient, err := mcp.NewClient(
		"http://localhost:3004/mcp",
		clientInfo,
		mcp.WithClientGetSSEEnabled(true), // Enable GET SSE
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// Register notification handler.
	mcpClient.RegisterNotificationHandler("notifications/message", handleNotification)

	// Initialize client.
	log.Printf("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	log.Printf("Initialization succeeded: Server=%s %s, Protocol=%s.",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Printf("Server capabilities: %+v.", initResp.Capabilities)

	// Get session info.
	sessionID := mcpClient.GetSessionID()
	if sessionID == "" {
		log.Printf("Warning: No session ID received. Server may not be properly configured for stateful mode.")
	} else {
		log.Printf("Session established, ID: %s.", sessionID)
	}

	// Get available tools list.
	log.Printf("Listing tools...")
	tools, err := mcpClient.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
	}

	log.Printf("Server provides %d tools.", len(tools.Tools))
	for _, tool := range tools.Tools {
		log.Printf("- Tool: %s (%s).", tool.Name, tool.Description)
	}

	// Call greet tool.
	log.Printf("Calling greet tool...")
	callToolReq := mcp.CallToolRequest{}
	callToolReq.Params.Name = "greet"
	callToolReq.Params.Arguments = map[string]interface{}{"name": "JSON+GETSSE client user"}
	callResult, err := mcpClient.CallTool(ctx, &callToolReq)
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
	}

	// Show call result.
	log.Printf("Call result:")
	for _, content := range callResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s.", textContent.Text)
		} else {
			log.Printf("- Other type content: %+v.", content)
		}
	}

	// Call counter tool, demonstrate session state keeping.
	log.Printf("Calling counter tool first time...")
	counterResult1, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "counter",
			Arguments: map[string]interface{}{
				"increment": 1,
			},
			Meta: &mcp.Meta{
				ProgressToken: 123,
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to call counter tool: %v", err)
	}

	// Show counter result.
	log.Printf("Counter result (first time):")
	for _, content := range counterResult1.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s.", textContent.Text)
		}
	}

	// Call counter tool again, verify state keeping.
	log.Printf("Calling counter tool second time...")
	counterResult2, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "counter",
			Arguments: map[string]interface{}{
				"increment": 2,
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to call counter tool: %v", err)
	}

	// Show counter result.
	log.Printf("Counter result (second time):")
	for _, content := range counterResult2.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s.", textContent.Text)
		}
	}

	// Call notification tool, demonstrate server push notification feature.
	log.Printf("Calling sendNotification tool, you will receive a notification after a delay...")
	notifyResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "sendNotification",
			Arguments: map[string]interface{}{
				"message": "This is a test notification message sent from JSON+GETSSE client",
				"delay":   2,
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to call notification tool: %v", err)
	}

	// Show call result.
	log.Printf("Notification tool call result:")
	for _, content := range notifyResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s.", textContent.Text)
		}
	}

	// Print session info.
	log.Printf("\nSession info: ID=%s.", mcpClient.GetSessionID())
	log.Printf("Client has enabled GET SSE connection, waiting for notifications...")
	log.Printf("Tip: you can use curl http://localhost:3004/sessions to check server session state.")

	// Wait a while to receive possible notifications.
	log.Printf("Waiting to receive notifications (60 seconds)...")
	time.Sleep(60 * time.Second)

	log.Printf("Client example finished, exiting soon...")
}

// handler function for server notifications.
func handleNotification(notification *mcp.JSONRPCNotification) error {
	paramsMap := notification.Params.AdditionalFields // Use AdditionalFields here.

	level, _ := paramsMap["level"].(string)
	dataMap, ok := paramsMap["data"].(map[string]interface{})
	if !ok {
		log.Printf(
			"Received notification [%s] (Level: %s), but 'data' field is invalid or missing: %+v.",
			notification.Method, level, paramsMap,
		)
		return fmt.Errorf("'data' field is invalid or missing")
	}

	notificationType, typeOk := dataMap["type"].(string)
	if !typeOk {
		log.Printf(
			"Received notification [%s] (Level: %s), but 'type' field in data is invalid or missing: %+v",
			notification.Method, level, dataMap,
		)
		return fmt.Errorf("'type' field in notification data is invalid or missing")
	}

	log.Printf(
		"Received notification [%s] (Level: %s, Type: %s): %+v.",
		notification.Method, level, notificationType, dataMap,
	)

	// Handle specific notification types.
	switch notificationType {
	case "test_notification":
		// Handle test notification.
		if message, exists := dataMap["message"].(string); exists {
			log.Printf("  Test notification content: %s.", message)
		}
	case "system_status":
		// Handle system status notification.
		if cpu, exists := dataMap["cpu"].(string); exists {
			log.Printf("  System status - CPU: %s.", cpu)
		}
		if memory, exists := dataMap["memory"].(string); exists {
			log.Printf("  System status - Memory: %s.", memory)
		}
	default:
		log.Printf("  Unknown notification data type: %s.", notificationType)
	}
	return nil
}
