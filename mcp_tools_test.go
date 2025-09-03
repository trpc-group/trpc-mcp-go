// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
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

func TestParseCallToolResult(t *testing.T) {
	testCases := []struct {
		name                 string
		inputJSON            string
		expectedContent      int
		expectedIsError      bool
		hasStructuredContent bool
		expectedStructured   map[string]interface{}
	}{
		{
			name: "Basic text content only",
			inputJSON: `{
				"content": [
					{"type": "text", "text": "Hello World"}
				]
			}`,
			expectedContent:      1,
			expectedIsError:      false,
			hasStructuredContent: false,
		},
		{
			name: "With structured content",
			inputJSON: `{
				"content": [
					{"type": "text", "text": "Tool executed successfully"}
				],
				"structuredContent": {
					"operation": "add",
					"a": 10,
					"b": 5,
					"result": 15,
					"message": "Addition completed successfully"
				}
			}`,
			expectedContent:      1,
			expectedIsError:      false,
			hasStructuredContent: true,
			expectedStructured: map[string]interface{}{
				"operation": "add",
				"a":         float64(10), // JSON unmarshals numbers as float64
				"b":         float64(5),
				"result":    float64(15),
				"message":   "Addition completed successfully",
			},
		},
		{
			name: "Error result with structured content",
			inputJSON: `{
				"content": [
					{"type": "text", "text": "Error occurred"}
				],
				"isError": true,
				"structuredContent": {
					"errorCode": "VALIDATION_FAILED",
					"details": {
						"field": "age",
						"message": "Age must be between 0 and 150"
					}
				}
			}`,
			expectedContent:      1,
			expectedIsError:      true,
			hasStructuredContent: true,
			expectedStructured: map[string]interface{}{
				"errorCode": "VALIDATION_FAILED",
				"details": map[string]interface{}{
					"field":   "age",
					"message": "Age must be between 0 and 150",
				},
			},
		},
		{
			name: "Complex structured content",
			inputJSON: `{
				"content": [
					{"type": "text", "text": "Weather data"}
				],
				"structuredContent": {
					"location": "Beijing",
					"temperature": 22.5,
					"units": "celsius",
					"timestamp": "2025-08-28T19:32:27+08:00",
					"conditions": ["partly_cloudy", "windy"],
					"forecast": {
						"tomorrow": {
							"high": 25.0,
							"low": 18.0
						}
					}
				}
			}`,
			expectedContent:      1,
			expectedIsError:      false,
			hasStructuredContent: true,
			expectedStructured: map[string]interface{}{
				"location":    "Beijing",
				"temperature": float64(22.5),
				"units":       "celsius",
				"timestamp":   "2025-08-28T19:32:27+08:00",
				"conditions":  []interface{}{"partly_cloudy", "windy"},
				"forecast": map[string]interface{}{
					"tomorrow": map[string]interface{}{
						"high": float64(25.0),
						"low":  float64(18.0),
					},
				},
			},
		},
		{
			name: "Empty structured content",
			inputJSON: `{
				"content": [
					{"type": "text", "text": "Result"}
				],
				"structuredContent": {}
			}`,
			expectedContent:      1,
			expectedIsError:      false,
			hasStructuredContent: true,
			expectedStructured:   map[string]interface{}{},
		},
		{
			name: "Null structured content",
			inputJSON: `{
				"content": [
					{"type": "text", "text": "Result"}
				],
				"structuredContent": null
			}`,
			expectedContent:      1,
			expectedIsError:      false,
			hasStructuredContent: false, // null means no structured content
			expectedStructured:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert JSON string to RawMessage
			rawMessage := json.RawMessage(tc.inputJSON)

			// Parse the result
			result, err := parseCallToolResult(&rawMessage)

			// Basic assertions
			assert.NoError(t, err, "parseCallToolResult should not return error")
			assert.NotNil(t, result, "result should not be nil")

			// Test content
			assert.Len(t, result.Content, tc.expectedContent, "content length should match")
			if tc.expectedContent > 0 {
				textContent, ok := result.Content[0].(TextContent)
				assert.True(t, ok, "first content should be TextContent")
				assert.Equal(t, "text", textContent.Type)
			}

			// Test isError flag
			assert.Equal(t, tc.expectedIsError, result.IsError, "isError flag should match")

			// Test structured content
			if tc.hasStructuredContent {
				assert.NotNil(t, result.StructuredContent, "StructuredContent should not be nil")
				if tc.expectedStructured != nil {
					assert.Equal(t, tc.expectedStructured, result.StructuredContent, "structured content should match")
				}
			} else {
				assert.Nil(t, result.StructuredContent, "StructuredContent should be nil when not present")
			}
		})
	}
}

func TestParseCallToolResult_Errors(t *testing.T) {
	errorCases := []struct {
		name      string
		inputJSON string
		errorMsg  string
	}{
		{
			name:      "Invalid JSON",
			inputJSON: `{"content": [}`,
			errorMsg:  "failed to unmarshal response",
		},
		{
			name: "Missing content",
			inputJSON: `{
				"isError": false
			}`,
			errorMsg: "content is missing",
		},
		{
			name: "Content not array",
			inputJSON: `{
				"content": "not an array"
			}`,
			errorMsg: "content is not an array",
		},
		{
			name: "Content item not object",
			inputJSON: `{
				"content": ["not an object"]
			}`,
			errorMsg: "content is not an object",
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			rawMessage := json.RawMessage(tc.inputJSON)
			result, err := parseCallToolResult(&rawMessage)

			assert.Error(t, err, "should return error")
			assert.Nil(t, result, "result should be nil on error")
			assert.Contains(t, err.Error(), tc.errorMsg, "error message should contain expected text")
		})
	}
}

