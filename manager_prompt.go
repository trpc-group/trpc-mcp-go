// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"sync"

	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// promptManager manages prompt templates
//
// Prompt functionality follows these enabling mechanisms:
//  1. By default, prompt functionality is disabled
//  2. When the first prompt is registered, prompt functionality is automatically enabled without
//     additional configuration
//  3. When prompt functionality is enabled but no prompts exist, ListPrompts will return an empty
//     prompt list rather than an error
//  4. Clients can determine if the server supports prompt functionality through the capabilities
//     field in the initialization response
//
// This design simplifies API usage, eliminating the need for explicit configuration parameters to
// enable or disable prompt functionality.
type promptManager struct {
	// Prompt mapping table
	prompts map[string]*registeredPrompt

	// Mutex
	mu sync.RWMutex

	// Track insertion order of prompts
	promptsOrder []string

	// Prompt list filter function
	promptListFilter PromptListFilter
}

// newPromptManager creates a new prompt manager
//
// Note: Simply creating a prompt manager does not enable prompt functionality,
// it is only enabled when the first prompt is added.
func newPromptManager() *promptManager {
	return &promptManager{
		prompts: make(map[string]*registeredPrompt),
	}
}

// withPromptListFilter sets the prompt list filter.
func (m *promptManager) withPromptListFilter(filter PromptListFilter) *promptManager {
	m.promptListFilter = filter
	return m
}

// registerPrompt registers a prompt
func (m *promptManager) registerPrompt(prompt *Prompt, handler promptHandler, options ...registerdPromptOption) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if prompt == nil || prompt.Name == "" {
		return
	}

	if _, exists := m.prompts[prompt.Name]; !exists {
		// Only add to order slice if it's a new prompt
		m.promptsOrder = append(m.promptsOrder, prompt.Name)
	}

	m.prompts[prompt.Name] = &registeredPrompt{
		Prompt:  prompt,
		Handler: handler,
	}

	// Apply options to the registered prompt
	for _, opt := range options {
		opt(m.prompts[prompt.Name])
	}
}

// getPrompt retrieves a prompt
func (m *promptManager) getPrompt(name string) (*Prompt, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	registeredPrompt, exists := m.prompts[name]
	if !exists {
		return nil, false
	}
	return registeredPrompt.Prompt, true
}

// getPrompts retrieves all prompts
func (m *promptManager) getPrompts() []*Prompt {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prompts := make([]*Prompt, 0, len(m.prompts))
	for _, registeredPrompt := range m.prompts {
		prompts = append(prompts, registeredPrompt.Prompt)
	}
	return prompts
}

func (m *promptManager) hasCompletionCompleteHandler() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, registeredPrompt := range m.prompts {
		if registeredPrompt.CompletionCompleteHandler != nil {
			return true
		}
	}
	return false
}

// handleListPrompts handles listing prompts requests
func (m *promptManager) handleListPrompts(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
	// Get all prompts
	promptPtrs := m.getPrompts()

	// Apply filter if available
	if m.promptListFilter != nil {
		promptPtrs = m.promptListFilter(ctx, promptPtrs)
	}

	// Convert []*mcp.Prompt to []mcp.Prompt for the result
	resultPrompts := make([]Prompt, len(promptPtrs))
	for i, prompt := range promptPtrs {
		if prompt != nil {
			resultPrompts[i] = *prompt
		}
	}

	result := &ListPromptsResult{
		Prompts: resultPrompts,
	}

	return result, nil
}

// Helper: Parse and validate parameters for GetPrompt
func parseGetPromptParams(req *JSONRPCRequest) (name string, arguments map[string]interface{}, errResp JSONRPCMessage, ok bool) {
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return "", nil, newJSONRPCErrorResponse(
			req.ID,
			ErrCodeInvalidParams,
			errors.ErrInvalidParams.Error(),
			nil,
		), false
	}
	name, ok = paramsMap["name"].(string)
	if !ok {
		return "", nil, newJSONRPCErrorResponse(
			req.ID,
			ErrCodeInvalidParams,
			errors.ErrMissingParams.Error(),
			nil,
		), false
	}
	arguments, _ = paramsMap["arguments"].(map[string]interface{})
	return name, arguments, nil, true
}

// Helper: Build prompt messages for GetPrompt
func buildPromptMessages(prompt *Prompt, arguments map[string]interface{}) []PromptMessage {
	messages := []PromptMessage{}
	userPrompt := fmt.Sprintf("This is an example rendering of the %s prompt.", prompt.Name)
	for _, arg := range prompt.Arguments {
		if value, ok := arguments[arg.Name]; ok {
			userPrompt += fmt.Sprintf("\nParameter %s: %v", arg.Name, value)
		} else if arg.Required {
			userPrompt += fmt.Sprintf("\nParameter %s: [not provided]", arg.Name)
		}
	}
	messages = append(messages, PromptMessage{
		Role: "user",
		Content: TextContent{
			Type: "text",
			Text: userPrompt,
		},
	})
	return messages
}

