// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Print client startup information.
	log.Printf("Starting SSE Compatibility client (2024-11-05 protocol)...")

	// Create context, set 60-second timeout (increase timeout).
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create client information.
	clientInfo := mcp.Implementation{
		Name:    "SSE-Compatibility-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	// Use NewSSEClient to create an SSE client that supports the 2024-11-05 protocol.
	serverURL := "http://localhost:4000/sse"
	log.Printf("Creating SSE client for 2024-11-05 protocol compatibility...")
	log.Printf("Connecting to %s...", serverURL)

	mcpClient, err := mcp.NewSSEClient(
		serverURL,
		clientInfo,
		mcp.WithProtocolVersion(mcp.ProtocolVersion_2024_11_05), // Explicitly specify protocol version.
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// Register notification handler.
	mcpClient.RegisterNotificationHandler("notifications/message", handleNotification)
	log.Printf("Registered notification handler for 'notifications/message'")

	// Initialize client.
	log.Printf("Initializing client with 2024-11-05 protocol version...")
	initReq := &mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.ProtocolVersion_2024_11_05, // Use old version protocol.
			ClientInfo:      clientInfo,
			Capabilities: mcp.ClientCapabilities{
				Experimental: map[string]interface{}{
					"notifications": []string{"notifications/message"},
				},
			},
		},
	}

	// Print initialization request details.
	initReqJSON, _ := json.Marshal(initReq)
	log.Printf("Sending initialization request: %s", string(initReqJSON))

	// Initialize client.
	initResp, err := mcpClient.Initialize(ctx, initReq)
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	log.Printf("Initialization succeeded: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Printf("Server capabilities: %+v", initResp.Capabilities)

	// Get available tool list.
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
	greetResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "greet",
			Arguments: map[string]interface{}{
				"name": "SSE compatibility client user",
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to call greet tool: %v", err)
	}

	// Display call result.
	log.Printf("Greet result:")
	for _, content := range greetResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s", textContent.Text)
		} else {
			log.Printf("- Other type content: %+v", content)
		}
	}

	// Call weather tool.
	log.Printf("Calling weather tool for Beijing...")
	weatherResult1, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "weather",
			Arguments: map[string]interface{}{
				"city": "Beijing",
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to call weather tool: %v", err)
	}

	// Display Beijing weather result.
	log.Printf("Weather result for Beijing:")
	for _, content := range weatherResult1.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s", textContent.Text)
		}
	}

	// Call weather tool again, query Shanghai weather.
	log.Printf("Calling weather tool for Shanghai...")
	weatherResult2, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "weather",
			Arguments: map[string]interface{}{
				"city": "Shanghai",
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to call weather tool: %v", err)
	}

	// Display Shanghai weather result.
	log.Printf("Weather result for Shanghai:")
	for _, content := range weatherResult2.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s", textContent.Text)
		}
	}

	// Test large response tool to reproduce channel closure issue.
	log.Printf("Testing large response tool to reproduce channel closure...")

	// Test with small response first (1KB).
	log.Printf("Calling large_response tool with 1KB...")
	smallResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "large_response",
			Arguments: map[string]interface{}{
				"size": 1,
			},
		},
	})
	if err != nil {
		log.Printf("Failed to call large_response tool (1KB): %v", err)
	} else {
		log.Printf("Small response succeeded, content length: %d chars",
			len(fmt.Sprintf("%+v", smallResult.Content)))
	}

	// Test with medium response (10KB).
	log.Printf("Calling large_response tool with 10KB...")
	mediumResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "large_response",
			Arguments: map[string]interface{}{
				"size": 10,
			},
		},
	})
	if err != nil {
		log.Printf("Failed to call large_response tool (10KB): %v", err)
	} else {
		log.Printf("Medium response succeeded, content length: %d chars",
			len(fmt.Sprintf("%+v", mediumResult.Content)))
	}

	// Test with large response (100KB) - this might trigger the channel closure.
	log.Printf("Calling large_response tool with 100KB...")
	largeResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "large_response",
			Arguments: map[string]interface{}{
				"size": 100,
			},
		},
	})
	if err != nil {
		log.Printf("Failed to call large_response tool (100KB): %v", err)
		log.Printf("This might be the 'response channel closed' error we're investigating!")
	} else {
		log.Printf("Large response succeeded, content length: %d chars",
			len(fmt.Sprintf("%+v", largeResult.Content)))
	}

	// Test with very large response (500KB) - this should definitely trigger issues.
	log.Printf("Calling large_response tool with 500KB...")
	veryLargeResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "large_response",
			Arguments: map[string]interface{}{
				"size": 500,
			},
		},
	})
	if err != nil {
		log.Printf("Failed to call large_response tool (500KB): %v", err)
		log.Printf("ERROR REPRODUCED! Check the error message above.")
	} else {
		log.Printf("Very large response succeeded, content length: %d chars",
			len(fmt.Sprintf("%+v", veryLargeResult.Content)))
	}

	log.Printf("Client example completed.")
}

// handleNotification handles SSE incremental update callback function.
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
