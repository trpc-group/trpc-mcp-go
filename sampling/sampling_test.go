// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package sampling

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Mock implementations for testing

// MockLLMClient - Mock LLM client for testing
type MockLLMClient struct {
	responses map[string]*LLMResponse
	errors    map[string]error
}

func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		responses: make(map[string]*LLMResponse),
		errors:    make(map[string]error),
	}
}

func (m *MockLLMClient) SetResponse(model string, response *LLMResponse) {
	m.responses[model] = response
}

func (m *MockLLMClient) SetError(model string, err error) {
	m.errors[model] = err
}

func (m *MockLLMClient) CreateCompletion(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	if err, exists := m.errors[req.Model]; exists {
		return nil, err
	}
	if resp, exists := m.responses[req.Model]; exists {
		return resp, nil
	}
	return &LLMResponse{
		Choices: []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: "Mock response",
				},
				FinishReason: "stop",
			},
		},
		Model: req.Model,
	}, nil
}

// MockUserApprovalService - Mock user approval service for testing
type MockUserApprovalService struct {
	approvalResults map[string]bool
	errors          map[string]error
}

func NewMockUserApprovalService() *MockUserApprovalService {
	return &MockUserApprovalService{
		approvalResults: make(map[string]bool),
		errors:          make(map[string]error),
	}
}

func (m *MockUserApprovalService) SetApproval(key string, approved bool) {
	m.approvalResults[key] = approved
}

func (m *MockUserApprovalService) SetError(key string, err error) {
	m.errors[key] = err
}

func (m *MockUserApprovalService) RequestApproval(ctx context.Context, req *SamplingCreateMessageRequest) (bool, error) {
	key := fmt.Sprintf("%v", req.ID)
	if err, exists := m.errors[key]; exists {
		return false, err
	}
	if result, exists := m.approvalResults[key]; exists {
		return result, nil
	}
	return true, nil // Default approve
}

// MockSamplingSender - Mock sampling sender for testing
type MockSamplingSender struct {
	responses       map[string]*SamplingCreateMessageResult
	errors          map[string]error
	defaultResponse *SamplingCreateMessageResult
	defaultError    error
	forceError      bool
}

func NewMockSamplingSender() *MockSamplingSender {
	return &MockSamplingSender{
		responses: make(map[string]*SamplingCreateMessageResult),
		errors:    make(map[string]error),
	}
}

func (m *MockSamplingSender) SetResponse(key string, response *SamplingCreateMessageResult) {
	m.responses[key] = response
}

func (m *MockSamplingSender) SetError(key string, err error) {
	m.errors[key] = err
}

func (m *MockSamplingSender) SetDefaultResponse(response *SamplingCreateMessageResult) {
	m.defaultResponse = response
}

func (m *MockSamplingSender) SetDefaultError(err error) {
	m.defaultError = err
	m.forceError = true
}

func (m *MockSamplingSender) ClearDefaultError() {
	m.defaultError = nil
	m.forceError = false
}

func (m *MockSamplingSender) SendSamplingRequest(ctx context.Context, req *SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	// Check for forced error first
	if m.forceError && m.defaultError != nil {
		return nil, m.defaultError
	}

	key := fmt.Sprintf("%v", req.ID)
	if err, exists := m.errors[key]; exists {
		return nil, err
	}
	if resp, exists := m.responses[key]; exists {
		return resp, nil
	}

	// Return default response
	if m.defaultResponse != nil {
		return m.defaultResponse, nil
	}

	return &SamplingCreateMessageResult{
		Role:       "assistant",
		Content:    TextContent{Type: "text", Text: "Mock response"},
		Model:      "gpt-3.5-turbo",
		StopReason: "stop",
	}, nil
}

// Test Types

func TestTextContent_GetType(t *testing.T) {
	content := TextContent{Type: "text", Text: "Hello"}
	if content.GetType() != "text" {
		t.Errorf("Expected type 'text', got '%s'", content.GetType())
	}
}

