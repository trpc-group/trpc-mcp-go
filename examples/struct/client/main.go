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

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	log.Println("Starting Struct-First Demo Client...")
	log.Println("This demo showcases the new struct-first features:")
	log.Println("  • Automatic schema generation from Go structs")
	log.Println("  • Type-safe tool handlers with validation")
	log.Println("  • Structured input/output with rich metadata")
	log.Println("")

	ctx := context.Background()

	// Initialize client
	client, err := initializeClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	defer client.Close()

	// Demonstrate struct-first tools
	if err := demonstrateStructFirstTools(ctx, client); err != nil {
		log.Fatalf("Demo failed: %v", err)
	}

	log.Println("✅ Struct-first demo completed successfully!")
}

func initializeClient(ctx context.Context) (*mcp.Client, error) {
	log.Println("===== Initialize Client =====")

	serverURL := "http://localhost:3002/mcp"
	mcpClient, err := mcp.NewClient(
		serverURL,
		mcp.Implementation{
			Name:    "Struct-First-Demo-Client",
			Version: "1.0.0",
		},
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
	log.Printf("Session ID: %s", mcpClient.GetSessionID())
	return mcpClient, nil
}

func demonstrateStructFirstTools(ctx context.Context, client *mcp.Client) error {
	log.Println("===== List Available Tools =====")

	listToolsResp, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	log.Printf("Found %d struct-first tools:", len(listToolsResp.Tools))
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

	// Demo 1: Weather Tool
	log.Println("===== Demo 1: Weather Tool =====")
	log.Println("Calling weather tool with structured input...")

	weatherResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_weather",
			Arguments: map[string]any{
				"location": "Beijing",
				"units":    "celsius",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("weather tool failed: %w", err)
	}

	log.Printf("✅ Weather result received:")
	if weatherResult.StructuredContent != nil {
		structuredJSON, _ := json.MarshalIndent(weatherResult.StructuredContent, "  ", "  ")
		log.Printf("  Structured Content: %s", string(structuredJSON))
	}

	// Demo 2: Calculator Tool
	log.Println("===== Demo 2: Calculator Tool =====")
	log.Println("Calling calculator tool with validation...")

	calcResult, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "calculator",
			Arguments: map[string]any{
				"operation": "multiply",
				"a":         15.5,
				"b":         3.2,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("calculator tool failed: %w", err)
	}

	log.Printf("✅ Calculator result received:")
	if calcResult.StructuredContent != nil {
		structuredJSON, _ := json.MarshalIndent(calcResult.StructuredContent, "  ", "  ")
		log.Printf("  Structured Content: %s", string(structuredJSON))
	}

	return nil
}
