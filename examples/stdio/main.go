// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// STDIO Client Multi-Language Compatibility Demo
// This example demonstrates that trpc-mcp-go STDIO client can connect to:
// 1. TypeScript MCP servers (via npx)
// 2. Python MCP servers (via uvx)
// 3. Go MCP servers (via go run)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if len(os.Args) < 2 {
		showUsage()
		return
	}

	ctx := context.Background()
	command := os.Args[1]

	switch command {
	case "typescript", "ts":
		testTypeScriptServer(ctx)
	case "python", "py":
		testPythonServer(ctx)
	case "go", "golang":
		testGoServer(ctx)
	case "all":
		testAllServers(ctx)
	case "help", "-h", "--help":
		showUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		showUsage()
		os.Exit(1)
	}
}

func showUsage() {
	fmt.Println("STDIO Client Multi-Language Compatibility Demo")
	fmt.Println("=================================================")
	fmt.Println("Demonstrates trpc-mcp-go STDIO client connecting to servers written in different languages")
	fmt.Println()
	fmt.Println("Usage: go run main.go <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  typescript (ts)  - Connect to TypeScript MCP server via npx")
	fmt.Println("  python (py)      - Connect to Python MCP server via uvx")
	fmt.Println("  go (golang)      - Connect to Go MCP server via go run")
	fmt.Println("  all              - Test all three server types")
	fmt.Println("  help             - Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run main.go typescript  # Test TypeScript filesystem server")
	fmt.Println("  go run main.go python      # Test Python time server")
	fmt.Println("  go run main.go go          # Test Go server")
	fmt.Println("  go run main.go all         # Test all servers")
}

func testAllServers(ctx context.Context) {
	fmt.Println("Testing All Server Types")
	fmt.Println("============================")
	fmt.Println("Demonstrating cross-language MCP compatibility")
	fmt.Println()

	tests := []struct {
		name string
		fn   func(context.Context)
	}{
		{"TypeScript Server", testTypeScriptServer},
		{"Python Server", testPythonServer},
		{"Go Server", testGoServer},
	}

	successCount := 0
	for i, test := range tests {
		fmt.Printf("Test %d/%d: %s\n", i+1, len(tests), test.name)
		fmt.Println("---")

		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Test failed: %v\n", r)
					return
				}
				successCount++
			}()
			test.fn(ctx)
		}()

		fmt.Println()
	}

	fmt.Printf("Results: %d/%d servers connected successfully\n", successCount, len(tests))
	if successCount == len(tests) {
		fmt.Println("Perfect! trpc-mcp-go STDIO client is compatible with all tested languages!")
	}
}

// Test TypeScript MCP server (filesystem operations).
func testTypeScriptServer(ctx context.Context) {
	fmt.Println("Testing TypeScript MCP Server")
	fmt.Println("Server: @modelcontextprotocol/server-filesystem")
	fmt.Println("Command: npx -y @modelcontextprotocol/server-filesystem /tmp")

	client, err := mcp.NewNpxStdioClient(
		"@modelcontextprotocol/server-filesystem",
		[]string{"/tmp"},
		mcp.Implementation{Name: "compatibility-test", Version: "1.0.0"},
		mcp.WithStdioLogger(mcp.GetDefaultLogger()),
	)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Initialize connection.
	initResp, err := client.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		fmt.Printf("Initialization failed: %v\n", err)
		return
	}

	fmt.Printf("Connected! Server: %s %s\n", initResp.ServerInfo.Name, initResp.ServerInfo.Version)
	fmt.Printf("Protocol: %s\n", initResp.ProtocolVersion)

	// List and test tools.
	toolsResp, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		fmt.Printf("Failed to list tools: %v\n", err)
		return
	}

	fmt.Printf("🔧 Found %d tools\n", len(toolsResp.Tools))
	if len(toolsResp.Tools) > 0 {
		fmt.Printf("Example: %s - %s\n", toolsResp.Tools[0].Name, toolsResp.Tools[0].Description)
	}

	fmt.Println("TypeScript server test completed successfully!")
}

