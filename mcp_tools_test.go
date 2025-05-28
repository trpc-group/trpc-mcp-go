// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockTool is a mock tool implementation for testing
type MockTool struct {
	name        string
	description string
	arguments   map[string]interface{}
	executeFunc func(ctx context.Context, args map[string]interface{}) (*CallToolResult, error)
}

func (t *MockTool) Name() string {
	return t.name
}

func (t *MockTool) Description() string {
	return t.description
}

func (t *MockTool) GetArgumentsSchema() map[string]interface{} {
	return t.arguments
}

func (t *MockTool) Execute(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	if t.executeFunc != nil {
		return t.executeFunc(ctx, args)
	}
	return &CallToolResult{
		Content: []Content{
			NewTextContent("Mock tool execution result"),
		},
	}, nil
}

func TestNewTextContent(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		text     string
		expected TextContent
	}{
		{
			name: "Empty text",
			text: "",
			expected: TextContent{
				Type: "text",
				Text: "",
			},
		},
		{
			name: "Simple text",
			text: "Hello, world!",
			expected: TextContent{
				Type: "text",
				Text: "Hello, world!",
			},
		},
		{
			name: "Multiline text",
			text: "Line 1\nLine 2\nLine 3",
			expected: TextContent{
				Type: "text",
				Text: "Line 1\nLine 2\nLine 3",
			},
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := NewTextContent(tc.text)
			assert.Equal(t, tc.expected.Type, result.Type)
			assert.Equal(t, tc.expected.Text, result.Text)
			assert.Nil(t, result.Annotations)
		})
	}
}

func TestNewImageContent(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		data     string
		mimeType string
		expected ImageContent
	}{
		{
			name:     "Empty image data",
			data:     "",
			mimeType: "image/png",
			expected: ImageContent{
				Type:     "image",
				Data:     "",
				MimeType: "image/png",
			},
		},
		{
			name:     "JPEG image",
			data:     "base64data...",
			mimeType: "image/jpeg",
			expected: ImageContent{
				Type:     "image",
				Data:     "base64data...",
				MimeType: "image/jpeg",
			},
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := NewImageContent(tc.data, tc.mimeType)
			assert.Equal(t, tc.expected.Type, result.Type)
			assert.Equal(t, tc.expected.Data, result.Data)
			assert.Equal(t, tc.expected.MimeType, result.MimeType)
			assert.Nil(t, result.Annotations)
		})
	}
}

func TestMockTool(t *testing.T) {
	// Create mock tool
	mockTool := &MockTool{
		name:        "mock-tool",
		description: "A mock tool for testing",
		arguments: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"arg1": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	// Test basic properties
	assert.Equal(t, "mock-tool", mockTool.Name())
	assert.Equal(t, "A mock tool for testing", mockTool.Description())
	assert.NotNil(t, mockTool.GetArgumentsSchema())

	// Test execution function
	ctx := context.Background()
	result, err := mockTool.Execute(ctx, map[string]interface{}{"arg1": "test"})
	assert.NoError(t, err)
	assert.Len(t, result.Content, 1)

	// Use type assertion to check content type
	textContent, ok := result.Content[0].(TextContent)
	assert.True(t, ok, "Content should be TextContent type")
	assert.Equal(t, "text", textContent.Type)
	assert.Equal(t, "Mock tool execution result", textContent.Text)

	// Test custom execution function
	customResult := &CallToolResult{
		Content: []Content{
			NewTextContent("Custom result"),
		},
	}
	mockTool.executeFunc = func(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
		return customResult, nil
	}

	result, err = mockTool.Execute(ctx, map[string]interface{}{"arg1": "test"})
	assert.NoError(t, err)
	assert.Equal(t, customResult, result)
}
