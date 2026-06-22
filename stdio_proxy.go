// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"time"
)

const defaultStdioProxyDiscoveryTimeout = 30 * time.Second

// StreamableStdioProxyConfig configures a Streamable HTTP server backed by an
// external stdio MCP server process.
type StreamableStdioProxyConfig struct {
	// ServerName is the public Streamable HTTP MCP server name.
	ServerName string
	// ServerVersion is the public Streamable HTTP MCP server version.
	ServerVersion string
	// Stdio configures how the external stdio MCP server process is launched.
	Stdio StdioTransportConfig
	// ClientInfo is used when this proxy initializes the stdio MCP server.
	// If empty, ServerName and ServerVersion are used.
	ClientInfo Implementation
	// InitializeRequest optionally overrides the initialize params sent to the
	// stdio MCP server.
	InitializeRequest *InitializeRequest
	// DiscoveryTimeout bounds initialization and capability discovery. Defaults
	// to 30 seconds when unset.
	DiscoveryTimeout time.Duration
	// ServerOptions are passed to NewServer for the public Streamable HTTP server.
	ServerOptions []ServerOption
	// ClientOptions are passed to NewStdioClient for the stdio side.
	ClientOptions []StdioClientOption
}

// StreamableStdioProxy owns the stdio client process used by a Streamable HTTP
// proxy server.
type StreamableStdioProxy struct {
	client           *StdioClient
	initializeResult *InitializeResult
}

// Close closes the underlying stdio client and terminates the external process.
func (p *StreamableStdioProxy) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	return p.client.Close()
}

// Client returns the underlying stdio client.
func (p *StreamableStdioProxy) Client() *StdioClient {
	if p == nil {
		return nil
	}
	return p.client
}

// InitializeResult returns the stdio server initialize result discovered during
// proxy startup.
func (p *StreamableStdioProxy) InitializeResult() *InitializeResult {
	if p == nil {
		return nil
	}
	return p.initializeResult
}

// NewStreamableServerWithStdio creates a Streamable HTTP MCP server that
// exposes an external stdio MCP server by discovering its tools, resources, and
// prompts and registering local forwarding handlers.
func NewStreamableServerWithStdio(
	ctx context.Context,
	config StreamableStdioProxyConfig,
) (*Server, *StreamableStdioProxy, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if config.ServerName == "" {
		return nil, nil, fmt.Errorf("server name cannot be empty")
	}
	if config.ServerVersion == "" {
		return nil, nil, fmt.Errorf("server version cannot be empty")
	}

	clientInfo := config.ClientInfo
	if clientInfo.Name == "" {
		clientInfo.Name = config.ServerName
	}
	if clientInfo.Version == "" {
		clientInfo.Version = config.ServerVersion
	}
	if config.Stdio.Timeout <= 0 {
		config.Stdio.Timeout = defaultStdioProxyDiscoveryTimeout
	}

	stdioClient, err := NewStdioClient(config.Stdio, clientInfo, config.ClientOptions...)
	if err != nil {
		return nil, nil, fmt.Errorf("create stdio client: %w", err)
	}

	timeout := config.DiscoveryTimeout
	if timeout <= 0 {
		timeout = defaultStdioProxyDiscoveryTimeout
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	initReq := config.InitializeRequest
	if initReq == nil {
		initReq = &InitializeRequest{}
	}
	initResult, err := stdioClient.Initialize(discoveryCtx, initReq)
	if err != nil {
		_ = stdioClient.Close()
		return nil, nil, fmt.Errorf("initialize stdio server: %w", err)
	}

	server := NewServer(config.ServerName, config.ServerVersion, config.ServerOptions...)
	proxy := &StreamableStdioProxy{
		client:           stdioClient,
		initializeResult: initResult,
	}

	if err := registerStdioProxyCapabilities(discoveryCtx, server, proxy, initResult.Capabilities); err != nil {
		_ = proxy.Close()
		return nil, nil, err
	}

	return server, proxy, nil
}

// NewStreamableServerWithStdioParams is a convenience wrapper around
// NewStreamableServerWithStdio for common use cases.
func NewStreamableServerWithStdioParams(
	ctx context.Context,
	serverName string,
	serverVersion string,
	stdio StdioTransportConfig,
	options ...ServerOption,
) (*Server, *StreamableStdioProxy, error) {
	return NewStreamableServerWithStdio(ctx, StreamableStdioProxyConfig{
		ServerName:    serverName,
		ServerVersion: serverVersion,
		Stdio:         stdio,
		ServerOptions: options,
	})
}

func registerStdioProxyCapabilities(
	ctx context.Context,
	server *Server,
	proxy *StreamableStdioProxy,
	capabilities ServerCapabilities,
) error {
	if capabilities.Tools != nil {
		if err := registerStdioProxyTools(ctx, server, proxy); err != nil {
			return fmt.Errorf("register stdio proxy tools: %w", err)
		}
	}
	if capabilities.Resources != nil {
		if err := registerStdioProxyResources(ctx, server, proxy); err != nil {
			return fmt.Errorf("register stdio proxy resources: %w", err)
		}
	}
	if capabilities.Prompts != nil {
		if err := registerStdioProxyPrompts(ctx, server, proxy); err != nil {
			return fmt.Errorf("register stdio proxy prompts: %w", err)
		}
	}
	return nil
}

func registerStdioProxyTools(ctx context.Context, server *Server, proxy *StreamableStdioProxy) error {
	result, err := listAllStdioProxyTools(ctx, proxy.client)
	if err != nil {
		return err
	}
	for _, tool := range result.Tools {
		toolCopy := tool
		server.RegisterTool(&toolCopy, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return proxy.client.CallTool(ctx, req)
		})
	}
	return nil
}

