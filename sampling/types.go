// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package sampling

import (
	"context"
)

// Content - Sampling message content
type Content interface {
	GetType() string
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (t TextContent) GetType() string { return t.Type }

type ImageContent struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

func (i ImageContent) GetType() string { return i.Type }

type AudioContent struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

func (a AudioContent) GetType() string { return a.Type }

type SamplingMessage struct {
	Role    string  `json:"role"` // "user", "assistant", "system"
	Content Content `json:"content"`
}

type ModelPreferences struct {
	Hints                []string `json:"hints,omitempty"`
	CostPriority         *float64 `json:"costPriority,omitempty"`         // 0-1
	SpeedPriority        *float64 `json:"speedPriority,omitempty"`        // 0-1
	IntelligencePriority *float64 `json:"intelligencePriority,omitempty"` // 0-1
}

type SamplingCreateMessageRequest struct {
	JSONRPC string                      `json:"jsonrpc"`
	ID      interface{}                 `json:"id"`
	Method  string                      `json:"method"`
	Params  SamplingCreateMessageParams `json:"params"`
}

type SamplingCreateMessageParams struct {
	Messages         []SamplingMessage `json:"messages"`
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	SystemPrompt     *string           `json:"systemPrompt,omitempty"`
	MaxTokens        *int              `json:"maxTokens,omitempty"`
	Temperature      *float64          `json:"temperature,omitempty"`
	StopSequences    []string
}

type SamplingCreateMessageResult struct {
	Role       string  `json:"role"`
	Content    Content `json:"content"`
	Model      string  `json:"model"`
	StopReason string  `json:"stopReason"`
	Usage      interface{}
}

type SamplingSender interface {
	SendSamplingRequest(ctx context.Context, req *SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error)
}
