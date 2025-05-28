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
	"os"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

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

// printContent formats and prints different types of content with role information
func printContent(content interface{}, index int, role mcp.Role) {
	switch c := content.(type) {
	case mcp.TextContent:
		log.Printf(
			"[%d] %s message: %s",
			index, role, truncateString(c.Text, 50),
		)
	case mcp.ImageContent:
		log.Printf("[%d] %s message (image)", index, role)
	case mcp.AudioContent:
		log.Printf("[%d] %s message (audio)", index, role)
	case mcp.EmbeddedResource:
		log.Printf("[%d] %s message (embedded resource)", index, role)
	default:
		log.Printf("[%d] %s message (unknown content type)", index, role)
	}
}

// handlePrompts manages prompt-related operations including listing and retrieving prompts
func handlePrompts(ctx context.Context, client *mcp.Client) error {
	log.Println("===== List prompts =====")
	promptsResult, err := client.ListPrompts(ctx, &mcp.ListPromptsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list prompts: %v", err)
	}

	log.Printf("Found %d prompts:", len(promptsResult.Prompts))
	for _, prompt := range promptsResult.Prompts {
		log.Printf("- %s: %s", prompt.Name, prompt.Description)
		if len(prompt.Arguments) > 0 {
			log.Printf("  Arguments:")
			for _, arg := range prompt.Arguments {
				required := ""
				if arg.Required {
					required = " (required)"
				}
				log.Printf("  - %s: %s%s", arg.Name, arg.Description, required)
			}
		}
	}

	if len(promptsResult.Prompts) == 0 {
		return nil
	}

	// Get the first prompt
	arguments := make(map[string]string)
	for _, arg := range promptsResult.Prompts[0].Arguments {
		if arg.Required {
			arguments[arg.Name] = "example value"
		}
	}

	log.Printf("===== Get prompt: %s =====", promptsResult.Prompts[0].Name)
	getPromptReq := &mcp.GetPromptRequest{}
	getPromptReq.Params.Name = promptsResult.Prompts[0].Name
	getPromptReq.Params.Arguments = arguments
	promptContent, err := client.GetPrompt(ctx, getPromptReq)
	if err != nil {
		return fmt.Errorf("failed to get prompt: %v", err)
	}

	log.Printf("Successfully got prompt, message count: %d", len(promptContent.Messages))
	if promptContent.Description != "" {
		log.Printf("Prompt description: %s", promptContent.Description)
	}

	for i, msg := range promptContent.Messages {
		printContent(msg.Content, i, msg.Role)
	}

	return nil
}

// handleResources manages resource-related operations including listing and reading resources
func handleResources(ctx context.Context, client *mcp.Client) error {
	log.Printf("===== List resources =====")
	resourcesResult, err := client.ListResources(ctx, &mcp.ListResourcesRequest{})
	if err != nil {
		return fmt.Errorf("failed to list resources: %v", err)
	}

	log.Printf("Found %d resources:", len(resourcesResult.Resources))
	for _, resource := range resourcesResult.Resources {
		log.Printf("- %s: %s (%s)", resource.Name, resource.Description, resource.URI)
	}

	if len(resourcesResult.Resources) == 0 {
		return nil
	}

	// Read the first resource
	log.Printf("===== Read resource: %s =====", resourcesResult.Resources[0].Name)
	readResourceReq := &mcp.ReadResourceRequest{}
	readResourceReq.Params.URI = resourcesResult.Resources[0].URI
	resourceContent, err := client.ReadResource(ctx, readResourceReq)
	if err != nil {
		return fmt.Errorf("failed to read resource: %v", err)
	}

	log.Printf("Successfully read resource, content item count: %d", len(resourceContent.Contents))
	for i, content := range resourceContent.Contents {
		switch c := content.(type) {
		case mcp.TextResourceContents:
			log.Printf(
				"[%d] Text resource: %s (first 50 chars: %s...)",
				i, c.URI, truncateString(c.Text, 50),
			)
		case mcp.BlobResourceContents:
			log.Printf(
				"[%d] Binary resource: %s (size: %d bytes)",
				i, c.URI, len(c.Blob),
			)
		default:
			log.Printf("[%d] Unknown resource type", i)
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

func main() {
	// Initialize log.
	log.Println("Starting example client...")

	// Create context.
	ctx := context.Background()

	// Initialize client
	client, err := initializeClient(ctx)
	if err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Handle tools
	if err := handleTools(ctx, client); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Handle prompts
	if err := handlePrompts(ctx, client); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Handle resources
	if err := handleResources(ctx, client); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Terminate session
	if err := terminateSession(ctx, client); err != nil {
		log.Printf("Error: %v\n", err)
	}

	log.Printf("Example finished.")
}

// truncateString shortens a string to the specified maximum length and adds ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
