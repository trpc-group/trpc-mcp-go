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

func TestPromptManager_HandleCompletionComplete(t *testing.T) {
	// Create prompt manager
	manager := newPromptManager()
	ctx := context.Background()

	// Test prompt with completion handler
	testPrompt := &Prompt{
		Name:        "test-completion-prompt",
		Description: "A test prompt with completion",
		Arguments: []PromptArgument{
			{
				Name:        "query",
				Description: "Search query",
				Required:    true,
			},
		},
	}

	// Mock completion handler
	completionHandler := func(ctx context.Context, req *CompleteCompletionRequest) (*CompleteCompletionResult, error) {
		// Return mock completion values based on argument value
		result := &CompleteCompletionResult{}
		if req.Params.Argument.Name == "query" {
			if req.Params.Context.Arguments != nil {
				// Use context argument if provided
				context, ok := req.Params.Context.Arguments["context"]
				if ok && context != "" {
					result.Completion.Values = []string{context}
				} else {
					result.Completion.Values = []string{"context is empty"}
				}
			} else {
				result.Completion.Values = []string{"search term 1", "search term 2", "search term 3"}
			}
		} else {
			result.Completion.Values = []string{"name is not query"}
		}

		result.Completion.Total = len(result.Completion.Values)
		result.Completion.HasMore = false

		return result, nil
	}

	// Register prompt with completion handler
	manager.registerPrompt(testPrompt, nil,
		WithPromptCompletion(completionHandler),
	)

	t.Run("Valid completion request", func(t *testing.T) {
		req := newJSONRPCRequest(1, MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/prompt",
				"name": "test-completion-prompt",
			},
			"argument": map[string]interface{}{
				"name":  "query",
				"value": "test",
			},
		})

		// Handle completion request
		result, err := manager.handleCompletionComplete(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)

		completionResult, ok := result.(*CompleteCompletionResult)
		require.True(t, ok, "Expected *CompleteCompletionResult but got %T", result)

		// Verify completion values
		assert.Len(t, completionResult.Completion.Values, 3)
		assert.Contains(t, completionResult.Completion.Values, "search term 1")
		assert.Contains(t, completionResult.Completion.Values, "search term 2")
		assert.Contains(t, completionResult.Completion.Values, "search term 3")
		assert.Equal(t, 3, completionResult.Completion.Total)
		assert.False(t, completionResult.Completion.HasMore)
	})

	t.Run("Prompt not found", func(t *testing.T) {
		req := newJSONRPCRequest(2, MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/prompt",
				"name": "non-existent-prompt",
			},
			"argument": map[string]interface{}{
				"name":  "query",
				"value": "test",
			},
		})

		result, err := manager.handleCompletionComplete(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)

		errorResp, ok := result.(*JSONRPCError)
		require.True(t, ok, "Expected *JSONRPCError but got %T", result)
		assert.Equal(t, ErrCodeMethodNotFound, errorResp.Error.Code)
	})

	t.Run("Prompt without completion handler", func(t *testing.T) {
		promptWithoutCompletion := &Prompt{
			Name:        "no-completion-prompt",
			Description: "A prompt without completion",
		}

		manager.registerPrompt(promptWithoutCompletion, nil)

		req := newJSONRPCRequest(3, MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/prompt",
				"name": "no-completion-prompt",
			},
			"argument": map[string]interface{}{
				"name":  "query",
				"value": "test",
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
		req := newJSONRPCRequest(4, MethodCompletionComplete, map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/prompt",
				"name": "test-completion-prompt",
			},
			"argument": map[string]interface{}{
				"name":  "query",
				"value": "test_context",
			},
			"context": map[string]interface{}{
				"arguments": map[string]interface{}{
					"context": "test context",
				},
			},
		})

		result, err := manager.handleCompletionComplete(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, result)

		completionResult, ok := result.(*CompleteCompletionResult)
		require.True(t, ok, "Expected *CompleteCompletionResult but got %T", result)
		assert.Len(t, completionResult.Completion.Values, 1)
		assert.Contains(t, completionResult.Completion.Values, "test context")
	})
}

func TestPromptManager_HasCompletionCompleteHandler(t *testing.T) {
	manager := newPromptManager()

	// Initially should have no completion handlers
	assert.False(t, manager.hasCompletionCompleteHandler())

	// Register a prompt without completion handler
	prompt1 := &Prompt{
		Name:        "prompt-1",
		Description: "Prompt without completion",
	}
	manager.registerPrompt(prompt1, nil)
	assert.False(t, manager.hasCompletionCompleteHandler())

	// Register a prompt with completion handler
	prompt2 := &Prompt{
		Name:        "prompt-2",
		Description: "Prompt with completion",
	}

	completionHandler := func(ctx context.Context, req *CompleteCompletionRequest) (*CompleteCompletionResult, error) {
		return &CompleteCompletionResult{}, nil
	}

	manager.registerPrompt(prompt2, nil,
		WithPromptCompletion(completionHandler),
	)
	assert.True(t, manager.hasCompletionCompleteHandler())
}
