// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"
	"trpc.group/trpc-go/trpc-mcp-go/sampling"
)

// ===============================================
// Sampling related type definitions
// ===============================================

// SamplingContent - Sampling message content interface
type SamplingContent interface {
	GetType() string
}

// SamplingTextContent - text content
type SamplingTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (t SamplingTextContent) GetType() string { return t.Type }

// SamplingImageContent - image content
type SamplingImageContent struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

func (i SamplingImageContent) GetType() string { return i.Type }

// SamplingMessage - Sampling message
type SamplingMessage struct {
	Role    string          `json:"role"` // "user", "assistant", "system"
	Content SamplingContent `json:"content"`
}

// SamplingModelPreferences - Model Preferences
type SamplingModelPreferences struct {
	Hints                []string `json:"hints,omitempty"`
	CostPriority         *float64 `json:"costPriority,omitempty"`         // 0-1
	SpeedPriority        *float64 `json:"speedPriority,omitempty"`        // 0-1
	IntelligencePriority *float64 `json:"intelligencePriority,omitempty"` // 0-1
}

// SamplingUsage - Token usage
type SamplingUsage struct {
	InputTokens  *int `json:"inputTokens,omitempty"`
	OutputTokens *int `json:"outputTokens,omitempty"`
	TotalTokens  *int `json:"totalTokens,omitempty"`
}

// SamplingCreateMessageParams - Sampling request parameters
type SamplingCreateMessageParams struct {
	Messages         []SamplingMessage         `json:"messages"`
	ModelPreferences *SamplingModelPreferences `json:"modelPreferences,omitempty"`
	SystemPrompt     *string                   `json:"systemPrompt,omitempty"`
	MaxTokens        *int                      `json:"maxTokens,omitempty"`
	Temperature      *float64                  `json:"temperature,omitempty"`
	StopSequences    []string                  `json:"stopSequences,omitempty"`
}

// SamplingCreateMessageResult - Sampling response
type SamplingCreateMessageResult struct {
	Role       string          `json:"role"`
	Content    SamplingContent `json:"content"`
	Model      string          `json:"model"`
	StopReason string          `json:"stopReason"`
	Usage      *SamplingUsage  `json:"usage,omitempty"`
}

// SamplingHandler - Sampling Processor Interface
type SamplingHandler interface {
	HandleSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error)
}

// SamplingSender - Sampling transmitter interface
type SamplingSender interface {
	SendSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error)
}

// ===============================================
// Client Sampling Support
// ===============================================

type SamplingClientConfig struct {
	DefaultModel        string            `json:"default_model"`
	AutoApprove         bool              `json:"auto_approve"`
	MaxTokensPerRequest int               `json:"max_tokens_per_request"`
	ModelMappings       map[string]string `json:"model_mappings"`
	TimeoutSeconds      int               `json:"timeout_seconds"`
}

// Extend the fields of an existing Client structure (via embedding)
type ClientSamplingSupport struct {
	SamplingHandler SamplingHandler       `json:"-"`
	samplingConfig  *SamplingClientConfig `json:"sampling_config,omitempty"`
	SamplingEnabled bool                  `json:"sampling_enabled"`
}

// Global mapping to store client Sampling support information
var ClientSamplingMap = make(map[*Client]*ClientSamplingSupport)

// WithSamplingHandler - Set the option function of the Sampling processor
func WithSamplingHandler(handler SamplingHandler) ClientOption {
	return func(c *Client) {
		if ClientSamplingMap[c] == nil {
			ClientSamplingMap[c] = &ClientSamplingSupport{}
		}
		ClientSamplingMap[c].SamplingHandler = handler
		ClientSamplingMap[c].SamplingEnabled = true
	}
}

// WithSamplingConfig - Set the option function of Sampling configuration
func WithSamplingConfig(config *SamplingClientConfig) ClientOption {
	return func(c *Client) {
		if ClientSamplingMap[c] == nil {
			ClientSamplingMap[c] = &ClientSamplingSupport{}
		}
		ClientSamplingMap[c].samplingConfig = config
		if config != nil {
			ClientSamplingMap[c].SamplingEnabled = true
		}
	}
}

