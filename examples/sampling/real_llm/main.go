// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"fmt"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// 1. Use OpenAI and Claude
	realHandler := mcp.NewRealLLMHandler(
		"your-openai-api-key",
		"your-claude-api-key",
		&mcp.SamplingClientConfig{
			DefaultModel:        "gpt-3.5-turbo",
			MaxTokensPerRequest: 4000,
			ModelMappings: map[string]string{
				"claude-3-sonnet": "claude-3-sonnet-20240229",
				"gpt-4":           "gpt-4",
			},
			TimeoutSeconds: 120,
		},
	)

	// 2. Use local model Ollama
	ollamaHandler := mcp.NewOllamaHandler(
		"http://localhost:11434",
		&mcp.SamplingClientConfig{
			DefaultModel:        "llama2",
			MaxTokensPerRequest: 4000,
			ModelMappings: map[string]string{
				"claude-3-sonnet": "llama2:13b",
				"gpt-4":           "llama2:13b",
				"gpt-3.5-turbo":   "llama2:7b",
			},
			TimeoutSeconds: 180,
		},
	)

	//Use the real handler when creating the client
	client, err := mcp.NewClient(
		"http://localhost:3000/mcp",
		mcp.Implementation{
			Name:    "Real-LLM-Client",
			Version: "1.0.0",
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
		mcp.WithSamplingHandler(realHandler),
	)
	if err != nil {
		panic(err)
	}

	// Or use Ollama Handler
	_ = ollamaHandler

	fmt.Println("Real LLM Sampling Handler initialized")
	defer client.Close()
}
