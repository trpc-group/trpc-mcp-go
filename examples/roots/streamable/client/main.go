// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

const (
	clientName    = "HTTP-Roots-Example-Client"
	clientVersion = "1.0.0"
	serverURL     = "http://localhost:3001/mcp"
)

func main() {
	log.Printf("Starting MCP HTTP Roots Example Client...")

	ctx := context.Background()

	// Initialize client
	mcpClient, err := initializeClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	defer mcpClient.Close()

	// Setup roots provider
	log.Printf("Setting up roots provider...")
	rootsProvider := setupRootsProvider(mcpClient)

	// Start interactive mode
	startInteractiveMode(ctx, mcpClient, rootsProvider)
}

// initializeClient initializes the MCP client with server connection and session setup
func initializeClient(ctx context.Context) (*mcp.Client, error) {
	log.Println("Creating HTTP MCP client...")
	mcpClient, err := mcp.NewClient(
		serverURL,
		mcp.Implementation{
			Name:    clientName,
			Version: clientVersion,
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	log.Printf("Initializing connection...")
	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.ProtocolVersion_2025_03_26,
			ClientInfo: mcp.Implementation{
				Name:    clientName,
				Version: clientVersion,
			},
			Capabilities: mcp.ClientCapabilities{
				Roots: &mcp.RootsCapability{
					ListChanged: true, // Support for roots change notifications
				},
			},
		},
	})
	if err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("initialization failed: %v", err)
	}

	log.Printf("Connected! Server: %s %s, Protocol: %s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	if initResp.Instructions != "" {
		log.Printf("Server instructions: %s", initResp.Instructions)
	}

	sessionID := mcpClient.GetSessionID()
	if sessionID != "" {
		log.Printf("Session ID: %s", sessionID)
	}

	return mcpClient, nil
}

func setupRootsProvider(client *mcp.Client) *mcp.DefaultRootsProvider {
	rootsProvider := mcp.NewDefaultRootsProvider()

	// Add initial root directories
	currentDir, _ := os.Getwd()
	rootsProvider.AddRoot(currentDir, "Working Directory")
	rootsProvider.AddRoot(filepath.Dir(currentDir), "Parent Directory")
	rootsProvider.AddRoot(os.TempDir(), "Temporary Directory")

	// Add user home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		rootsProvider.AddRoot(homeDir, "Home Directory")
	}

	// Set the provider on the client
	client.SetRootsProvider(rootsProvider)

	// Display configured roots
	roots := rootsProvider.GetRoots()
	log.Printf("Configured %d root directories:", len(roots))
	for i, root := range roots {
		log.Printf("  %d. %s (%s)", i+1, root.Name, root.URI)
	}

	return rootsProvider
}

func startInteractiveMode(ctx context.Context, client *mcp.Client, rootsProvider *mcp.DefaultRootsProvider) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println()
	fmt.Println("=== MCP HTTP Roots Interactive Demo ===")
	fmt.Println("Commands:")
	fmt.Println("  1. Add root directory")
	fmt.Println("  2. Remove root directory")
	fmt.Println("  3. List current root directories")
	fmt.Println("  4. Send roots list changed notification")
	fmt.Println("  5. Test server tools (demonstrate server-to-client requests)")
	fmt.Println("  q. Quit")
	fmt.Println()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		switch input {
		case "1":
			addRootDirectory(scanner, rootsProvider)
		case "2":
			removeRootDirectory(scanner, rootsProvider)
		case "3":
			listRootDirectories(rootsProvider)
		case "4":
			sendRootsChangedNotification(ctx, client)
		case "5":
			testServerTools(ctx, client)
		case "q":
			fmt.Println("Closing connection...")
			return
		default:
			fmt.Println("Invalid command. Please enter 1-5 or 'q'.")
		}
		fmt.Println()
	}
}