// HandleSamplingRequest - Processing Sampling requests from the server
func (c *Client) HandleSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	samplingSupport := ClientSamplingMap[c]
	if samplingSupport == nil || !samplingSupport.SamplingEnabled {
		return nil, fmt.Errorf("sampling not enabled")
	}

	if samplingSupport.SamplingHandler == nil {
		return nil, fmt.Errorf("sampling handler not configured")
	}

	// Apply client configuration restrictions
	if samplingSupport.samplingConfig != nil {
		if req.Params.MaxTokens != nil && *req.Params.MaxTokens > samplingSupport.samplingConfig.MaxTokensPerRequest {
			return nil, fmt.Errorf("max tokens (%d) exceeds limit (%d)",
				*req.Params.MaxTokens, samplingSupport.samplingConfig.MaxTokensPerRequest)
		}
	}

	// Using timeout context
	timeout := 60 * time.Second
	if samplingSupport.samplingConfig != nil && samplingSupport.samplingConfig.TimeoutSeconds > 0 {
		timeout = time.Duration(samplingSupport.samplingConfig.TimeoutSeconds) * time.Second
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 委托给具体的处理器
	return samplingSupport.SamplingHandler.HandleSamplingRequest(ctxWithTimeout, req)
}

// GetSamplingConfig - Get Sampling Configuration
func (c *Client) GetSamplingConfig() *SamplingClientConfig {
	samplingSupport := ClientSamplingMap[c]
	if samplingSupport == nil {
		return nil
	}
	return samplingSupport.samplingConfig
}

// IsSamplingEnabled - Check whether Sampling is enabled
func (c *Client) IsSamplingEnabled() bool {
	samplingSupport := ClientSamplingMap[c]
	return samplingSupport != nil && samplingSupport.SamplingEnabled
}

// ===============================================
// Server Sampling Support
// ===============================================

// SamplingServerConfig - Server Sampling Configuration
type SamplingServerConfig struct {
	MaxTokensLimit      int      `json:"max_tokens_limit"`
	RateLimitPerMinute  int      `json:"rate_limit_per_minute"`
	AllowedContentTypes []string `json:"allowed_content_types"`
	RequireApproval     bool     `json:"require_approval"`
}

// Extend the fields of an existing Server structure (via embedding)
type serverSamplingSupport struct {
	SamplingEnabled bool                  `json:"sampling_enabled"`
	samplingConfig  *SamplingServerConfig `json:"sampling_config,omitempty"`
	SamplingHandler SamplingHandler
}

// Global mapping to store the server's sampling support information
var ServerSamplingMap = make(map[*Server]*serverSamplingSupport)

// WithSamplingEnabled - Option function to enable Sampling function
func WithSamplingEnabled(enabled bool) ServerOption {
	return func(s *Server) {
		if ServerSamplingMap[s] == nil {
			ServerSamplingMap[s] = &serverSamplingSupport{}
		}
		ServerSamplingMap[s].SamplingEnabled = enabled
	}
}

// WithSamplingConfigServer - Set the option function for Sampling configuration (server version)
func WithSamplingConfigServer(config *SamplingServerConfig) ServerOption {
	return func(s *Server) {
		if ServerSamplingMap[s] == nil {
			ServerSamplingMap[s] = &serverSamplingSupport{}
		}
		ServerSamplingMap[s].samplingConfig = config
		ServerSamplingMap[s].SamplingEnabled = true
	}
}

// SendSamplingRequest - Server implements the SamplingSender interface
func (s *Server) SendSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	// Check if Sampling is enabled
	samplingSupport := ServerSamplingMap[s]
	if samplingSupport == nil || !samplingSupport.SamplingEnabled {
		return nil, fmt.Errorf("sampling not enabled")
	}

	// Check if SamplingHandler exists
	if samplingSupport.SamplingHandler == nil {
		return nil, fmt.Errorf("sampling handler not configured")
	}

	// Call SamplingHandler to process the request
	return samplingSupport.SamplingHandler.HandleSamplingRequest(ctx, req)
}

// IsSamplingEnabled - Check whether Sampling is enabled
func (s *Server) IsSamplingEnabled() bool {
	samplingSupport := ServerSamplingMap[s]
	return samplingSupport != nil && samplingSupport.SamplingEnabled
}

