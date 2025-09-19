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
	"strings"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// MockSamplingHandler implements a simple sampling handler for demonstration
type MockSamplingHandler struct{}

func (h *MockSamplingHandler) CreateMessage(ctx context.Context, params *mcp.CreateMessageParams) (*mcp.CreateMessageResult, error) {
	log.Printf("[Client] Received sampling request with %d messages", len(params.Messages))

	if len(params.Messages) == 0 {
		return &mcp.CreateMessageResult{
			Model: "mock-model-v1",
			SamplingMessage: mcp.SamplingMessage{
				Role: mcp.RoleAssistant,
				Content: []mcp.TextContent{{
					Type: mcp.ContentTypeText,
					Text: "No messages provided",
				}},
			},
		}, nil
	}

	// Get the user's prompt from the first message
	userMessage := params.Messages[0]
	var userText string

	// Enhanced content parsing
	switch content := userMessage.Content.(type) {
	case []mcp.TextContent:
		if len(content) > 0 {
			userText = content[0].Text
		}
	case mcp.TextContent:
		userText = content.Text
	case []interface{}:
		// Handle JSON-deserialized content
		if len(content) > 0 {
			if item, ok := content[0].(map[string]interface{}); ok {
				if text, exists := item["text"]; exists {
					if textStr, ok := text.(string); ok {
						userText = textStr
					}
				}
			}
		}
		if userText == "" {
			userText = "Could not parse interface{} content"
		}
	case []map[string]interface{}:
		// Handle another JSON structure
		if len(content) > 0 {
			if text, exists := content[0]["text"]; exists {
				if textStr, ok := text.(string); ok {
					userText = textStr
				}
			}
		}
		if userText == "" {
			userText = "Could not parse map content"
		}
	default:
		log.Printf("[Client] Unknown content type: %T, value: %+v", content, content)
		userText = fmt.Sprintf("Unknown content type: %T", content)
	}

	log.Printf("[Client] Processing prompt: %q", userText)

	// Simple mock responses based on prompt content
	var response string
	switch {
	case contains(userText, "capital", "France"):
		response = "The capital of France is Paris."
	case contains(userText, "15.5", "3.2", "calculate"):
		response = "15.5 × 3.2 = 49.6. This calculation multiplies 15.5 by 3.2 to get 49.6."
	default:
		response = fmt.Sprintf("I received your message: %s. This is a mock response from the client-side sampling handler.", userText)
	}

	return &mcp.CreateMessageResult{
		Model: "mock-model-v1",
		SamplingMessage: mcp.SamplingMessage{
			Role: mcp.RoleAssistant,
			Content: []mcp.TextContent{{
				Type: mcp.ContentTypeText,
				Text: response,
			}},
		},
	}, nil
}

// Helper function to check if text contains any of the given substrings (case insensitive)
func contains(text string, substrings ...string) bool {
	lowerText := strings.ToLower(text)
	for _, substr := range substrings {
		if strings.Contains(lowerText, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}

func main() {
	log.Println("Starting Sampling Demo Client...")

	ctx := context.Background()

	// Initialize client
	client, err := initializeClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	defer client.Close()

	// Demonstrate sampling functionality
	if err := demonstrateSamplingTools(ctx, client); err != nil {
		log.Fatalf("Demo failed: %v", err)
	}

	log.Println("✅ Sampling demo completed successfully!")
}

func initializeClient(ctx context.Context) (*mcp.Client, error) {
	log.Println("===== Initialize Client =====")

	serverURL := "http://localhost:3002/mcp"

	// Create client with sampling handler
	mcpClient, err := mcp.NewClient(
		serverURL,
		mcp.Implementation{
			Name:    "Sampling-Demo-Client",
			Version: "1.0.0",
		},
		mcp.WithSampling(&MockSamplingHandler{}), // Add sampling handler
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	log.Printf("Server: %s %s", initResp.ServerInfo.Name, initResp.ServerInfo.Version)

	// Only print session ID for HTTP clients
	if sessionID := mcpClient.GetSessionID(); sessionID != "" {
		log.Printf("Session ID: %s", sessionID)
	} else {
		log.Println("Client Type: stdio (no session)")
	}

	return mcpClient, nil
}

func demonstrateSamplingTools(ctx context.Context, client *mcp.Client) error {
	log.Println("===== List Available Tools =====")

	listToolsResp, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	log.Printf("Found %d tools:", len(listToolsResp.Tools))
	for _, tool := range listToolsResp.Tools {
		log.Printf("  • %s: %s", tool.Name, tool.Description)

		// Show the generated schemas
		if tool.InputSchema != nil {
			inputJSON, _ := json.MarshalIndent(tool.InputSchema, "    ", "  ")
			log.Printf("    Input Schema: %s", string(inputJSON))
		}
		if tool.OutputSchema != nil {
			outputJSON, _ := json.MarshalIndent(tool.OutputSchema, "    ", "  ")
			log.Printf("    Output Schema: %s", string(outputJSON))
		}
		log.Println("")
	}

	// Demo: Trigger Sampling Tool
	log.Println("===== Demo: Trigger Sampling Tool =====")
	log.Println("Calling trigger_sampling tool to demonstrate server→client sampling...")

	samplingResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "trigger_sampling",
			Arguments: map[string]any{
				"prompt": "What is the capital of France? Please answer in one sentence.",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("trigger_sampling tool failed: %w", err)
	}

	log.Printf("✅ Sampling result received:")
	if samplingResult.StructuredContent != nil {
		structuredJSON, _ := json.MarshalIndent(samplingResult.StructuredContent, "  ", "  ")
		log.Printf("  Structured Content: %s", string(structuredJSON))
	}

	// Demo another sampling call with a different prompt
	log.Println("\n===== Demo 2: Another Sampling Call =====")
	log.Println("Calling trigger_sampling with a math question...")

	mathResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "trigger_sampling",
			Arguments: map[string]any{
				"prompt": "Calculate 15.5 * 3.2 and explain the calculation.",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("second trigger_sampling call failed: %w", err)
	}

	log.Printf("✅ Second sampling result received:")
	if mathResult.StructuredContent != nil {
		structuredJSON, _ := json.MarshalIndent(mathResult.StructuredContent, "  ", "  ")
		log.Printf("  Structured Content: %s", string(structuredJSON))
	}

	return nil
}
