package main

import (
	"context"
	"os"

	"log"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Initialize log.
	log.Printf("Starting example client...")
	// Basic settings.
	serverURL := "http://localhost:3000/mcp"
	clientInfo := mcp.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}

	// Create MCP client.
	log.Printf("===== Create client =====")
	newClient, err := mcp.NewClient(serverURL, clientInfo)
	if err != nil {
		log.Printf("Error creating client: %v", err)
		os.Exit(1)
	}

	// Initialize client.
	log.Printf("===== Initialize client =====")
	ctx := context.Background()
	initResult, err := newClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Printf("Initialization error: %v", err)
		os.Exit(1)
	}
	log.Printf("Connected to server: %s %s", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	// List resources.
	log.Printf("===== List resources =====")
	resources, err := newClient.ListResources(ctx, &mcp.ListResourcesRequest{})
	if err != nil {
		log.Printf("List resources error: %v", err)
	} else {
		log.Printf("Found %d resources:", len(resources.Resources))
		for _, resource := range resources.Resources {
			log.Printf("- %s: %s (%s)", resource.Name, resource.Description, resource.URI)
		}

		// Read the first resource (if any).
		if len(resources.Resources) > 0 {
			log.Printf("===== Read resource: %s =====", resources.Resources[0].Name)
			readResourceReq := &mcp.ReadResourceRequest{}
			readResourceReq.Params.URI = resources.Resources[0].URI
			resourceContent, err := newClient.ReadResource(ctx, readResourceReq)
			if err != nil {
				log.Printf("Read resource error: %v", err)
			} else {
				log.Printf("Successfully read resource, content item count: %d", len(resourceContent.Contents))
				for i, content := range resourceContent.Contents {
					switch c := content.(type) {
					case mcp.TextResourceContents:
						log.Printf("[%d] Text resource: %s (first 50 chars: %s...)",
							i, c.URI, truncateString(c.Text, 50))
					case mcp.BlobResourceContents:
						log.Printf("[%d] Binary resource: %s (size: %d bytes)",
							i, c.URI, len(c.Blob))
					default:
						log.Printf("[%d] Unknown resource type", i)
					}
				}
			}
		}
	}

	// List prompts.
	log.Printf("===== List prompts =====")
	prompts, err := newClient.ListPrompts(ctx, &mcp.ListPromptsRequest{})
	if err != nil {
		log.Printf("List prompts error: %v", err)
	} else {
		log.Printf("Found %d prompts:", len(prompts.Prompts))
		for _, prompt := range prompts.Prompts {
			log.Printf("- %s: %s", prompt.Name, prompt.Description)
			if len(prompt.Arguments) > 0 {
				log.Printf("  Arguments:")
				for _, arg := range prompt.Arguments {
					required := ""
					if arg.Required {
						required = " (required)"
					}
					log.Printf("  - %s: %s%s", arg.Name, arg.Description, required)
				}
			}
		}

		// Get the first prompt (if any).
		if len(prompts.Prompts) > 0 {
			// Create an empty parameter map (should provide required params in real use).
			arguments := make(map[string]string)
			for _, arg := range prompts.Prompts[0].Arguments {
				if arg.Required {
					// Provide an example value for required parameters.
					arguments[arg.Name] = "example value"
				}
			}

			log.Printf("===== Get prompt: %s =====", prompts.Prompts[0].Name)
			getPromptReq := &mcp.GetPromptRequest{}
			getPromptReq.Params.Name = prompts.Prompts[0].Name
			getPromptReq.Params.Arguments = arguments
			promptContent, err := newClient.GetPrompt(ctx, getPromptReq)
			if err != nil {
				log.Printf("Get prompt error: %v", err)
			} else {
				log.Printf("Successfully got prompt, message count: %d", len(promptContent.Messages))
				if promptContent.Description != "" {
					log.Printf("Prompt description: %s", promptContent.Description)
				}

				for i, msg := range promptContent.Messages {
					switch c := msg.Content.(type) {
					case mcp.TextContent:
						log.Printf("[%d] %s message (Text): %s", i, msg.Role, truncateString(c.Text, 50))
					case mcp.ImageContent:
						log.Printf("[%d] %s message (Image): MIME=%s, DataLen=%d", i, msg.Role, c.MimeType, len(c.Data))
					case mcp.AudioContent:
						log.Printf("[%d] %s message (Audio): MIME=%s, DataLen=%d", i, msg.Role, c.MimeType, len(c.Data))
					case mcp.EmbeddedResource:
						var resourceURI string
						if textResource, ok := c.Resource.(mcp.TextResourceContents); ok {
							resourceURI = textResource.URI
						} else if blobResource, ok := c.Resource.(mcp.BlobResourceContents); ok {
							resourceURI = blobResource.URI
						}
						log.Printf("[%d] %s message (Resource): URI=%s", i, msg.Role, resourceURI)
					default:
						log.Printf("[%d] %s message (unknown content type: %T)", i, msg.Role, c)
					}
				}
			}
		}
	}

	// List tools.
	log.Printf("===== List tools =====")
	tools, err := newClient.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		log.Printf("List tools error: %v", err)
	} else {
		log.Printf("Found %d tools:", len(tools.Tools))
		for _, tool := range tools.Tools {
			log.Printf("- %s: %s", tool.Name, tool.Description)
		}
	}

	callToolReq := &mcp.CallToolRequest{}
	callToolReq.Params.Name = "greet"
	callToolReq.Params.Arguments = map[string]interface{}{"name": "MCP User"}
	callToolResult, err := newClient.CallTool(ctx, callToolReq)
	if err != nil {
		log.Printf("Call tool error: %v", err)
	} else {
		log.Printf("Successfully called tool, message count: %d", len(callToolResult.Content))
		for i, content := range callToolResult.Content {
			switch c := content.(type) {
			case mcp.TextContent:
				log.Printf("[%d] Text content: %s", i, truncateString(c.Text, 50))
			default:
				log.Printf("[%d] Unknown content type", i)
			}
		}
	}

	// close client (cleanup resources).
	if err := newClient.Close(); err != nil {
		log.Printf("close client error: %v", err)
	}

	log.Printf("Test finished!")
}

// truncateString truncates a string and adds ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
