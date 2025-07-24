package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionComplete_Prompt_Success(t *testing.T) {
	handler := newMCPHandler()

	prompt := &Prompt{
		Name:        "test-prompt",
		Description: "测试补全prompt",
		Arguments: []PromptArgument{
			{Name: "input", Description: "输入", Required: false},
		},
	}
	completionHandler := func(ctx context.Context, req *CompletionCompleteRequest) (*CompletionCompleteResult, error) {
		input := req.Params.Argument["input"]
		var suggestions []string
		if input == "he" {
			suggestions = []string{"hello", "help"}
		} else {
			suggestions = []string{"default"}
		}
		return &CompletionCompleteResult{
			Completion: Completion{
				Values:  suggestions,
				Total:   len(suggestions),
				HasMore: false,
			},
		}, nil
	}
	handler.promptManager.registerPromptWithCompletion(prompt, nil, completionHandler)

	req := newJSONRPCRequest(1, MethodCompletionComplete, map[string]interface{}{
		"ref": map[string]interface{}{
			"type": "ref/prompt",
			"name": "test-prompt",
		},
		"argument": map[string]interface{}{
			"input": "he",
		},
	})
	ctx := context.Background()
	resp, err := handler.handleRequest(ctx, req, newSession())

	require.NoError(t, err)
	assert.NotNil(t, resp)
	t.Logf("resp type: %T", resp)
	result, ok := resp.(*CompletionCompleteResult)
	assert.True(t, ok)
	assert.Equal(t, []string{"hello", "help"}, result.Completion.Values)
	assert.Equal(t, 2, result.Completion.Total)
	assert.False(t, result.Completion.HasMore)
}

func TestCompletionComplete_Prompt_NotFound(t *testing.T) {
	handler := newMCPHandler()

	req := newJSONRPCRequest(1, MethodCompletionComplete, map[string]interface{}{
		"ref": map[string]interface{}{
			"type": "ref/prompt",
			"name": "not-exist",
		},
		"argument": map[string]interface{}{},
	})
	ctx := context.Background()
	resp, err := handler.handleRequest(ctx, req, newSession())

	require.NoError(t, err)
	assert.NotNil(t, resp)
	errResp, ok := resp.(*JSONRPCError)
	assert.True(t, ok)
	assert.Contains(t, errResp.Error.Message, "prompt not found")
}

func TestCompletionComplete_Prompt_NoCompletionHandler(t *testing.T) {
	handler := newMCPHandler()
	prompt := &Prompt{
		Name:        "no-completion",
		Description: "无补全handler",
	}
	handler.promptManager.registerPrompt(prompt, nil)

	req := newJSONRPCRequest(1, MethodCompletionComplete, map[string]interface{}{
		"ref": map[string]interface{}{
			"type": "ref/prompt",
			"name": "no-completion",
		},
		"argument": map[string]interface{}{},
	})
	ctx := context.Background()
	resp, err := handler.handleRequest(ctx, req, newSession())

	require.NoError(t, err)
	assert.NotNil(t, resp)
	result, ok := resp.(*CompletionCompleteResult)
	assert.True(t, ok)
	assert.Empty(t, result.Completion.Values)
	assert.Equal(t, 0, result.Completion.Total)
	assert.False(t, result.Completion.HasMore)
}
