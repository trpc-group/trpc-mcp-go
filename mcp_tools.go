// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// ListToolsRequest represents a request to list available tools
type ListToolsRequest struct {
	PaginatedRequest
}

// PaginatedRequest represents a request with pagination support
type PaginatedRequest struct {
	Request
	Params struct {
		Cursor Cursor `json:"cursor,omitempty"`
	} `json:"params,omitempty"`
}

// ListToolsResult represents the result of listing tools
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
	Meta      *struct {
		ProgressToken ProgressToken `json:"progressToken,omitempty"`
	} `json:"_meta,omitempty"`
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
}

// toolHandler defines the function type for handling tool execution
type toolHandler func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error)

// registeredTool combines a Tool with its handler function
type registeredTool struct {
	Tool    *Tool
	Handler toolHandler
}

// ToolOption represents a function that configures a Tool
type ToolOption func(*Tool)

// PropertyOption represents a function that configures a schema property
type PropertyOption func(*openapi3.Schema)

// ToolListFilter defines a function type for filtering tools based on context.
// The filter receives the request context and all registered tools, and returns
// a filtered list of tools that should be visible to the client.
type ToolListFilter func(ctx context.Context, tools []*Tool) []*Tool

// NewTool creates a new tool
func NewTool(
	name string,
	opts ...ToolOption,
) *Tool {
	tool := &Tool{
		Name: name,
		InputSchema: &openapi3.Schema{
			Type:       &openapi3.Types{openapi3.TypeObject},
			Properties: make(openapi3.Schemas),
			Required:   []string{},
		},
	}

	for _, opt := range opts {
		opt(tool)
	}

	return tool
}

// WithDescription common option function
func WithDescription(description string) ToolOption {
	return func(t *Tool) {
		t.Description = description
	}
}

// WithString adds a string parameter to the tool's input schema
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

// WithNumber adds a number parameter to the tool's input schema
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

// WithInteger adds an integer parameter to the tool's input schema
func WithInteger(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := &openapi3.Schema{
			Type: &openapi3.Types{openapi3.TypeInteger},
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

// WithBoolean adds a boolean parameter to the tool's input schema
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

// WithObject adds an object parameter to the tool's input schema.
func WithObject(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := &openapi3.Schema{
			Type:       &openapi3.Types{openapi3.TypeObject},
			Properties: make(openapi3.Schemas),
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

// Description adds a description to the tool's input schema.
func Description(desc string) PropertyOption {
	return func(s *openapi3.Schema) {
		s.Description = desc
	}
}

// Required marks the parameter as required.
func Required() PropertyOption {
	return func(s *openapi3.Schema) {
		s.Required = []string{"true"}
	}
}

// Default sets a default value for the parameter.
func Default(value interface{}) PropertyOption {
	return func(s *openapi3.Schema) {
		s.Default = value
	}
}

// Title sets a title for the parameter.
func Title(title string) PropertyOption {
	return func(s *openapi3.Schema) {
		s.Title = title
	}
}

// Enum adds an enum to the tool's input schema.
func Enum(values ...string) PropertyOption {
	return func(s *openapi3.Schema) {
		enum := make([]any, len(values))
		for i, v := range values {
			enum[i] = v
		}
		s.Enum = enum
	}
}

// Properties adds properties to the tool's input schema.
func Properties(props openapi3.Schemas) PropertyOption {
	return func(s *openapi3.Schema) {
		s.Properties = props
	}
}

// WithArray adds an array to the tool's input schema.
func WithArray(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := &openapi3.Schema{
			Type: &openapi3.Types{openapi3.TypeArray},
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

// Items adds items to the tool's input schema.
func Items(itemSchema *openapi3.Schema) PropertyOption {
	return func(s *openapi3.Schema) {
		s.Items = openapi3.NewSchemaRef("", itemSchema)
	}
}

// MinItems sets the minimum number of items in the array.
func MinItems(min int) PropertyOption {
	return func(s *openapi3.Schema) {
		val := uint64(min)
		s.MinItems = val
	}
}

// MaxItems sets the maximum number of items in the array.
func MaxItems(max int) PropertyOption {
	return func(s *openapi3.Schema) {
		val := uint64(max)
		s.MaxItems = &val
	}
}

// UniqueItems sets whether the array contains unique items.
func UniqueItems(unique bool) PropertyOption {
	return func(s *openapi3.Schema) {
		s.UniqueItems = unique
	}
}

// NewTextResult creates a new text result.
func NewTextResult(text string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{NewTextContent(text)},
	}
}

// NewErrorResult creates a new error result.
func NewErrorResult(text string) *CallToolResult {
	return &CallToolResult{
		IsError: true,
		Content: []Content{NewTextContent(text)},
	}
}

func parseCallToolResult(rawMessage *json.RawMessage) (*CallToolResult, error) {
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
		content, err := parseContent(contentMap)
		if err != nil {
			return nil, err
		}

		result.Content = append(result.Content, content)
	}

	return &result, nil
}

func parseContent(contentMap map[string]any) (Content, error) {
	contentType := extractString(contentMap, "type")

	switch contentType {
	case "text":
		return parseTextContent(contentMap)
	case "image":
		return parseImageContent(contentMap)
	case "resource":
		return parseResourceContent(contentMap)
	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// parseTextContent parses text content
func parseTextContent(contentMap map[string]any) (Content, error) {
	text := extractString(contentMap, "text")
	if text == "" {
		return nil, fmt.Errorf("text is missing")
	}
	return NewTextContent(text), nil
}

// parseImageContent parses image content
func parseImageContent(contentMap map[string]any) (Content, error) {
	data := extractString(contentMap, "data")
	mimeType := extractString(contentMap, "mimeType")
	if data == "" || mimeType == "" {
		return nil, fmt.Errorf("image data or mimeType is missing")
	}
	return NewImageContent(data, mimeType), nil
}

// parseResourceContent parses resource content
func parseResourceContent(contentMap map[string]any) (Content, error) {
	resourceMap := extractMap(contentMap, "resource")
	if resourceMap == nil {
		return nil, fmt.Errorf("resource is missing")
	}
	resourceContents, err := parseResourceContents(resourceMap)
	if err != nil {
		return nil, err
	}
	return NewEmbeddedResource(resourceContents), nil
}

// extractString extracts a string value from a map by key
func extractString(data map[string]any, key string) string {
	if value, ok := data[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// extractMap extracts a map value from a map by key
func extractMap(data map[string]any, key string) map[string]any {
	if value, ok := data[key]; ok {
		if m, ok := value.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func parseResourceContents(contentMap map[string]any) (ResourceContents, error) {
	uri := extractString(contentMap, "uri")
	if uri == "" {
		return nil, fmt.Errorf("resource uri is missing")
	}

	mimeType := extractString(contentMap, "mimeType")

	if text := extractString(contentMap, "text"); text != "" {
		return TextResourceContents{
			URI:      uri,
			MIMEType: mimeType,
			Text:     text,
		}, nil
	}

	if blob := extractString(contentMap, "blob"); blob != "" {
		return BlobResourceContents{
			URI:      uri,
			MIMEType: mimeType,
			Blob:     blob,
		}, nil
	}

	return nil, fmt.Errorf("unsupported resource type")
}
