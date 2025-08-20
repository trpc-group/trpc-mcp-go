// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Create server.
	mcpServer := mcp.NewServer(
		"Resource-Prompt-Example",      // Server name
		"0.1.0",                        // Server version
		mcp.WithServerAddress(":3000"), // Server address
	)

	// Register resources.
	registerExampleResources(mcpServer)

	// Register prompts.
	registerExamplePrompts(mcpServer)

	// Register tools.
	registerExampleTools(mcpServer)

	// Start server.
	log.Printf("MCP server started on :3000, path /mcp")
	err := mcpServer.Start()
	if err != nil && err != http.ErrServerClosed {
		log.Printf("Server error: %v\n", err)
	}
}

// Register example resources.
func registerExampleResources(s *mcp.Server) {
	// Register text resource.
	textResource := &mcp.Resource{
		URI:         "resource://example/text",
		Name:        "example-text",
		Description: "Example text resource",
		MimeType:    "text/plain",
	}

	// Define text resource handler
	textHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) (mcp.ResourceContents, error) {
		return mcp.TextResourceContents{
			URI:      textResource.URI,
			MIMEType: textResource.MimeType,
			Text:     "This is an example text resource content.",
		}, nil
	}

	s.RegisterResource(textResource, textHandler)
	log.Printf("Registered text resource: %s", textResource.Name)

	// Register image resource.
	imageResource := &mcp.Resource{
		URI:         "resource://example/image",
		Name:        "example-image",
		Description: "Example image resource",
		MimeType:    "image/png",
	}

	// Define image resource handler
	imageHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// In a real application, you would read the actual image data
		// For this example, we'll return a placeholder base64-encoded image
		return []mcp.ResourceContents{
			mcp.BlobResourceContents{
				URI:      imageResource.URI,
				MIMEType: imageResource.MimeType,
				Blob:     "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==", // 1x1 transparent PNG
			},
		}, nil
	}

	// Recommend: Use RegisterResources() for resources with multiple contents.
	s.RegisterResources(imageResource, imageHandler)
	log.Printf("Registered image resource: %s", imageResource.Name)

	// Register resource completion.
	textResourceCompletion := &mcp.Resource{
		URI:         "resource://example/completion",
		Name:        "example-text-completion",
		Description: "Example text resource with completion",
		MimeType:    "text/plain",
	}

	// Define completion resource handler
	resourceCompletionHandler := func(ctx context.Context, req *mcp.CompleteCompletionRequest) (*mcp.CompleteCompletionResult, error) {
		result := &mcp.CompleteCompletionResult{}
		if req.Params.Argument.Name == "query" {
			if req.Params.Context.Arguments != nil {
				// Use context argument if provided
				context, ok := req.Params.Context.Arguments["context"]
				if ok && context != "" {
					result.Completion.Values = []string{
						fmt.Sprintf("Query param: %s", req.Params.Argument.Value),
						fmt.Sprintf("Context: %s", context),
					}
				} else {
					result.Completion.Values = []string{"Context is empty"}
				}
			} else {
				result.Completion.Values = []string{"First document text", "Second document text", "Third document text"}
			}
		} else {
			result.Completion.Values = []string{"Unknown argument"}
		}
		result.Completion.Total = len(result.Completion.Values)
		result.Completion.HasMore = false
		return result, nil
	}
	s.RegisterResource(textResourceCompletion, textHandler,
		mcp.WithResourceCompletion(resourceCompletionHandler),
	)
	log.Printf("Registered text resource completion: %s", textResourceCompletion.Name)

	// Register resource template with completion
	fileTemplate := mcp.NewResourceTemplate(
		"file://{filename}",
		"example-file-template-completion",
		mcp.WithTemplateDescription("Example file resource template with completion"),
		mcp.WithTemplateMIMEType("text/plain"),
	)

	// Define file template handler
	fileTemplateHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      fileTemplate.URITemplate.Raw(),
				MIMEType: "text/plain",
				Text:     "This is an example file resource template content.",
			}}, nil
	}

	// Define completion handler for file paths
	fileCompletionHandler := func(ctx context.Context, req *mcp.CompleteCompletionRequest, params map[string]string) (*mcp.CompleteCompletionResult, error) {
		result := &mcp.CompleteCompletionResult{}
		if req.Params.Argument.Name == "keyword" {
			keyword := req.Params.Argument.Value
			result.Completion.Values = []string{
				fmt.Sprintf("file://test?keyword=%s&paramFilename=%s", keyword, params["filename"]),
				fmt.Sprintf("Param argument 'keyword' value: %s", keyword),
				fmt.Sprintf("Param template 'filename' value: %s", params["filename"]),
			}
		} else {
			result.Completion.Values = []string{"unknown argument"}
		}

		total := len(result.Completion.Values)
		result.Completion.Total = total
		result.Completion.HasMore = false

		return result, nil
	}

	s.RegisterResourceTemplate(
		fileTemplate,
		fileTemplateHandler,
		mcp.WithTemplateCompletion(fileCompletionHandler),
	)
	log.Printf("Registered file resource template completion: %s", fileTemplate.Name)
}

