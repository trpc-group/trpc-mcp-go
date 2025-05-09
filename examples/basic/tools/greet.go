package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

// NewGreetTool creates a simple greeting tool.
func NewGreetTool() *schema.Tool {
	return schema.NewTool("greet",
		func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
			// Check if context is cancelled.
			select {
			case <-ctx.Done():
				return schema.NewErrorResult("Request cancelled"), ctx.Err()
			default:
				// Continue execution.
			}

			// Extract name parameter.
			name := "World"
			if nameArg, ok := req.Params.Arguments["name"]; ok {
				if nameStr, ok := nameArg.(string); ok && nameStr != "" {
					name = nameStr
				}
			}

			// Create greeting message.
			greeting := fmt.Sprintf("Hello, %s!", name)

			// Create tool result.
			return schema.NewTextResult(greeting), nil
		},
		schema.WithDescription("A simple greeting tool that returns a greeting message."),
		schema.WithString("name",
			schema.Description("The name to greet."),
		),
	)
}

// Add a more advanced tool example.
func NewAdvancedGreetTool() *schema.Tool {
	return schema.NewTool("advanced-greet",
		func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
			// Extract parameters.
			name := "World"
			if nameArg, ok := req.Params.Arguments["name"]; ok {
				if nameStr, ok := nameArg.(string); ok && nameStr != "" {
					name = nameStr
				}
			}

			format := "text"
			if formatArg, ok := req.Params.Arguments["format"]; ok {
				if formatStr, ok := formatArg.(string); ok && formatStr != "" {
					format = formatStr
				}
			}

			// Example: if name is "error", return an error result.
			if name == "error" {
				return schema.NewErrorResult(fmt.Sprintf("Cannot greet '%s': name not allowed.", name)), nil
			}

			// Return different content types based on format.
			switch format {
			case "json":
				// JSON format is no longer supported, fallback to text.
				jsonMessage := fmt.Sprintf("JSON format: {\"greeting\":\"Hello, %s!\",\"timestamp\":\"2025-05-14T12:00:00Z\"}", name)
				return &schema.CallToolResult{
					Content: []schema.ToolContent{
						schema.NewTextContent(jsonMessage),
					},
				}, nil
			case "html":
				// HTML format is no longer supported, fallback to text.
				htmlContent := fmt.Sprintf("<h1>Greeting</h1><p>Hello, <strong>%s</strong>!</p>", name)
				return &schema.CallToolResult{
					Content: []schema.ToolContent{
						schema.NewTextContent(htmlContent),
					},
				}, nil
			default:
				// Default: return plain text.
				return schema.NewTextResult(fmt.Sprintf("Hello, %s!", name)), nil
			}
		},
		schema.WithDescription("An enhanced greeting tool supporting multiple output formats."),
		schema.WithString("name", schema.Description("The name to greet.")),
		schema.WithString("format",
			schema.Description("Output format: text, json, or html."),
			schema.Default("text")),
	)
}

// BasicGreet handler function.
func BasicGreet(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
	// Extract name.
	name := "World"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Create greeting message.
	greeting := fmt.Sprintf("Hello, %s! This is a simple greeting.", name)

	// Return result.
	return &schema.CallToolResult{
		Content: []schema.ToolContent{
			schema.NewTextContent(greeting),
		},
	}, nil
}

// FancyGreet handler function.
func FancyGreet(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
	// Extract name.
	name := "World"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Extract format.
	format := "standard" // Default format.
	if formatArg, ok := req.Params.Arguments["format"]; ok {
		if formatStr, ok := formatArg.(string); ok && formatStr != "" {
			format = formatStr
		}
	}

	// Create greeting message.
	// 创建问候消息
	var greeting string
	switch strings.ToLower(format) {
	case "fancy":
		greeting = fmt.Sprintf("✨ 欢迎，尊敬的 %s! ✨\n这是一个*华丽*的问候。", name)
	case "minimal":
		greeting = fmt.Sprintf("Hi %s", name)
	default:
		greeting = fmt.Sprintf("你好，%s！这是一个标准问候。", name)
	}

	// 返回结果
	return &schema.CallToolResult{
		Content: []schema.ToolContent{
			schema.NewTextContent(greeting),
		},
	}, nil
}
