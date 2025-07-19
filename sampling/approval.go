// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package sampling

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// InteractiveUserApprovalService - Interactive User Approval Service
type InteractiveUserApprovalService struct {
	reader *bufio.Reader
}

// NewInteractiveUserApprovalService - Creating an interactive user approval service
func NewInteractiveUserApprovalService() *InteractiveUserApprovalService {
	return &InteractiveUserApprovalService{
		reader: bufio.NewReader(os.Stdin),
	}
}

// RequestApproval - Request user approval
func (s *InteractiveUserApprovalService) RequestApproval(ctx context.Context, req *SamplingCreateMessageRequest) (bool, error) {
	// 1. Content security check
	if s.hasBlockedContent(req) {
		fmt.Println("The request contains prohibited content and is automatically rejected")
		return false, nil
	}

	// 2. Display request information
	s.displayRequest(req)

	// 3. Get user confirmation
	return s.getUserConfirmation()
}

// hasBlockedContent - Check for prohibited content
func (s *InteractiveUserApprovalService) hasBlockedContent(req *SamplingCreateMessageRequest) bool {
	blockedKeywords := []string{
		"hack", "exploit", "malware", "virus", "attack",
	}

	// Check the message content
	for _, msg := range req.Params.Messages {
		if textContent, ok := msg.Content.(TextContent); ok {
			text := strings.ToLower(textContent.Text)
			for _, keyword := range blockedKeywords {
				if strings.Contains(text, strings.ToLower(keyword)) {
					return true
				}
			}
		}
	}

	// Check system prompts
	if req.Params.SystemPrompt != nil {
		text := strings.ToLower(*req.Params.SystemPrompt)
		for _, keyword := range blockedKeywords {
			if strings.Contains(text, strings.ToLower(keyword)) {
				return true
			}
		}
	}

	return false
}

// displayRequest - Display request information
func (s *InteractiveUserApprovalService) displayRequest(req *SamplingCreateMessageRequest) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("MCP Sampling Request a review")
	fmt.Println(strings.Repeat("=", 50))

	// 显示请求ID和方法
	fmt.Printf("Request ID: %v\n", req.ID)
	fmt.Printf("Method: %s\n", req.Method)

	// Display Message
	fmt.Printf("Number of messages: %d\n", len(req.Params.Messages))
	for i, msg := range req.Params.Messages {
		fmt.Printf("  Message %d [%s]: %s\n", i+1, msg.Role, s.getContentPreview(msg.Content))
	}

	// Display Model Preferences
	if prefs := req.Params.ModelPreferences; prefs != nil {
		fmt.Println("Model Preference:")
		if len(prefs.Hints) > 0 {
			fmt.Printf("  Hints: %v\n", prefs.Hints)
		}
		if prefs.IntelligencePriority != nil {
			fmt.Printf("  Intelligence Prioritization: %.2f\n", *prefs.IntelligencePriority)
		}
		if prefs.SpeedPriority != nil {
			fmt.Printf("  Speed Priority: %.2f\n", *prefs.SpeedPriority)
		}
		if prefs.CostPriority != nil {
			fmt.Printf("  Cost Priority: %.2f\n", *prefs.CostPriority)
		}
	}

	// Display other parameters
	if req.Params.MaxTokens != nil {
		fmt.Printf("Max Token: %d\n", *req.Params.MaxTokens)
	}
	if req.Params.Temperature != nil {
		fmt.Printf("Temperature: %.2f\n", *req.Params.Temperature)
	}
	if req.Params.SystemPrompt != nil {
		fmt.Printf("System prompts: %s\n", *req.Params.SystemPrompt)
	}

	fmt.Println(strings.Repeat("=", 50))
}

