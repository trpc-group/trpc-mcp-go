// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func TestStreamableServerWithStdio_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	proxyServer, proxy, err := mcp.NewStreamableServerWithStdio(ctx, mcp.StreamableStdioProxyConfig{
		ServerName:    "stdio-proxy",
		ServerVersion: "1.0.0",
		Stdio: mcp.StdioTransportConfig{
			ServerParams: mcp.StdioServerParameters{
				Command: "go",
				Args:    []string{"run", "./test_server/main.go"},
			},
			Timeout: 10 * time.Second,
		},
		DiscoveryTimeout: 20 * time.Second,
	})
	require.NoError(t, err)
	defer proxy.Close()

	require.NotNil(t, proxy.InitializeResult())
	require.NotNil(t, proxy.InitializeResult().Capabilities.Tools)

	httpServer := httptest.NewServer(proxyServer.HTTPHandler())
	defer httpServer.Close()

	client, err := mcp.NewClient(httpServer.URL+"/mcp", mcp.Implementation{
		Name:    "streamable-proxy-test-client",
		Version: "1.0.0",
	})
	require.NoError(t, err)
	defer client.Close()

	initResult, err := client.Initialize(ctx, &mcp.InitializeRequest{})
	require.NoError(t, err)
	assert.Equal(t, "stdio-proxy", initResult.ServerInfo.Name)
	require.NotNil(t, initResult.Capabilities.Tools)

	tools, err := client.ListTools(ctx, &mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, tools.Tools)
	assert.Contains(t, toolNames(tools.Tools), "echo")

	result, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "echo",
			Arguments: map[string]interface{}{
				"text": "hello through streamable proxy",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "hello through streamable proxy")
}

func toolNames(tools []mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}
