// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"

	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// State represents the client state.
type State string

// Client state constants.
const (
	// StateDisconnected indicates the client is not connected to any server.
	StateDisconnected State = "disconnected"
	// StateConnected indicates the client has established a connection but not initialized.
	StateConnected State = "connected"
	// StateInitialized indicates the client is fully initialized and ready for use.
	StateInitialized State = "initialized"
)

// String returns the string representation of the state.
func (s State) String() string {
	return string(s)
}

// Connector defines the core interface that all MCP clients must implement.
// This provides a unified interface for different transport implementations.
type Connector interface {
	// Initialize establishes connection and initializes the MCP client.
	Initialize(ctx context.Context, req *InitializeRequest) (*InitializeResult, error)
	// Close closes the client connection and cleans up resources.
	Close() error
	// GetState returns the current client state.
	GetState() State
	// ListTools retrieves all available tools from the server.
	ListTools(ctx context.Context, req *ListToolsRequest) (*ListToolsResult, error)
	// CallTool executes a specific tool with given parameters.
	CallTool(ctx context.Context, req *CallToolRequest) (*CallToolResult, error)
	// ListPrompts retrieves all available prompts from the server.
	ListPrompts(ctx context.Context, req *ListPromptsRequest) (*ListPromptsResult, error)
	// GetPrompt retrieves a specific prompt by name.
	GetPrompt(ctx context.Context, req *GetPromptRequest) (*GetPromptResult, error)
	// ListResources retrieves all available resources from the server.
	ListResources(ctx context.Context, req *ListResourcesRequest) (*ListResourcesResult, error)
	// ReadResource reads the content of a specific resource.
	ReadResource(ctx context.Context, req *ReadResourceRequest) (*ReadResourceResult, error)
	// RegisterNotificationHandler registers a handler for server notifications.
	RegisterNotificationHandler(method string, handler NotificationHandler)
	// UnregisterNotificationHandler removes a notification handler.
	UnregisterNotificationHandler(method string)
	// SetRootsProvider sets the provider for responding to server's roots/list requests.
	SetRootsProvider(provider RootsProvider)
	// SendRootsListChangedNotification notifies server that roots changed.
	SendRootsListChangedNotification(ctx context.Context) error
}

// SessionClient extends Connector with session management capabilities.
// This is primarily for HTTP-based transports that support sessions.
type SessionClient interface {
	Connector
	// GetSessionID returns the current session ID.
	GetSessionID() string
	// TerminateSession terminates the current session.
	TerminateSession(ctx context.Context) error
}

// ProcessClient extends Connector with process management capabilities.
// This is for stdio-based transports that manage external processes.
type ProcessClient interface {
	Connector

	// GetProcessID returns the process ID of the managed process.
	GetProcessID() int
	// GetCommandLine returns the command line used to start the process.
	GetCommandLine() []string
	// IsProcessRunning checks if the managed process is still running.
	IsProcessRunning() bool
	// RestartProcess restarts the managed process.
	RestartProcess(ctx context.Context) error
}

// TransportInfo provides information about the underlying transport.
type TransportInfo struct {
	Type         string                 `json:"type"`         // "http", "stdio", "sse"
	Description  string                 `json:"description"`  // Human readable description
	Capabilities map[string]interface{} `json:"capabilities"` // Transport-specific capabilities
}

// TransportAware allows clients to expose transport information.
type TransportAware interface {
	// GetTransportInfo returns information about the underlying transport
	GetTransportInfo() TransportInfo
}

// Client represents an MCP client.
type Client struct {
	transport        httpTransport          // transport layer.
	clientInfo       Implementation         // Client information.
	protocolVersion  string                 // Protocol version.
	initialized      bool                   // Whether the client is initialized.
	requestID        atomic.Int64           // Atomic counter for request IDs.
	capabilities     map[string]interface{} // Capabilities.
	state            State                  // State.
	transportOptions []transportOption

	// transport configuration.
	transportConfig *transportConfig

	logger Logger // Logger for client transport (optional).

	// Roots support.
	rootsProvider RootsProvider // Provider for roots information.
	rootsMu       sync.RWMutex  // Mutex for protecting the rootsProvider.
}

