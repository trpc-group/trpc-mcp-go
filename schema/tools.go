package schema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// CallToolRequest represents a tool call request (conforming to MCP specification)
type CallToolRequest struct {
	Method string         `json:"method"` // Fixed value: "tools/call"
	Params CallToolParams `json:"params"`
}

// CallToolParams represents tool call parameters
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// RequestMeta represents request metadata
type RequestMeta struct {
	ProgressToken interface{} `json:"progressToken,omitempty"`
}

// CallToolResult represents tool call result
type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
	Meta    *ResultMeta   `json:"_meta,omitempty"`
}

// ResultMeta represents result metadata
type ResultMeta struct {
	AdditionalData map[string]interface{} `json:"-"`
}

// ToolContent represents the interface for tool response content items
type ToolContent interface {
	GetType() string
}

// TextContent represents text content
type TextContent struct {
	Type        string       `json:"type"`
	Text        string       `json:"text"`
	Annotations *Annotations `json:"annotations,omitempty"`
}

func (t TextContent) GetType() string {
	return t.Type
}

// ImageContent represents image content
type ImageContent struct {
	Type        string       `json:"type"`
	Data        string       `json:"data"` // base64 encoded image data
	MimeType    string       `json:"mimeType"`
	Annotations *Annotations `json:"annotations,omitempty"`
}

func (i ImageContent) GetType() string {
	return i.Type
}

// AudioContent represents audio content
type AudioContent struct {
	Type        string       `json:"type"`
	Data        string       `json:"data"` // base64 encoded audio data
	MimeType    string       `json:"mimeType"`
	Annotations *Annotations `json:"annotations,omitempty"`
}

func (a AudioContent) GetType() string {
	return a.Type
}

// EmbeddedResource represents an embedded resource
type EmbeddedResource struct {
	Type        string       `json:"type"`
	Resource    interface{}  `json:"resource"` // Using generic interface type
	Annotations *Annotations `json:"annotations,omitempty"`
}

func (e EmbeddedResource) GetType() string {
	return e.Type
}

// Tool represents an MCP tool
type Tool struct {
	// Tool name
	Name string `json:"name"`

	// Tool description
	Description string `json:"description,omitempty"`

	// Input parameter schema
	InputSchema *openapi3.Schema `json:"inputSchema"`

	// Raw schema (for custom schemas)
	RawInputSchema json.RawMessage `json:"-"`

	// Tool execution function - new version
	ExecuteFunc func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) `json:"-"`
}

// ToolOption represents tool configuration option
type ToolOption func(*Tool)

// NewTool creates a new tool
func NewTool(name string, executeFunc func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error), opts ...ToolOption) *Tool {
	tool := &Tool{
		Name: name,
		InputSchema: &openapi3.Schema{
			Type:       &openapi3.Types{openapi3.TypeObject},
			Properties: make(openapi3.Schemas),
			Required:   []string{},
		},
		ExecuteFunc: executeFunc,
	}

	for _, opt := range opts {
		opt(tool)
	}

	return tool
}

// PropertyOption represents property configuration option
type PropertyOption func(*openapi3.Schema)

// WithDescription common option function
func WithDescription(description string) ToolOption {
	return func(t *Tool) {
		t.Description = description
	}
}

func WithString(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := &openapi3.Schema{
			Type: &openapi3.Types{openapi3.TypeString},
		}
		for _, opt := range opts {
			opt(schema)
		}
		t.InputSchema.Properties[name] = openapi3.NewSchemaRef("", schema)
		if schema.Required != nil && len(schema.Required) > 0 {
			t.InputSchema.Required = append(t.InputSchema.Required, name)
		}
	}
}

func WithNumber(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := &openapi3.Schema{
			Type: &openapi3.Types{openapi3.TypeNumber},
		}
		for _, opt := range opts {
			opt(schema)
		}
		t.InputSchema.Properties[name] = openapi3.NewSchemaRef("", schema)
		if schema.Required != nil && len(schema.Required) > 0 {
			t.InputSchema.Required = append(t.InputSchema.Required, name)
		}
	}
}

func WithBoolean(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := &openapi3.Schema{
			Type: &openapi3.Types{openapi3.TypeBoolean},
		}
		for _, opt := range opts {
			opt(schema)
		}
		t.InputSchema.Properties[name] = openapi3.NewSchemaRef("", schema)
		if schema.Required != nil && len(schema.Required) > 0 {
			t.InputSchema.Required = append(t.InputSchema.Required, name)
		}
	}
}

// Property option functions
func Description(desc string) PropertyOption {
	return func(s *openapi3.Schema) {
		s.Description = desc
	}
}

func Required() PropertyOption {
	return func(s *openapi3.Schema) {
		s.Required = []string{"true"}
	}
}

func Default(value interface{}) PropertyOption {
	return func(s *openapi3.Schema) {
		s.Default = value
	}
}

// Helper functions for content creation
func NewTextContent(text string) TextContent {
	return TextContent{
		Type: "text",
		Text: text,
	}
}

func NewImageContent(data string, mimeType string) ImageContent {
	return ImageContent{
		Type:     "image",
		Data:     data,
		MimeType: mimeType,
	}
}

