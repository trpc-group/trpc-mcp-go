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
	log.Printf("Starting annotations example server...")

	// Create server using the new API style:
	// - First two required parameters: server name and version
	// - WithServerAddress sets the address to listen on (default: "localhost:3000")
	// - WithServerPath sets the API path prefix
	// - WithServerLogger injects logger at the server level
	mcpServer := mcp.NewServer(
		"Annotations-Example-Server",
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

	// Register delete file tool to demonstrate destructive operations.
	deleteFileTool := NewDeleteFileTool()
	deleteFileHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := ""
		if pathArg, ok := req.Params.Arguments["path"]; ok {
			if pathStr, ok := pathArg.(string); ok {
				path = pathStr
			}
		}

		// This is a demonstration - we don't actually delete files
		message := fmt.Sprintf("Would delete file: %s (simulation only)", path)
		return mcp.NewTextResult(message), nil
	}
	mcpServer.RegisterTool(deleteFileTool, deleteFileHandler)
	log.Printf("Registered delete file tool: delete_file")

	// Register calculator tool to demonstrate pure computation.
	calcTool := NewCalculateTool()
	calcHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		expression := ""
		if exprArg, ok := req.Params.Arguments["expression"]; ok {
			if exprStr, ok := exprArg.(string); ok {
				expression = exprStr
			}
		}

		// Simple demonstration - just echo the expression
		message := fmt.Sprintf("Expression: %s = [calculated result would appear here]", expression)
		return mcp.NewTextResult(message), nil
	}
	mcpServer.RegisterTool(calcTool, calcHandler)
	log.Printf("Registered calculator tool: calculate")

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

// NewGreetTool creates a simple greeting tool with annotations.
func NewGreetTool() *mcp.Tool {
	return mcp.NewTool("greet",
		mcp.WithDescription("A simple greeting tool that returns a greeting message."),
		mcp.WithString("name",
			mcp.Description("The name to greet."),
		),
		mcp.WithToolAnnotations(&mcp.ToolAnnotations{
			Title:           "Simple Greeter",
			ReadOnlyHint:    mcp.BoolPtr(true),  // Only generates text, doesn't modify anything
			DestructiveHint: mcp.BoolPtr(false), // Safe operation
			IdempotentHint:  mcp.BoolPtr(true),  // Same input always gives same output
			OpenWorldHint:   mcp.BoolPtr(false), // No external dependencies
		}),
	)
}

// NewAdvancedGreetTool Add a more advanced tool example with different annotations.
func NewAdvancedGreetTool() *mcp.Tool {
	return mcp.NewTool("advanced-greet",
		mcp.WithDescription("An enhanced greeting tool supporting multiple output formats."),
		mcp.WithString("name", mcp.Description("The name to greet.")),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, or html."),
			mcp.Default("text")),
		mcp.WithToolAnnotations(&mcp.ToolAnnotations{
			Title:           "Advanced Formatter",
			ReadOnlyHint:    mcp.BoolPtr(true),  // Read-only text generation
			DestructiveHint: mcp.BoolPtr(false), // Non-destructive
			IdempotentHint:  mcp.BoolPtr(true),  // Deterministic output
			OpenWorldHint:   mcp.BoolPtr(false), // Self-contained logic
		}),
	)
}

// NewDeleteFileTool creates a destructive file deletion tool.
func NewDeleteFileTool() *mcp.Tool {
	return mcp.NewTool("delete_file",
		mcp.WithDescription("Delete a file from the filesystem (destructive operation)."),
		mcp.WithString("path", mcp.Description("The file path to delete.")),
		mcp.WithToolAnnotations(&mcp.ToolAnnotations{
			Title:           "File Deleter",
			ReadOnlyHint:    mcp.BoolPtr(false), // Modifies the filesystem
			DestructiveHint: mcp.BoolPtr(true),  // Permanently deletes data
			IdempotentHint:  mcp.BoolPtr(false), // Multiple calls have different effects
			OpenWorldHint:   mcp.BoolPtr(true),  // Interacts with filesystem
		}),
	)
}

// NewCalculateTool creates a non-destructive calculation tool.
func NewCalculateTool() *mcp.Tool {
	return mcp.NewTool("calculate",
		mcp.WithDescription("Perform mathematical calculations."),
		mcp.WithString("expression", mcp.Description("Mathematical expression to evaluate (e.g., '2+2').")),
		mcp.WithToolAnnotations(&mcp.ToolAnnotations{
			Title:           "Math Calculator",
			ReadOnlyHint:    mcp.BoolPtr(true),  // Pure computation
			DestructiveHint: mcp.BoolPtr(false), // No side effects
			IdempotentHint:  mcp.BoolPtr(true),  // Same expression always yields same result
			OpenWorldHint:   mcp.BoolPtr(false), // Self-contained math operations
		}),
	)
}
