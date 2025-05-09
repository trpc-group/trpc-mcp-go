package main

import (
	"context"
	"os"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/client"
	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

func main() {
	// Initialize log.
	log.Info("Starting example client...")

	// Wait for server to start.
	log.Info("Waiting for server to start...")
	time.Sleep(2 * time.Second)

	// Basic settings.
	serverURL := "http://localhost:3000/mcp"
	clientInfo := schema.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}

	// Create MCP client.
	log.Info("===== Create client =====")
	mcp, err := client.NewClient(serverURL, clientInfo)
	if err != nil {
		log.Errorf("Error creating client: %v", err)
		os.Exit(1)
	}

	// Initialize client.
	log.Info("===== Initialize client =====")
	ctx := context.Background()
	initResult, err := mcp.Initialize(ctx)
	if err != nil {
		log.Errorf("Initialization error: %v", err)
		os.Exit(1)
	}
	log.Infof("Connected to server: %s %s", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	// List resources.
	log.Info("===== List resources =====")
	resources, err := mcp.ListResources(ctx)
	if err != nil {
		log.Errorf("List resources error: %v", err)
	} else {
		log.Infof("Found %d resources:", len(resources))
		for _, resource := range resources {
			log.Infof("- %s: %s (%s)", resource.Name, resource.Description, resource.URI)
		}

		// Read the first resource (if any).
		if len(resources) > 0 {
			log.Infof("===== Read resource: %s =====", resources[0].Name)
			resourceContent, err := mcp.ReadResource(ctx, resources[0].URI)
			if err != nil {
				log.Errorf("Read resource error: %v", err)
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

	// List prompts.
	log.Info("===== List prompts =====")
	prompts, err := mcp.ListPrompts(ctx)
	if err != nil {
		log.Errorf("List prompts error: %v", err)
	} else {
		log.Infof("Found %d prompts:", len(prompts))
		for _, prompt := range prompts {
			log.Infof("- %s: %s", prompt.Name, prompt.Description)
			if len(prompt.Arguments) > 0 {
				log.Info("  Arguments:")
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
			// Create an empty parameter map (should provide required params in real use).
			arguments := make(map[string]string)
			for _, arg := range prompts[0].Arguments {
				if arg.Required {
					// Provide an example value for required parameters.
					arguments[arg.Name] = "example value"
				}
			}

			log.Infof("===== Get prompt: %s =====", prompts[0].Name)
			promptContent, err := mcp.GetPrompt(ctx, prompts[0].Name, arguments)
			if err != nil {
				log.Errorf("Get prompt error: %v", err)
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

	// List tools.
	log.Info("===== List tools =====")
	tools, err := mcp.ListTools(ctx)
	if err != nil {
		log.Errorf("List tools error: %v", err)
	} else {
		log.Infof("Found %d tools:", len(tools))
		for _, tool := range tools {
			log.Infof("- %s: %s", tool.Name, tool.Description)
		}
	}

	// Close client (cleanup resources).
	if err := mcp.Close(); err != nil {
		log.Errorf("Close client error: %v", err)
	}

	log.Info("Test finished!")
}

// truncateString truncates a string and adds ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
