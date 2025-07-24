// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package sampling

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var requestIDCounter int64

// Generate a unique request ID
func generateRequestID() interface{} {
	// 方法1: 使用原子计数器
	id := atomic.AddInt64(&requestIDCounter, 1)
	return id
}

// Generate UUID-style request ID
func generateUUIDRequestID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Generate a timestamped request ID
func generateTimestampRequestID() string {
	timestamp := time.Now().UnixNano()
	counter := atomic.AddInt64(&requestIDCounter, 1)
	return fmt.Sprintf("%d-%d", timestamp, counter)
}

// Pointer utility functions
func floatPtr(f float64) *float64 { return &f }
func intPtr(i int) *int           { return &i }
func stringPtr(s string) *string  { return &s }
func boolPtr(b bool) *bool        { return &b }

// String utility functions
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func hasPrefix(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
}

func hasSuffix(s, suffix string) bool {
	return strings.HasSuffix(strings.ToLower(s), strings.ToLower(suffix))
}

// JSON-RPCError code constants
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// JSONRPCError - JSON-RPC error type
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// NewParseError - Create a standard JSON-RPC error
func NewParseError(data interface{}) *JSONRPCError {
	return &JSONRPCError{
		Code:    ParseError,
		Message: "Parse error",
		Data:    data,
	}
}

func NewInvalidRequestError(data interface{}) *JSONRPCError {
	return &JSONRPCError{
		Code:    InvalidRequest,
		Message: "Invalid Request",
		Data:    data,
	}
}

func NewMethodNotFoundError(method string) *JSONRPCError {
	return &JSONRPCError{
		Code:    MethodNotFound,
		Message: "Method not found",
		Data:    fmt.Sprintf("Method '%s' not found", method),
	}
}

func NewInvalidParamsError(data interface{}) *JSONRPCError {
	return &JSONRPCError{
		Code:    InvalidParams,
		Message: "Invalid params",
		Data:    data,
	}
}

func NewInternalError(data interface{}) *JSONRPCError {
	return &JSONRPCError{
		Code:    InternalError,
		Message: "Internal error",
		Data:    data,
	}
}

// Verification tool function
func validateSamplingRequest(req *SamplingCreateMessageRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.Method != "sampling/createMessage" {
		return fmt.Errorf("invalid method: %s", req.Method)
	}

	if len(req.Params.Messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}

	// Verify message content
	for i, msg := range req.Params.Messages {
		if msg.Role == "" {
			return fmt.Errorf("message %d: role cannot be empty", i)
		}

		if msg.Role != "user" && msg.Role != "assistant" && msg.Role != "system" {
			return fmt.Errorf("message %d: invalid role '%s'", i, msg.Role)
		}

		if msg.Content == nil {
			return fmt.Errorf("message %d: content cannot be nil", i)
		}
	}

	// Verify model preference
	if prefs := req.Params.ModelPreferences; prefs != nil {
		if prefs.CostPriority != nil && (*prefs.CostPriority < 0 || *prefs.CostPriority > 1) {
			return fmt.Errorf("costPriority must be between 0 and 1")
		}
		if prefs.SpeedPriority != nil && (*prefs.SpeedPriority < 0 || *prefs.SpeedPriority > 1) {
			return fmt.Errorf("speedPriority must be between 0 and 1")
		}
		if prefs.IntelligencePriority != nil && (*prefs.IntelligencePriority < 0 || *prefs.IntelligencePriority > 1) {
			return fmt.Errorf("intelligencePriority must be between 0 and 1")
		}
	}

	// Verification token restrictions
	if req.Params.MaxTokens != nil && *req.Params.MaxTokens <= 0 {
		return fmt.Errorf("maxTokens must be positive")
	}

	// Verify temperature parameters
	if req.Params.Temperature != nil && (*req.Params.Temperature < 0 || *req.Params.Temperature > 2) {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	return nil
}

// 内容类型检查工具
func getContentType(content Content) string {
	if content == nil {
		return "unknown"
	}
	return content.GetType()
}

func isTextContent(content Content) bool {
	_, ok := content.(TextContent)
	return ok
}

func isImageContent(content Content) bool {
	_, ok := content.(ImageContent)
	return ok
}

func isAudioContent(content Content) bool {
	_, ok := content.(AudioContent)
	return ok
}

// 内容转换工具
func contentToText(content Content) (string, error) {
	if textContent, ok := content.(TextContent); ok {
		return textContent.Text, nil
	}
	return "", fmt.Errorf("content is not text type")
}

func textToContent(text string) TextContent {
	return TextContent{
		Type: "text",
		Text: text,
	}
}

type CallToolRequest struct {
	Params struct {
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"params"`
}

type CallToolResult struct {
	Content interface{} `json:"content"`
}

func NewErrorResult(msg string) *CallToolResult {
	return &CallToolResult{Content: map[string]string{"error": msg}}
}

func NewTextResult(text string) *CallToolResult {
	return &CallToolResult{Content: map[string]string{"text": text}}
}

// 日志工具函数
func logSamplingRequest(req *SamplingCreateMessageRequest) {
	fmt.Printf("[SAMPLING] Request ID: %v, Method: %s, Messages: %d\n",
		req.ID, req.Method, len(req.Params.Messages))

	if req.Params.ModelPreferences != nil {
		prefs := req.Params.ModelPreferences
		fmt.Printf("[SAMPLING] Preferences - Hints: %v, Intelligence: %v, Speed: %v, Cost: %v\n",
			prefs.Hints, prefs.IntelligencePriority, prefs.SpeedPriority, prefs.CostPriority)
	}
}

func logSamplingResponse(result *SamplingCreateMessageResult) {
	fmt.Printf("[SAMPLING] Response - Role: %s, Model: %s, StopReason: %s\n",
		result.Role, result.Model, result.StopReason)
}

// ModelInfo - Model information structure (for model selection)
type ModelInfo struct {
	Name              string   `json:"name"`
	Provider          string   `json:"provider"`
	CostIndex         float64  `json:"cost_index"`         // 0-1
	SpeedIndex        float64  `json:"speed_index"`        // 0-1
	IntelligenceIndex float64  `json:"intelligence_index"` // 0-1
	MaxTokens         int      `json:"max_tokens"`
	SupportedTypes    []string `json:"supported_types"` // text, image, audio
}

// DefaultModels - Predefined model information
var DefaultModels = []ModelInfo{
	{
		Name:              "gpt-4",
		Provider:          "openai",
		CostIndex:         0.2, // expensive
		SpeedIndex:        0.6, // middle speed
		IntelligenceIndex: 0.9, // high intelligent
		MaxTokens:         8192,
		SupportedTypes:    []string{"text", "image"},
	},
	{
		Name:              "gpt-3.5-turbo",
		Provider:          "openai",
		CostIndex:         0.8, // cheap
		SpeedIndex:        0.9, // high speed
		IntelligenceIndex: 0.7, // middle intelligent
		MaxTokens:         4096,
		SupportedTypes:    []string{"text"},
	},
	{
		Name:              "claude-3-sonnet",
		Provider:          "anthropic",
		CostIndex:         0.4,  // middle price
		SpeedIndex:        0.7,  // middle fast
		IntelligenceIndex: 0.85, // high intelligent
		MaxTokens:         200000,
		SupportedTypes:    []string{"text", "image"},
	},
}
