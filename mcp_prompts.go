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
		// Use mcp.parseContent (from mcp/tools.go, assuming it's accessible)
		// to parse the raw JSON into the correct concrete Content type.
		var contentMap map[string]interface{}
		if err := json.Unmarshal(temp.Content, &contentMap); err != nil {
			return fmt.Errorf("failed to unmarshal content field to map for parseContent: %w", err)
		}

		// Assuming parseContent is a function in the mcp package (e.g., mcp.parseContent)
		// If it's not directly accessible, this call needs adjustment (e.g., qualifying with package if different and public).
		// For this example, we assume it can be called directly if in the same package or mcp.parseContent if parseContent is public.
		concreteContent, err := parseContent(contentMap) // This function is in mcp/tools.go (mcp.parseContent)
		if err != nil {
			return fmt.Errorf("failed to parse concrete content using parseContent: %w", err)
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