func TestImageContent_GetType(t *testing.T) {
	content := ImageContent{Type: "image", Data: "base64data", MimeType: "image/png"}
	if content.GetType() != "image" {
		t.Errorf("Expected type 'image', got '%s'", content.GetType())
	}
}

func TestAudioContent_GetType(t *testing.T) {
	content := AudioContent{Type: "audio", Data: "base64data", MimeType: "audio/mp3"}
	if content.GetType() != "audio" {
		t.Errorf("Expected type 'audio', got '%s'", content.GetType())
	}
}

// Test Utils

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == id2 {
		t.Error("Expected different request IDs")
	}
}

func TestGenerateUUIDRequestID(t *testing.T) {
	id := generateUUIDRequestID()
	if len(id) != 32 { // 16 bytes * 2 chars per byte
		t.Errorf("Expected UUID length 32, got %d", len(id))
	}
}

func TestPointerUtilities(t *testing.T) {
	f := 3.14
	i := 42
	s := "test"
	b := true

	if *floatPtr(f) != f {
		t.Error("floatPtr failed")
	}
	if *intPtr(i) != i {
		t.Error("intPtr failed")
	}
	if *stringPtr(s) != s {
		t.Error("stringPtr failed")
	}
	if *boolPtr(b) != b {
		t.Error("boolPtr failed")
	}
}