// GetSamplingConfig - Get Sampling Configuration
func (s *Server) GetSamplingConfig() *SamplingServerConfig {
	samplingSupport := ServerSamplingMap[s]
	if samplingSupport == nil {
		return nil
	}
	return samplingSupport.samplingConfig
}

// ===============================================
// Sampling context support
// ===============================================

// Context keys for sampling
type samplingContextKey string

const (
	SamplingSenderKey samplingContextKey = "sampling_sender"
)

// GetSamplingSender - Get the Sampling sender from the context
func GetSamplingSender(ctx context.Context) (SamplingSender, bool) {
	sender, ok := ctx.Value(SamplingSenderKey).(SamplingSender)
	return sender, ok
}

// SetSamplingSender - Set the Sampling sender to the context
func SetSamplingSender(ctx context.Context, sender *Server) context.Context {
	return context.WithValue(ctx, SamplingSenderKey, sender)
}

// ===============================================
// Default Sampling implementation
// ===============================================

type DefaultSamplingHandler struct {
	config *SamplingClientConfig
}

// NewDefaultSamplingHandler - Creating a default Sampling processor
func NewDefaultSamplingHandler(config *SamplingClientConfig) SamplingHandler {
	if config == nil {
		config = &SamplingClientConfig{
			DefaultModel:        "gpt-3.5-turbo",
			AutoApprove:         false,
			MaxTokensPerRequest: 2000,
			TimeoutSeconds:      60,
		}
	}
	return &DefaultSamplingHandler{
		config: config,
	}
}

// HandleSamplingRequest - Processing Sampling requests (simulation implementation)
func (h *DefaultSamplingHandler) HandleSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	model := h.config.DefaultModel

	// Check Model Hints
	if req.Params.ModelPreferences != nil && len(req.Params.ModelPreferences.Hints) > 0 {
		for _, hint := range req.Params.ModelPreferences.Hints {
			if mappedModel, exists := h.config.ModelMappings[hint]; exists {
				model = mappedModel
				break
			}
		}
	}

	// Mock response
	responseText := "这是一个模拟的AI响应。在实际使用时，这里应该调用真实的LLM API。"
	if len(req.Params.Messages) > 0 {
		if textContent, ok := req.Params.Messages[len(req.Params.Messages)-1].Content.(SamplingTextContent); ok {
			responseText = fmt.Sprintf("收到您的消息：%s\n\n这是一个模拟的AI回复。", textContent.Text)
		}
	}

	return &SamplingCreateMessageResult{
		Role: "assistant",
		Content: SamplingTextContent{
			Type: "text",
			Text: responseText,
		},
		Model:      model,
		StopReason: "stop",
		Usage: &SamplingUsage{
			InputTokens:  intPtr(100),
			OutputTokens: intPtr(50),
			TotalTokens:  intPtr(150),
		},
	}, nil
}

// ===============================================
// OpenAI API structure definition
// ===============================================

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model            string          `json:"model"`
	Messages         []OpenAIMessage `json:"messages"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
	Stream           bool            `json:"stream"`
	User             string          `json:"user,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Usage   OpenAIUsage    `json:"usage"`
	Choices []OpenAIChoice `json:"choices"`
	Error   *OpenAIError   `json:"error,omitempty"`
}

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// ===============================================
// Claude API structure definition
// ===============================================

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeRequest struct {
	Model         string          `json:"model"`
	MaxTokens     int             `json:"max_tokens"`
	Messages      []ClaudeMessage `json:"messages"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	System        string          `json:"system,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ClaudeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ClaudeResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Content      []ClaudeContent `json:"content"`
	Model        string          `json:"model"`
	StopReason   string          `json:"stop_reason"`
	StopSequence string          `json:"stop_sequence,omitempty"`
	Usage        ClaudeUsage     `json:"usage"`
	Error        *ClaudeError    `json:"error,omitempty"`
}

type ClaudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ===============================================
// Local Model Ollama Support
// ===============================================

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type OllamaResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
	Error     string        `json:"error,omitempty"`
}

type OllamaHandler struct {
	baseURL    string
	config     *SamplingClientConfig
	httpClient *http.Client
}

// ===============================================
// Real LLM Handler Implementation
// ===============================================

type RealLLMHandler struct {
	openaiAPIKey  string
	claudeAPIKey  string
	config        *SamplingClientConfig
	httpClient    *http.Client
	openaiBaseURL string
	claudeBaseURL string
}

