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
	"os"

	"log"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Initialize log for the client
	log.Printf("Starting example client...")

	// Create context
	ctx := context.Background()

	// Initialize client
	client, err := initializeClient(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
	defer client.Close()

	// Handle resources
	if err := handleResources(ctx, client); err != nil {
		log.Printf("Error: %v", err)
	}

	// Handle prompts
	if err := handlePrompts(ctx, client); err != nil {
		log.Printf("Error: %v", err)
	}

	// Handle tools
	if err := handleTools(ctx, client); err != nil {
		log.Printf("Error: %v", err)
	}

	// Handle completions
	if err := handleCompletions(ctx, client); err != nil {
		log.Printf("Error: %v", err)
	}

	log.Printf("Test finished!")
}

// initializeClient initializes the MCP client with server connection and session setup
func initializeClient(ctx context.Context) (*mcp.Client, error) {
	log.Printf("===== Create client =====")
	serverURL := "http://localhost:3000/mcp"
	clientInfo := mcp.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}

	newClient, err := mcp.NewClient(serverURL, clientInfo)
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err)
	}

	log.Printf("===== Initialize client =====")
	initResult, err := newClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		newClient.Close()
		return nil, fmt.Errorf("initialization error: %v", err)
	}
	log.Printf("Connected to server: %s %s", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	// Check server capabilities
	capabilitiesJSON, _ := json.Marshal(initResult.Capabilities)
	log.Printf("Server capabilities:%v", string(capabilitiesJSON))

	return newClient, nil
}

// printResourceContent formats and prints different types of resource content with index information
func printResourceContent(content interface{}, index int) {
	switch c := content.(type) {
	case mcp.TextResourceContents:
		log.Printf("[%d] Text resource: %s (first 50 chars: %s...)",
			index, c.URI, truncateString(c.Text, 50))
	case mcp.BlobResourceContents:
		log.Printf("[%d] Binary resource: %s (size: %d bytes)",
			index, c.URI, len(c.Blob))
	default:
		log.Printf("[%d] Unknown resource type", index)
	}
}

// handleResources manages resource-related operations including listing and reading resources
func handleResources(ctx context.Context, client *mcp.Client) error {
	log.Printf("===== List resources =====")
	resources, err := client.ListResources(ctx, &mcp.ListResourcesRequest{})
	if err != nil {
		return fmt.Errorf("list resources error: %v", err)
	}

	log.Printf("Found %d resources:", len(resources.Resources))
	for _, resource := range resources.Resources {
		log.Printf("- %s: %s (%s)", resource.Name, resource.Description, resource.URI)
	}

	if len(resources.Resources) == 0 {
		return nil
	}

	// Read the first resource
	log.Printf("===== Read resource: %s =====", resources.Resources[0].Name)
	readResourceReq := &mcp.ReadResourceRequest{}
	readResourceReq.Params.URI = resources.Resources[0].URI
	resourceContent, err := client.ReadResource(ctx, readResourceReq)
	if err != nil {
		return fmt.Errorf("read resource error: %v", err)
	}

	log.Printf("Successfully read resource, content item count: %d", len(resourceContent.Contents))
	for i, content := range resourceContent.Contents {
		printResourceContent(content, i)
	}

	return nil
}

// printPromptContent formats and prints different types of prompt content with role information
func printPromptContent(content interface{}, index int, role mcp.Role) {
	switch c := content.(type) {
	case mcp.TextContent:
		log.Printf("[%d] %s message (Text): %s", index, role, truncateString(c.Text, 50))
	case mcp.ImageContent:
		log.Printf(
			"[%d] %s message (Image): MIME=%s, DataLen=%d",
			index, role, c.MimeType, len(c.Data),
		)
	case mcp.AudioContent:
		log.Printf(
			"[%d] %s message (Audio): MIME=%s, DataLen=%d",
			index, role, c.MimeType, len(c.Data),
		)
	case mcp.EmbeddedResource:
		var resourceURI string
		if textResource, ok := c.Resource.(mcp.TextResourceContents); ok {
			resourceURI = textResource.URI
		} else if blobResource, ok := c.Resource.(mcp.BlobResourceContents); ok {
			resourceURI = blobResource.URI
		}
		log.Printf("[%d] %s message (Resource): URI=%s", index, role, resourceURI)
	default:
		log.Printf("[%d] %s message (unknown content type: %T)", index, role, c)
	}
}

