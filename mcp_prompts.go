// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// promptHandler defines the function type for handling prompt requests
type promptHandler func(ctx context.Context, req *GetPromptRequest) (*GetPromptResult, error)

// PromptListFilter defines a function type for filtering prompts based on context.
// The filter receives the request context and all registered prompts, and returns
// a filtered list of prompts that should be visible to the client.
type PromptListFilter func(ctx context.Context, prompts []*Prompt) []*Prompt

// completionCompleteHandler defines the function type for handling completion requests
type completionCompleteHandler func(ctx context.Context, req *CompleteCompletionRequest) (*CompleteCompletionResult, error)

// registerdPromptOption is a function that configures a registeredPrompt.
type registerdPromptOption func(*registeredPrompt)

// registeredPrompt combines a Prompt with its handler function
type registeredPrompt struct {
	Prompt                    *Prompt
	Handler                   promptHandler
	CompletionCompleteHandler completionCompleteHandler
}

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

// CompleteCompletionRequest describes a request for completion.
type CompleteCompletionRequest struct {
	Request
	Params struct {
		// Name is the name of the prompt or prompt template
		Ref struct {
			// Name is intended for programmatic or logical use, but may be used as
			// a display name in past specs or as a fallback if Title isn't present.
			Name string `json:"name"`

			// Title is an optional, user-friendly display name intended for UI and end-user contexts.
			Title string `json:"title,omitempty"`

			// Type is the type of the prompt or resource, "ref/prompt", "ref/resource".
			Type string `json:"type"`

			// URI is an optional URI for the type of "ref/resource"
			URI string `json:"uri,omitempty"`
		} `json:"ref"`

		// Argument contains information about the argument for completion matching
		Argument struct {
			// Name is the name of the argument
			Name string `json:"name"`

			// Value is the value of the argument to use for completion matching
			Value string `json:"value"`
		} `json:"argument"`

		// Context provides optional additional context for completions
		Context struct {
			// Arguments are previously-resolved variables in a URI template or prompt
			Arguments map[string]string `json:"arguments,omitempty"`
		} `json:"context,omitempty"`
	} `json:"params"`
}

// CompleteCompletionResult describes the result of a completion.
type CompleteCompletionResult struct {
	Result
	Completion struct {
		// Values is an array of completion values. Must not exceed 100 items.
		Values []string `json:"values"`

		// Total is the total number of completion options available. This can exceed the
		// number of values actually sent in the response.
		Total int `json:"total,omitempty"`

		// HasMore indicates whether there are additional completion options beyond those
		// provided in the current response, even if the exact total is unknown.
		HasMore bool `json:"hasMore,omitempty"`
	} `json:"completion"`
}

// PromptListChangedNotification represents a notification that the prompt list has changed
type PromptListChangedNotification struct {
	Notification
}

// Prompt represents a prompt or prompt template provided by the server.
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

	if len(temp.Content) > 0 {
		// Check for JSON null value first
		if string(temp.Content) == "null" {
			pm.Content = nil
			return nil
		}

		// Parse the content as a map for further processing
		var contentMap map[string]interface{}
		if err := json.Unmarshal(temp.Content, &contentMap); err != nil {
			return fmt.Errorf("failed to unmarshal content field: %w", err)
		}

		// If not directly accessible, adjust the call (e.g., qualify with package if needed).
		// We assume it can be called directly in same package or as mcp.parseContent if public.
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

func WithPromptCompletion(handler completionCompleteHandler) registerdPromptOption {
	return func(rp *registeredPrompt) {
		rp.CompletionCompleteHandler = handler
	}
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
