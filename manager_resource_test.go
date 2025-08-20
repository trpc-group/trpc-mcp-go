// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceManager_HandleCompletionComplete(t *testing.T) {
	// Create resource manager
	manager := newResourceManager()
	ctx := context.Background()

	// Test resource with completion handler
	testResource := &Resource{
		Name:        "test-resource",
		URI:         "data://test_resource",
		Description: "A test resource",
		MimeType:    "text/plain",
		Size:        1024,
	}

	// Resource completion handler
	resourceCompletionHandler := func(ctx context.Context, req *CompleteCompletionRequest) (*CompleteCompletionResult, error) {
		// Return completion values
		result := &CompleteCompletionResult{}
		result.Completion.Values = []string{"This is a test resource", "Another test resource"}
		result.Completion.Total = 2
		result.Completion.HasMore = false
		return result, nil
	}

	// Register test resource
	manager.registerResource(testResource, nil,
		WithResourceCompletion(resourceCompletionHandler),
	)

	// Test resource template with completion handler
	testTemplate := NewResourceTemplate(
		"file://{path}",
		"test-template",
		WithTemplateDescription("A test template with completion"),
	)

	// Template completion handler
	completionHandler := func(ctx context.Context, req *CompleteCompletionRequest, params map[string]string) (*CompleteCompletionResult, error) {
		// Return completion values based on argument value
		result := &CompleteCompletionResult{}
		if req.Params.Argument.Name == "path" {
			if req.Params.Context.Arguments != nil {
				// Use context argument if provided
				context, ok := req.Params.Context.Arguments["context"]
				if ok && context != "" {
					result.Completion.Values = []string{context}
				} else {
					result.Completion.Values = []string{"context is empty"}
				}
			} else {
				result.Completion.Values = []string{params["path"] + "/1.txt", params["path"] + "/2.txt"}
			}
		} else {
			result.Completion.Values = []string{"name is not path"}
		}
		result.Completion.Total = len(result.Completion.Values)
		result.Completion.HasMore = false

		return result, nil
	}

	// Register template with completion handler
	err := manager.registerTemplate(testTemplate, nil,
		WithTemplateCompletion(completionHandler),
	)
	require.NoError(t, err)

	t.Run("Valid resource completion request", func(t *testing.T) {
		req := newJSONRPCRequest("completion-1", MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/resource",
				"uri":  "data://test_resource",
			},
			"argument": map[string]interface{}{
				"name":  "path",
				"value": "file",
			},
		})

		result, err := manager.handleCompletionComplete(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)

		completionResult, ok := result.(*CompleteCompletionResult)
		require.True(t, ok, "Expected *CompleteCompletionResult but got %T", result)

		assert.Len(t, completionResult.Completion.Values, 2)
		assert.Contains(t, completionResult.Completion.Values, "This is a test resource")
		assert.Contains(t, completionResult.Completion.Values, "Another test resource")
		assert.NotNil(t, completionResult.Completion.Total)
		assert.Equal(t, 2, completionResult.Completion.Total)
		assert.NotNil(t, completionResult.Completion.HasMore)
		assert.False(t, completionResult.Completion.HasMore)
	})

	t.Run("Valid template completion request", func(t *testing.T) {
		req := newJSONRPCRequest("completion-1", MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/resource",
				"uri":  "file://test_path",
			},
			"argument": map[string]interface{}{
				"name":  "path",
				"value": "file",
			},
		})

		result, err := manager.handleCompletionComplete(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)

		completionResult, ok := result.(*CompleteCompletionResult)
		require.True(t, ok, "Expected *CompleteCompletionResult but got %T", result)

		assert.Len(t, completionResult.Completion.Values, 2)
		assert.Contains(t, completionResult.Completion.Values, "test_path/1.txt")
		assert.Contains(t, completionResult.Completion.Values, "test_path/2.txt")
		assert.NotNil(t, completionResult.Completion.Total)
		assert.Equal(t, 2, completionResult.Completion.Total)
		assert.NotNil(t, completionResult.Completion.HasMore)
		assert.False(t, completionResult.Completion.HasMore)
	})

	t.Run("Missing URI in resource ref", func(t *testing.T) {
		req := newJSONRPCRequest("completion-1", MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/resource",
			},
			"argument": map[string]interface{}{
				"name":  "path",
				"value": "file",
			},
		})

		result, err := manager.handleCompletionComplete(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)

		errorResp, ok := result.(*JSONRPCError)
		require.True(t, ok, "Expected *JSONRPCError but got %T", result)
		assert.Equal(t, ErrCodeInvalidParams, errorResp.Error.Code)
	})

	t.Run("Template not found", func(t *testing.T) {
		req := newJSONRPCRequest("completion-1", MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/resource",
				"uri":  "data://test_not_found",
			},
			"argument": map[string]interface{}{
				"name":  "path",
				"value": "file",
			},
		})

		result, err := manager.handleCompletionComplete(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)

		errorResp, ok := result.(*JSONRPCError)
		require.True(t, ok, "Expected *JSONRPCError but got %T", result)
		assert.Equal(t, ErrCodeMethodNotFound, errorResp.Error.Code)
	})

	t.Run("Template without completion handler", func(t *testing.T) {
		templateWithoutCompletion := NewResourceTemplate(
			"git://{path}",
			"git-template",
			WithTemplateDescription("A git template without completion"),
		)

		err := manager.registerTemplate(templateWithoutCompletion, nil)
		require.NoError(t, err)

		req := newJSONRPCRequest("completion-1", MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/resource",
				"uri":  "git://test_repo",
			},
			"argument": map[string]interface{}{
				"name":  "path",
				"value": "file",
			},
		})

		result, err := manager.handleCompletionComplete(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)

		errorResp, ok := result.(*JSONRPCError)
		require.True(t, ok, "Expected *JSONRPCError but got %T", result)
		assert.Equal(t, ErrCodeMethodNotFound, errorResp.Error.Code)
	})

	t.Run("With context arguments", func(t *testing.T) {
		req := newJSONRPCRequest("completion-1", MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/resource",
				"uri":  "file://test_path",
			},
			"argument": map[string]interface{}{
				"name":  "path",
				"value": "t",
			},
			"context": map[string]interface{}{
				"arguments": map[string]interface{}{
					"context": "test/context_file",
				},
			},
		})

		result, err := manager.handleCompletionComplete(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)

		completionResult, ok := result.(*CompleteCompletionResult)
		require.True(t, ok, "Expected *CompleteCompletionResult but got %T", result)
		assert.Len(t, completionResult.Completion.Values, 1)
		assert.Contains(t, completionResult.Completion.Values, "test/context_file")
	})
}

func TestResourceManager_HasCompletionCompleteHandler(t *testing.T) {
	manager := newResourceManager()

	// Initially should have no completion handlers
	assert.False(t, manager.hasCompletionCompleteHandler())

	// Register a template without completion handler
	template1 := NewResourceTemplate(
		"file://{path}",
		"template-1",
		WithTemplateDescription("Template without completion"),
	)

	err := manager.registerTemplate(template1, nil)
	require.NoError(t, err)
	assert.False(t, manager.hasCompletionCompleteHandler())

	template2 := NewResourceTemplate(
		"file://{path}",
		"template-2",
		WithTemplateDescription("Template with completion"),
	)

	completionHandler := func(ctx context.Context, req *CompleteCompletionRequest, params map[string]string) (*CompleteCompletionResult, error) {
		return &CompleteCompletionResult{}, nil
	}

	err = manager.registerTemplate(template2, nil,
		WithTemplateCompletion(completionHandler),
	)
	require.NoError(t, err)
	assert.True(t, manager.hasCompletionCompleteHandler())
}
