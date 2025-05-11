package main

import (
	"context"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/client"
	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

func main() {
	// Set log level.
	log.SetLevel(log.InfoLevel)
	log.Info("Starting Stateless JSON No GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := mcp.Implementation{
		Name:    "Stateless-JSON-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	mcpClient, err := client.NewClient("http://localhost:3001/mcp", clientInfo)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// Initialize client.
	log.Info("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx)
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	log.Infof("Initialization successful: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Infof("Server capabilities: %+v", initResp.Capabilities)

	// Get an available tools list.
	log.Info("Listing tools...")
	listToolsResp, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
	}

	log.Infof("Server provides %d tools", len(listToolsResp.Tools))
	for _, tool := range listToolsResp.Tools {
		log.Infof("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call greet tool.
	log.Info("Calling greet tool...")
	callResult, err := mcpClient.CallTool(ctx, "greet", map[string]interface{}{
		"name": "Client user",
	})
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
	}

	// Show call result.
	log.Info("Call result:")
	for _, content := range callResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		} else {
			log.Infof("- Other type content: %+v", content)
		}
	}

	// Wait a while for user to see output.
	log.Info("Client example finished, exiting in 3 seconds...")
	time.Sleep(3 * time.Second)
}
