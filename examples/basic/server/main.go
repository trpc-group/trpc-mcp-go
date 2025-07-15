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
	"syscall"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Print startup message.
	log.Printf("Starting basic example server...")

	// Create server using the new API style:
	// - First two required parameters: server name and version
	// - WithServerAddress sets the address to listen on (default: "localhost:3000")
	// - WithServerPath sets the API path prefix
	// - WithServerLogger injects logger at the server level
	mcpServer := mcp.NewServer(
		"Basic-Example-Server",
		"0.1.0",
		mcp.WithServerAddress(":3000"),
		mcp.WithServerPath("/mcp"),
		mcp.WithServerLogger(mcp.GetDefaultLogger()),
	)

	// Register basic greet tool.
	greetTool := NewGreetTool()
	greetHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Check if the context is cancelled.
		select {
		case <-ctx.Done():
			return mcp.NewErrorResult("Request cancelled"), ctx.Err()
		default:
			// Continue execution.
		}

		// Extract name parameter.
		name := "World"
		if nameArg, ok := req.Params.Arguments["name"]; ok {
			if nameStr, ok := nameArg.(string); ok && nameStr != "" {
				name = nameStr
			}
		}

		// Create greeting message.
		greeting := fmt.Sprintf("Hello, %s!", name)

		// Create tool result.
		return mcp.NewTextResult(greeting), nil
	}

	mcpServer.RegisterTool(greetTool, greetHandler)
	log.Printf("Registered basic greet tool: greet")

	// Register advanced greet tool.
	advancedGreetTool := NewAdvancedGreetTool()
	advancedGreetHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract parameters.
		name := "World"
		if nameArg, ok := req.Params.Arguments["name"]; ok {
			if nameStr, ok := nameArg.(string); ok && nameStr != "" {
				name = nameStr
			}
		}

		format := "text"
		if formatArg, ok := req.Params.Arguments["format"]; ok {
			if formatStr, ok := formatArg.(string); ok && formatStr != "" {
				format = formatStr
			}
		}

		// Example: if name is "error", return an error result.
		if name == "error" {
			return mcp.NewErrorResult(fmt.Sprintf("Cannot greet '%s': name not allowed.", name)), nil
		}

		// Return different content types based on format.
		switch format {
		case "json":
			// JSON format is no longer supported, fallback to text.
			jsonMessage := fmt.Sprintf(
				"JSON format: {\"greeting\":\"Hello, %s!\",\"timestamp\":\"2025-05-14T12:00:00Z\"}",
				name,
			)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(jsonMessage),
				},
			}, nil
		case "html":
			// HTML format is no longer supported, fallback to text.
			htmlContent := fmt.Sprintf(
				"<h1>Greeting</h1><p>Hello, <strong>%s</strong>!</p>",
				name,
			)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(htmlContent),
				},
			}, nil
		default:
			// Default: return plain text.
			return mcp.NewTextResult(fmt.Sprintf("Hello, %s!", name)), nil
		}
	}

	mcpServer.RegisterTool(advancedGreetTool, advancedGreetHandler)
	log.Printf("Registered advanced greet tool: advanced-greet")

	// Set up a graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server (run in goroutine).
	go func() {
		log.Printf("MCP server started, listening on port 3000, path /mcp")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for termination signal.
	<-stop
	log.Printf("Shutting down server...")
}

// NewGreetTool creates a simple greeting tool.
func NewGreetTool() *mcp.Tool {
	return mcp.NewTool("greet",
		mcp.WithDescription("A simple greeting tool that returns a greeting message."),
		mcp.WithString("name",
			mcp.Description("The name to greet."),
		),
	)
}

// NewAdvancedGreetTool Add a more advanced tool example.
func NewAdvancedGreetTool() *mcp.Tool {
	return mcp.NewTool("advanced-greet",
		mcp.WithDescription("An enhanced greeting tool supporting multiple output formats."),
		mcp.WithString("name", mcp.Description("The name to greet.")),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, or html."),
			mcp.Default("text")),
	)
}
