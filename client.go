// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"sync/atomic"

	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// State represents the client state.
type State string

// Client state constants.
const (
	StateDisconnected State = "disconnected"
	StateConnected    State = "connected"
	StateInitialized  State = "initialized"
)

// String returns the string representation of the state.
func (s State) String() string {
	return string(s)
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

	logger Logger // Logger for client transport (optional).
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
	}

	// Apply options.
	for _, option := range options {
		option(client)
	}

	// Create transport layer if not previously set via options.
	if client.transport == nil {
		client.transport = newStreamableHTTPClientTransport(parsedURL, client.transportOptions...)
	}

	return client, nil
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

func WithClientPath(path string) ClientOption {
	return func(c *Client) {
		c.transportOptions = append(c.transportOptions, withClientTransportPath(path))
	}
}

// WithHTTPReqHandler sets a custom HTTP request handler for the client
func WithHTTPReqHandler(handler HTTPReqHandler) ClientOption {
	return func(c *Client) {
		c.transportOptions = append(c.transportOptions, withTransportHTTPReqHandler(handler))
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
	req := newJSONRPCRequest(requestID, MethodToolsList, nil)

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
	}
}

// UnregisterNotificationHandler unregisters a notification handler.
func (c *Client) UnregisterNotificationHandler(method string) {
	if httpTransport, ok := c.transport.(*streamableHTTPClientTransport); ok {
		httpTransport.unregisterNotificationHandler(method)
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

func isZeroStruct(x interface{}) bool {
	return reflect.ValueOf(x).IsZero()
}
