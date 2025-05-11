package main

import (
	"context"
	"os"

	"trpc.group/trpc-go/trpc-mcp-go/client"
	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

func main() {
	// Initialize log.
	log.Info("Starting example client...")

	// Create context.
	ctx := context.Background()

	// Create client.
	mcpClient, err := client.NewClient("http://localhost:3000/mcp", mcp.Implementation{
		Name:    "MCP-Go-Client",
		Version: "1.0.0",
	}, client.WithProtocolVersion(mcp.ProtocolVersion_2024_11_05)) // Specify protocol version explicitly.
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		os.Exit(1)
	}
	defer mcpClient.Close()

	// Initialize client.
	log.Info("===== Initialize client =====")
	initResp, err := mcpClient.Initialize(ctx)
	if err != nil {
		log.Errorf("Initialization failed: %v", err)
		os.Exit(1)
	}

	log.Infof("Server info: %s %s", initResp.ServerInfo.Name, initResp.ServerInfo.Version)
	log.Infof("Protocol version: %s", initResp.ProtocolVersion)
	if initResp.Instructions != "" {
		log.Infof("Server instructions: %s", initResp.Instructions)
	}

	// Get session ID.
	sessionID := mcpClient.GetSessionID()
	if sessionID != "" {
		log.Infof("Session ID: %s", sessionID)
	}

	// List tools.
	log.Info("===== List available tools =====")
	listToolsResp, err := mcpClient.ListTools(ctx)
	tools := listToolsResp.Tools
	if err != nil {
		log.Error("Failed to list tools: %v", err)
	} else {
		if len(tools) == 0 {
			log.Infof("No available tools.")
		} else {
			log.Infof("Found %d tools:", len(tools))
			for _, tool := range tools {
				log.Infof("- %s: %s", tool.Name, tool.Description)
			}
		}

		// Call tool (if any).
		if len(tools) > 0 {
			log.Info("===== Call tool: %s =====", tools[0].Name)
			callToolResp, err := mcpClient.CallTool(ctx, tools[0].Name, map[string]interface{}{
				"name": "MCP User",
			})
			if err != nil {
				log.Errorf("Failed to call tool: %v", err)
			} else {
				content := callToolResp.Content
				log.Infof("Tool result:")
				for _, item := range content {
					// Type assertion for different content types.
					if textContent, ok := item.(mcp.TextContent); ok {
						log.Infof("  %s", textContent.Text)
					}
				}
			}
		}
	}

	// List prompts.
	log.Info("===== List prompts =====")
	promptsResult, err := mcpClient.ListPrompts(ctx)
	if err != nil {
		log.Errorf("Failed to list prompts: %v", err)
	} else {
		log.Infof("Found %d prompts:", len(promptsResult.Prompts))
		for _, prompt := range promptsResult.Prompts {
			log.Info("- %s: %s", prompt.Name, prompt.Description)
			if len(prompt.Arguments) > 0 {
				log.Infof("  Arguments:")
				for _, arg := range prompt.Arguments {
					required := ""
					if arg.Required {
						required = " (required)"
					}
					log.Infof("  - %s: %s%s", arg.Name, arg.Description, required)
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

			log.Infof("===== Get prompt: %s =====", promptsResult.Prompts[0].Name)
			promptContent, err := mcpClient.GetPrompt(ctx, promptsResult.Prompts[0].Name, arguments)
			if err != nil {
				log.Errorf("Failed to get prompt: %v", err)
			} else {
				log.Infof("Successfully got prompt, message count: %d", len(promptContent.Messages))
				if promptContent.Description != "" {
					log.Infof("Prompt description: %s", promptContent.Description)
				}

				for i, msg := range promptContent.Messages {
					switch content := msg.Content.(type) {
					case mcp.TextContent:
						log.Infof("[%d] %s message: %s", i, msg.Role, truncateString(content.Text, 50))
					case mcp.ImageContent:
						log.Infof("[%d] %s message (image)", i, msg.Role)
					case mcp.AudioContent:
						log.Infof("[%d] %s message (audio)", i, msg.Role)
					case mcp.EmbeddedResource:
						log.Infof("[%d] %s message (embedded resource)", i, msg.Role)
					default:
						log.Infof("[%d] %s message (unknown content type)", i, msg.Role)
					}
				}
			}
		}
	}

	// List resources.
	log.Infof("===== List resources =====")
	resourcesResult, err := mcpClient.ListResources(ctx)
	if err != nil {
		log.Errorf("Failed to list resources: %v", err)
	} else {
		log.Infof("Found %d resources:", len(resourcesResult.Resources))
		for _, resource := range resourcesResult.Resources {
			log.Infof("- %s: %s (%s)", resource.Name, resource.Description, resource.URI)
		}

		// Read the first resource (if any).
		if len(resourcesResult.Resources) > 0 {
			log.Infof("===== Read resource: %s =====", resourcesResult.Resources[0].Name)
			resourceContent, err := mcpClient.ReadResource(ctx, resourcesResult.Resources[0].URI)
			if err != nil {
				log.Errorf("Failed to read resource: %v", err)
			} else {
				log.Infof("Successfully read resource, content item count: %d", len(resourceContent.Contents))
				for i, content := range resourceContent.Contents {
					switch c := content.(type) {
					case mcp.TextResourceContents:
						log.Infof("[%d] Text resource: %s (first 50 chars: %s...)",
							i, c.URI, truncateString(c.Text, 50))
					case mcp.BlobResourceContents:
						log.Infof("[%d] Binary resource: %s (size: %d bytes)",
							i, c.URI, len(c.Blob))
					default:
						log.Infof("[%d] Unknown resource type", i)
					}
				}
			}
		}
	}

	// Terminate session.
	if sessionID != "" {
		log.Infof("===== Terminate session =====")
		if err := mcpClient.TerminateSession(ctx); err != nil {
			log.Errorf("Failed to terminate session: %v", err)
		} else {
			log.Infof("Session terminated.")
		}
	}

	log.Infof("Example finished.")
}

// truncateString truncates a string and adds ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
