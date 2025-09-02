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

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Initialize log.
	log.Println("Starting annotations demo client...")

	// Create context.
	ctx := context.Background()

	// Initialize client
	client, err := initializeClient(ctx)
	if err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Handle tools with annotations demonstration
	if err := handleTools(ctx, client); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Terminate session
	if err := terminateSession(ctx, client); err != nil {
		log.Printf("Error: %v\n", err)
	}

	log.Printf("Annotations demo finished.")
}

// initializeClient initializes the MCP client with server connection and session setup
func initializeClient(ctx context.Context) (*mcp.Client, error) {
	log.Println("===== Initialize client =====")
	serverURL := "http://localhost:3000/mcp"
	mcpClient, err := mcp.NewClient(
		serverURL,
		mcp.Implementation{
			Name:    "MCP-Go-Client",
			Version: "1.0.0",
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("initialization failed: %v", err)
	}

	log.Printf("Server info: %s %s", initResp.ServerInfo.Name, initResp.ServerInfo.Version)
	log.Printf("Protocol version: %s", initResp.ProtocolVersion)
	if initResp.Instructions != "" {
		log.Printf("Server instructions: %s", initResp.Instructions)
	}

	sessionID := mcpClient.GetSessionID()
	if sessionID != "" {
		log.Printf("Session ID: %s", sessionID)
	}

	return mcpClient, nil
}

// handleTools manages tool-related operations including listing and calling tools
func handleTools(ctx context.Context, client *mcp.Client) error {
	log.Println("===== List available tools =====")
	listToolsResp, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %v", err)
	}

	tools := listToolsResp.Tools
	if len(tools) == 0 {
		log.Printf("No available tools.")
		return nil
	}

	log.Printf("Found %d tools:", len(tools))
	for _, tool := range tools {
		log.Printf("- %s: %s", tool.Name, tool.Description)
		// 显示annotations信息
		if tool.Annotations != nil {
			log.Printf("  Annotations:")
			log.Printf("    Title: %s", tool.Annotations.Title)
			if tool.Annotations.ReadOnlyHint != nil {
				log.Printf("    ReadOnlyHint: %v", *tool.Annotations.ReadOnlyHint)
			}
			if tool.Annotations.DestructiveHint != nil {
				log.Printf("    DestructiveHint: %v", *tool.Annotations.DestructiveHint)
			}
			if tool.Annotations.IdempotentHint != nil {
				log.Printf("    IdempotentHint: %v", *tool.Annotations.IdempotentHint)
			}
			if tool.Annotations.OpenWorldHint != nil {
				log.Printf("    OpenWorldHint: %v", *tool.Annotations.OpenWorldHint)
			}
		} else {
			log.Printf("  No annotations")
		}
	}

	// Call the first tool
	log.Printf("===== Call tool: %s =====", tools[0].Name)
	callToolReq := &mcp.CallToolRequest{}
	callToolReq.Params.Name = tools[0].Name
	callToolReq.Params.Arguments = map[string]interface{}{
		"name": "MCP User",
	}
	callToolResp, err := client.CallTool(ctx, callToolReq)
	if err != nil {
		return fmt.Errorf("failed to call tool: %v", err)
	}

	log.Printf("Tool result:")
	for _, item := range callToolResp.Content {
		if textContent, ok := item.(mcp.TextContent); ok {
			log.Printf("  %s", textContent.Text)
		}
	}

	return nil
}

// terminateSession handles the termination of the current session
func terminateSession(ctx context.Context, client *mcp.Client) error {
	sessionID := client.GetSessionID()
	if sessionID == "" {
		return nil
	}

	log.Printf("===== Terminate session =====")
	if err := client.TerminateSession(ctx); err != nil {
		return fmt.Errorf("failed to terminate session: %v", err)
	}
	log.Printf("Session terminated.")
	return nil
}
