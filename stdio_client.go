// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// StdioTransportConfig defines the complete configuration for a stdio MCP transport.
type StdioTransportConfig struct {
	ServerParams StdioServerParameters `json:"server_params"`
	Timeout      time.Duration         `json:"timeout"`
}

// Validate checks if the StdioTransportConfig is valid.
func (c StdioTransportConfig) Validate() error {
	if c.ServerParams.Command == "" {
		return fmt.Errorf("command cannot be empty")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

// StdioClient represents a specialized MCP client for stdio-based servers.
// It implements both Connector and ProcessClient interfaces.
type StdioClient struct {
	transport       *stdioClientTransport
	clientInfo      Implementation
	protocolVersion string
	initialized     atomic.Bool
	requestID       atomic.Int64
	capabilities    map[string]interface{}
	state           atomic.Value // stores State
	logger          Logger

	// Roots support.
	rootsProvider RootsProvider // Provider for roots information.
	rootsMu       sync.RWMutex  // Mutex for protecting the rootsProvider.
}

// StdioClientOption defines configuration options for StdioClient.
type StdioClientOption func(*StdioClient)

// NewStdioClient creates a new stdio-based MCP client
func NewStdioClient(config StdioTransportConfig, clientInfo Implementation, options ...StdioClientOption) (*StdioClient, error) {
	// Validate configuration.
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create client.
	client := &StdioClient{
		clientInfo:      clientInfo,
		protocolVersion: ProtocolVersion_2025_03_26,
		capabilities:    make(map[string]interface{}),
		logger:          GetDefaultLogger(),
	}

	// Set initial state.
	client.state.Store(StateDisconnected)

	// Apply options.
	for _, option := range options {
		option(client)
	}

	// Create transport options.
	var transportOptions []stdioTransportOption
	if config.Timeout > 0 {
		transportOptions = append(transportOptions, withStdioTransportTimeout(config.Timeout))
	}
	if client.logger != nil {
		transportOptions = append(transportOptions, withStdioTransportLogger(client.logger))
	}

	// Create transport.
	client.transport = newStdioClientTransport(config.ServerParams, transportOptions...)

	// Set client reference in transport for roots handling.
	client.transport.client = client

	return client, nil
}

// WithStdioLogger sets the logger for the client.
func WithStdioLogger(logger Logger) StdioClientOption {
	return func(c *StdioClient) {
		c.logger = logger
	}
}

// WithStdioProtocolVersion sets the protocol version.
func WithStdioProtocolVersion(version string) StdioClientOption {
	return func(c *StdioClient) {
		c.protocolVersion = version
	}
}

// WithStdioCapabilities sets client capabilities.
func WithStdioCapabilities(capabilities map[string]interface{}) StdioClientOption {
	return func(c *StdioClient) {
		for k, v := range capabilities {
			c.capabilities[k] = v
		}
	}
}

// Initialize initializes the client connection
func (c *StdioClient) Initialize(ctx context.Context, req *InitializeRequest) (*InitializeResult, error) {
	if c.initialized.Load() {
		return nil, fmt.Errorf("client already initialized")
	}

	// Create initialization request
	requestID := c.requestID.Add(1)
	jsonReq := newJSONRPCRequest(requestID, MethodInitialize, map[string]interface{}{
		"protocolVersion": c.protocolVersion,
		"clientInfo":      c.clientInfo,
		"capabilities":    c.capabilities,
	})

	// Override with provided params if any.
	if req != nil && !isZeroStruct(req.Params) {
		jsonReq.Params = req.Params
	}

	// Send request
	rawResp, err := c.transport.sendRequest(ctx, jsonReq)
	if err != nil {
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	// Update state
	c.setState(StateConnected)

	// Check for error response
	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			c.setState(StateDisconnected)
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("initialization error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response
	initResult, err := parseInitializeResultFromJSON(rawResp)
	if err != nil {
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("failed to parse initialization response: %w", err)
	}

	// Send initialized notification
	if err := c.sendInitialized(ctx); err != nil {
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	// Mark as initialized and update state.
	c.initialized.Store(true)
	c.setState(StateInitialized)

	return initResult, nil
}

// sendInitialized sends the initialized notification
func (c *StdioClient) sendInitialized(ctx context.Context) error {
	notification := NewInitializedNotification()
	return c.transport.sendNotification(ctx, notification)
}

// Close closes the client and terminates the process.
func (c *StdioClient) Close() error {
	if c.transport != nil {
		err := c.transport.close()
		c.setState(StateDisconnected)
		c.initialized.Store(false)
		return err
	}
	return nil
}

// GetState returns the current client state.
func (c *StdioClient) GetState() State {
	if state := c.state.Load(); state != nil {
		return state.(State)
	}
	return StateDisconnected
}

// setState sets the client state thread-safely.
func (c *StdioClient) setState(state State) {
	c.state.Store(state)
}

// ListTools lists available tools.
func (c *StdioClient) ListTools(ctx context.Context, req *ListToolsRequest) (*ListToolsResult, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("client not initialized")
	}

	requestID := c.requestID.Add(1)
	jsonReq := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodToolsList,
		},
		Params: req.Params,
	}

	rawResp, err := c.transport.sendRequest(ctx, jsonReq)
	if err != nil {
		return nil, fmt.Errorf("list tools request failed: %w", err)
	}

	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list tools error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	return parseListToolsResultFromJSON(rawResp)
}

// CallTool calls a specific tool.
func (c *StdioClient) CallTool(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("client not initialized")
	}

	requestID := c.requestID.Add(1)
	jsonReq := newJSONRPCRequest(requestID, MethodToolsCall, map[string]interface{}{
		"name":      req.Params.Name,
		"arguments": req.Params.Arguments,
	})

	rawResp, err := c.transport.sendRequest(ctx, jsonReq)
	if err != nil {
		return nil, fmt.Errorf("call tool request failed: %w", err)
	}

	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("call tool error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	return parseCallToolResult(rawResp)
}

// ListPrompts lists available prompts.
func (c *StdioClient) ListPrompts(ctx context.Context, req *ListPromptsRequest) (*ListPromptsResult, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("client not initialized")
	}

	requestID := c.requestID.Add(1)
	jsonReq := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodPromptsList,
		},
		Params: req.Params,
	}

	rawResp, err := c.transport.sendRequest(ctx, jsonReq)
	if err != nil {
		return nil, fmt.Errorf("list prompts request failed: %w", err)
	}

	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list prompts error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	return parseListPromptsResultFromJSON(rawResp)
}

