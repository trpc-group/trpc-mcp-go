// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exampleDir, err := currentDir()
	if err != nil {
		log.Fatalf("get example directory: %v", err)
	}

	proxyServer, proxy, err := mcp.NewStreamableServerWithStdio(ctx, mcp.StreamableStdioProxyConfig{
		ServerName:    "stdio-to-streamable-proxy",
		ServerVersion: "1.0.0",
		Stdio: mcp.StdioTransportConfig{
			ServerParams: mcp.StdioServerParameters{
				Command:    "go",
				Args:       []string{"run", "./stdio_server"},
				WorkingDir: exampleDir,
			},
			Timeout: 10 * time.Second,
		},
		ServerOptions: []mcp.ServerOption{
			mcp.WithServerPath("/mcp"),
			mcp.WithPostSSEEnabled(false),
			mcp.WithGetSSEEnabled(false),
		},
		DiscoveryTimeout: 20 * time.Second,
	})
	if err != nil {
		log.Fatalf("create stdio-to-streamable proxy: %v", err)
	}
	defer proxy.Close()

	httpServer := httptest.NewServer(proxyServer.HTTPHandler())
	defer httpServer.Close()

	client, err := mcp.NewClient(httpServer.URL+"/mcp", mcp.Implementation{
		Name:    "stdio-to-streamable-example-client",
		Version: "1.0.0",
	}, mcp.WithClientGetSSEEnabled(false))
	if err != nil {
		log.Fatalf("create streamable client: %v", err)
	}
	defer client.Close()

	initResult, err := client.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("initialize streamable client: %v", err)
	}
	fmt.Printf("Connected to proxy server: %s %s\n", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	tools, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("list tools through proxy: %v", err)
	}
	fmt.Printf("Discovered %d tools through stdio proxy:\n", len(tools.Tools))
	for _, tool := range tools.Tools {
		fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
	}

	echo, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "echo",
			Arguments: map[string]interface{}{
				"text": "hello from streamable client",
			},
		},
	})
	if err != nil {
		log.Fatalf("call echo through proxy: %v", err)
	}
	fmt.Printf("echo result: %s\n", firstText(echo))

	add, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "add",
			Arguments: map[string]interface{}{
				"a": 20,
				"b": 22,
			},
		},
	})
	if err != nil {
		log.Fatalf("call add through proxy: %v", err)
	}
	fmt.Printf("add result: %s\n", firstText(add))
}

func currentDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime caller unavailable")
	}
	return filepath.Dir(file), nil
}

func firstText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if text, ok := result.Content[0].(mcp.TextContent); ok {
		return text.Text
	}
	return fmt.Sprintf("%v", result.Content[0])
}
