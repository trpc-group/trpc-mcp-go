// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// NewTypedToolHandler creates a tool handler that automatically marshals/unmarshals typed input and output.
// It provides type safety for tool handlers while maintaining compatibility with the MCP protocol.
//
// Parameters:
//   - I: Input type (struct that will be unmarshaled from CallToolRequest.Params.Arguments)
//   - O: Output type (struct that will be marshaled to CallToolResult.StructuredContent)
//   - handler: Typed handler function that processes the unmarshaled input and returns typed output
//
// The handler automatically:
//   - Unmarshals CallToolRequest.Params.Arguments into type I
//   - Calls the typed handler with the unmarshaled input
//   - Marshals the typed output to CallToolResult.StructuredContent
//   - Handles binding errors gracefully
//
// Example usage:
//
//	type WeatherInput struct {
//	    Location string `json:"location" jsonschema:"required,description=Location to get weather for"`
//	    Units    string `json:"units,omitempty" jsonschema:"description=Temperature units,enum=celsius,enum=fahrenheit"`
//	}
//
//	type WeatherOutput struct {
//	    Temperature float64 `json:"temperature"`
//	    Description string  `json:"description"`
//	    Units       string  `json:"units"`
//	}
//
//	handler := NewTypedToolHandler(func(ctx context.Context, req *CallToolRequest, input WeatherInput) (WeatherOutput, error) {
//	    // Implementation here
//	    return WeatherOutput{Temperature: 25.0, Description: "Sunny", Units: input.Units}, nil
//	})
func NewTypedToolHandler[I any, O any](handler TypedToolHandler[I, O]) func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
	return func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		// Unmarshal arguments into typed input
		var input I
		if err := bindArguments(req.Params.Arguments, &input); err != nil {
			return NewErrorResult(fmt.Sprintf("Failed to bind arguments: %v", err)), nil
		}

		// Call the typed handler
		output, err := handler(ctx, req, input)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("Tool execution failed: %v", err)), nil
		}

		// Create result with structured content and backwards-compatible text
		// Convert to JSON string for backward compatibility
		var fallbackText string
		if jsonBytes, err := json.Marshal(output); err != nil {
			fallbackText = fmt.Sprintf("Error serializing structured content: %v", err)
		} else {
			fallbackText = string(jsonBytes)
		}

		return &CallToolResult{
			Content:           []Content{NewTextContent(fallbackText)},
			StructuredContent: output,
		}, nil
	}
}

// NewStructuredToolHandler creates a tool handler that only handles structured output without input validation.
// This is useful when you want to use the builder pattern for input schema but still return structured output.
//
// Parameters:
//   - O: Output type (struct that will be marshaled to CallToolResult.StructuredContent)
//   - handler: Handler function that returns typed output
//
// Example usage:
//
//	type WeatherOutput struct {
//	    Temperature float64 `json:"temperature"`
//	    Description string  `json:"description"`
//	}
//
//	handler := NewStructuredToolHandler(func(ctx context.Context, req *CallToolRequest) (WeatherOutput, error) {
//	    location := req.Params.Arguments["location"].(string)
//	    // Implementation here
//	    return WeatherOutput{Temperature: 25.0, Description: "Sunny"}, nil
//	})
func NewStructuredToolHandler[O any](handler func(ctx context.Context, req *CallToolRequest) (O, error)) func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
	return func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		// Call the handler
		output, err := handler(ctx, req)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("Tool execution failed: %v", err)), nil
		}

		// Create result with structured content and backwards-compatible text
		// Convert to JSON string for backward compatibility
		var fallbackText string
		if jsonBytes, err := json.Marshal(output); err != nil {
			fallbackText = fmt.Sprintf("Error serializing structured content: %v", err)
		} else {
			fallbackText = string(jsonBytes)
		}

		return &CallToolResult{
			Content:           []Content{NewTextContent(fallbackText)},
			StructuredContent: output,
		}, nil
	}
}

// bindArguments unmarshals a map[string]any into a typed struct
func bindArguments(arguments map[string]any, target any) error {
	if arguments == nil {
		return nil
	}

	// Convert map to JSON then unmarshal to target type
	jsonData, err := json.Marshal(arguments)
	if err != nil {
		return fmt.Errorf("failed to marshal arguments: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal arguments into target type: %w", err)
	}

	return nil
}
