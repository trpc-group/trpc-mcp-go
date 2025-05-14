package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

type ListToolsRequest struct {
	PaginatedRequest
}

type PaginatedRequest struct {
	Request
	Params struct {
		Cursor Cursor `json:"cursor,omitempty"`
	} `json:"params,omitempty"`
}

type ListToolsResult struct {
	PaginatedResult
	Tools []Tool `json:"tools"`
}

// CallToolRequest represents a tool call request (conforming to MCP specification)
type CallToolRequest struct {
	Request
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
	Result
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// ResultMeta represents result metadata
type ResultMeta struct {
	AdditionalData map[string]interface{} `json:"-"`
}

// ToolListChangedNotification describes a tool list changed notification.
type ToolListChangedNotification struct {
	Notification
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

func NewTextResult(text string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{NewTextContent(text)},
	}
}

func NewErrorResult(text string) *CallToolResult {
	return &CallToolResult{
		IsError: true,
		Content: []Content{NewTextContent(text)},
	}
}

// ParseListToolsResult parses a tool list response
func ParseListToolsResult(result interface{}) (*ListToolsResult, error) {
	// Type assertion to map
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, errors.ErrInvalidToolListFormat
	}

	// Create result object
	toolsResult := &ListToolsResult{}

	// Parse next page cursor
	if cursor, ok := resultMap["nextCursor"].(string); ok {
		toolsResult.NextCursor = Cursor(cursor)
	}

	// Parse tool list
	toolsArray, ok := resultMap["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: tools field not found or invalid type", errors.ErrInvalidToolListFormat)
	}

	// Create a slice of Tool (not *Tool)
	tools := make([]Tool, 0, len(toolsArray))
	for _, item := range toolsArray {
		tool, err := parseToolItem(item)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tool item: %w", err)
		}
		tools = append(tools, *tool)
	}

	// Create result
	return &ListToolsResult{
		Tools: tools,
	}, nil
}

// parseToolItem parses a single tool item
func parseToolItem(item interface{}) (*Tool, error) {
	toolMap, ok := item.(map[string]interface{})
	if !ok {
		return nil, errors.ErrInvalidToolFormat
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

func ParseCallToolResult(rawMessage *json.RawMessage) (*CallToolResult, error) {
	var jsonContent map[string]any
	if err := json.Unmarshal(*rawMessage, &jsonContent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var result CallToolResult

	meta, ok := jsonContent["_meta"]
	if ok {
		if metaMap, ok := meta.(map[string]any); ok {
			result.Meta = metaMap
		}
	}

	isError, ok := jsonContent["isError"]
	if ok {
		if isErrorBool, ok := isError.(bool); ok {
			result.IsError = isErrorBool
		}
	}

	contents, ok := jsonContent["content"]
	if !ok {
		return nil, fmt.Errorf("content is missing")
	}

	contentArr, ok := contents.([]any)
	if !ok {
		return nil, fmt.Errorf("content is not an array")
	}

	for _, content := range contentArr {
		// Extract content.
		contentMap, ok := content.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("content is not an object")
		}

		// Process content.
		content, err := ParseContent(contentMap)
		if err != nil {
			return nil, err
		}

		result.Content = append(result.Content, content)
	}

	return &result, nil
}

func ParseContent(contentMap map[string]any) (Content, error) {
	contentType := ExtractString(contentMap, "type")

	switch contentType {
	case "text":
		text := ExtractString(contentMap, "text")
		if text == "" {
			return nil, fmt.Errorf("text is missing")
		}
		return NewTextContent(text), nil

	case "image":
		data := ExtractString(contentMap, "data")
		mimeType := ExtractString(contentMap, "mimeType")
		if data == "" || mimeType == "" {
			return nil, fmt.Errorf("image data or mimeType is missing")
		}
		return NewImageContent(data, mimeType), nil

	case "resource":
		resourceMap := ExtractMap(contentMap, "resource")
		if resourceMap == nil {
			return nil, fmt.Errorf("resource is missing")
		}

		resourceContents, err := ParseResourceContents(resourceMap)
		if err != nil {
			return nil, err
		}

		return NewEmbeddedResource(resourceContents), nil
	}

	return nil, fmt.Errorf("unsupported content type: %s", contentType)
}

func ExtractString(data map[string]any, key string) string {
	if value, ok := data[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func ExtractMap(data map[string]any, key string) map[string]any {
	if value, ok := data[key]; ok {
		if m, ok := value.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func ParseResourceContents(contentMap map[string]any) (ResourceContents, error) {
	uri := ExtractString(contentMap, "uri")
	if uri == "" {
		return nil, fmt.Errorf("resource uri is missing")
	}

	mimeType := ExtractString(contentMap, "mimeType")

	if text := ExtractString(contentMap, "text"); text != "" {
		return TextResourceContents{
			URI:      uri,
			MIMEType: mimeType,
			Text:     text,
		}, nil
	}

	if blob := ExtractString(contentMap, "blob"); blob != "" {
		return BlobResourceContents{
			URI:      uri,
			MIMEType: mimeType,
			Blob:     blob,
		}, nil
	}

	return nil, fmt.Errorf("unsupported resource type")
}
