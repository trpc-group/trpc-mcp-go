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
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

type TriggerSamplingInput struct {
	Prompt string `json:"prompt" jsonschema:"required,description=User prompt for sampling"`
}
type TriggerSamplingOutput struct {
	Model   string `json:"model"`
	Message string `json:"message"`
}

func main() {
	log.Println("Starting Sampling Demo Server...")

	server := mcp.NewServer(
		"Sampling-Demo-Server",
		"1.0.0",
		mcp.WithServerAddress(":3002"),
		mcp.WithServerPath("/mcp"),
		mcp.WithSamplingEnabled(true),
	)

	// Register a tool
	// When the client calls it, the server triggers sampling/createMessage under the session ctx
	tool := mcp.NewTool(
		"trigger_sampling",
		mcp.WithDescription("Trigger server→client sampling using current session"),
		mcp.WithInputStruct[TriggerSamplingInput](),
		mcp.WithOutputStruct[TriggerSamplingOutput](),
	)
	handler := mcp.NewTypedToolHandler(func(ctx context.Context, req *mcp.CallToolRequest, in TriggerSamplingInput) (TriggerSamplingOutput, error) {
		log.Printf("[Server] tool called, will request sampling with prompt: %q", in.Prompt)

		params := &mcp.CreateMessageParams{
			Messages: []mcp.SamplingMessage{
				{Role: mcp.RoleUser, Content: []mcp.TextContent{{Type: mcp.ContentTypeText, Text: in.Prompt}}},
			},
			// These build parameters are optional.
			MaxTokens:   128,
			Temperature: 0.7,
		}

		// Use the "ctx of the current request" to initiate sampling (ctx contains the session)
		// and you can route it to the correct client.
		cres, err := server.RequestSampling(ctx, params)
		if err != nil {
			return TriggerSamplingOutput{}, fmt.Errorf("RequestSampling failed: %w", err)
		}

		// Extract text from the response with better handling
		text := extractTextFromContent(cres.SamplingMessage.Content)

		log.Printf("[Server] sampling done. model=%s, text=%q", cres.Model, text)
		return TriggerSamplingOutput{
			Model:   cres.Model,
			Message: text,
		}, nil
	})
	server.RegisterTool(tool, handler)

	// Graceful exit
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log.Println("Shutting down server...")
		os.Exit(0)
	}()

	log.Println("Server listening on http://localhost:3002/mcp")
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}

	_ = time.Second
}

// extractTextFromContent extracts text from various content types
func extractTextFromContent(content interface{}) string {
	switch c := content.(type) {
	case []mcp.TextContent:
		if len(c) > 0 {
			return c[0].Text
		}
		return "empty text content array"
	case mcp.TextContent:
		return c.Text
	case string:
		return c
	case []interface{}:
		// Handle JSON-deserialized content
		if len(c) > 0 {
			if item, ok := c[0].(map[string]interface{}); ok {
				if text, exists := item["text"]; exists {
					if textStr, ok := text.(string); ok {
						return textStr
					}
				}
			}
		}
		log.Printf("[Server] Empty or invalid []interface{} content: %+v", c)
		return "empty interface array"
	case []map[string]interface{}:
		// Handle another possible JSON structure
		if len(c) > 0 {
			if text, exists := c[0]["text"]; exists {
				if textStr, ok := text.(string); ok {
					return textStr
				}
			}
		}
		log.Printf("[Server] Empty or invalid []map content: %+v", c)
		return "empty map array"
	default:
		log.Printf("[Server] Unknown content type: %T, value: %+v", content, content)
		return fmt.Sprintf("unknown content type: %T", content)
	}
}