func NewRealLLMHandler(openaiKey, claudeKey string, config *SamplingClientConfig) SamplingHandler {
	if config == nil {
		config = &SamplingClientConfig{
			DefaultModel:        "gpt-3.5-turbo",
			AutoApprove:         false,
			MaxTokensPerRequest: 4000,
			ModelMappings: map[string]string{
				"claude-3-sonnet": "claude-3-sonnet-20240229",
				"claude-3-haiku":  "claude-3-haiku-20240307",
				"gpt-4":           "gpt-4",
				"gpt-3.5-turbo":   "gpt-3.5-turbo",
				"gpt-4-turbo":     "gpt-4-turbo-preview",
			},
			TimeoutSeconds: 120,
		}
	}

	return &RealLLMHandler{
		openaiAPIKey:  openaiKey,
		claudeAPIKey:  claudeKey,
		config:        config,
		openaiBaseURL: "https://api.openai.com/v1",
		claudeBaseURL: "https://api.anthropic.com/v1",
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
		},
	}
}

func (h *RealLLMHandler) HandleSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	// Verify Request
	if err := h.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Select Model
	model := h.selectModel(req.Params.ModelPreferences)

	//Call different APIs based on model type
	if strings.Contains(model, "claude") {
		return h.callClaudeAPI(ctx, req, model)
	} else if strings.Contains(model, "gpt") {
		return h.callOpenAIAPI(ctx, req, model)
	}

	return nil, fmt.Errorf("unsupported model: %s", model)
}

func (h *RealLLMHandler) selectModel(prefs *sampling.ModelPreferences) string {
	if prefs == nil {
		return h.config.DefaultModel
	}

	// Check hints
	for _, hint := range prefs.Hints {
		if mappedModel, exists := h.config.ModelMappings[hint]; exists {
			return mappedModel
		}
	}

	// Select based on priority
	if prefs.IntelligencePriority != nil && *prefs.IntelligencePriority > 0.8 {
		if h.claudeAPIKey != "" {
			return "claude-3-sonnet-20240229"
		}
		return "gpt-4"
	}
	if prefs.SpeedPriority != nil && *prefs.SpeedPriority > 0.8 {
		if h.claudeAPIKey != "" {
			return "claude-3-haiku-20240307"
		}
		return "gpt-3.5-turbo"
	}
	if prefs.CostPriority != nil && *prefs.CostPriority > 0.8 {
		return "gpt-3.5-turbo"
	}

	return h.config.DefaultModel
}

func (h *RealLLMHandler) callOpenAIAPI(ctx context.Context, req *sampling.SamplingCreateMessageRequest, model string) (*SamplingCreateMessageResult, error) {
	if h.openaiAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Convert message format
	var messages []OpenAIMessage
	for _, msg := range req.Params.Messages {
		if textContent, ok := msg.Content.(SamplingTextContent); ok {
			messages = append(messages, OpenAIMessage{
				Role:    msg.Role,
				Content: textContent.Text,
			})
		}
	}

	// Add system prompt
	if req.Params.SystemPrompt != nil && *req.Params.SystemPrompt != "" {
		messages = append([]OpenAIMessage{{
			Role:    "system",
			Content: *req.Params.SystemPrompt,
		}}, messages...)
	}

	// Building a request
	openaiReq := OpenAIRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.Params.MaxTokens,
		Temperature: req.Params.Temperature,
		Stop:        req.Params.StopSequences,
		Stream:      false,
	}

	//Send Request
	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", h.openaiBaseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+h.openaiAPIKey)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if openaiResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from OpenAI")
	}

	choice := openaiResp.Choices[0]
	return &SamplingCreateMessageResult{
		Role: choice.Message.Role,
		Content: SamplingTextContent{
			Type: "text",
			Text: choice.Message.Content,
		},
		Model:      openaiResp.Model,
		StopReason: choice.FinishReason,
		Usage: &SamplingUsage{
			InputTokens:  &openaiResp.Usage.PromptTokens,
			OutputTokens: &openaiResp.Usage.CompletionTokens,
			TotalTokens:  &openaiResp.Usage.TotalTokens,
		},
	}, nil
}