func NewAudioContent(data string, mimeType string) AudioContent {
	return AudioContent{
		Type:     "audio",
		Data:     data,
		MimeType: mimeType,
	}
}

func NewEmbeddedResource(resource interface{}) EmbeddedResource {
	return EmbeddedResource{
		Type:     "resource",
		Resource: resource,
	}
}

// Result creation helper functions
func NewCallToolResult(content []ToolContent) *CallToolResult {
	return &CallToolResult{
		Content: content,
	}
}

func NewTextResult(text string) *CallToolResult {
	return &CallToolResult{
		Content: []ToolContent{NewTextContent(text)},
	}
}

func NewErrorResult(text string) *CallToolResult {
	return &CallToolResult{
		IsError: true,
		Content: []ToolContent{NewTextContent(text)},
	}
}

func NewMultiContentResult(contents ...ToolContent) *CallToolResult {
	return &CallToolResult{
		Content: contents,
	}
}

// Backwards compatibility types and functions
// ToolResult is a type alias kept for backwards compatibility
type ToolResult = CallToolResult

// ListToolsResult represents tool list response
type ListToolsResult struct {
	// Tool list
	Tools []*Tool `json:"tools"`
	// Next page cursor
	NextCursor string `json:"nextCursor,omitempty"`
}

// ParseListToolsResult parses a tool list response
func ParseListToolsResult(result interface{}) (*ListToolsResult, error) {
	// Type assertion to map
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tool list response format")
	}

	// Create result object
	toolsResult := &ListToolsResult{}

	// Parse next page cursor
	if cursor, ok := resultMap["nextCursor"].(string); ok {
		toolsResult.NextCursor = cursor
	}

	// Parse tool list
	toolsArray, ok := resultMap["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing tool list in response")
	}

	tools := make([]*Tool, 0, len(toolsArray))
	for _, item := range toolsArray {
		tool, err := parseToolItem(item)
		if err != nil {
			continue // or return error
		}
		tools = append(tools, tool)
	}
	toolsResult.Tools = tools

	return toolsResult, nil
}

// parseToolItem parses a single tool item
func parseToolItem(item interface{}) (*Tool, error) {
	toolMap, ok := item.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tool format")
	}

	// Create tool object
	tool := &Tool{}

	// Extract name
	if name, ok := toolMap["name"].(string); ok {
		tool.Name = name
	} else {
		return nil, fmt.Errorf("tool missing name")
	}

	// Extract description
	if description, ok := toolMap["description"].(string); ok {
		tool.Description = description
	}

	// Parse input schema
	if schema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
		// Convert map to JSON then parse to Schema
		schemaBytes, err := json.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize schema: %w", err)
		}

		inputSchema := &openapi3.Schema{}
		if err := json.Unmarshal(schemaBytes, inputSchema); err != nil {
			return nil, fmt.Errorf("failed to parse schema: %w", err)
		}

		tool.InputSchema = inputSchema
	}

	return tool, nil
}

// ParseCallToolResult parses a tool call result
func ParseCallToolResult(result interface{}) (*CallToolResult, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tool call result format")
	}

	// Create result object
	toolResult := &CallToolResult{}

	// Parse content list
	contentArray, ok := resultMap["content"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing content list in response")
	}

	// Parse content items
	contents := make([]ToolContent, 0, len(contentArray))
	for _, item := range contentArray {
		content, err := parseToolContentItem(item)
		if err != nil {
			continue // or return error
		}
		contents = append(contents, content)
	}
	toolResult.Content = contents

	// Parse error flag
	if isError, ok := resultMap["isError"].(bool); ok {
		toolResult.IsError = isError
	}

	// Parse metadata (if present)
	if meta, ok := resultMap["_meta"].(map[string]interface{}); ok {
		toolResult.Meta = &ResultMeta{
			AdditionalData: meta,
		}
	}

	return toolResult, nil
}

// parseToolContentItem parses a single content item
func parseToolContentItem(item interface{}) (ToolContent, error) {
	contentMap, ok := item.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid content item format")
	}

	// Extract type
	contentType, ok := contentMap["type"].(string)
	if !ok {
		return nil, fmt.Errorf("content item missing type")
	}

	// Create appropriate content object based on type
	switch contentType {
	case "text":
		text, _ := contentMap["text"].(string)
		return NewTextContent(text), nil
	case "image":
		data, _ := contentMap["data"].(string)
		mimeType, _ := contentMap["mimeType"].(string)
		return NewImageContent(data, mimeType), nil
	case "audio":
		data, _ := contentMap["data"].(string)
		mimeType, _ := contentMap["mimeType"].(string)
		return NewAudioContent(data, mimeType), nil
	case "resource":
		// Resource content needs special handling
		if resource, ok := contentMap["resource"].(map[string]interface{}); ok {
			// Use the raw resource data directly
			return EmbeddedResource{
				Type:     "resource",
				Resource: resource,
			}, nil
		}
		return nil, fmt.Errorf("invalid resource content format")
	default:
		// For unknown types, create a generic content
		return TextContent{
			Type: contentType,
			Text: fmt.Sprintf("Unknown content type: %s", contentType),
		}, nil
	}
}