// getContentPreview - Get content preview
func (s *InteractiveUserApprovalService) getContentPreview(content Content) string {
	switch c := content.(type) {
	case TextContent:
		text := c.Text
		if len(text) > 100 {
			return text[:100] + "..."
		}
		return text
	case ImageContent:
		return fmt.Sprintf("[Image content - %s]", c.MimeType)
	case AudioContent:
		return fmt.Sprintf("[Audio content - %s]", c.MimeType)
	default:
		return fmt.Sprintf("[%s Content]", content.GetType())
	}
}

// getUserConfirmation - Get user confirmation
func (s *InteractiveUserApprovalService) getUserConfirmation() (bool, error) {
	fmt.Print("Do you want to approve this Sampling request? (y/n): ")

	input, err := s.reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	input = strings.ToLower(strings.TrimSpace(input))

	switch input {
	case "y", "yes":
		fmt.Println("Request approved")
		return true, nil
	case "n", "no":
		fmt.Println("Request denied")
		return false, nil
	default:
		fmt.Println("Invalid input, please enter y(yes) or n(no)")
		return s.getUserConfirmation() // Recursive Retry
	}
}

// ContentFilterRule - Content filtering rules
type ContentFilterRule struct {
	Name            string   `json:"name"`
	BlockedKeywords []string `json:"blockedKeywords"`
	AllowedModels   []string `json:"allowedModels"`
	MaxTokens       *int     `json:"maxTokens,omitempty"`
	Enabled         bool     `json:"enabled"`
}

// AdvancedUserApprovalService - Advanced User Approval Service (optional)
type AdvancedUserApprovalService struct {
	reader      *bufio.Reader
	filterRules []ContentFilterRule
}

// NewAdvancedUserApprovalService - Creating a Power User Approval Service
func NewAdvancedUserApprovalService(rules []ContentFilterRule) *AdvancedUserApprovalService {
	return &AdvancedUserApprovalService{
		reader:      bufio.NewReader(os.Stdin),
		filterRules: rules,
	}
}

// RequestApproval - Advanced Approval Process
func (s *AdvancedUserApprovalService) RequestApproval(ctx context.Context, req *SamplingCreateMessageRequest) (bool, error) {
	// 1. Apply filter rules
	for _, rule := range s.filterRules {
		if !rule.Enabled {
			continue
		}

		if blocked, reason := s.evaluateRule(rule, req); blocked {
			fmt.Printf("Request to be ruled '%s' reject: %s\n", rule.Name, reason)
			return false, nil
		}
	}

	// 2. Display request information and get user confirmation
	s.displayAdvancedRequest(req)
	return s.getUserConfirmation()
}

// evaluateRule - Evaluating filter rules
func (s *AdvancedUserApprovalService) evaluateRule(rule ContentFilterRule, req *SamplingCreateMessageRequest) (bool, string) {
	// Check keywords
	for _, msg := range req.Params.Messages {
		if textContent, ok := msg.Content.(TextContent); ok {
			text := strings.ToLower(textContent.Text)
			for _, keyword := range rule.BlockedKeywords {
				if strings.Contains(text, strings.ToLower(keyword)) {
					return true, fmt.Sprintf("Contains banned keywords: %s", keyword)
				}
			}
		}
	}

	// Check Token Limits
	if rule.MaxTokens != nil && req.Params.MaxTokens != nil {
		if *req.Params.MaxTokens > *rule.MaxTokens {
			return true, fmt.Sprintf("Exceeding the Token Limit: %d > %d", *req.Params.MaxTokens, *rule.MaxTokens)
		}
	}

	// 检查模型限制
	if len(rule.AllowedModels) > 0 && req.Params.ModelPreferences != nil {
		modelAllowed := false
		for _, hint := range req.Params.ModelPreferences.Hints {
			for _, allowed := range rule.AllowedModels {
				if strings.Contains(strings.ToLower(hint), strings.ToLower(allowed)) {
					modelAllowed = true
					break
				}
			}
			if modelAllowed {
				break
			}
		}
		if !modelAllowed {
			return true, fmt.Sprintf("模型不在允许列表中: %v", rule.AllowedModels)
		}
	}

	return false, ""
}