func (h *RealLLMHandler) callClaudeAPI(ctx context.Context, req *sampling.SamplingCreateMessageRequest, model string) (*SamplingCreateMessageResult, error) {
	if h.claudeAPIKey == "" {
		return nil, fmt.Errorf("Claude API key not configured")
	}

	// Convert message format
	var messages []ClaudeMessage
	for _, msg := range req.Params.Messages {
		if textContent, ok := msg.Content.(SamplingTextContent); ok {
			// Claude API 不支持 system 角色在 messages 中
			if msg.Role != "system" {
				messages = append(messages, ClaudeMessage{
					Role:    msg.Role,
					Content: textContent.Text,
				})
			}
		}
	}

	// Set default max_tokens
	maxTokens := 1000
	if req.Params.MaxTokens != nil {
		maxTokens = *req.Params.MaxTokens
	}

	// Building a request
	claudeReq := ClaudeRequest{
		Model:         model,
		MaxTokens:     maxTokens,
		Messages:      messages,
		Temperature:   req.Params.Temperature,
		StopSequences: req.Params.StopSequences,
	}

	// Add system prompt
	if req.Params.SystemPrompt != nil && *req.Params.SystemPrompt != "" {
		claudeReq.System = *req.Params.SystemPrompt
	}

	// Send Request
	reqBody, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", h.claudeBaseURL+"/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", h.claudeAPIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if claudeResp.Error != nil {
		return nil, fmt.Errorf("Claude API error: %s", claudeResp.Error.Message)
	}

	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("no content returned from Claude")
	}

	content := claudeResp.Content[0]
	return &SamplingCreateMessageResult{
		Role: claudeResp.Role,
		Content: SamplingTextContent{
			Type: "text",
			Text: content.Text,
		},
		Model:      claudeResp.Model,
		StopReason: claudeResp.StopReason,
		Usage: &SamplingUsage{
			InputTokens:  &claudeResp.Usage.InputTokens,
			OutputTokens: &claudeResp.Usage.OutputTokens,
			TotalTokens:  func() *int { total := claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens; return &total }(),
		},
	}, nil
}

func (h *RealLLMHandler) validateRequest(req *sampling.SamplingCreateMessageRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.Method != "sampling/createMessage" {
		return fmt.Errorf("invalid method: %s", req.Method)
	}

	if len(req.Params.Messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}

	// Verification Token Limitations
	if req.Params.MaxTokens != nil && *req.Params.MaxTokens > h.config.MaxTokensPerRequest {
		return fmt.Errorf("max tokens (%d) exceeds limit (%d)", *req.Params.MaxTokens, h.config.MaxTokensPerRequest)
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

	return nil
}

func NewOllamaHandler(baseURL string, config *SamplingClientConfig) SamplingHandler {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	if config == nil {
		config = &SamplingClientConfig{
			DefaultModel:        "llama2",
			AutoApprove:         false,
			MaxTokensPerRequest: 4000,
			ModelMappings: map[string]string{
				"claude-3-sonnet": "llama2",
				"claude-3-haiku":  "llama2:7b",
				"gpt-4":           "llama2:13b",
				"gpt-3.5-turbo":   "llama2:7b",
			},
			TimeoutSeconds: 120,
		}
	}

	return &OllamaHandler{
		baseURL: baseURL,
		config:  config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
		},
	}
}

func (h *OllamaHandler) HandleSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	// Select Model
	model := h.selectModel(req.Params.ModelPreferences)

	// Convert message format
	var messages []OllamaMessage
	for _, msg := range req.Params.Messages {
		if textContent, ok := msg.Content.(SamplingTextContent); ok {
			messages = append(messages, OllamaMessage{
				Role:    msg.Role,
				Content: textContent.Text,
			})
		}
	}

	// Add system prompt
	if req.Params.SystemPrompt != nil && *req.Params.SystemPrompt != "" {
		messages = append([]OllamaMessage{{
			Role:    "system",
			Content: *req.Params.SystemPrompt,
		}}, messages...)
	}

	// Building a request
	ollamaReq := OllamaRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Options:  make(map[string]interface{}),
	}

	// 设置选项
	if req.Params.Temperature != nil {
		ollamaReq.Options["temperature"] = *req.Params.Temperature
	}
	if req.Params.MaxTokens != nil {
		ollamaReq.Options["num_predict"] = *req.Params.MaxTokens
	}

	// 发送请求
	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", h.baseURL+"/api/chat", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if ollamaResp.Error != "" {
		return nil, fmt.Errorf("Ollama error: %s", ollamaResp.Error)
	}

	return &SamplingCreateMessageResult{
		Role: ollamaResp.Message.Role,
		Content: SamplingTextContent{
			Type: "text",
			Text: ollamaResp.Message.Content,
		},
		Model:      ollamaResp.Model,
		StopReason: "stop",
		Usage: &SamplingUsage{
			InputTokens:  IntPtr(0),
			OutputTokens: IntPtr(0),
			TotalTokens:  IntPtr(0),
		},
	}, nil
}