// GetPrompt gets a specific prompt.
func (c *StdioClient) GetPrompt(ctx context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("client not initialized")
	}

	requestID := c.requestID.Add(1)
	jsonReq := newJSONRPCRequest(requestID, MethodPromptsGet, map[string]interface{}{
		"name":      req.Params.Name,
		"arguments": req.Params.Arguments,
	})

	rawResp, err := c.transport.sendRequest(ctx, jsonReq)
	if err != nil {
		return nil, fmt.Errorf("get prompt request failed: %w", err)
	}

	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("get prompt error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	return parseGetPromptResultFromJSON(rawResp)
}

// ListResources lists available resources.
func (c *StdioClient) ListResources(ctx context.Context, req *ListResourcesRequest) (*ListResourcesResult, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("client not initialized")
	}

	requestID := c.requestID.Add(1)
	jsonReq := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodResourcesList,
		},
		Params: req.Params,
	}

	rawResp, err := c.transport.sendRequest(ctx, jsonReq)
	if err != nil {
		return nil, fmt.Errorf("list resources request failed: %w", err)
	}

	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list resources error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	return parseListResourcesResultFromJSON(rawResp)
}

// ReadResource reads a specific resource.
func (c *StdioClient) ReadResource(ctx context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("client not initialized")
	}

	requestID := c.requestID.Add(1)
	jsonReq := newJSONRPCRequest(requestID, MethodResourcesRead, map[string]interface{}{
		"uri":       req.Params.URI,
		"arguments": req.Params.Arguments,
	})

	rawResp, err := c.transport.sendRequest(ctx, jsonReq)
	if err != nil {
		return nil, fmt.Errorf("read resource request failed: %w", err)
	}

	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("read resource error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	return parseReadResourceResultFromJSON(rawResp)
}

