package main

import (
	"context"
	"time"

	"log"

	"trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// Print startup message.
	log.Printf("Starting Stateless JSON No GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := mcp.Implementation{
		Name:    "Stateless-JSON-Client",
		Version: "1.0.0",
	}

	// Create client, connect to server.
	mcpClient, err := mcp.NewClient("http://localhost:3001/mcp", clientInfo)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// Initialize client.
	log.Printf("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx)
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	log.Printf("Initialization successful: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Printf("Server capabilities: %+v", initResp.Capabilities)

	// Get an available tools list.
	log.Printf("Listing tools...")
	listToolsResp, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
	}

	log.Printf("Server provides %d tools", len(listToolsResp.Tools))
	for _, tool := range listToolsResp.Tools {
		log.Printf("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call greet tool.
	log.Printf("Calling greet tool...")
	callResult, err := mcpClient.CallTool(ctx, "greet", map[string]interface{}{
		"name": "Client user",
	})
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
	}

	// Show call result.
	log.Printf("Call result:")
	for _, content := range callResult.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Printf("- Text: %s", textContent.Text)
		} else {
			log.Printf("- Other type content: %+v", content)
		}
	}

	// Wait a while for user to see output.
	log.Printf("Client example finished, exiting in 3 seconds...")
	time.Sleep(3 * time.Second)
}
