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
	log.Info("Starting Stateful JSON No GET SSE mode client...")

	// Create context.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client info.
	clientInfo := mcp.Implementation{
		Name:    "Stateful-JSON-Client",
		Version: "1.0.0",
	}

	// Create client, use JSON instead of SSE mode.
	mcpClient, err := client.NewClient(
		"http://localhost:3003/mcp",
		clientInfo,
	)
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

	log.Infof("Initialization succeeded: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Infof("Server capabilities: %+v", initResp.Capabilities)

	// Get available tools list.
	log.Info("Listing tools...")
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to get tools list: %v", err)
	}

	log.Infof("Server provides %d tools", len(tools.Tools))
	for _, tool := range tools.Tools {
		log.Infof("- Tool: %s (%s)", tool.Name, tool.Description)
	}

	// Call greet tool.
	log.Info("Calling greet tool...")
	callResult, err := mcpClient.CallTool(ctx, "greet", map[string]interface{}{
		"name": "Stateful client user",
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

	// Call counter tool, demonstrate session state keeping.
	log.Info("\nCalling counter tool first time...")
	counterResult1, err := mcpClient.CallTool(ctx, "counter", map[string]interface{}{
		"increment": 1,
	})
	if err != nil {
		log.Fatalf("Failed to call counter tool: %v", err)
	}

	// Show counter result.
	log.Info("Counter result (first time):")
	for _, content := range counterResult1.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Call counter tool again, verify state keeping.
	log.Info("\nCalling counter tool second time...")
	counterResult2, err := mcpClient.CallTool(ctx, "counter", map[string]interface{}{
		"increment": 2,
	})
	if err != nil {
		log.Fatalf("Failed to call counter tool: %v", err)
	}

	// Show counter result.
	log.Info("Counter result (second time):")
	for _, content := range counterResult2.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Call counter tool third time, continue verifying state keeping.
	log.Info("\nCalling counter tool third time...")
	counterResult3, err := mcpClient.CallTool(ctx, "counter", map[string]interface{}{
		"increment": 3,
	})
	if err != nil {
		log.Fatalf("Failed to call counter tool: %v", err)
	}

	// Show counter result.
	log.Info("Counter result (third time):")
	for _, content := range counterResult3.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			log.Infof("- Text: %s", textContent.Text)
		}
	}

	// Print session info.
	log.Infof("\nSession info: ID=%s", mcpClient.GetSessionID())
	log.Info("Client keeps session alive, you can check session state in another terminal...")
	log.Info("Tip: you can use curl http://localhost:3003/sessions to check server session state")

	// Wait a while for user to see output.
	log.Info("Client example finished, exiting in 3 seconds...")
	time.Sleep(3 * time.Second)
}
