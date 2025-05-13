package main

import (
	"context"
	"log"
	"net/http"

	"trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Create server.
	mcpServer := mcp.NewServer(":3000", mcp.Implementation{
		Name:    "Resource-Prompt-Example",
		Version: "0.1.0",
	})

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
	err := s.RegisterResource(textResource)
	if err != nil {
		log.Printf("Error registering text resource: %v\n", err)
	} else {
		log.Printf("Registered text resource: %s\n", textResource.Name)
	}

	// Register image resource.
	imageResource := &mcp.Resource{
		URI:         "resource://example/image",
		Name:        "example-image",
		Description: "Example image resource",
		MimeType:    "image/png",
	}
	err = s.RegisterResource(imageResource)
	if err != nil {
		log.Printf("Error registering image resource: %v\n", err)
	} else {
		log.Printf("Registered image resource: %s\n", imageResource.Name)
	}
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
	err := s.RegisterPrompt(basicPrompt)
	if err != nil {
		log.Printf("Error registering basic prompt: %v\n", err)
	} else {
		log.Printf("Registered basic prompt: %s\n", basicPrompt.Name)
	}

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
	err = s.RegisterPrompt(advancedPrompt)
	if err != nil {
		log.Printf("Error registering advanced prompt: %v\n", err)
	} else {
		log.Printf("Registered advanced prompt: %s\n", advancedPrompt.Name)
	}
}

// Register example tools.
func registerExampleTools(s *mcp.Server) {
	// Register a simple greeting tool.
	greetTool := mcp.NewTool("greet", func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract name from parameters.
		name, _ := req.Params.Arguments["name"].(string)
		if name == "" {
			name = "World"
		}

		// Create response content.
		greeting := "Hello, " + name + "! Welcome to the resource and prompt example server."
		return mcp.NewTextResult(greeting), nil
	}, mcp.WithDescription("Greeting tool"))

	err := s.RegisterTool(greetTool)
	if err != nil {
		log.Printf("Error registering greeting tool: %v\n", err)
	} else {
		log.Printf("Registered greeting tool: %s\n", greetTool.Name)
	}
}
