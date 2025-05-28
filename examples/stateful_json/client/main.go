// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"time"

	"log"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Print client start message.
	log.Printf("Starting Stateful JSON No GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := mcp.Implementation{
		Name:    "Stateful-JSON-Client",
		Version: "1.0.0",
	}

	// Create client, use JSON instead of SSE mode.
	mcpClient, err := mcp.NewClient(
		"http://localhost:3003/mcp",
		clientInfo,
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// Initialize client.
	log.Printf("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	log.Printf("Initialization succeeded: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Printf("Server capabilities: %+v", initResp.Capabilities)

	// Get available tools list.
	log.Printf("Listing tools...")
	tools, err := mcpClient.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
	}

	log.Printf("Server provides %d tools", len(tools.Tools))
	for _, tool := range tools.Tools {
		log.Printf("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call greet tool.
	log.Printf("Calling greet tool...")
	callToolReq := &mcp.CallToolRequest{}
	callToolReq.Params.Name = "greet"
	callToolReq.Params.Arguments = map[string]interface{}{
		"name": "Stateful client user",
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
	log.Printf("\nCalling counter tool first time...")
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
	log.Printf("\nCalling counter tool second time...")
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

	// Call counter tool third time, continue verifying state keeping.
	log.Printf("\nCalling counter tool third time...")
	counterResult3, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "counter",
			Arguments: map[string]interface{}{
				"increment": 3,
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to call counter tool: %v", err)
	}

	// Show counter result.
	log.Printf("Counter result (third time):")
	for _, content := range counterResult3.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s", textContent.Text)
		}
	}

	// Print session info.
	log.Printf("\nSession info: ID=%s", mcpClient.GetSessionID())
	log.Printf("Client keeps session alive, you can check session state in another terminal...")
	log.Printf("Tip: you can use curl http://localhost:3003/sessions to check server session state")

	// Wait a while for user to see output.
	log.Printf("Client example finished, exiting in 3 seconds...")
	time.Sleep(3 * time.Second)
}
