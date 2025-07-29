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

type SamplingHandler interface {
	HandleSamplingRequest(ctx context.Context, req *SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error)
}

type UserApprovalService interface {
	RequestApproval(ctx context.Context, req *SamplingCreateMessageRequest) (bool, error)
}

type LLMClient interface {
	CreateCompletion(ctx context.Context, req *LLMRequest) (*LLMResponse, error)
}

type LLMRequest struct {
	Messages    []LLMMessage `json:"messages"`
	Model       string       `json:"model"`
	MaxTokens   *int         `json:"max_tokens,omitempty"`
	Temperature *float64     `json:"temperature,omitempty"`
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Model string `json:"model"`
}

type DefaultSamplingHandler struct {
	LLMClient    LLMClient
	UserApproval UserApprovalService
	ModelConfig  map[string]string // 模型映射配置
}

func (h *DefaultSamplingHandler) HandleSamplingRequest(ctx context.Context, req *SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	// User approval check
	if h.UserApproval != nil {
		approved, err := h.UserApproval.RequestApproval(ctx, req)
		if err != nil || !approved {
			return nil, fmt.Errorf("sampling request not approved")
		}
	}

	// Select model
	selectedModel := h.selectModel(req.Params.ModelPreferences)

	// Construct LLM request
	llmReq := h.buildLLMRequest(req, selectedModel)

	// Calling LLM
	llmResp, err := h.LLMClient.CreateCompletion(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// convert response
	return h.convertLLMResponse(llmResp, selectedModel), nil
}

func (h *DefaultSamplingHandler) selectModel(prefs *ModelPreferences) string {
	if prefs == nil {
		return "gpt-3.5-turbo" // 默认模型
	}

	// Process model hints
	if len(prefs.Hints) > 0 {
		for _, hint := range prefs.Hints {
			// Check if there is a mapped model
			if mappedModel, exists := h.ModelConfig[hint]; exists {
				return mappedModel
			}
			// Fuzzy matching
			if model := h.findModelByHint(hint); model != "" {
				return model
			}
		}
	}

	// Select model based on priority
	return h.selectByPriorities(prefs)
}

func (h *DefaultSamplingHandler) findModelByHint(hint string) string {
	// Simplified model matching logic
	switch {
	case contains(hint, "claude"):
		return "gpt-4" // Mapping to available models
	case contains(hint, "gpt-4"):
		return "gpt-4"
	case contains(hint, "sonnet"):
		return "gpt-4"
	case contains(hint, "haiku"):
		return "gpt-3.5-turbo"
	default:
		return ""
	}
}

func (h *DefaultSamplingHandler) selectByPriorities(prefs *ModelPreferences) string {
	// Simplified priority selection logic
	if prefs.IntelligencePriority != nil && *prefs.IntelligencePriority > 0.7 {
		return "gpt-4"
	}
	if prefs.SpeedPriority != nil && *prefs.SpeedPriority > 0.7 {
		return "gpt-3.5-turbo"
	}
	if prefs.CostPriority != nil && *prefs.CostPriority > 0.7 {
		return "gpt-3.5-turbo"
	}
	return "gpt-3.5-turbo" // default
}

func (h *DefaultSamplingHandler) buildLLMRequest(req *SamplingCreateMessageRequest, model string) *LLMRequest {
	llmReq := &LLMRequest{
		Model:       model,
		MaxTokens:   req.Params.MaxTokens,
		Temperature: req.Params.Temperature,
		Messages:    make([]LLMMessage, 0),
	}

	// Add system prompt
	if req.Params.SystemPrompt != nil {
		llmReq.Messages = append(llmReq.Messages, LLMMessage{
			Role:    "system",
			Content: *req.Params.SystemPrompt,
		})
	}

	// Convert message
	for _, msg := range req.Params.Messages {
		if textContent, ok := msg.Content.(TextContent); ok {
			llmReq.Messages = append(llmReq.Messages, LLMMessage{
				Role:    msg.Role,
				Content: textContent.Text,
			})
		}
	}

	return llmReq
}

func (h *DefaultSamplingHandler) convertLLMResponse(resp *LLMResponse, model string) *SamplingCreateMessageResult {
	if len(resp.Choices) == 0 {
		return &SamplingCreateMessageResult{
			Role:       "assistant",
			Content:    TextContent{Type: "text", Text: ""},
			Model:      model,
			StopReason: "error",
		}
	}

	choice := resp.Choices[0]
	return &SamplingCreateMessageResult{
		Role: choice.Message.Role,
		Content: TextContent{
			Type: "text",
			Text: choice.Message.Content,
		},
		Model:      resp.Model,
		StopReason: choice.FinishReason,
	}
}