func addRootDirectory(scanner *bufio.Scanner, rootsProvider *mcp.DefaultRootsProvider) {
	fmt.Print("Enter directory path: ")
	if !scanner.Scan() {
		return
	}
	path := strings.TrimSpace(scanner.Text())
	if path == "" {
		fmt.Println("Path cannot be empty.")
		return
	}

	fmt.Print("Enter directory name: ")
	if !scanner.Scan() {
		return
	}
	name := strings.TrimSpace(scanner.Text())
	if name == "" {
		name = filepath.Base(path)
	}

	// Handle file:// prefix URI format
	localPath := path
	if strings.HasPrefix(path, "file://") {
		localPath = strings.TrimPrefix(path, "file://")
	}

	// Validate path exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		fmt.Printf("Directory does not exist: %s\n", path)
		return
	}

	// If input is a local path, convert to absolute path
	if !strings.HasPrefix(path, "file://") {
		absPath, err := filepath.Abs(localPath)
		if err != nil {
			fmt.Printf("Failed to get absolute path: %v\n", err)
			return
		}
		// Use file:// prefix format to add root directory
		rootsProvider.AddRoot("file://"+absPath, name)
		fmt.Printf("✅ Added root directory: %s (file://%s)\n", name, absPath)
	} else {
		// Use file:// format URI directly
		rootsProvider.AddRoot(path, name)
		fmt.Printf("✅ Added root directory: %s (%s)\n", name, path)
	}
}

func removeRootDirectory(scanner *bufio.Scanner, rootsProvider *mcp.DefaultRootsProvider) {
	roots := rootsProvider.GetRoots()
	if len(roots) == 0 {
		fmt.Println("No root directories to remove.")
		return
	}

	fmt.Println("Current root directories:")
	for i, root := range roots {
		fmt.Printf("  %d. %s (%s)\n", i+1, root.Name, root.URI)
	}

	fmt.Print("Enter number to remove (1-" + strconv.Itoa(len(roots)) + "): ")
	if !scanner.Scan() {
		return
	}

	input := strings.TrimSpace(scanner.Text())
	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(roots) {
		fmt.Println("Invalid selection.")
		return
	}

	rootToRemove := roots[index-1]
	// Use URI directly to remove root directory, not extract path
	rootsProvider.RemoveRoot(rootToRemove.URI)
	fmt.Printf("✅ Removed root directory: %s (%s)\n", rootToRemove.Name, rootToRemove.URI)
}

func listRootDirectories(rootsProvider *mcp.DefaultRootsProvider) {
	roots := rootsProvider.GetRoots()
	fmt.Printf("Current root directories (%d total):\n", len(roots))
	if len(roots) == 0 {
		fmt.Println("  No root directories configured.")
	} else {
		for i, root := range roots {
			fmt.Printf("  %d. %s\n     URI: %s\n", i+1, root.Name, root.URI)
		}
	}
}

func sendRootsChangedNotification(ctx context.Context, client *mcp.Client) {
	fmt.Println("Sending roots list changed notification to server...")
	err := client.SendRootsListChangedNotification(ctx)
	if err != nil {
		fmt.Printf("❌ Error sending notification: %v\n", err)
	} else {
		fmt.Println("✅ Roots list changed notification sent successfully")
		fmt.Println("   The server should now request the updated roots list")
	}
}

func testServerTools(ctx context.Context, client *mcp.Client) {
	fmt.Println("🧪 Testing server tools (demonstrates server-to-client communication)...")
	fmt.Println()

	// Test different tools to demonstrate server's ability to request client roots
	tools := []struct {
		name string
		desc string
		args map[string]interface{}
	}{
		{"list_files", "List files in client's root directories", map[string]interface{}{}},
	}

	for i, tool := range tools {
		fmt.Printf("%d. Testing tool: %s\n", i+1, tool.name)
		fmt.Printf("   Description: %s\n", tool.desc)

		result, err := client.CallTool(ctx, &mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      tool.name,
				Arguments: tool.args,
			},
		})

		if err != nil {
			fmt.Printf("   ❌ Failed: %v\n", err)
		} else {
			fmt.Printf("   ✅ Success! Result:\n")
			for j, content := range result.Content {
				if textContent, ok := content.(mcp.TextContent); ok {
					// Add indentation to each line
					lines := strings.Split(textContent.Text, "\n")
					for _, line := range lines {
						if strings.TrimSpace(line) != "" {
							fmt.Printf("      %s\n", line)
						}
					}
				} else {
					fmt.Printf("      Content %d: %T\n", j+1, content)
				}
			}
		}
		fmt.Println()
	}

	fmt.Println("🎉 Server tools test completed!")
	fmt.Println("   This demonstrates that the server can successfully request")
	fmt.Println("   root directory information from the client via HTTP transport.")
}