// handlePrompts manages prompt-related operations including listing and retrieving prompts
func handlePrompts(ctx context.Context, client *mcp.Client) error {
	log.Printf("===== List prompts =====")
	prompts, err := client.ListPrompts(ctx, &mcp.ListPromptsRequest{})
	if err != nil {
		return fmt.Errorf("list prompts error: %v", err)
	}

	log.Printf("Found %d prompts:", len(prompts.Prompts))
	for _, prompt := range prompts.Prompts {
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

	if len(prompts.Prompts) == 0 {
		return nil
	}

	// Get the first prompt
	arguments := make(map[string]string)
	for _, arg := range prompts.Prompts[0].Arguments {
		if arg.Required {
			arguments[arg.Name] = "example value"
		}
	}

	log.Printf("===== Get prompt: %s =====", prompts.Prompts[0].Name)
	getPromptReq := &mcp.GetPromptRequest{}
	getPromptReq.Params.Name = prompts.Prompts[0].Name
	getPromptReq.Params.Arguments = arguments
	promptContent, err := client.GetPrompt(ctx, getPromptReq)
	if err != nil {
		return fmt.Errorf("get prompt error: %v", err)
	}

	log.Printf("Successfully got prompt, message count: %d", len(promptContent.Messages))
	if promptContent.Description != "" {
		log.Printf("Prompt description: %s", promptContent.Description)
	}

	for i, msg := range promptContent.Messages {
		printPromptContent(msg.Content, i, msg.Role)
	}

	return nil
}

// handleCompletions demonstrates completion functionality for prompts and resources
func handleCompletions(ctx context.Context, client *mcp.Client) error {
	log.Printf("===== Prompt completion =====")

	// Test completion for prompt arguments (code_review -> language)
	promptCompletionReq := &mcp.CompleteCompletionRequest{}
	promptCompletionReq.Params.Ref.Type = "ref/prompt"
	promptCompletionReq.Params.Ref.Name = "code_review"
	promptCompletionReq.Params.Argument.Name = "language"
	promptCompletionReq.Params.Argument.Value = "p"

	promptCompletionResult, err := client.CompleteCompletion(ctx, promptCompletionReq)
	if err != nil {
		log.Printf("Prompt completion error: %v", err)
	} else {
		log.Printf("Prompt completion for '%s' with '%s': found %d suggestions",
			promptCompletionReq.Params.Ref.Name,
			promptCompletionReq.Params.Argument.Value,
			len(promptCompletionResult.Completion.Values))
		for i, value := range promptCompletionResult.Completion.Values {
			log.Printf("  [%d] %s", i, value)
		}
	}

	log.Printf("===== Resource completion =====")
	// Test completion for resource (resource://example/completion)
	resourceCompletionReq := &mcp.CompleteCompletionRequest{}
	resourceCompletionReq.Params.Ref.Type = "ref/resource"
	resourceCompletionReq.Params.Ref.URI = "resource://example/completion"
	resourceCompletionReq.Params.Argument.Name = "query"
	resourceCompletionReq.Params.Argument.Value = "get-context"
	resourceCompletionReq.Params.Context.Arguments = map[string]string{"context": "example context"}

	resourceCompletionResult, err := client.CompleteCompletion(ctx, resourceCompletionReq)
	if err != nil {
		log.Printf("Resource completion error: %v", err)
	} else {
		log.Printf("Resource completion from '%s' for '%s' with '%s': found %d suggestions",
			resourceCompletionReq.Params.Ref.URI,
			resourceCompletionReq.Params.Argument.Name,
			resourceCompletionReq.Params.Argument.Value,
			len(resourceCompletionResult.Completion.Values))
		for i, value := range resourceCompletionResult.Completion.Values {
			log.Printf("  [%d] %s", i, value)
		}
	}

	log.Printf("===== Resource template completion =====")

	// Test completion for resource template (file-template -> path)
	resourceTemplateCompletionReq := &mcp.CompleteCompletionRequest{}
	resourceTemplateCompletionReq.Params.Ref.Type = "ref/resource"
	resourceTemplateCompletionReq.Params.Ref.URI = "file://example_file"
	resourceTemplateCompletionReq.Params.Argument.Name = "keyword"
	resourceTemplateCompletionReq.Params.Argument.Value = "completion"

	resourceTemplateCompletionResult, err := client.CompleteCompletion(ctx, resourceTemplateCompletionReq)
	if err != nil {
		log.Printf("Resource template completion error: %v", err)
	} else {
		log.Printf("Resource template completion from '%s' for '%s' with '%s': found %d suggestions",
			resourceTemplateCompletionReq.Params.Ref.URI,
			resourceTemplateCompletionReq.Params.Argument.Name,
			resourceTemplateCompletionReq.Params.Argument.Value,
			len(resourceTemplateCompletionResult.Completion.Values))
		for i, value := range resourceTemplateCompletionResult.Completion.Values {
			log.Printf("  [%d] %s", i, value)
		}
	}

	return nil
}

// handleTools manages tool-related operations including listing and calling tools
func handleTools(ctx context.Context, client *mcp.Client) error {
	log.Printf("===== List tools =====")
	tools, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("list tools error: %v", err)
	}

	log.Printf("Found %d tools:", len(tools.Tools))
	for _, tool := range tools.Tools {
		log.Printf("- %s: %s", tool.Name, tool.Description)
	}

	// Call the greet tool
	callToolReq := &mcp.CallToolRequest{}
	callToolReq.Params.Name = "greet"
	callToolReq.Params.Arguments = map[string]interface{}{"name": "MCP User"}
	callToolResult, err := client.CallTool(ctx, callToolReq)
	if err != nil {
		return fmt.Errorf("call tool error: %v", err)
	}

	log.Printf("Successfully called tool, message count: %d", len(callToolResult.Content))
	for i, content := range callToolResult.Content {
		switch c := content.(type) {
		case mcp.TextContent:
			log.Printf("[%d] Text content: %s", i, truncateString(c.Text, 50))
		default:
			log.Printf("[%d] Unknown content type", i)
		}
	}

	return nil
}

// truncateString shortens a string to the specified maximum length and adds ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
