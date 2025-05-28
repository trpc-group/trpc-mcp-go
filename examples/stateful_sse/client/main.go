// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
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

// Callback function for handling SSE incremental updates.
func handleNotification(notification *mcp.JSONRPCNotification) error {
	paramsMap := notification.Params.AdditionalFields
	level, _ := paramsMap["level"].(string)
	dataMap, ok := paramsMap["data"].(map[string]interface{})
	if !ok {
		log.Printf(
			"Received notification [%s] (Level: %s), but 'data' field is invalid or missing: %+v",
			notification.Method, level, paramsMap,
		)
		return fmt.Errorf("'data' field is invalid or missing")
	}

	notificationType, _ := dataMap["type"].(string)
	log.Printf(
		"Received notification [%s] (Level: %s, Type: %s): %+v",
		notification.Method, level, notificationType, dataMap,
	)

	switch notificationType {
	case "process_started":
		if message, exists := dataMap["message"].(string); exists {
			log.Printf("  Stream processing started: %s (Steps: %v, Delay: %vms)", message, dataMap["steps"], dataMap["delayMs"])
		}
	case "process_progress":
		if message, exists := dataMap["message"].(string); exists {
			log.Printf(
				"  Stream processing progress: %s (Step: %v/%v, Progress: %.2f%%)",
				message, dataMap["step"], dataMap["total"], dataMap["progress"],
			)
		}
	default:
		log.Printf("  Received other type of notification '%s': %+v", notificationType, dataMap)
	}
	return nil
}

func main() {
	// Print client start message.
	log.Printf("Starting Stateful SSE No GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := mcp.Implementation{
		Name:    "Stateful-SSE-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	mcpClient, err := mcp.NewClient(
		"http://localhost:3005/mcp",
		clientInfo,
		mcp.WithClientGetSSEEnabled(false), // Disable GET SSE
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	mcpClient.RegisterNotificationHandler("notifications/message", handleNotification)

	// Initialize client.
	log.Printf("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	log.Printf("Initialization succeeded: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Printf("Server capabilities: %+v", initResp.Capabilities)

	// Get session info.
	sessionID := mcpClient.GetSessionID()
	if sessionID == "" {
		log.Printf("Warning: No session ID received. Server may not be properly configured for stateful mode.")
	} else {
		log.Printf("Session established, ID: %s", sessionID)
	}

	// Get available tools list.
	log.Printf("Listing tools...")
	toolsResult, err := mcpClient.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
	}

	log.Printf("Server provides %d tools", len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		log.Printf("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call greet tool.
	log.Printf("Calling greet tool...")
	callToolReq := &mcp.CallToolRequest{}
	callToolReq.Params.Name = "greet"
	callToolReq.Params.Arguments = map[string]interface{}{
		"name": "SSE client user",
	}
	callResult, err := mcpClient.CallTool(ctx, callToolReq)
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
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

	// Call counter tool, demonstrate session state keeping.
	log.Printf("Calling counter tool first time...")
	counterResult1, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "counter",
			Arguments: map[string]interface{}{
				"increment": 1,
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
			log.Printf("- Text: %s", textContent.Text)
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
			log.Printf("- Text: %s", textContent.Text)
		}
	}

	// Call delayedResponse tool, demonstrate the advantage of SSE streaming response.
	log.Printf("Calling delayedResponse tool, experience streaming response...")

	// Use streaming API to call tool.
	_, err = mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "delayedResponse",
			Arguments: map[string]interface{}{
				"steps":   5,
				"delayMs": 500,
			},
		},
	})

	if err != nil {
		log.Fatalf("Failed to call delayedResponse tool: %v", err)
	}

	log.Printf("delayedResponse tool streaming call finished")

	// Print session info.
	log.Printf("\nSession info: ID=%s", mcpClient.GetSessionID())
	log.Printf("Client has not enabled GET SSE connection, cannot receive independent notifications")
	log.Printf("Tip: you can use curl http://localhost:3005/sessions to check server session state")

	// Wait a while for user to see output.
	log.Printf("Client example finished, exiting in 5 seconds...")
	time.Sleep(5 * time.Second)
}
