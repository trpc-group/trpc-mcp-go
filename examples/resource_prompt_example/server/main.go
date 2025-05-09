package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/server"
)

func main() {
	// Create server.
	mcpServer := server.NewServer(":3000", schema.Implementation{
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
	fmt.Println("MCP server started on :3000, path /mcp")
	err := mcpServer.Start()
	if err != nil && err != http.ErrServerClosed {
		fmt.Printf("Server error: %v\n", err)
	}
}

// Register example resources.
func registerExampleResources(s *server.Server) {
	// Register text resource.
	textResource := &schema.Resource{
		URI:         "resource://example/text",
		Name:        "example-text",
		Description: "Example text resource",
		MimeType:    "text/plain",
	}
	err := s.RegisterResource(textResource)
	if err != nil {
		fmt.Printf("Error registering text resource: %v\n", err)
	} else {
		fmt.Printf("Registered text resource: %s\n", textResource.Name)
	}

	// Register image resource.
	imageResource := &schema.Resource{
		URI:         "resource://example/image",
		Name:        "example-image",
		Description: "Example image resource",
		MimeType:    "image/png",
	}
	err = s.RegisterResource(imageResource)
	if err != nil {
		fmt.Printf("Error registering image resource: %v\n", err)
	} else {
		fmt.Printf("Registered image resource: %s\n", imageResource.Name)
	}
}

// Register example prompts.
func registerExamplePrompts(s *server.Server) {
	// Register basic prompt.
	basicPrompt := &schema.Prompt{
		Name:        "basic-prompt",
		Description: "Basic prompt example",
		Arguments: []schema.PromptArgument{
			{
				Name:        "name",
				Description: "User name",
				Required:    true,
			},
		},
	}
	err := s.RegisterPrompt(basicPrompt)
	if err != nil {
		fmt.Printf("Error registering basic prompt: %v\n", err)
	} else {
		fmt.Printf("Registered basic prompt: %s\n", basicPrompt.Name)
	}

	// Register advanced prompt.
	advancedPrompt := &schema.Prompt{
		Name:        "advanced-prompt",
		Description: "Advanced prompt example",
		Arguments: []schema.PromptArgument{
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
		fmt.Printf("Error registering advanced prompt: %v\n", err)
	} else {
		fmt.Printf("Registered advanced prompt: %s\n", advancedPrompt.Name)
	}
}

// Register example tools.
func registerExampleTools(s *server.Server) {
	// Register a simple greeting tool.
	greetTool := schema.NewTool("greet", func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
		// Extract name from parameters.
		name, _ := req.Params.Arguments["name"].(string)
		if name == "" {
			name = "World"
		}

		// Create response content.
		return schema.NewTextResult(fmt.Sprintf("Hello, %s! Welcome to the resource and prompt example server.", name)), nil
	}, schema.WithDescription("Greeting tool"))

	err := s.RegisterTool(greetTool)
	if err != nil {
		fmt.Printf("Error registering greeting tool: %v\n", err)
	} else {
		fmt.Printf("Registered greeting tool: %s\n", greetTool.Name)
	}
}