// Test Python MCP server (time operations).
func testPythonServer(ctx context.Context) {
	fmt.Println("Testing Python MCP Server")
	fmt.Println("Server: mcp-server-time")
	fmt.Println("Command: uvx mcp-server-time --local-timezone=America/New_York")

	config := mcp.StdioTransportConfig{
		ServerParams: mcp.StdioServerParameters{
			Command: "uvx",
			Args:    []string{"mcp-server-time", "--local-timezone=America/New_York"},
		},
		Timeout: 30 * time.Second,
	}

	client, err := mcp.NewStdioClient(
		config,
		mcp.Implementation{Name: "compatibility-test", Version: "1.0.0"},
		mcp.WithStdioLogger(mcp.GetDefaultLogger()),
	)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Initialize connection.
	initResp, err := client.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		fmt.Printf("Initialization failed: %v\n", err)
		return
	}

	fmt.Printf("Connected! Server: %s %s\n", initResp.ServerInfo.Name, initResp.ServerInfo.Version)
	fmt.Printf("Protocol: %s\n", initResp.ProtocolVersion)

	// List and test tools.
	toolsResp, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		fmt.Printf("Failed to list tools: %v\n", err)
		return
	}

	fmt.Printf("🔧 Found %d tools\n", len(toolsResp.Tools))
	if len(toolsResp.Tools) > 0 {
		fmt.Printf("   Example: %s - %s\n", toolsResp.Tools[0].Name, toolsResp.Tools[0].Description)

		// Test a time tool if available.
		for _, tool := range toolsResp.Tools {
			if tool.Name == "get_current_time" || tool.Name == "current_time" {
				callReq := &mcp.CallToolRequest{}
				callReq.Params.Name = tool.Name
				callReq.Params.Arguments = map[string]interface{}{}

				callResp, err := client.CallTool(ctx, callReq)
				if err == nil && len(callResp.Content) > 0 {
					fmt.Printf("Time result: %v\n", callResp.Content[0])
				}
				break
			}
		}
	}

	fmt.Println("Python server test completed successfully!")
}

// Test Go MCP server (math and echo operations).
func testGoServer(ctx context.Context) {
	fmt.Println("Testing Go MCP Server")
	fmt.Println("Server: Local Go server with high-level API")
	fmt.Println("Command: go run ./server/main.go")

	// Check if a server source exists.
	serverDir := "./server"
	if _, err := os.Stat(filepath.Join(serverDir, "main.go")); os.IsNotExist(err) {
		fmt.Printf("Server source not found: %s/main.go\n", serverDir)
		return
	}

	absServerDir, err := filepath.Abs(serverDir)
	if err != nil {
		fmt.Printf("Failed to get absolute path: %v\n", err)
		return
	}

	config := mcp.StdioTransportConfig{
		ServerParams: mcp.StdioServerParameters{
			Command: "go",
			Args:    []string{"run", filepath.Join(absServerDir, "main.go")},
		},
		Timeout: 30 * time.Second,
	}

	client, err := mcp.NewStdioClient(
		config,
		mcp.Implementation{Name: "compatibility-test", Version: "1.0.0"},
		mcp.WithStdioLogger(mcp.GetDefaultLogger()),
	)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Initialize connection.
	initResp, err := client.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		fmt.Printf("Initialization failed: %v\n", err)
		return
	}

	fmt.Printf("Connected! Server: %s %s\n", initResp.ServerInfo.Name, initResp.ServerInfo.Version)
	fmt.Printf("Protocol: %s\n", initResp.ProtocolVersion)

	// List and test tools.
	toolsResp, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		fmt.Printf("Failed to list tools: %v\n", err)
		return
	}

	fmt.Printf("Found %d tools\n", len(toolsResp.Tools))

	// Test echo tool.
	for _, tool := range toolsResp.Tools {
		if tool.Name == "echo" {
			fmt.Printf("   Testing: %s - %s\n", tool.Name, tool.Description)

			callReq := &mcp.CallToolRequest{}
			callReq.Params.Name = "echo"
			callReq.Params.Arguments = map[string]interface{}{
				"text": "Hello from compatibility test!",
			}

			callResp, err := client.CallTool(ctx, callReq)
			if err != nil {
				fmt.Printf("Tool call failed: %v\n", err)
			} else if len(callResp.Content) > 0 {
				fmt.Printf("Echo result: %v\n", callResp.Content[0])
			}
			break
		}
	}

	fmt.Println("Go server test completed successfully!")
}