// Refactored: handleGetPrompt with logic unchanged, now using helpers
func (m *promptManager) handleGetPrompt(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
	name, arguments, errResp, ok := parseGetPromptParams(req)
	if !ok {
		return errResp, nil
	}
	registeredPrompt, exists := m.prompts[name]
	if !exists {
		return newJSONRPCErrorResponse(
			req.ID,
			ErrCodeMethodNotFound,
			fmt.Sprintf("%v: %s", errors.ErrPromptNotFound, name),
			nil,
		), nil
	}

	// Create prompt get request
	getReq := &GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name:      name,
			Arguments: make(map[string]string),
		},
	}

	// Convert arguments to string map
	for k, v := range arguments {
		if str, ok := v.(string); ok {
			getReq.Params.Arguments[k] = str
		}
	}

	// Call prompt handler if available
	if registeredPrompt.Handler != nil {
		result, err := registeredPrompt.Handler(ctx, getReq)
		if err != nil {
			return newJSONRPCErrorResponse(req.ID, ErrCodeInternal, err.Error(), nil), nil
		}
		return result, nil
	}

	// Use default implementation if no handler is provided
	if arguments == nil {
		arguments = make(map[string]interface{})
	}
	messages := buildPromptMessages(registeredPrompt.Prompt, arguments)
	result := &GetPromptResult{
		Description: registeredPrompt.Prompt.Description,
		Messages:    messages,
	}
	return result, nil
}

// Helper: Parse and validate parameters for CompletionComplete
func parseCompletionCompleteParams(req *JSONRPCRequest) (promptName, promptTitle, resourceURI, argName, argValue string, ctxArguments map[string]interface{}, errResp JSONRPCMessage, ok bool) {
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return "", "", "", "", "", nil, newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrInvalidParams.Error(), nil), false
	}

	ref, ok := paramsMap["ref"].(map[string]interface{})
	if !ok {
		return "", "", "", "", "", nil, newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), false
	}
	refType, ok := ref["type"].(string)
	if !ok || (refType != "ref/prompt" && refType != "ref/resource") {
		return "", "", "", "", "", nil, newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrInvalidParams.Error(), nil), false
	}

	promptName, ok = ref["name"].(string)
	// For ref/prompt, name is required
	if refType == "ref/prompt" && !ok {
		return "", "", "", "", "", nil, newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), false
	}
	promptTitle, _ = ref["title"].(string)
	resourceURI, ok = ref["uri"].(string)
	// For ref/resource, uri is required
	if refType == "ref/resource" && !ok {
		return "", "", "", "", "", nil, newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), false
	}

	argument, ok := paramsMap["argument"].(map[string]interface{})
	if !ok {
		return "", "", "", "", "", nil, newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrInvalidParams.Error(), nil), false
	}
	if argument == nil {
		return "", "", "", "", "", nil, newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), false
	}
	// Check that argument has "name" and "value" as string parameters
	argName, ok = argument["name"].(string)
	if !ok {
		return "", "", "", "", "", nil, newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), false
	}
	argValue, ok = argument["value"].(string)
	if !ok {
		return "", "", "", "", "", nil, newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), false
	}

	context, ok := paramsMap["context"].(map[string]interface{})
	if ok && context != nil {
		ctxArguments, _ = context["arguments"].(map[string]interface{})
	}
	return promptName, promptTitle, resourceURI, argName, argValue, ctxArguments, nil, true
}

// Refactored: handleCompletionComplete with logic unchanged, now using helpers
func (m *promptManager) handleCompletionComplete(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
	promptName, promptTitle, _, argName, argValue, ctxArguments, errResp, ok := parseCompletionCompleteParams(req)
	if !ok {
		return errResp, nil
	}

	// create a new request for completion
	completionReq := &CompleteCompletionRequest{}
	completionReq.Params.Ref.Name = promptName
	completionReq.Params.Ref.Title = promptTitle
	completionReq.Params.Ref.Type = "ref/prompt"
	completionReq.Params.Argument.Name = argName
	completionReq.Params.Argument.Value = argValue

	// Convert ctxArguments to map[string]string if needed
	if len(ctxArguments) > 0 {
		completionReq.Params.Context = struct {
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Arguments: make(map[string]string),
		}
		// Convert arguments to string map
		for k, v := range ctxArguments {
			if str, ok := v.(string); ok {
				completionReq.Params.Context.Arguments[k] = str
			}
		}
	}

	// Business logic remains unchanged, can be further split if needed
	return m.handlePromptCompletion(ctx, completionReq, req)
}

// Helper: Handle prompt completion business logic (can be further split if needed)
func (m *promptManager) handlePromptCompletion(ctx context.Context, completionReq *CompleteCompletionRequest, req *JSONRPCRequest) (JSONRPCMessage, error) {
	registeredPrompt, exists := m.prompts[completionReq.Params.Ref.Name]
	if !exists {
		return newJSONRPCErrorResponse(
			req.ID,
			ErrCodeMethodNotFound,
			fmt.Sprintf("%v: %s", errors.ErrPromptNotFound, completionReq.Params.Ref.Name),
			nil,
		), nil
	}

	// Check if completionCompleteHandler is available
	if registeredPrompt.CompletionCompleteHandler == nil {
		return newJSONRPCErrorResponse(
			req.ID,
			ErrCodeMethodNotFound,
			fmt.Sprintf("%v: %s", errors.ErrMethodNotFound, completionReq.Params.Ref.Name),
			nil,
		), nil
	}

	result, err := registeredPrompt.CompletionCompleteHandler(ctx, completionReq)
	if err != nil {
		return newJSONRPCErrorResponse(
			req.ID,
			ErrCodeInternal,
			err.Error(),
			nil,
		), nil
	}

	return result, nil
}
