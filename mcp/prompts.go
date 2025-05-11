package mcp

import (
	"encoding/json"
	"fmt"
)

// ListPromptsRequest describes a request to list prompts.
type ListPromptsRequest struct {
	PaginatedRequest
}

// ListPromptsResult describes a result of listing prompts.
type ListPromptsResult struct {
	PaginatedResult
	Prompts []Prompt `json:"prompts"`
}

// GetPromptRequest describes a request to get a prompt.
type GetPromptRequest struct {
	Request
	Params struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments,omitempty"`
	} `json:"params"`
}

// GetPromptResult describes a result of getting a prompt.
type GetPromptResult struct {
	Result
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

type PromptListChangedNotification struct {
	Notification
}

// Prompt represents a prompt or prompt template provided by the server
type Prompt struct {
	// Name is the name of the prompt or prompt template
	// Corresponds to schema: "name": {"description": "The name of the prompt or prompt template."}
	Name string `json:"name"`

	// Description is an optional description of the prompt
	// Corresponds to schema: "description": {"description": "An optional description of what this prompt provides"}
	Description string `json:"description,omitempty"`

	// Arguments is a list of prompt parameters
	// Corresponds to schema: "arguments": {"description": "A list of arguments to use for templating the prompt."}
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes the parameters accepted by the prompt
type PromptArgument struct {
	// Parameter name
	Name string `json:"name"`

	// Parameter description (optional)
	Description string `json:"description,omitempty"`

	// Whether the parameter is required
	Required bool `json:"required,omitempty"`
}

// PromptMessage describes the message returned by the prompt
type PromptMessage struct {
	// Message role
	Role Role `json:"role"`

	// Message content
	Content Content `json:"content"`
}

// UnmarshalJSON implements custom unmarshaling for PromptMessage to handle polymorphic Content.
func (pm *PromptMessage) UnmarshalJSON(data []byte) error {
	type Alias PromptMessage // Create an alias to avoid recursion with UnmarshalJSON.
	temp := &struct {
		Content json.RawMessage `json:"content"` // Capture content as raw message first.
		*Alias
	}{
		Alias: (*Alias)(pm),
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("failed to unmarshal prompt message structure: %w", err)
	}

	if temp.Content != nil && len(temp.Content) > 0 && string(temp.Content) != "null" {
		// Use mcp.ParseContent (from mcp/tools.go, assuming it's accessible)
		// to parse the raw JSON into the correct concrete Content type.
		var contentMap map[string]interface{}
		if err := json.Unmarshal(temp.Content, &contentMap); err != nil {
			return fmt.Errorf("failed to unmarshal content field to map for ParseContent: %w", err)
		}

		// Assuming ParseContent is a function in the mcp package (e.g., mcp.ParseContent)
		// If it's not directly accessible, this call needs adjustment (e.g., qualifying with package if different and public).
		// For this example, we assume it can be called directly if in the same package or mcp.ParseContent if ParseContent is public.
		concreteContent, err := ParseContent(contentMap) // This function is in mcp/tools.go (mcp.ParseContent)
		if err != nil {
			return fmt.Errorf("failed to parse concrete content using ParseContent: %w", err)
		}
		pm.Content = concreteContent
	} else {
		// If content is null or not present, set it to nil.
		pm.Content = nil
	}
	return nil
}

// GetPromptResponse represents the response when getting a prompt
type GetPromptResponse struct {
	// Prompt description (optional)
	Description string `json:"description,omitempty"`

	// List of prompt messages
	Messages []PromptMessage `json:"messages"`
}

// ListPromptsResponse represents the response when listing prompts
type ListPromptsResponse struct {
	// List of prompts
	Prompts []Prompt `json:"prompts"`

	// Next page cursor (optional)
	NextCursor string `json:"nextCursor,omitempty"`
}

// ParseListPromptsResult parses the prompt list response
func ParseListPromptsResult(result interface{}) (*ListPromptsResponse, error) {
	// Type assertion to map
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid prompt list response format")
	}

	// Create result object
	promptsResponse := &ListPromptsResponse{}

	// Parse next page cursor
	if cursor, ok := resultMap["nextCursor"].(string); ok {
		promptsResponse.NextCursor = cursor
	}

	// Parse prompt list
	promptsArray, ok := resultMap["prompts"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing prompt list in response")
	}

	prompts := make([]Prompt, 0, len(promptsArray))
	for _, item := range promptsArray {
		prompt, err := parsePromptItem(item)
		if err != nil {
			continue // or return error
		}
		prompts = append(prompts, prompt)
	}
	promptsResponse.Prompts = prompts

	return promptsResponse, nil
}

// parsePromptItem parses a single prompt item
func parsePromptItem(item interface{}) (Prompt, error) {
	promptMap, ok := item.(map[string]interface{})
	if !ok {
		return Prompt{}, fmt.Errorf("invalid prompt format")
	}

	// Create prompt object
	prompt := Prompt{}

	// Extract name
	if name, ok := promptMap["name"].(string); ok {
		prompt.Name = name
	} else {
		return Prompt{}, fmt.Errorf("prompt missing name")
	}

	// Extract description
	if description, ok := promptMap["description"].(string); ok {
		prompt.Description = description
	}

	// Parse parameter list
	if argsArray, ok := promptMap["arguments"].([]interface{}); ok && len(argsArray) > 0 {
		args := make([]PromptArgument, 0, len(argsArray))

		for _, argItem := range argsArray {
			argMap, ok := argItem.(map[string]interface{})
			if !ok {
				continue
			}

			arg := PromptArgument{}

			// Extract parameter name
			if name, ok := argMap["name"].(string); ok {
				arg.Name = name
			} else {
				continue // Parameter must have a name
			}

			// Extract parameter description
			if description, ok := argMap["description"].(string); ok {
				arg.Description = description
			}

			// Extract whether required
			if required, ok := argMap["required"].(bool); ok {
				arg.Required = required
			}

			args = append(args, arg)
		}

		prompt.Arguments = args
	}

	return prompt, nil
}

// ParseGetPromptResult parses the get prompt response
func ParseGetPromptResult(result interface{}) (*GetPromptResponse, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid get prompt response format")
	}

	// Create result object
	promptResponse := &GetPromptResponse{}

	// Extract description
	if description, ok := resultMap["description"].(string); ok {
		promptResponse.Description = description
	}

	// Parse message list
	messagesArray, ok := resultMap["messages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing message list in response")
	}

	messages := make([]PromptMessage, 0, len(messagesArray))
	for _, item := range messagesArray {
		message, err := parsePromptMessageItem(item)
		if err != nil {
			continue
		}
		messages = append(messages, message)
	}
	promptResponse.Messages = messages

	return promptResponse, nil
}

// parsePromptMessageItem parses a prompt message item
func parsePromptMessageItem(item interface{}) (PromptMessage, error) {
	msgMap, ok := item.(map[string]interface{})
	if !ok {
		return PromptMessage{}, fmt.Errorf("invalid message format")
	}

	// Create message object
	message := PromptMessage{}

	// Extract role
	if roleStr, ok := msgMap["role"].(string); ok {
		message.Role = Role(roleStr)
	} else {
		return PromptMessage{}, fmt.Errorf("message missing role")
	}

	// Extract content
	if content, ok := msgMap["content"]; ok {
		message.Content = content.(Content)
	} else {
		return PromptMessage{}, fmt.Errorf("message missing content")
	}

	return message, nil
}