func registerStdioProxyResources(ctx context.Context, server *Server, proxy *StreamableStdioProxy) error {
	result, err := listAllStdioProxyResources(ctx, proxy.client)
	if err != nil {
		return err
	}
	for _, resource := range result.Resources {
		resourceCopy := resource
		server.RegisterResources(&resourceCopy, func(ctx context.Context, req *ReadResourceRequest) ([]ResourceContents, error) {
			readResult, err := proxy.client.ReadResource(ctx, req)
			if err != nil {
				return nil, err
			}
			return readResult.Contents, nil
		})
	}
	return nil
}

func registerStdioProxyPrompts(ctx context.Context, server *Server, proxy *StreamableStdioProxy) error {
	result, err := listAllStdioProxyPrompts(ctx, proxy.client)
	if err != nil {
		return err
	}
	for _, prompt := range result.Prompts {
		promptCopy := prompt
		server.RegisterPrompt(&promptCopy, func(ctx context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
			return proxy.client.GetPrompt(ctx, req)
		})
	}
	return nil
}

func listAllStdioProxyTools(ctx context.Context, client *StdioClient) (*ListToolsResult, error) {
	result := &ListToolsResult{}
	req := &ListToolsRequest{}
	for {
		page, err := client.ListTools(ctx, req)
		if err != nil {
			return nil, err
		}
		result.Tools = append(result.Tools, page.Tools...)
		if page.NextCursor == "" {
			return result, nil
		}
		req.Params.Cursor = page.NextCursor
	}
}

func listAllStdioProxyResources(ctx context.Context, client *StdioClient) (*ListResourcesResult, error) {
	result := &ListResourcesResult{}
	req := &ListResourcesRequest{}
	for {
		page, err := client.ListResources(ctx, req)
		if err != nil {
			return nil, err
		}
		result.Resources = append(result.Resources, page.Resources...)
		if page.NextCursor == "" {
			return result, nil
		}
		req.Params.Cursor = page.NextCursor
	}
}

func listAllStdioProxyPrompts(ctx context.Context, client *StdioClient) (*ListPromptsResult, error) {
	result := &ListPromptsResult{}
	req := &ListPromptsRequest{}
	for {
		page, err := client.ListPrompts(ctx, req)
		if err != nil {
			return nil, err
		}
		result.Prompts = append(result.Prompts, page.Prompts...)
		if page.NextCursor == "" {
			return result, nil
		}
		req.Params.Cursor = page.NextCursor
	}
}