// ClientOption client option function
type ClientOption func(*Client)

// NewClient creates a new MCP client.
func NewClient(serverURL string, clientInfo Implementation, options ...ClientOption) (*Client, error) {
	// Parse the server URL.
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrInvalidServerURL, err)
	}

	// Create client.
	client := &Client{
		clientInfo:       clientInfo,
		protocolVersion:  ProtocolVersion_2025_03_26, // Default compatible version.
		capabilities:     make(map[string]interface{}),
		state:            StateDisconnected,
		transportOptions: []transportOption{},
		transportConfig:  newDefaultTransportConfig(),
	}

	// set server URL.
	client.transportConfig.serverURL = parsedURL

	// Apply options.
	for _, option := range options {
		option(client)
	}

	// Create transport layer if not previously set via options.
	if client.transport == nil {
		client.transport = newStreamableHTTPClientTransport(client.transportConfig, client.transportOptions...)

		// Set client reference in transport for roots handling.
		if streamableTransport, ok := client.transport.(*streamableHTTPClientTransport); ok {
			streamableTransport.client = client
		}
	}

	return client, nil
}

// transportConfig includes transport layer configuration.
type transportConfig struct {
	serverURL    *url.URL // server URL
	httpClient   *http.Client
	httpHeaders  http.Header
	logger       Logger
	enableGetSSE bool   // for streamable transport
	path         string // for streamable transport

	// HTTP request handler for custom implementations.
	// This field stores the custom HTTP request handler to be used by transport layers.
	httpReqHandler HTTPReqHandler

	// Service name for custom HTTP request handlers.
	// This field is typically not used by the default handler, but may be used by custom
	// implementations that replace the default NewHTTPReqHandler function.
	serviceName string
	// HTTP request handler options.
	// These options are typically not used by the default handler, but may be used by custom
	// implementations that replace the default NewHTTPReqHandler function for extensibility.
	httpReqHandlerOptions []HTTPReqHandlerOption
}

// newDefaultTransportConfig creates a default transport configuration.
func newDefaultTransportConfig() *transportConfig {
	return &transportConfig{
		httpClient:            &http.Client{},
		httpHeaders:           make(http.Header),
		logger:                GetDefaultLogger(),
		serviceName:           "",
		httpReqHandlerOptions: []HTTPReqHandlerOption{},
		enableGetSSE:          true,
		path:                  "",
	}
}

// extractTransportConfig extracts transport configuration from client options.
func extractTransportConfig(options []ClientOption) *transportConfig {
	// create a temporary client to collect configuration.
	tempClient := &Client{
		transportConfig: newDefaultTransportConfig(),
	}

	// apply all options.
	for _, option := range options {
		option(tempClient)
	}

	return tempClient.transportConfig
}

// WithProtocolVersion sets the protocol version.
func WithProtocolVersion(version string) ClientOption {
	return func(c *Client) {
		c.protocolVersion = version
	}
}

// WithClientLogger sets the logger for the client transport.
func WithClientLogger(logger Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
		c.transportOptions = append(c.transportOptions, withClientTransportLogger(logger))
	}
}

// WithClientGetSSEEnabled sets whether to enable GET SSE.
func WithClientGetSSEEnabled(enabled bool) ClientOption {
	return func(c *Client) {
		c.transportOptions = append(c.transportOptions, withClientTransportGetSSEEnabled(enabled))
	}
}

// WithClientPath sets a custom path for the client transport.
func WithClientPath(path string) ClientOption {
	return func(c *Client) {
		c.transportOptions = append(c.transportOptions, withClientTransportPath(path))
	}
}

// WithHTTPReqHandler sets a custom HTTP request handler for the client
func WithHTTPReqHandler(handler HTTPReqHandler) ClientOption {
	return func(c *Client) {
		// This is needed for SSE clients which read directly from transportConfig.
		c.transportConfig.httpReqHandler = handler
		// Also set in transportOptions for streamable transport compatibility.
		c.transportOptions = append(c.transportOptions, withTransportHTTPReqHandler(handler))
	}
}

