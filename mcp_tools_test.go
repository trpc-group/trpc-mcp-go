// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/trpc-mcp-go/internal/openapi"
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

// TestCompatibilityLayerWithCurrentVersion verifies that our compatibility layer
// produces the same results as the original code for current users
func TestCompatibilityLayerWithCurrentVersion(t *testing.T) {
	// This test verifies that existing users (using kin-openapi v0.132.0)
	// get exactly the same behavior as before

	// Test NewTool compatibility
	t.Run("NewTool compatibility", func(t *testing.T) {
		tool := NewTool("test-tool")

		// Verify the schema was created correctly
		assert.NotNil(t, tool.InputSchema)
		assert.NotNil(t, tool.InputSchema.Properties)
		assert.Equal(t, []string{}, tool.InputSchema.Required)

		// For v0.132.0, Type should be *openapi3.Types
		assert.NotNil(t, tool.InputSchema.Type)
		assert.IsType(t, &openapi3.Types{}, tool.InputSchema.Type)
		assert.Equal(t, openapi3.Types{openapi3.TypeObject}, *tool.InputSchema.Type)
	})

	// Test WithString compatibility
	t.Run("WithString compatibility", func(t *testing.T) {
		tool := NewTool("test-tool", WithString("name"))

		nameSchema := tool.InputSchema.Properties["name"]
		assert.NotNil(t, nameSchema)
		assert.NotNil(t, nameSchema.Value)

		// For v0.132.0, Type should be *openapi3.Types
		assert.NotNil(t, nameSchema.Value.Type)
		assert.IsType(t, &openapi3.Types{}, nameSchema.Value.Type)
		assert.Equal(t, openapi3.Types{openapi3.TypeString}, *nameSchema.Value.Type)
	})

	// Test all other types
	t.Run("All types compatibility", func(t *testing.T) {
		tool := NewTool("test-tool",
			WithString("str_field"),
			WithNumber("num_field"),
			WithInteger("int_field"),
			WithBoolean("bool_field"),
			WithArray("arr_field"),
			WithObject("obj_field"),
		)

		testCases := []struct {
			fieldName    string
			expectedType string
		}{
			{"str_field", openapi3.TypeString},
			{"num_field", openapi3.TypeNumber},
			{"int_field", openapi3.TypeInteger},
			{"bool_field", openapi3.TypeBoolean},
			{"arr_field", openapi3.TypeArray},
			{"obj_field", openapi3.TypeObject},
		}

		for _, tc := range testCases {
			t.Run(tc.fieldName, func(t *testing.T) {
				schema := tool.InputSchema.Properties[tc.fieldName]
				assert.NotNil(t, schema)
				assert.NotNil(t, schema.Value)
				assert.NotNil(t, schema.Value.Type)
				assert.IsType(t, &openapi3.Types{}, schema.Value.Type)
				assert.Equal(t, openapi3.Types{tc.expectedType}, *schema.Value.Type)
			})
		}
	})
}

// TestOriginalVsCompatibilityEquivalence verifies that our compatibility methods
// produce exactly the same result as the original code would for current users
func TestOriginalVsCompatibilityEquivalence(t *testing.T) {
	// Simulate what the original code would have produced
	originalSchema := &openapi3.Schema{
		Type: &openapi3.Types{openapi3.TypeString},
	}

	// What our compatibility layer produces
	compatSchema := openapi.DefaultCompat.CreateStringSchema()

	// They should be equivalent
	assert.Equal(t, *originalSchema.Type, *compatSchema.Type)
	// Both should have Type field set
	assert.NotNil(t, originalSchema.Type)
	assert.NotNil(t, compatSchema.Type)
}