// Register example prompts.
func registerExamplePrompts(s *mcp.Server) {
	// Register basic prompt.
	basicPrompt := &mcp.Prompt{
		Name:        "basic-prompt",
		Description: "Basic prompt example",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "name",
				Description: "User name",
				Required:    true,
			},
		},
	}

	// Define basic prompt handler
	basicPromptHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		name := req.Params.Arguments["name"]
		return &mcp.GetPromptResult{
			Description: basicPrompt.Description,
			Messages: []mcp.PromptMessage{
				{
					Role: "user",
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Hello, %s! This is a basic prompt example.", name),
					},
				},
			},
		}, nil
	}

	s.RegisterPrompt(basicPrompt, basicPromptHandler)
	log.Printf("Registered basic prompt: %s", basicPrompt.Name)

	// Register advanced prompt.
	advancedPrompt := &mcp.Prompt{
		Name:        "advanced-prompt",
		Description: "Advanced prompt example",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "topic",
				Description: "Topic",
				Required:    true,
			},
			{
				Name:        "length",
				Description: "Length",
				Required:    false,
			},
		},
	}

	// Define advanced prompt handler
	advancedPromptHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		topic := req.Params.Arguments["topic"]
		length := req.Params.Arguments["length"]
		if length == "" {
			length = "medium"
		}

		return &mcp.GetPromptResult{
			Description: advancedPrompt.Description,
			Messages: []mcp.PromptMessage{
				{
					Role: "user",
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Let's discuss about %s. Please provide a %s length response.", topic, length),
					},
				},
			},
		}, nil
	}

	s.RegisterPrompt(advancedPrompt, advancedPromptHandler)
	log.Printf("Registered advanced prompt: %s", advancedPrompt.Name)

	// Register prompt with completion support
	codeReviewPrompt := &mcp.Prompt{
		Name:        "code_review",
		Description: "Code review prompt completion",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "language",
				Description: "Programming language of the code",
				Required:    true,
			},
		},
	}

	// Define completion prompt handler
	codeReviewPromptHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: "user",
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Please review the following %s code:\n%s", req.Params.Arguments["language"], req.Params.Arguments["code"]),
					},
				},
			},
		}, nil
	}

	// Define completion handler for prompt arguments
	codeReviewPromptCompletionHandler := func(ctx context.Context, req *mcp.CompleteCompletionRequest) (*mcp.CompleteCompletionResult, error) {
		result := &mcp.CompleteCompletionResult{}
		if req.Params.Argument.Name == "language" {
			prefix := req.Params.Argument.Value
			categories := []string{
				"python", "pytorch", "javascript", "typescript",
				"go", "java", "c++", "c#", "ruby", "php",
			}

			var matches []string
			for _, cat := range categories {
				if prefix == "" || len(prefix) == 0 {
					matches = append(matches, cat)
				} else if len(cat) >= len(prefix) && cat[:len(prefix)] == prefix {
					matches = append(matches, cat)
				}
			}

			// Limit to first 10 matches
			if len(matches) > 10 {
				matches = matches[:10]
			}

			result.Completion.Values = matches
		} else {
			result.Completion.Values = []string{"unknown argument"}
		}

		total := len(result.Completion.Values)
		result.Completion.Total = total
		result.Completion.HasMore = false

		return result, nil
	}

	s.RegisterPrompt(codeReviewPrompt, codeReviewPromptHandler, mcp.WithPromptCompletion(codeReviewPromptCompletionHandler))
	log.Printf("Registered prompt completion: %s", codeReviewPrompt.Name)
}

// Register example tools.
func registerExampleTools(s *mcp.Server) {
	// Register a simple greeting tool.
	greetTool := mcp.NewTool("greet", mcp.WithDescription("Greeting tool"))

	// Define the handler function
	greetHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract name from parameters.
		name, _ := req.Params.Arguments["name"].(string)
		if name == "" {
			name = "World"
		}

		// Create response content.
		greeting := "Hello, " + name + "! Welcome to the resource and prompt example server."
		return mcp.NewTextResult(greeting), nil
	}

	s.RegisterTool(greetTool, greetHandler)
	log.Printf("Registered greeting tool: %s", greetTool.Name)
}