// WithHTTPHeaders sets custom HTTP headers for all requests.
// Headers will be applied to all HTTP requests made by the client,
// including initialization, tool calls, notifications, and SSE connections.
func WithHTTPHeaders(headers http.Header) ClientOption {
	return func(c *Client) {
		// Set headers in transportConfig for direct use by extractTransportConfig.
		if c.transportConfig.httpHeaders == nil {
			c.transportConfig.httpHeaders = make(http.Header)
		}
		for k, v := range headers {
			c.transportConfig.httpHeaders[k] = v
		}
		// Also set in transportOptions for streamable transport compatibility.
		c.transportOptions = append(c.transportOptions, withTransportHTTPHeaders(headers))
	}
}

// WithServiceName sets the service name for custom HTTP request handlers.
// This is typically only needed when using custom implementations of HTTPReqHandler.
func WithServiceName(serviceName string) ClientOption {
	return func(c *Client) {
		c.transportOptions = append(c.transportOptions, withTransportServiceName(serviceName))
	}
}

// WithHTTPReqHandlerOption adds one or more options for HTTP request handler.
// This is typically only needed when using custom implementations of HTTPReqHandler
// that support additional configuration options.
func WithHTTPReqHandlerOption(options ...HTTPReqHandlerOption) ClientOption {
	return func(c *Client) {
		for _, option := range options {
			c.transportOptions = append(c.transportOptions, withTransportHTTPReqHandlerOption(option))
		}
	}
}

// GetState returns the current client state.
func (c *Client) GetState() State {
	return c.state
}

// setState sets the client state.
func (c *Client) setState(state State) {
	c.state = state
}

// Initialize initializes the client connection.
func (c *Client) Initialize(ctx context.Context, initReq *InitializeRequest) (*InitializeResult, error) {
	// Check if already initialized.
	if c.initialized {
		return nil, errors.ErrAlreadyInitialized
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := newJSONRPCRequest(requestID, MethodInitialize, map[string]interface{}{
		"protocolVersion": c.protocolVersion,
		"clientInfo":      c.clientInfo,
		"capabilities":    c.capabilities,
	})

	if initReq != nil && !isZeroStruct(initReq.Params) {
		req.Params = initReq.Params
	}

	// Send request and wait for response
	rawResp, err := c.transport.sendRequest(ctx, req)
	if err != nil {
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("initialization request failed: %w", err)
	}

	// Connection is established successfully at this point
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

	// Parse the response using our specialized parser
	initResult, err := parseInitializeResultFromJSON(rawResp)
	if err != nil {
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("failed to parse initialization response: %w", err)
	}

	// Send initialized notification.
	if err := c.SendInitialized(ctx); err != nil {
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("failed to send initialized notification: %v", err)
	}

	// Update state and initialized flag
	c.initialized = true
	c.setState(StateInitialized)

	// Try to establish GET SSE connection if transport supports it
	if t, ok := c.transport.(*streamableHTTPClientTransport); ok {
		// Start GET SSE connection asynchronously to avoid blocking
		go t.establishGetSSEConnection()
	}

	return initResult, nil
}

// SendInitialized sends an initialized notification.
func (c *Client) SendInitialized(ctx context.Context) error {
	notification := NewInitializedNotification()
	return c.transport.sendNotification(ctx, notification)
}

// ListTools lists available tools.
func (c *Client) ListTools(ctx context.Context, listToolsReq *ListToolsRequest) (*ListToolsResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, errors.ErrNotInitialized
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodToolsList,
		},
		Params: listToolsReq.Params,
	}

	rawResp, err := c.transport.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list tools request failed: %v", err)
	}

	// Check for error response
	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list tools error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return parseListToolsResultFromJSON(rawResp)
}

// CallTool calls a tool.
func (c *Client) CallTool(ctx context.Context, callToolReq *CallToolRequest) (*CallToolResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, errors.ErrNotInitialized
	}

	// Create request
	requestID := c.requestID.Add(1)
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodToolsCall,
		},
		Params: callToolReq.Params,
	}

	rawResp, err := c.transport.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tool call request failed: %w", err)
	}

	// Check for error response
	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("tool call error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	return parseCallToolResult(rawResp)
}