func (h *OllamaHandler) selectModel(prefs *sampling.ModelPreferences) string {
	if prefs == nil {
		return h.config.DefaultModel
	}

	// Inspection Tips
	for _, hint := range prefs.Hints {
		if mappedModel, exists := h.config.ModelMappings[hint]; exists {
			return mappedModel
		}
	}

	return h.config.DefaultModel
}

// ===============================================
// OpenAI Sampling processor implementation
// ===============================================

// OpenAISamplingHandler - Sampling processor integrated with OpenAI API
type OpenAISamplingHandler struct {
	apiKey  string
	baseURL string
	config  *SamplingClientConfig
}

// NewOpenAISamplingHandler - Creating the OpenAI Sampling Processor
func NewOpenAISamplingHandler(apiKey string, config *SamplingClientConfig) SamplingHandler {
	if config == nil {
		config = &SamplingClientConfig{
			DefaultModel:        "gpt-3.5-turbo",
			AutoApprove:         false,
			MaxTokensPerRequest: 2000,
			ModelMappings: map[string]string{
				"claude-3-sonnet": "gpt-4",
				"claude-3-haiku":  "gpt-3.5-turbo",
			},
			TimeoutSeconds: 60,
		}
	}

	return &OpenAISamplingHandler{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		config:  config,
	}
}

// HandleSamplingRequest - Handling Sampling Requests (OpenAI Integration)
func (h *OpenAISamplingHandler) HandleSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	// Verify Request
	if err := validateSamplingRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	//Select Model
	model := h.selectModel(req.Params.ModelPreferences)

	//Simulating OpenAI API calls
	responseText := h.generateMockResponse(req, model)

	return &SamplingCreateMessageResult{
		Role: "assistant",
		Content: SamplingTextContent{
			Type: "text",
			Text: responseText,
		},
		Model:      model,
		StopReason: "stop",
		Usage: &SamplingUsage{
			InputTokens:  IntPtr(100),
			OutputTokens: IntPtr(50),
			TotalTokens:  IntPtr(150),
		},
	}, nil
}

// selectModel - Select model
func (h *OpenAISamplingHandler) selectModel(prefs *sampling.ModelPreferences) string {
	if prefs == nil {
		return h.config.DefaultModel
	}

	// Inspection Tips
	for _, hint := range prefs.Hints {
		if mappedModel, exists := h.config.ModelMappings[hint]; exists {
			return mappedModel
		}
	}

	// Select based on priority
	if prefs.IntelligencePriority != nil && *prefs.IntelligencePriority > 0.8 {
		return "gpt-4"
	}
	if prefs.SpeedPriority != nil && *prefs.SpeedPriority > 0.8 {
		return "gpt-3.5-turbo"
	}
	if prefs.CostPriority != nil && *prefs.CostPriority > 0.8 {
		return "gpt-3.5-turbo"
	}

	return h.config.DefaultModel
}

// generateMockResponse - Generate simulated responses
func (h *OpenAISamplingHandler) generateMockResponse(req *sampling.SamplingCreateMessageRequest, model string) string {
	if len(req.Params.Messages) == 0 {
		return "Hello! How can I help you today?"
	}

	lastMessage := req.Params.Messages[len(req.Params.Messages)-1]
	if textContent, ok := lastMessage.Content.(SamplingTextContent); ok {
		return fmt.Sprintf("Using the Model %s Processing your request：%s\n\nThis is a simulated AI response，demonstrating how the MCP Sampling feature works.", model, textContent.Text)
	}

	return "This is a simulated AI response。"
}

