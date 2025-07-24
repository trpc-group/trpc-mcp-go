// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package sampling

import (
	"context"
	"fmt"
)

type contextKey string

const SamplingSenderKey contextKey = "sampling_sender"

// GetSamplingSender - Get the Sampling sender from the context
func GetSamplingSender(ctx context.Context) (SamplingSender, bool) {
	sender, ok := ctx.Value(SamplingSenderKey).(SamplingSender)
	return sender, ok
}

// HandleAnalysisWithSampling - Example of using sampling in a tool processor
func HandleAnalysisWithSampling(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
	// Get the sampling transmitter
	samplingSender, hasSampling := GetSamplingSender(ctx)
	if !hasSampling {
		return NewErrorResult("Sampling not available"),
			fmt.Errorf("sampling not supported")
	}

	// Extracting data for analysis
	data, ok := req.Params.Arguments["data"].(string)
	if !ok {
		return NewErrorResult("Invalid data parameter"),
			fmt.Errorf("data parameter required")
	}

	// Prepare sampling request
	samplingReq := &SamplingCreateMessageRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "sampling/createMessage",
		Params: SamplingCreateMessageParams{
			Messages: []SamplingMessage{
				{
					Role: "user",
					Content: TextContent{
						Type: "text",
						Text: fmt.Sprintf("Please analyze the following data and provide insights: %s", data),
					},
				},
			},
			ModelPreferences: &ModelPreferences{
				Hints:                []string{"claude-3-sonnet", "gpt-4"},
				IntelligencePriority: floatPtr(0.8),
				SpeedPriority:        floatPtr(0.5),
			},
			MaxTokens:   intPtr(1000),
			Temperature: floatPtr(0.7),
		},
	}

	// Send a sampling request
	result, err := samplingSender.SendSamplingRequest(ctx, samplingReq)
	if err != nil {
		return NewErrorResult("Failed to get AI analysis"), err
	}

	// Process result
	if textContent, ok := result.Content.(TextContent); ok {
		return NewTextResult(fmt.Sprintf("AI分析结果: %s", textContent.Text)), nil
	}

	return NewErrorResult("Invalid response format"),
		fmt.Errorf("unexpected content type")
}