// Close closes the client connection and cleans up resources.
func (c *Client) Close() error {
	if c.transport != nil {
		err := c.transport.close()
		c.setState(StateDisconnected)
		c.initialized = false
		return err
	}
	return nil
}

// GetSessionID gets the session ID.
func (c *Client) GetSessionID() string {
	return c.transport.getSessionID()
}

// TerminateSession terminates the session.
func (c *Client) TerminateSession(ctx context.Context) error {
	return c.transport.terminateSession(ctx)
}

// RegisterNotificationHandler registers a notification handler.
func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	if httpTransport, ok := c.transport.(*streamableHTTPClientTransport); ok {
		httpTransport.registerNotificationHandler(method, handler)
	} else if stdioTransport, ok := c.transport.(*stdioClientTransport); ok {
		stdioTransport.registerNotificationHandler(method, handler)
	}
}

// UnregisterNotificationHandler unregisters a notification handler.
func (c *Client) UnregisterNotificationHandler(method string) {
	if httpTransport, ok := c.transport.(*streamableHTTPClientTransport); ok {
		httpTransport.unregisterNotificationHandler(method)
	} else if stdioTransport, ok := c.transport.(*stdioClientTransport); ok {
		stdioTransport.unregisterNotificationHandler(method)
	}
}

// ListPrompts lists available prompts.
func (c *Client) ListPrompts(ctx context.Context, listPromptsReq *ListPromptsRequest) (*ListPromptsResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, errors.ErrNotInitialized
	}

	// Create request
	requestID := c.requestID.Add(1)
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodPromptsList,
		},
		Params: listPromptsReq.Params,
	}

	rawResp, err := c.transport.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list prompts request failed: %w", err)
	}

	// Check for error response
	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list prompts error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return parseListPromptsResultFromJSON(rawResp)
}

// GetPrompt gets a specific prompt.
func (c *Client) GetPrompt(ctx context.Context, getPromptReq *GetPromptRequest) (*GetPromptResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, errors.ErrNotInitialized
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodPromptsGet,
		},
		Params: getPromptReq.Params,
	}

	rawResp, err := c.transport.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get prompt request failed: %v", err)
	}

	// Check for error response
	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("get prompt error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return parseGetPromptResultFromJSON(rawResp)
}

// ListResources lists available resources.
func (c *Client) ListResources(ctx context.Context, listResourcesReq *ListResourcesRequest) (*ListResourcesResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("%w", errors.ErrNotInitialized)
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodResourcesList,
		},
		Params: listResourcesReq.Params,
	}

	rawResp, err := c.transport.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list resources request failed: %v", err)
	}

	// Check for error response
	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list resources error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return parseListResourcesResultFromJSON(rawResp)
}

// ReadResource reads a specific resource.
func (c *Client) ReadResource(ctx context.Context, readResourceReq *ReadResourceRequest) (*ReadResourceResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("%w", errors.ErrNotInitialized)
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodResourcesRead,
		},
		Params: readResourceReq.Params,
	}

	rawResp, err := c.transport.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("read resource request failed: %v", err)
	}

	// Check for error response
	if isErrorResponse(rawResp) {
		errResp, err := parseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("read resource error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return parseReadResourceResultFromJSON(rawResp)
}

// SetRootsProvider sets the provider for responding to server's roots/list requests.
func (c *Client) SetRootsProvider(provider RootsProvider) {
	c.rootsMu.Lock()
	defer c.rootsMu.Unlock()
	c.rootsProvider = provider
}

// SendRootsListChangedNotification notifies server that roots changed.
func (c *Client) SendRootsListChangedNotification(ctx context.Context) error {
	// Create roots list changed notification.
	notification := NewJSONRPCNotificationFromMap(MethodNotificationsRootsListChanged, nil)
	return c.transport.sendNotification(ctx, notification)
}

func isZeroStruct(x interface{}) bool {
	return reflect.ValueOf(x).IsZero()
}
