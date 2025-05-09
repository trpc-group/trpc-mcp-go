package protocol

import (
	"context"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

// PromptManager manages prompt templates
//
// Prompt functionality follows these enabling mechanisms:
// 1. By default, prompt functionality is disabled
// 2. When the first prompt is registered, prompt functionality is automatically enabled without additional configuration
// 3. When prompt functionality is enabled but no prompts exist, ListPrompts will return an empty prompt list rather than an error
// 4. Clients can determine if the server supports prompt functionality through the capabilities field in the initialization response
//
// This design simplifies API usage, eliminating the need for explicit configuration parameters to enable or disable prompt functionality.
type PromptManager struct {
	// Prompt mapping table
	prompts map[string]*schema.Prompt

	// Mutex
	mu sync.RWMutex
}

// NewPromptManager creates a new prompt manager
//
// Note: Simply creating a prompt manager does not enable prompt functionality,
// it is only enabled when the first prompt is added.
func NewPromptManager() *PromptManager {
	return &PromptManager{
		prompts: make(map[string]*schema.Prompt),
	}
}

// RegisterPrompt registers a prompt
func (m *PromptManager) RegisterPrompt(prompt *schema.Prompt) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if prompt == nil {
		return fmt.Errorf("prompt cannot be nil")
	}

	if prompt.Name == "" {
		return fmt.Errorf("prompt name cannot be empty")
	}

	if _, exists := m.prompts[prompt.Name]; exists {
		return fmt.Errorf("prompt %s already exists", prompt.Name)
	}

	m.prompts[prompt.Name] = prompt
	return nil
}

// GetPrompt retrieves a prompt
func (m *PromptManager) GetPrompt(name string) (*schema.Prompt, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prompt, exists := m.prompts[name]
	return prompt, exists
}

// GetPrompts retrieves all prompts
func (m *PromptManager) GetPrompts() []*schema.Prompt {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prompts := make([]*schema.Prompt, 0, len(m.prompts))
	for _, prompt := range m.prompts {
		prompts = append(prompts, prompt)
	}
	return prompts
}

// HandleListPrompts handles listing prompts requests
func (m *PromptManager) HandleListPrompts(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	prompts := m.GetPrompts()
	result := &schema.ListPromptsResponse{
		Prompts: make([]schema.Prompt, len(prompts)),
	}

	for i, prompt := range prompts {
		result.Prompts[i] = *prompt
	}

	return schema.NewResponse(req.ID, result), nil
}

// HandleGetPrompt handles retrieving prompt requests
func (m *PromptManager) HandleGetPrompt(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	// Get prompt name from parameters
	params := req.Params
	name, ok := params["name"].(string)
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing prompt name", nil), nil
	}

	// Get prompt
	prompt, exists := m.GetPrompt(name)
	if !exists {
		return schema.NewErrorResponse(req.ID, schema.ErrMethodNotFound, fmt.Sprintf("prompt %s does not exist", name), nil), nil
	}

	// Get arguments
	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		arguments = make(map[string]interface{})
	}

	// Create response
	// Generate actual messages based on prompt type and parameters
	messages := []map[string]interface{}{}

	// Add an example user message
	userPrompt := fmt.Sprintf("This is an example rendering of the %s prompt.", prompt.Name)

	// Check if parameter values are provided
	for _, arg := range prompt.Arguments {
		if value, ok := arguments[arg.Name]; ok {
			userPrompt += fmt.Sprintf("\nParameter %s: %v", arg.Name, value)
		} else if arg.Required {
			userPrompt += fmt.Sprintf("\nParameter %s: [not provided]", arg.Name)
		}
	}

	// Add user message
	messages = append(messages, map[string]interface{}{
		"role": "user",
		"content": map[string]interface{}{
			"type": "text",
			"text": userPrompt,
		},
	})

	result := map[string]interface{}{
		"description": prompt.Description,
		"messages":    messages,
	}

	return schema.NewResponse(req.ID, result), nil
}

// HandleCompletionComplete handles prompt completion requests
func (m *PromptManager) HandleCompletionComplete(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	// Get reference type and name from parameters
	params := req.Params
	ref, ok := params["ref"].(map[string]interface{})
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing reference information", nil), nil
	}

	// Check reference type
	refType, ok := ref["type"].(string)
	if !ok || refType != "ref/prompt" {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "invalid reference type", nil), nil
	}

	// Get prompt name
	promptName, ok := ref["name"].(string)
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing prompt name", nil), nil
	}

	// Get prompt
	prompt, exists := m.GetPrompt(promptName)
	if !exists {
		return schema.NewErrorResponse(req.ID, schema.ErrMethodNotFound, fmt.Sprintf("prompt %s does not exist", promptName), nil), nil
	}

	// Get arguments
	argument, ok := params["argument"].(map[string]interface{})
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing arguments", nil), nil
	}

	// Extract argument name and value
	argName, ok := argument["name"].(string)
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing argument name", nil), nil
	}

	argValue, ok := argument["value"].(string)
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing argument value", nil), nil
	}

	// Check if argument is valid
	var foundArg *schema.PromptArgument
	for _, arg := range prompt.Arguments {
		if arg.Name == argName {
			foundArg = &arg
			break
		}
	}

	if foundArg == nil {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, fmt.Sprintf("argument %s not found in prompt", argName), nil), nil
	}

	// Create a response with completion results
	// In a real implementation, you would process the prompt with the given argument
	// Here we just return an example completion
	result := map[string]interface{}{
		"completion": fmt.Sprintf("Completion for %s with %s=%s", promptName, argName, argValue),
	}

	return schema.NewResponse(req.ID, result), nil
}
