package main

import (
	"context"
	"os"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/client"
	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

func main() {
	// Initialize log.
	log.Info("Starting example client...")

	// Wait for the server to start.
	log.Info("Waiting for server to start...")
	time.Sleep(2 * time.Second)

	// Basic settings.
	serverURL := "http://localhost:3000/mcp"
	clientInfo := mcp.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}

	// Create MCP client.
	log.Info("===== Create client =====")
	newClient, err := client.NewClient(serverURL, clientInfo)
	if err != nil {
		log.Errorf("Error creating client: %v", err)
		os.Exit(1)
	}

	// Initialize client.
	log.Info("===== Initialize client =====")
	ctx := context.Background()
	initResult, err := newClient.Initialize(ctx)
	if err != nil {
		log.Errorf("Initialization error: %v", err)
		os.Exit(1)
	}
	log.Infof("Connected to server: %s %s", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	// List resources.
	log.Info("===== List resources =====")
	resources, err := newClient.ListResources(ctx)
	if err != nil {
		log.Errorf("List resources error: %v", err)
	} else {
		log.Infof("Found %d resources:", len(resources.Resources))
		for _, resource := range resources.Resources {
			log.Infof("- %s: %s (%s)", resource.Name, resource.Description, resource.URI)
		}

		// Read the first resource (if any).
		if len(resources.Resources) > 0 {
			log.Infof("===== Read resource: %s =====", resources.Resources[0].Name)
			resourceContent, err := newClient.ReadResource(ctx, resources.Resources[0].URI)
			if err != nil {
				log.Errorf("Read resource error: %v", err)
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

	// List prompts.
	log.Info("===== List prompts =====")
	prompts, err := newClient.ListPrompts(ctx)
	if err != nil {
		log.Errorf("List prompts error: %v", err)
	} else {
		log.Infof("Found %d prompts:", len(prompts.Prompts))
		for _, prompt := range prompts.Prompts {
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
		if len(prompts.Prompts) > 0 {
			// Create an empty parameter map (should provide required params in real use).
			arguments := make(map[string]string)
			for _, arg := range prompts.Prompts[0].Arguments {
				if arg.Required {
					// Provide an example value for required parameters.
					arguments[arg.Name] = "example value"
				}
			}

			log.Infof("===== Get prompt: %s =====", prompts.Prompts[0].Name)
			promptContent, err := newClient.GetPrompt(ctx, prompts.Prompts[0].Name, arguments)
			if err != nil {
				log.Errorf("Get prompt error: %v", err)
			} else {
				log.Infof("Successfully got prompt, message count: %d", len(promptContent.Messages))
				if promptContent.Description != "" {
					log.Infof("Prompt description: %s", promptContent.Description)
				}

				for i, msg := range promptContent.Messages {
					switch c := msg.Content.(type) {
					case mcp.TextContent:
						log.Infof("[%d] %s message (Text): %s", i, msg.Role, truncateString(c.Text, 50))
					case mcp.ImageContent:
						log.Infof("[%d] %s message (Image): MIME=%s, DataLen=%d", i, msg.Role, c.MimeType, len(c.Data))
					case mcp.AudioContent:
						log.Infof("[%d] %s message (Audio): MIME=%s, DataLen=%d", i, msg.Role, c.MimeType, len(c.Data))
					case mcp.EmbeddedResource:
						var resourceURI string
						if textResource, ok := c.Resource.(mcp.TextResourceContents); ok {
							resourceURI = textResource.URI
						} else if blobResource, ok := c.Resource.(mcp.BlobResourceContents); ok {
							resourceURI = blobResource.URI
						}
						log.Infof("[%d] %s message (Resource): URI=%s", i, msg.Role, resourceURI)
					default:
						log.Infof("[%d] %s message (unknown content type: %T)", i, msg.Role, c)
					}
				}
			}
		}
	}

	// List tools.
	log.Info("===== List tools =====")
	tools, err := newClient.ListTools(ctx)
	if err != nil {
		log.Errorf("List tools error: %v", err)
	} else {
		log.Infof("Found %d tools:", len(tools.Tools))
		for _, tool := range tools.Tools {
			log.Infof("- %s: %s", tool.Name, tool.Description)
		}
	}

	// Close client (cleanup resources).
	if err := newClient.Close(); err != nil {
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