// displayAdvancedRequest - Show advanced request information
func (s *AdvancedUserApprovalService) displayAdvancedRequest(req *SamplingCreateMessageRequest) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Advanced MCP Sampling Request Review")
	fmt.Println(strings.Repeat("=", 60))

	// 显示基本信息
	fmt.Printf("Request ID: %v\n", req.ID)
	fmt.Printf("Method: %s\n", req.Method)

	// 显示消息详情
	fmt.Printf("Number of messages: %d\n", len(req.Params.Messages))
	for i, msg := range req.Params.Messages {
		fmt.Printf("  %d. [%s] %s\n", i+1, msg.Role, s.getContentPreview(msg.Content))
	}

	// Show model preference details
	if prefs := req.Params.ModelPreferences; prefs != nil {
		fmt.Println("Model Preferences:")
		if len(prefs.Hints) > 0 {
			fmt.Printf(" Prompt Model: %v\n", prefs.Hints)
		}
		if prefs.IntelligencePriority != nil {
			fmt.Printf("  Intelligence Prioritization: %.1f%%\n", *prefs.IntelligencePriority*100)
		}
		if prefs.SpeedPriority != nil {
			fmt.Printf("  Speed Priority: %.1f%%\n", *prefs.SpeedPriority*100)
		}
		if prefs.CostPriority != nil {
			fmt.Printf("  Cost Priority: %.1f%%\n", *prefs.CostPriority*100)
		}
	}

	// 显示技术参数
	fmt.Println(" Technical Parameters:")
	if req.Params.MaxTokens != nil {
		fmt.Printf("  Max Token: %d\n", *req.Params.MaxTokens)
	}
	if req.Params.Temperature != nil {
		fmt.Printf("  Temperature: %.2f\n", *req.Params.Temperature)
	}
	if req.Params.SystemPrompt != nil {
		fmt.Printf("  System prompt: %s\n", *req.Params.SystemPrompt)
	}

	fmt.Println(strings.Repeat("=", 60))
}

// getUserConfirmation - Get user confirmation (reuse)
func (s *AdvancedUserApprovalService) getUserConfirmation() (bool, error) {
	fmt.Print("Do you approve this sampling request? (y/n/d[details]): ")

	input, err := s.reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	input = strings.ToLower(strings.TrimSpace(input))

	switch input {
	case "y", "yes":
		fmt.Println(" Request approved")
		return true, nil
	case "n", "no":
		fmt.Println(" Request denied")
		return false, nil
	case "d", "details":
		fmt.Println(" Details are shown above")
		return s.getUserConfirmation()
	default:
		fmt.Println(" Invalid input, please enter y(yes), n(no) or d(details)")
		return s.getUserConfirmation()
	}
}

// getContentPreview - Get content preview (reuse)
func (s *AdvancedUserApprovalService) getContentPreview(content Content) string {
	switch c := content.(type) {
	case TextContent:
		text := c.Text
		if len(text) > 100 {
			return text[:100] + "..."
		}
		return text
	case ImageContent:
		return fmt.Sprintf("[Image content - %s]", c.MimeType)
	case AudioContent:
		return fmt.Sprintf("[Audio content - %s]", c.MimeType)
	default:
		return fmt.Sprintf("[%s Content]", content.GetType())
	}
}

// GetDefaultFilterRules - Predefined filtering rules
func GetDefaultFilterRules() []ContentFilterRule {
	return []ContentFilterRule{
		{
			Name: "Basic safety filtering",
			BlockedKeywords: []string{
				"hack", "exploit", "malware", "virus",
			},
			AllowedModels: []string{
				"gpt-3.5", "gpt-4", "claude", "gemini",
			},
			MaxTokens: intPtr(2000),
			Enabled:   true,
		},
		{
			Name: "Content Review",
			BlockedKeywords: []string{
				"violence", "pornography", "gambling",
			},
			Enabled: true,
		},
	}
}