func TestStringUtilities(t *testing.T) {
	tests := []struct {
		name     string
		function func(string, string) bool
		s1, s2   string
		expected bool
	}{
		{"contains_true", contains, "Hello World", "world", true},
		{"contains_false", contains, "Hello World", "xyz", false},
		{"hasPrefix_true", hasPrefix, "Hello World", "hello", true},
		{"hasPrefix_false", hasPrefix, "Hello World", "world", false},
		{"hasSuffix_true", hasSuffix, "Hello World", "world", true},
		{"hasSuffix_false", hasSuffix, "Hello World", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function(tt.s1, tt.s2)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestValidateSamplingRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *SamplingCreateMessageRequest
		wantErr bool
	}{
		{
			name:    "nil_request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "invalid_method",
			req: &SamplingCreateMessageRequest{
				Method: "invalid/method",
			},
			wantErr: true,
		},
		{
			name: "empty_messages",
			req: &SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: SamplingCreateMessageParams{
					Messages: []SamplingMessage{},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid_role",
			req: &SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: SamplingCreateMessageParams{
					Messages: []SamplingMessage{
						{Role: "invalid", Content: TextContent{Type: "text", Text: "test"}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid_request",
			req: &SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: SamplingCreateMessageParams{
					Messages: []SamplingMessage{
						{Role: "user", Content: TextContent{Type: "text", Text: "test"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid_priority_values",
			req: &SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: SamplingCreateMessageParams{
					Messages: []SamplingMessage{
						{Role: "user", Content: TextContent{Type: "text", Text: "test"}},
					},
					ModelPreferences: &ModelPreferences{
						CostPriority: floatPtr(1.5), // Invalid: > 1
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid_max_tokens",
			req: &SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: SamplingCreateMessageParams{
					Messages: []SamplingMessage{
						{Role: "user", Content: TextContent{Type: "text", Text: "test"}},
					},
					MaxTokens: intPtr(-1), // Invalid: negative
				},
			},
			wantErr: true,
		},
		{
			name: "invalid_temperature",
			req: &SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: SamplingCreateMessageParams{
					Messages: []SamplingMessage{
						{Role: "user", Content: TextContent{Type: "text", Text: "test"}},
					},
					Temperature: floatPtr(3.0), // Invalid: > 2
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSamplingRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSamplingRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContentUtilities(t *testing.T) {
	textContent := TextContent{Type: "text", Text: "Hello"}
	imageContent := ImageContent{Type: "image", Data: "data", MimeType: "image/png"}
	audioContent := AudioContent{Type: "audio", Data: "data", MimeType: "audio/mp3"}

	// Test content type checks
	if !isTextContent(textContent) {
		t.Error("Expected isTextContent to return true for TextContent")
	}
	if isTextContent(imageContent) {
		t.Error("Expected isTextContent to return false for ImageContent")
	}

	if !isImageContent(imageContent) {
		t.Error("Expected isImageContent to return true for ImageContent")
	}
	if isImageContent(textContent) {
		t.Error("Expected isImageContent to return false for TextContent")
	}

	if !isAudioContent(audioContent) {
		t.Error("Expected isAudioContent to return true for AudioContent")
	}
	if isAudioContent(textContent) {
		t.Error("Expected isAudioContent to return false for TextContent")
	}

	// Test content conversion
	text, err := contentToText(textContent)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if text != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", text)
	}

	_, err = contentToText(imageContent)
	if err == nil {
		t.Error("Expected error when converting non-text content")
	}

	// Test textToContent
	newTextContent := textToContent("Test")
	if newTextContent.Type != "text" || newTextContent.Text != "Test" {
		t.Error("textToContent failed")
	}
}

// Test Client

func TestDefaultSamplingHandler_SelectModel(t *testing.T) {
	handler := &DefaultSamplingHandler{
		ModelConfig: map[string]string{
			"custom-model": "gpt-4",
		},
	}

	tests := []struct {
		name     string
		prefs    *ModelPreferences
		expected string
	}{
		{
			name:     "nil_preferences",
			prefs:    nil,
			expected: "gpt-3.5-turbo",
		},
		{
			name: "hint_mapping",
			prefs: &ModelPreferences{
				Hints: []string{"custom-model"},
			},
			expected: "gpt-4",
		},
		{
			name: "claude_hint",
			prefs: &ModelPreferences{
				Hints: []string{"claude"},
			},
			expected: "gpt-4",
		},
		{
			name: "high_intelligence_priority",
			prefs: &ModelPreferences{
				IntelligencePriority: floatPtr(0.8),
			},
			expected: "gpt-4",
		},
		{
			name: "high_speed_priority",
			prefs: &ModelPreferences{
				SpeedPriority: floatPtr(0.8),
			},
			expected: "gpt-3.5-turbo",
		},
		{
			name: "high_cost_priority",
			prefs: &ModelPreferences{
				CostPriority: floatPtr(0.8),
			},
			expected: "gpt-3.5-turbo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.selectModel(tt.prefs)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDefaultSamplingHandler_BuildLLMRequest(t *testing.T) {
	handler := &DefaultSamplingHandler{}

	req := &SamplingCreateMessageRequest{
		Params: SamplingCreateMessageParams{
			Messages: []SamplingMessage{
				{Role: "user", Content: TextContent{Type: "text", Text: "Hello"}},
			},
			SystemPrompt: stringPtr("You are helpful"),
			MaxTokens:    intPtr(1000),
			Temperature:  floatPtr(0.7),
		},
	}

	llmReq := handler.buildLLMRequest(req, "gpt-3.5-turbo")

	if llmReq.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected model 'gpt-3.5-turbo', got '%s'", llmReq.Model)
	}

	if *llmReq.MaxTokens != 1000 {
		t.Errorf("Expected MaxTokens 1000, got %d", *llmReq.MaxTokens)
	}

	if *llmReq.Temperature != 0.7 {
		t.Errorf("Expected Temperature 0.7, got %f", *llmReq.Temperature)
	}

	if len(llmReq.Messages) != 2 { // system + user
		t.Errorf("Expected 2 messages, got %d", len(llmReq.Messages))
	}

	if llmReq.Messages[0].Role != "system" {
		t.Errorf("Expected first message role 'system', got '%s'", llmReq.Messages[0].Role)
	}
}

func TestDefaultSamplingHandler_ConvertLLMResponse(t *testing.T) {
	handler := &DefaultSamplingHandler{}

	// Test with valid response
	llmResp := &LLMResponse{
		Choices: []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: "Hello response",
				},
				FinishReason: "stop",
			},
		},
		Model: "gpt-3.5-turbo",
	}

	result := handler.convertLLMResponse(llmResp, "gpt-3.5-turbo")

	if result.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", result.Role)
	}

	textContent, ok := result.Content.(TextContent)
	if !ok {
		t.Error("Expected TextContent")
	}

	if textContent.Text != "Hello response" {
		t.Errorf("Expected 'Hello response', got '%s'", textContent.Text)
	}

	// Test with empty response
	emptyResp := &LLMResponse{Choices: []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}{}}

	emptyResult := handler.convertLLMResponse(emptyResp, "gpt-3.5-turbo")
	if emptyResult.StopReason != "error" {
		t.Errorf("Expected StopReason 'error', got '%s'", emptyResult.StopReason)
	}
}

func TestDefaultSamplingHandler_HandleSamplingRequest(t *testing.T) {
	mockLLM := NewMockLLMClient()
	mockApproval := NewMockUserApprovalService()

	handler := &DefaultSamplingHandler{
		LLMClient:    mockLLM,
		UserApproval: mockApproval,
		ModelConfig:  make(map[string]string),
	}

	req := &SamplingCreateMessageRequest{
		ID:     1,
		Method: "sampling/createMessage",
		Params: SamplingCreateMessageParams{
			Messages: []SamplingMessage{
				{Role: "user", Content: TextContent{Type: "text", Text: "Hello"}},
			},
		},
	}

	ctx := context.Background()

	// Test successful request
	result, err := handler.HandleSamplingRequest(ctx, req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Error("Expected result, got nil")
	}

	// Test approval denied
	mockApproval.SetApproval("1", false)
	_, err = handler.HandleSamplingRequest(ctx, req)
	if err == nil {
		t.Error("Expected error when approval denied")
	}

	// Test approval error
	mockApproval.SetError("1", errors.New("approval error"))
	_, err = handler.HandleSamplingRequest(ctx, req)
	if err == nil {
		t.Error("Expected error when approval fails")
	}

	// Test LLM error
	mockApproval.SetApproval("1", true)
	mockLLM.SetError("gpt-3.5-turbo", errors.New("LLM error"))
	_, err = handler.HandleSamplingRequest(ctx, req)
	if err == nil {
		t.Error("Expected error when LLM fails")
	}
}

// Test Approval Service

func TestInteractiveUserApprovalService_HasBlockedContent(t *testing.T) {
	service := NewInteractiveUserApprovalService()

	// Test with blocked content
	req := &SamplingCreateMessageRequest{
		Params: SamplingCreateMessageParams{
			Messages: []SamplingMessage{
				{Role: "user", Content: TextContent{Type: "text", Text: "How to hack a system"}},
			},
		},
	}

	if !service.hasBlockedContent(req) {
		t.Error("Expected blocked content to be detected")
	}

	// Test with allowed content
	req.Params.Messages[0].Content = TextContent{Type: "text", Text: "Hello world"}
	if service.hasBlockedContent(req) {
		t.Error("Expected content to be allowed")
	}

	// Test with blocked system prompt
	req.Params.SystemPrompt = stringPtr("You are a virus")
	if !service.hasBlockedContent(req) {
		t.Error("Expected blocked content in system prompt to be detected")
	}
}

func TestInteractiveUserApprovalService_GetContentPreview(t *testing.T) {
	service := NewInteractiveUserApprovalService()

	// Test text content
	textContent := TextContent{Type: "text", Text: "Hello world"}
	preview := service.getContentPreview(textContent)
	if preview != "Hello world" {
		t.Errorf("Expected 'Hello world', got '%s'", preview)
	}

	// Test long text content
	longText := TextContent{Type: "text", Text: strings.Repeat("a", 150)}
	preview = service.getContentPreview(longText)
	if len(preview) != 103 { // 100 chars + "..."
		t.Errorf("Expected truncated text, got length %d", len(preview))
	}

	// Test image content
	imageContent := ImageContent{Type: "image", MimeType: "image/png"}
	preview = service.getContentPreview(imageContent)
	expected := "[Image content - image/png]"
	if preview != expected {
		t.Errorf("Expected '%s', got '%s'", expected, preview)
	}

	// Test audio content
	audioContent := AudioContent{Type: "audio", MimeType: "audio/mp3"}
	preview = service.getContentPreview(audioContent)
	expected = "[Audio content - audio/mp3]"
	if preview != expected {
		t.Errorf("Expected '%s', got '%s'", expected, preview)
	}
}

func TestAdvancedUserApprovalService_EvaluateRule(t *testing.T) {
	service := NewAdvancedUserApprovalService([]ContentFilterRule{})

	rule := ContentFilterRule{
		Name:            "test-rule",
		BlockedKeywords: []string{"hack", "exploit"},
		AllowedModels:   []string{"gpt-4"},
		MaxTokens:       intPtr(1000),
		Enabled:         true,
	}

	// Test blocked keyword
	req := &SamplingCreateMessageRequest{
		Params: SamplingCreateMessageParams{
			Messages: []SamplingMessage{
				{Role: "user", Content: TextContent{Type: "text", Text: "How to hack"}},
			},
		},
	}

	blocked, reason := service.evaluateRule(rule, req)
	if !blocked {
		t.Error("Expected rule to block content with blocked keyword")
	}
	if !strings.Contains(reason, "hack") {
		t.Errorf("Expected reason to mention 'hack', got '%s'", reason)
	}

	// Test token limit
	req.Params.Messages[0].Content = TextContent{Type: "text", Text: "Hello"}
	req.Params.MaxTokens = intPtr(2000)

	blocked, reason = service.evaluateRule(rule, req)
	if !blocked {
		t.Error("Expected rule to block content exceeding token limit")
	}

	// Test model restriction
	req.Params.MaxTokens = intPtr(500)
	req.Params.ModelPreferences = &ModelPreferences{
		Hints: []string{"gpt-3.5"},
	}

	blocked, reason = service.evaluateRule(rule, req)
	if !blocked {
		t.Error("Expected rule to block disallowed model")
	}

	// Test allowed content
	req.Params.ModelPreferences.Hints = []string{"gpt-4"}
	blocked, _ = service.evaluateRule(rule, req)
	if blocked {
		t.Error("Expected rule to allow valid content")
	}
}

func TestGetDefaultFilterRules(t *testing.T) {
	rules := GetDefaultFilterRules()

	if len(rules) == 0 {
		t.Error("Expected default filter rules")
	}

	for _, rule := range rules {
		if rule.Name == "" {
			t.Error("Expected rule to have a name")
		}
		if !rule.Enabled {
			t.Error("Expected default rules to be enabled")
		}
	}
}

// Test Server

func TestGetSamplingSender(t *testing.T) {
	mockSender := NewMockSamplingSender()

	// Test with sender in context
	ctx := context.WithValue(context.Background(), SamplingSenderKey, mockSender)
	sender, ok := GetSamplingSender(ctx)
	if !ok {
		t.Error("Expected to find sampling sender in context")
	}
	if sender != mockSender {
		t.Error("Expected sender to match mock sender")
	}

	// Test without Sender in context
	ctx = context.Background()
	_, ok = GetSamplingSender(ctx)
	if ok {
		t.Error("Expected not to find sampling sender in empty context")
	}
}

func TestHandleAnalysisWithSampling(t *testing.T) {
	mockSender := NewMockSamplingSender()

	// Test without sampling sender
	ctx := context.Background()
	req := &CallToolRequest{
		Params: struct {
			Arguments map[string]interface{} `json:"arguments"`
		}{
			Arguments: map[string]interface{}{
				"data": "test data",
			},
		},
	}

	result, err := HandleAnalysisWithSampling(ctx, req)
	if err == nil {
		t.Error("Expected error when sampling not available")
	}
	if result == nil {
		t.Error("Expected error result")
	}

	// Test with invalid data parameter
	ctx = context.WithValue(context.Background(), SamplingSenderKey, mockSender)
	req.Params.Arguments["data"] = 123 // Invalid type

	result, err = HandleAnalysisWithSampling(ctx, req)
	if err == nil {
		t.Error("Expected error with invalid data parameter")
	}

	// Test successful request
	req.Params.Arguments["data"] = "test data"
	mockSender.SetDefaultResponse(&SamplingCreateMessageResult{
		Role:       "assistant",
		Content:    TextContent{Type: "text", Text: "Analysis result"},
		Model:      "gpt-4",
		StopReason: "stop",
	})

	result, err = HandleAnalysisWithSampling(ctx, req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Error("Expected successful result")
	}

	// Test with sampling error - use default error for any request
	mockSender.SetDefaultError(errors.New("sampling error"))

	result, err = HandleAnalysisWithSampling(ctx, req)
	if err == nil {
		t.Error("Expected error when sampling fails")
	}

	// Clear error and test with invalid response format
	mockSender.ClearDefaultError()
	mockSender.SetDefaultResponse(&SamplingCreateMessageResult{
		Role:       "assistant",
		Content:    ImageContent{Type: "image"}, // Invalid for analysis
		Model:      "gpt-4",
		StopReason: "stop",
	})

	result, err = HandleAnalysisWithSampling(ctx, req)
	if err == nil {
		t.Error("Expected error with invalid response format")
	}
}

// Test Error Types

func TestJSONRPCError(t *testing.T) {
	err := JSONRPCError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    "test data",
	}

	expected := "JSON-RPC error -32600: Invalid Request"
	if err.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, err.Error())
	}
}

func TestNewErrorFunctions(t *testing.T) {
	tests := []struct {
		name     string
		function func(interface{}) *JSONRPCError
		code     int
	}{
		{"ParseError", NewParseError, ParseError},
		{"InvalidRequestError", NewInvalidRequestError, InvalidRequest},
		{"InvalidParamsError", NewInvalidParamsError, InvalidParams},
		{"InternalError", NewInternalError, InternalError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.function("test data")
			if err.Code != tt.code {
				t.Errorf("Expected code %d, got %d", tt.code, err.Code)
			}
			if err.Data != "test data" {
				t.Errorf("Expected data 'test data', got %v", err.Data)
			}
		})
	}
}

func TestNewMethodNotFoundError(t *testing.T) {
	err := NewMethodNotFoundError("test/method")
	if err.Code != MethodNotFound {
		t.Errorf("Expected code %d, got %d", MethodNotFound, err.Code)
	}
	expected := "Method 'test/method' not found"
	if err.Data != expected {
		t.Errorf("Expected data '%s', got %v", expected, err.Data)
	}
}

// Test Tool Result Functions

func TestNewErrorResult(t *testing.T) {
	result := NewErrorResult("test error")
	content, ok := result.Content.(map[string]string)
	if !ok {
		t.Error("Expected map[string]string content")
	}
	if content["error"] != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", content["error"])
	}
}

func TestNewTextResult(t *testing.T) {
	result := NewTextResult("test text")
	content, ok := result.Content.(map[string]string)
	if !ok {
		t.Error("Expected map[string]string content")
	}
	if content["text"] != "test text" {
		t.Errorf("Expected text 'test text', got '%s'", content["text"])
	}
}

// Benchmark Tests

func BenchmarkGenerateRequestID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateRequestID()
	}
}

func BenchmarkGenerateUUIDRequestID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateUUIDRequestID()
	}
}

func BenchmarkValidateSamplingRequest(b *testing.B) {
	req := &SamplingCreateMessageRequest{
		Method: "sampling/createMessage",
		Params: SamplingCreateMessageParams{
			Messages: []SamplingMessage{
				{Role: "user", Content: TextContent{Type: "text", Text: "Hello"}},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validateSamplingRequest(req)
	}
}
