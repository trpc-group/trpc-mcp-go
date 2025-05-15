package main

import (
	"context"
	"log"
	"os"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Initialize log.
	log.Println("Starting example client...")

	// Create context.
	ctx := context.Background()

	// Create client.
	serverURL := "http://localhost:3000/mcp"
	// Inject custom logger via WithClientLogger.
	mcpClient, err := mcp.NewClient(
		serverURL,
		mcp.Implementation{
			Name:    "MCP-Go-Client",
			Version: "1.0.0",
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	) // End of NewClient
	if err != nil {
		log.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer mcpClient.Close()

	// Initialize client.
	log.Println("===== Initialize client =====")
	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Printf("Initialization failed: %v\n", err)
		os.Exit(1)
	}

	log.Printf("Server info: %s %s", initResp.ServerInfo.Name, initResp.ServerInfo.Version)
	log.Printf("Protocol version: %s", initResp.ProtocolVersion)
	if initResp.Instructions != "" {
		log.Printf("Server instructions: %s", initResp.Instructions)
	}

	// Get session ID.
	sessionID := mcpClient.GetSessionID()
	if sessionID != "" {
		log.Printf("Session ID: %s", sessionID)
	}

	// List tools.
	log.Println("===== List available tools =====")
	listToolsResp, err := mcpClient.ListTools(ctx, &mcp.ListToolsRequest{})
	tools := listToolsResp.Tools
	if err != nil {
		log.Printf("Failed to list tools: %v", err)
	} else {
		if len(tools) == 0 {
			log.Printf("No available tools.")
		} else {
			log.Printf("Found %d tools:", len(tools))
			for _, tool := range tools {
				log.Printf("- %s: %s", tool.Name, tool.Description)
			}
		}

		// Call tool (if any).
		if len(tools) > 0 {
			log.Println("===== Call tool: %s =====", tools[0].Name)
			callToolReq := &mcp.CallToolRequest{}
			callToolReq.Params.Name = tools[0].Name
			callToolReq.Params.Arguments = map[string]interface{}{
				"name": "MCP User",
			}
			callToolResp, err := mcpClient.CallTool(ctx, callToolReq)
			if err != nil {
				log.Printf("Failed to call tool: %v", err)
			} else {
				content := callToolResp.Content
				log.Printf("Tool result:")
				for _, item := range content {
					// Type assertion for different content types.
					if textContent, ok := item.(mcp.TextContent); ok {
						log.Printf("  %s", textContent.Text)
					}
				}
			}
		}
	}

	// List prompts.
	log.Println("===== List prompts =====")
	promptsResult, err := mcpClient.ListPrompts(ctx, &mcp.ListPromptsRequest{})
	if err != nil {
		log.Printf("Failed to list prompts: %v", err)
	} else {
		log.Printf("Found %d prompts:", len(promptsResult.Prompts))
		for _, prompt := range promptsResult.Prompts {
			log.Println("- %s: %s", prompt.Name, prompt.Description)
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
		if len(promptsResult.Prompts) > 0 {
			// Create a parameter map.
			arguments := make(map[string]string)
			for _, arg := range promptsResult.Prompts[0].Arguments {
				if arg.Required {
					// Provide an example value for required parameters.
					arguments[arg.Name] = "example value"
				}
			}

			log.Printf("===== Get prompt: %s =====", promptsResult.Prompts[0].Name)
			getPromptReq := &mcp.GetPromptRequest{}
			getPromptReq.Params.Name = promptsResult.Prompts[0].Name
			getPromptReq.Params.Arguments = arguments
			promptContent, err := mcpClient.GetPrompt(ctx, getPromptReq)
			if err != nil {
				log.Printf("Failed to get prompt: %v", err)
			} else {
				log.Printf("Successfully got prompt, message count: %d", len(promptContent.Messages))
				if promptContent.Description != "" {
					log.Printf("Prompt description: %s", promptContent.Description)
				}

				for i, msg := range promptContent.Messages {
					switch content := msg.Content.(type) {
					case mcp.TextContent:
						log.Printf("[%d] %s message: %s", i, msg.Role, truncateString(content.Text, 50))
					case mcp.ImageContent:
						log.Printf("[%d] %s message (image)", i, msg.Role)
					case mcp.AudioContent:
						log.Printf("[%d] %s message (audio)", i, msg.Role)
					case mcp.EmbeddedResource:
						log.Printf("[%d] %s message (embedded resource)", i, msg.Role)
					default:
						log.Printf("[%d] %s message (unknown content type)", i, msg.Role)
					}
				}
			}
		}
	}

	// List resources.
	log.Printf("===== List resources =====")
	resourcesResult, err := mcpClient.ListResources(ctx, &mcp.ListResourcesRequest{})
	if err != nil {
		log.Printf("Failed to list resources: %v", err)
	} else {
		log.Printf("Found %d resources:", len(resourcesResult.Resources))
		for _, resource := range resourcesResult.Resources {
			log.Printf("- %s: %s (%s)", resource.Name, resource.Description, resource.URI)
		}

		// Read the first resource (if any).
		if len(resourcesResult.Resources) > 0 {
			log.Printf("===== Read resource: %s =====", resourcesResult.Resources[0].Name)
			readResourceReq := &mcp.ReadResourceRequest{}
			readResourceReq.Params.URI = resourcesResult.Resources[0].URI
			resourceContent, err := mcpClient.ReadResource(ctx, readResourceReq)
			if err != nil {
				log.Printf("Failed to read resource: %v", err)
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

	// Terminate session.
	if sessionID != "" {
		log.Printf("===== Terminate session =====")
		if err := mcpClient.TerminateSession(ctx); err != nil {
			log.Printf("Failed to terminate session: %v", err)
		} else {
			log.Printf("Session terminated.")
		}
	}

	log.Printf("Example finished.")
}

// truncateString truncates a string and adds ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