// ===============================================
// Adapter support (handling external processors)
// ===============================================

// SamplingHandlerAdapter - Adapter structure
type SamplingHandlerAdapter struct {
	handler interface{}
}

// NewSamplingHandlerAdapter - Creating an Adapter
func NewSamplingHandlerAdapter(handler interface{}) SamplingHandler {
	return &SamplingHandlerAdapter{handler: handler}
}

// HandleSamplingRequest - Adapter Implementation
func (a *SamplingHandlerAdapter) HandleSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	// Using reflection to call external processors
	handlerValue := reflect.ValueOf(a.handler)
	if handlerValue.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("handler must be a pointer")
	}

	method := handlerValue.MethodByName("HandleSamplingRequest")
	if !method.IsValid() {
		return nil, fmt.Errorf("handler does not have HandleSamplingRequest method")
	}

	//Calling Methods
	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(req),
	})

	if len(results) != 2 {
		return nil, fmt.Errorf("unexpected number of return values")
	}

	// Handling return values
	var result *SamplingCreateMessageResult
	var err error

	if !results[0].IsNil() {
		if r, ok := results[0].Interface().(*SamplingCreateMessageResult); ok {
			result = r
		} else {
			return nil, fmt.Errorf("unexpected result type")
		}
	}

	if !results[1].IsNil() {
		if e, ok := results[1].Interface().(error); ok {
			err = e
		}
	}

	return result, err
}

// WrapSamplingHandler - Packaging External Processors
func WrapSamplingHandler(handler interface{}) SamplingHandler {
	return NewSamplingHandlerAdapter(handler)
}

// WithExternalSamplingHandler - Option functions to support external processors
func WithExternalSamplingHandler(handler interface{}) ClientOption {
	return func(c *Client) {
		if ClientSamplingMap[c] == nil {
			ClientSamplingMap[c] = &ClientSamplingSupport{}
		}
		ClientSamplingMap[c].SamplingHandler = WrapSamplingHandler(handler)
		ClientSamplingMap[c].SamplingEnabled = true
	}
}

// ===============================================
// Utility Functions
// ===============================================

// IntPtr FloatPtr StringPtr - Pointer utility functions
func IntPtr(i int) *int           { return &i }
func FloatPtr(f float64) *float64 { return &f }
func StringPtr(s string) *string  { return &s }

// GenerateRequestID - Request ID Generator
func GenerateRequestID() int64 {
	return time.Now().UnixNano()
}

// containsString - String Contains Check
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// validateSamplingRequest - Verify Sampling Request
func validateSamplingRequest(req *sampling.SamplingCreateMessageRequest) error {
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

	return nil
}

// ===============================================
// Cleanup Function
// ===============================================

// CleanupClientSampling - Clean up client sampling support
func CleanupClientSampling(c *Client) {
	delete(ClientSamplingMap, c)
}

// CleanupServerSampling - Clean up server sampling support
func CleanupServerSampling(s *Server) {
	delete(ServerSamplingMap, s)
}

// ===============================================
// Convenience constructor
// ===============================================

// NewSamplingHandler - Creating a default Sampling processor (convenience method)
func NewSamplingHandler(config *SamplingClientConfig) SamplingHandler {
	return NewDefaultSamplingHandler(config)
}

// NewOpenAIHandler - Convenient method for creating OpenAI processors
func NewOpenAIHandler(apiKey string, config *SamplingClientConfig) SamplingHandler {
	return NewOpenAISamplingHandler(apiKey, config)
}

// ===============================================
//Backwards-compatible function aliases
// ===============================================

// Keep the original lowercase function names to be compatible with existing code
func intPtr(i int) *int           { return IntPtr(i) }
func floatPtr(f float64) *float64 { return FloatPtr(f) }
func stringPtr(s string) *string  { return StringPtr(s) }

// RegisterSamplingHandler registers a Sampling processor
func (s *Server) RegisterSamplingHandler(handler SamplingHandler) {
	if ServerSamplingMap[s] == nil {
		ServerSamplingMap[s] = &serverSamplingSupport{}
	}
	ServerSamplingMap[s].SamplingHandler = handler
	ServerSamplingMap[s].SamplingEnabled = true
}
