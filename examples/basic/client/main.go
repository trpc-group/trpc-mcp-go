package main

import (
	"context"
	"os"

	"github.com/modelcontextprotocol/streamable-mcp/client"
	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

func main() {
	// Initialize log.
	log.Info("Starting example client...")

	// Create context.
	ctx := context.Background()

	// Create client.
	mcpClient, err := client.NewClient("http://localhost:3000/mcp", schema.Implementation{
		Name:    "MCP-Go-Client",
		Version: "1.0.0",
	}, client.WithProtocolVersion(schema.ProtocolVersion_2024_11_05)) // Specify protocol version explicitly.
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
	tools, err := mcpClient.ListTools(ctx)
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
			content, err := mcpClient.CallTool(ctx, tools[0].Name, map[string]interface{}{
				"name": "MCP User",
			})
			if err != nil {
				log.Errorf("Failed to call tool: %v", err)
			} else {
				log.Infof("Tool result:")
				for _, item := range content {
					// Type assertion for different content types.
					if textContent, ok := item.(schema.TextContent); ok {
						log.Infof("  %s", textContent.Text)
					} else {
						log.Infof("  [%s type content]", item.GetType())
					}
				}
			}
		}
	}

	// List prompts.
	log.Info("===== List prompts =====")
	prompts, err := mcpClient.ListPrompts(ctx)
	if err != nil {
		log.Errorf("Failed to list prompts: %v", err)
	} else {
		log.Infof("Found %d prompts:", len(prompts))
		for _, prompt := range prompts {
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
		if len(prompts) > 0 {
			// Create a parameter map.
			arguments := make(map[string]string)
			for _, arg := range prompts[0].Arguments {
				if arg.Required {
					// Provide an example value for required parameters.
					arguments[arg.Name] = "example value"
				}
			}

			log.Infof("===== Get prompt: %s =====", prompts[0].Name)
			promptContent, err := mcpClient.GetPrompt(ctx, prompts[0].Name, arguments)
			if err != nil {
				log.Errorf("Failed to get prompt: %v", err)
			} else {
				log.Infof("Successfully got prompt, message count: %d", len(promptContent.Messages))
				if promptContent.Description != "" {
					log.Infof("Prompt description: %s", promptContent.Description)
				}

				for i, msg := range promptContent.Messages {
					switch content := msg.Content.(type) {
					case map[string]interface{}:
						// Handle complex content (e.g., images, audio, etc.).
						if typeStr, ok := content["type"].(string); ok {
							log.Infof("[%d] %s message (type: %s)", i, msg.Role, typeStr)
						} else {
							log.Infof("[%d] %s message (complex content)", i, msg.Role)
						}
					case string:
						// Handle text content.
						log.Infof("[%d] %s message: %s", i, msg.Role, truncateString(content, 50))
					default:
						log.Infof("[%d] %s message (unknown content type)", i, msg.Role)
					}
				}
			}
		}
	}

	// List resources.
	log.Infof("===== List resources =====")
	resources, err := mcpClient.ListResources(ctx)
	if err != nil {
		log.Errorf("Failed to list resources: %v", err)
	} else {
		log.Infof("Found %d resources:", len(resources))
		for _, resource := range resources {
			log.Infof("- %s: %s (%s)", resource.Name, resource.Description, resource.URI)
		}

		// Read the first resource (if any).
		if len(resources) > 0 {
			log.Infof("===== Read resource: %s =====", resources[0].Name)
			resourceContent, err := mcpClient.ReadResource(ctx, resources[0].URI)
			if err != nil {
				log.Errorf("Failed to read resource: %v", err)
			} else {
				log.Infof("Successfully read resource, content item count: %d", len(resourceContent.Contents))
				for i, content := range resourceContent.Contents {
					switch c := content.(type) {
					case schema.TextResourceContents:
						log.Infof("[%d] Text resource: %s (first 50 chars: %s...)",
							i, c.URI, truncateString(c.Text, 50))
					case schema.BlobResourceContents:
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