// RegisterNotificationHandler registers a notification handler.
func (c *StdioClient) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.transport.registerNotificationHandler(method, handler)
}

// UnregisterNotificationHandler unregisters a notification handler.
func (c *StdioClient) UnregisterNotificationHandler(method string) {
	c.transport.unregisterNotificationHandler(method)
}

// GetProcessID returns the process ID.
func (c *StdioClient) GetProcessID() int {
	return c.transport.getProcessID()
}

// GetCommandLine returns the command line.
func (c *StdioClient) GetCommandLine() []string {
	return c.transport.getCommandLine()
}

// IsProcessRunning checks if the process is running.
func (c *StdioClient) IsProcessRunning() bool {
	return c.transport.isProcessRunning()
}

// RestartProcess restarts the server process.
func (c *StdioClient) RestartProcess(ctx context.Context) error {
	// Close current process
	if err := c.transport.close(); err != nil {
		c.logger.Warnf("Error closing current process: %v", err)
	}

	// Reset state.
	c.initialized.Store(false)
	c.setState(StateDisconnected)

	// Start new process
	if err := c.transport.startProcess(); err != nil {
		return fmt.Errorf("failed to restart process: %w", err)
	}

	return nil
}

// GetTransportInfo returns information about the transport.
func (c *StdioClient) GetTransportInfo() TransportInfo {
	capabilities := make(map[string]interface{})
	capabilities["process_management"] = true
	capabilities["command_line"] = c.GetCommandLine()
	capabilities["working_directory"] = c.transport.serverParams.WorkingDir

	if len(c.transport.serverParams.Env) > 0 {
		capabilities["environment_variables"] = c.transport.serverParams.Env
	}

	return TransportInfo{
		Type:         "stdio",
		Description:  "Standard Input/Output transport with process management",
		Capabilities: capabilities,
	}
}

// SetRootsProvider sets the provider for responding to server's roots/list requests.
func (c *StdioClient) SetRootsProvider(provider RootsProvider) {
	c.rootsMu.Lock()
	defer c.rootsMu.Unlock()
	c.rootsProvider = provider
}

// SendRootsListChangedNotification notifies server that roots changed.
func (c *StdioClient) SendRootsListChangedNotification(ctx context.Context) error {
	// Create roots list changed notification.
	notification := &JSONRPCNotification{
		JSONRPC: JSONRPCVersion,
		Notification: Notification{
			Method: MethodNotificationsRootsListChanged,
		},
	}

	return c.transport.sendNotification(ctx, notification)
}

// NewNpxStdioClient creates a new stdio client for NPX-based servers.
func NewNpxStdioClient(packageName string, args []string, clientInfo Implementation, options ...StdioClientOption) (*StdioClient, error) {
	config := StdioTransportConfig{
		ServerParams: StdioServerParameters{
			Command: "npx",
			Args:    append([]string{"-y", packageName}, args...),
		},
		Timeout: 30 * time.Second,
	}

	return NewStdioClient(config, clientInfo, options...)
}

// NewPythonStdioClient creates a new stdio client for Python-based servers.
func NewPythonStdioClient(scriptPath string, args []string, clientInfo Implementation, options ...StdioClientOption) (*StdioClient, error) {
	config := StdioTransportConfig{
		ServerParams: StdioServerParameters{
			Command: "python",
			Args:    append([]string{scriptPath}, args...),
		},
		Timeout: 30 * time.Second,
	}

	return NewStdioClient(config, clientInfo, options...)
}

// NewNodeStdioClient creates a new stdio client for Node.js-based servers.
func NewNodeStdioClient(scriptPath string, args []string, clientInfo Implementation, options ...StdioClientOption) (*StdioClient, error) {
	config := StdioTransportConfig{
		ServerParams: StdioServerParameters{
			Command: "node",
			Args:    append([]string{scriptPath}, args...),
		},
		Timeout: 30 * time.Second,
	}

	return NewStdioClient(config, clientInfo, options...)
}
