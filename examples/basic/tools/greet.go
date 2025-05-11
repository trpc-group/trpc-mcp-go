package tools

import (
	"context"
	"fmt"
	"strings"

	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

// NewGreetTool creates a simple greeting tool.
func NewGreetTool() *mcp.Tool {
	return mcp.NewTool("greet",
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Check if context is cancelled.
			select {
			case <-ctx.Done():
				return mcp.NewErrorResult("Request cancelled"), ctx.Err()
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
			return mcp.NewTextResult(greeting), nil
		},
		mcp.WithDescription("A simple greeting tool that returns a greeting message."),
		mcp.WithString("name",
			mcp.Description("The name to greet."),
		),
	)
}

// Add a more advanced tool example.
func NewAdvancedGreetTool() *mcp.Tool {
	return mcp.NewTool("advanced-greet",
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
				return mcp.NewErrorResult(fmt.Sprintf("Cannot greet '%s': name not allowed.", name)), nil
			}

			// Return different content types based on format.
			switch format {
			case "json":
				// JSON format is no longer supported, fallback to text.
				jsonMessage := fmt.Sprintf("JSON format: {\"greeting\":\"Hello, %s!\",\"timestamp\":\"2025-05-14T12:00:00Z\"}", name)
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.NewTextContent(jsonMessage),
					},
				}, nil
			case "html":
				// HTML format is no longer supported, fallback to text.
				htmlContent := fmt.Sprintf("<h1>Greeting</h1><p>Hello, <strong>%s</strong>!</p>", name)
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.NewTextContent(htmlContent),
					},
				}, nil
			default:
				// Default: return plain text.
				return mcp.NewTextResult(fmt.Sprintf("Hello, %s!", name)), nil
			}
		},
		mcp.WithDescription("An enhanced greeting tool supporting multiple output formats."),
		mcp.WithString("name", mcp.Description("The name to greet.")),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, or html."),
			mcp.Default("text")),
	)
}

// BasicGreet handler function.
func BasicGreet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(greeting),
		},
	}, nil
}

// FancyGreet handler function.
func FancyGreet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(greeting),
		},
	}, nil
}