func TestToolAnnotations(t *testing.T) {
	t.Run("WithToolAnnotations", func(t *testing.T) {
		annotations := &ToolAnnotations{
			Title:           "Test Tool",
			ReadOnlyHint:    BoolPtr(true),
			DestructiveHint: BoolPtr(false),
			IdempotentHint:  BoolPtr(true),
			OpenWorldHint:   BoolPtr(false),
		}

		tool := NewTool("test_tool",
			WithDescription("A test tool"),
			WithToolAnnotations(annotations),
		)

		assert.NotNil(t, tool.Annotations)
		assert.Equal(t, "Test Tool", tool.Annotations.Title)
		assert.Equal(t, true, *tool.Annotations.ReadOnlyHint)
		assert.Equal(t, false, *tool.Annotations.DestructiveHint)
		assert.Equal(t, true, *tool.Annotations.IdempotentHint)
		assert.Equal(t, false, *tool.Annotations.OpenWorldHint)
	})

	t.Run("BoolPtr utility", func(t *testing.T) {
		truePtr := BoolPtr(true)
		falsePtr := BoolPtr(false)

		assert.NotNil(t, truePtr)
		assert.NotNil(t, falsePtr)
		assert.Equal(t, true, *truePtr)
		assert.Equal(t, false, *falsePtr)
	})

	t.Run("JSON serialization", func(t *testing.T) {
		tool := NewTool("json_test",
			WithDescription("JSON test tool"),
			WithToolAnnotations(&ToolAnnotations{
				Title:           "JSON Test Tool",
				ReadOnlyHint:    BoolPtr(true),
				DestructiveHint: BoolPtr(false),
				IdempotentHint:  BoolPtr(true),
				OpenWorldHint:   BoolPtr(true),
			}),
		)

		// Marshal to JSON
		jsonData, err := json.Marshal(tool)
		assert.NoError(t, err)

		// Unmarshal back
		var unmarshaled Tool
		err = json.Unmarshal(jsonData, &unmarshaled)
		assert.NoError(t, err)

		// Verify annotations are preserved
		assert.NotNil(t, unmarshaled.Annotations)
		assert.Equal(t, "JSON Test Tool", unmarshaled.Annotations.Title)
		assert.Equal(t, true, *unmarshaled.Annotations.ReadOnlyHint)
		assert.Equal(t, false, *unmarshaled.Annotations.DestructiveHint)
		assert.Equal(t, true, *unmarshaled.Annotations.IdempotentHint)
		assert.Equal(t, true, *unmarshaled.Annotations.OpenWorldHint)
	})

	t.Run("Tool without annotations", func(t *testing.T) {
		tool := NewTool("basic_tool",
			WithDescription("Basic tool without annotations"),
		)

		assert.Nil(t, tool.Annotations)

		// JSON should omit annotations field
		jsonData, err := json.Marshal(tool)
		assert.NoError(t, err)

		var jsonMap map[string]interface{}
		err = json.Unmarshal(jsonData, &jsonMap)
		assert.NoError(t, err)

		_, hasAnnotations := jsonMap["annotations"]
		assert.False(t, hasAnnotations, "annotations field should be omitted when nil")
	})

	t.Run("Partial annotations", func(t *testing.T) {
		// Test with only some fields set
		tool := NewTool("partial_tool",
			WithToolAnnotations(&ToolAnnotations{
				Title:        "Partial Tool",
				ReadOnlyHint: BoolPtr(true),
				// Other hints not set (should be nil)
			}),
		)

		assert.NotNil(t, tool.Annotations)
		assert.Equal(t, "Partial Tool", tool.Annotations.Title)
		assert.Equal(t, true, *tool.Annotations.ReadOnlyHint)
		assert.Nil(t, tool.Annotations.DestructiveHint)
		assert.Nil(t, tool.Annotations.IdempotentHint)
		assert.Nil(t, tool.Annotations.OpenWorldHint)

		// JSON should omit nil hint fields
		jsonData, err := json.Marshal(tool)
		assert.NoError(t, err)

		var jsonMap map[string]interface{}
		err = json.Unmarshal(jsonData, &jsonMap)
		assert.NoError(t, err)

		annotations, _ := jsonMap["annotations"].(map[string]interface{})
		assert.NotNil(t, annotations)

		_, hasDestructive := annotations["destructiveHint"]
		_, hasIdempotent := annotations["idempotentHint"]
		_, hasOpenWorld := annotations["openWorldHint"]

		assert.False(t, hasDestructive, "nil destructiveHint should be omitted")
		assert.False(t, hasIdempotent, "nil idempotentHint should be omitted")
		assert.False(t, hasOpenWorld, "nil openWorldHint should be omitted")
	})
}
