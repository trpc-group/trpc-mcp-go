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
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/client"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
	"trpc.group/trpc-go/trpc-mcp-go/internal/retry"
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

	// OAuth provider
	oauthProvider client.OAuthClientProvider
	// OAuth token management
	accessToken  string
	refreshToken string

	// OAuth flow configuration
	authFlowConfig *AuthFlowConfig

	// transport configuration.
	transportConfig *transportConfig

	logger Logger // Logger for client transport (optional).

	// Retry configuration.
	retryConfig *retry.Config // Configuration for retry behavior (optional).

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

	// Handle OAuth authentication if configured
	if client.oauthProvider != nil {
		if tokens, err := client.oauthProvider.Tokens(); err == nil && tokens != nil {
			client.accessToken = tokens.AccessToken
			if tokens.RefreshToken != nil {
				client.refreshToken = *tokens.RefreshToken
			}
			if client.accessToken != "" {
				// Ensure a headers map exists
				if client.transportConfig.httpHeaders == nil {
					client.transportConfig.httpHeaders = make(http.Header)
				}
				client.transportConfig.httpHeaders.Set("Authorization", "Bearer "+client.accessToken)
				// Also push into transportOptions so the streamable transport sees it
				client.transportOptions = append(client.transportOptions, withTransportHTTPHeaders(client.transportConfig.httpHeaders))
			}
		} else if err != nil {
			// Optional: Surface a warning via logger if available
			if client.logger != nil {
				client.logger.Warnf("OAuth provider returned no tokens at client initialization: %v", err)
			}
		}
	}

	// Create transport layer if not previously set via options.
	if client.transport == nil {
		client.transport = newStreamableHTTPClientTransport(client.transportConfig, client.transportOptions...)

		// Set client reference in transport for roots handling.
		if streamableTransport, ok := client.transport.(*streamableHTTPClientTransport); ok {
			streamableTransport.client = client
		}
	}

	// Set retry config on transport if configured
	if client.retryConfig != nil {
		client.transport.setRetryConfig(client.retryConfig)
	}

	return client, nil
}

// AuthFlowConfig configures OAuth 2.0 authentication flow
type AuthFlowConfig struct {
	// ServerURL is the OAuth authorization server URL (required)
	ServerURL string

	// ClientMetadata contains OAuth client configuration
	ClientMetadata auth.OAuthClientMetadata

	// RedirectURL is the OAuth redirect URI (required)
	RedirectURL string

	// OnRedirect handles the authorization redirect (required)
	// This function should redirect the user to the authorization URL
	OnRedirect func(*url.URL) error

	// Scope defines the requested access permissions (optional)
	Scope *string

	// CustomFetchFunc allows custom HTTP client behavior (optional)
	CustomFetchFunc auth.FetchFunc

	// ResourceMetadataURL for discovering protected resource metadata (optional)
	ResourceMetadataURL *string

	// AuthorizationCode can be provided if you already have one (optional)
	// This is useful for handling the redirect callback
	AuthorizationCode *string

	// State for CSRF protection (optional)
	State *string
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

	oauthProvider client.OAuthClientProvider
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
		oauthProvider:         nil,
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
		c.transportConfig.logger = logger
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
		c.transportConfig.path = path
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
		c.transportConfig.serviceName = serviceName
		c.transportOptions = append(c.transportOptions, withTransportServiceName(serviceName))
	}
}

// WithHTTPReqHandlerOption adds one or more options for HTTP request handler.
// This is typically only needed when using custom implementations of HTTPReqHandler
// that support additional configuration options.
func WithHTTPReqHandlerOption(options ...HTTPReqHandlerOption) ClientOption {
	return func(c *Client) {
		c.transportConfig.httpReqHandlerOptions = append(c.transportConfig.httpReqHandlerOptions, options...)
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

	// If auth flow is configured, execute it first
	if c.authFlowConfig != nil {
		if err := c.executeAuthFlow(ctx); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}

		// Ensure transport uses the latest token
		if err := c.updateClientTokens(); err != nil {
			return nil, fmt.Errorf("failed to update tokens: %w", err)
		}
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

func WithOAuthClientProvider(p client.OAuthClientProvider) ClientOption {
	return func(c *Client) {
		// 1) 存一份在 Client（后续取初始 token / 刷新等会用到）
		c.oauthProvider = p
		// 2) 也下发给 transport（与 a2a 同步，走 transportOptions 管线）
		c.transportOptions = append(c.transportOptions, withTransportOAuthProvider(p))
	}
}

// WithAuthFlow creates a client option that configures and executes the complete OAuth flow
func WithAuthFlow(config AuthFlowConfig) ClientOption {
	return func(c *Client) {
		// Validate required configuration
		if config.ServerURL == "" {
			panic("AuthFlowConfig.ServerURL is required")
		}
		if config.RedirectURL == "" {
			panic("AuthFlowConfig.RedirectURL is required")
		}
		if config.OnRedirect == nil {
			panic("AuthFlowConfig.OnRedirect is required")
		}

		// Store config for later use
		c.authFlowConfig = &config

		// Create internal OAuth provider
		provider := client.NewInMemoryOAuthClientProvider(
			config.RedirectURL,
			config.ClientMetadata,
			config.OnRedirect,
		)

		c.oauthProvider = provider
		c.transportConfig.oauthProvider = provider
		c.transportOptions = append(c.transportOptions, withTransportOAuthProvider(provider))

	}
}

// executeAuthFlow runs the complete OAuth authentication flow using internal methods
func (c *Client) executeAuthFlow(ctx context.Context) error {
	if c.authFlowConfig == nil {
		return fmt.Errorf("auth flow not configured")
	}

	// Build auth options for internal flow
	authOptions := auth.AuthOptions{
		ServerUrl:           c.authFlowConfig.ServerURL,
		Scope:               c.authFlowConfig.Scope,
		FetchFn:             c.authFlowConfig.CustomFetchFunc,
		ResourceMetadataUrl: c.authFlowConfig.ResourceMetadataURL,
	}

	// Execute the complete internal authentication flow
	result, err := c.authInternal(authOptions)
	if err != nil {
		return fmt.Errorf("OAuth flow failed: %w", err)
	}

	// Handle authentication result
	return c.handleAuthResult(result)
}

// authInternal orchestrates the complete OAuth flow using internal methods
func (c *Client) authInternal(options auth.AuthOptions) (*client.AuthResult, error) {
	// Discover protected resource metadata (if available)
	var resourceMetadata *auth.OAuthProtectedResourceMetadata
	var authorizationServerUrl string

	metadata, err := c.discoverProtectedResource(options.ServerUrl, options)
	if err == nil {
		resourceMetadata = metadata
		if len(resourceMetadata.AuthorizationServers) > 0 {
			authorizationServerUrl = resourceMetadata.AuthorizationServers[0]
		}
	}

	if authorizationServerUrl == "" {
		authorizationServerUrl = options.ServerUrl
	}

	// Select resource URL
	resource, err := c.selectResourceURL(options.ServerUrl, resourceMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to select resource URL: %w", err)
	}

	// Discover authorization server metadata
	serverMetadata, err := c.discoverAuthServer(authorizationServerUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to discover authorization server: %w", err)
	}

	// Handle client registration if needed
	clientInfo, err := c.handleClientRegistration(authorizationServerUrl, serverMetadata, options)
	if err != nil {
		return nil, fmt.Errorf("client registration failed: %w", err)
	}

	// Try token refresh if refresh token exists
	if result, err := c.tryTokenRefresh(authorizationServerUrl, serverMetadata, clientInfo, resource, options); err == nil {
		return result, nil
	}

	// Exchange the authorization code for a token
	if options.AuthorizationCode != nil && *options.AuthorizationCode != "" {
		cv, err := c.oauthProvider.CodeVerifier()
		if err != nil || cv == "" {
			return nil, fmt.Errorf("missing code_verifier: %w", err)
		}

		var addClientAuth func(http.Header, url.Values, string) error
		if ap, ok := c.oauthProvider.(client.OAuthClientAuthProvider); ok {
			addClientAuth = ap.AddClientAuthentication
		}

		tokens, err := client.ExchangeAuthorization(authorizationServerUrl, client.ExchangeAuthorizationOptions{
			Metadata:                serverMetadata,
			ClientInformation:       clientInfo,
			AuthorizationCode:       *options.AuthorizationCode,
			CodeVerifier:            cv,
			RedirectURI:             c.oauthProvider.RedirectURL(),
			Resource:                resource,
			AddClientAuthentication: addClientAuth,
			FetchFn:                 options.FetchFn,
		})
		if err != nil {
			return nil, err
		}
		if err := c.oauthProvider.SaveTokens(*tokens); err != nil {
			return nil, fmt.Errorf("failed to save tokens: %w", err)
		}
		res := client.AuthResultAuthorized
		return &res, nil
	}

	// Start authorization flow
	return c.startAuthorizationFlow(authorizationServerUrl, serverMetadata, clientInfo, resource, options)
}

// discoverProtectedResource discovers OAuth protected resource metadata
func (c *Client) discoverProtectedResource(serverUrl string, options auth.AuthOptions) (*auth.OAuthProtectedResourceMetadata, error) {
	discoveryOptions := &auth.DiscoveryOptions{
		ResourceMetadataUrl: options.ResourceMetadataUrl,
	}

	return client.DiscoverOAuthProtectedResourceMetadata(serverUrl, discoveryOptions, options.FetchFn)
}

// discoverAuthServer discovers authorization server metadata
func (c *Client) discoverAuthServer(authServerUrl string) (auth.AuthorizationServerMetadata, error) {
	return client.DiscoverAuthorizationServerMetadata(context.Background(), authServerUrl, nil)
}

// handleClientRegistration handles dynamic client registration if needed
func (c *Client) handleClientRegistration(authServerUrl string, serverMetadata auth.AuthorizationServerMetadata, options auth.AuthOptions) (*auth.OAuthClientInformation, error) {
	clientInfo := c.oauthProvider.ClientInformation()

	if clientInfo == nil {
		// Need to register client
		if _, ok := c.oauthProvider.(client.OAuthClientInfoProvider); !ok {
			return nil, fmt.Errorf("OAuth client information must be saveable for dynamic registration")
		}

		fullInfo, err := client.RegisterClient(context.Background(), authServerUrl, client.RegisterClientOptions{
			Metadata:       serverMetadata,
			ClientMetadata: c.oauthProvider.ClientMetadata(),
			FetchFn:        options.FetchFn,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to register client: %w", err)
		}

		if clientInfoProvider, ok := c.oauthProvider.(client.OAuthClientInfoProvider); ok {
			if err := clientInfoProvider.SaveClientInformation(*fullInfo); err != nil {
				return nil, fmt.Errorf("failed to save client information: %w", err)
			}
		}

		clientInfo = &auth.OAuthClientInformation{
			ClientID:     fullInfo.ClientID,
			ClientSecret: fullInfo.ClientSecret,
		}
	}

	return clientInfo, nil
}

// selectResourceURL selects the appropriate resource URL
func (c *Client) selectResourceURL(serverUrl string, resourceMetadata *auth.OAuthProtectedResourceMetadata) (*url.URL, error) {
	defaultResource, err := auth.ResourceURLFromServerURL(serverUrl)
	if err != nil {
		return nil, err
	}

	// Use custom validator if available
	if validator, ok := c.oauthProvider.(client.OAuthResourceValidator); ok {
		return validator.ValidateResourceURL(defaultResource, resourceMetadata)
	}

	// Include resource param only when metadata exists
	if resourceMetadata == nil {
		return nil, nil
	}

	// Check metadata resource compatibility
	allowed, err := auth.CheckResourceAllowed(auth.CheckResourceAllowedParams{
		RequestedResource:  defaultResource,
		ConfiguredResource: resourceMetadata.Resource,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to validate resource: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("protected resource mismatch")
	}

	return url.Parse(resourceMetadata.Resource)
}

// tryTokenRefresh attempts to refresh existing tokens
func (c *Client) tryTokenRefresh(authServerUrl string, serverMetadata auth.AuthorizationServerMetadata, clientInfo *auth.OAuthClientInformation, resource *url.URL, options auth.AuthOptions) (*client.AuthResult, error) {
	tokens, err := c.oauthProvider.Tokens()
	if err != nil || tokens == nil || tokens.RefreshToken == nil || *tokens.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	var addClientAuth func(http.Header, url.Values, string) error
	if authProvider, ok := c.oauthProvider.(client.OAuthClientAuthProvider); ok {
		addClientAuth = authProvider.AddClientAuthentication
	}

	newTokens, err := client.RefreshAuthorization(authServerUrl, client.RefreshAuthorizationOptions{
		Metadata:                serverMetadata,
		ClientInformation:       clientInfo,
		RefreshToken:            *tokens.RefreshToken,
		Resource:                resource,
		AddClientAuthentication: addClientAuth,
		FetchFn:                 options.FetchFn,
	})
	if err != nil {
		return nil, err
	}

	if err := c.oauthProvider.SaveTokens(*newTokens); err != nil {
		return nil, fmt.Errorf("failed to save refreshed tokens: %w", err)
	}

	result := client.AuthResultAuthorized
	return &result, nil
}

// startAuthorizationFlow starts the authorization code flow
func (c *Client) startAuthorizationFlow(authServerUrl string, serverMetadata auth.AuthorizationServerMetadata, clientInfo *auth.OAuthClientInformation, resource *url.URL, options auth.AuthOptions) (*client.AuthResult, error) {
	var state *string
	if c.authFlowConfig.State != nil {
		state = c.authFlowConfig.State
	} else if stateProvider, ok := c.oauthProvider.(client.OAuthStateProvider); ok {
		stateValue, err := stateProvider.State()
		if err != nil {
			return nil, fmt.Errorf("failed to get state: %w", err)
		}
		state = &stateValue
	}

	scope := options.Scope
	if scope == nil {
		clientMetadata := c.oauthProvider.ClientMetadata()
		if clientMetadata.Scope != nil {
			scope = clientMetadata.Scope
		}
	}

	authResult, err := client.StartAuthorization(authServerUrl, client.StartAuthorizationOptions{
		Metadata:          serverMetadata,
		ClientInformation: *clientInfo,
		State:             state,
		RedirectURL:       c.oauthProvider.RedirectURL(),
		Scope:             scope,
		Resource:          resource,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start authorization: %w", err)
	}

	if err := c.oauthProvider.SaveCodeVerifier(authResult.CodeVerifier); err != nil {
		return nil, fmt.Errorf("failed to save code verifier: %w", err)
	}

	if err := c.oauthProvider.RedirectToAuthorization(authResult.AuthorizationURL); err != nil {
		return nil, fmt.Errorf("failed to redirect to authorization: %w", err)
	}

	result := client.AuthResultRedirect
	return &result, nil
}

// handleAuthResult processes the authentication result
func (c *Client) handleAuthResult(result *client.AuthResult) error {
	switch *result {
	case client.AuthResultAuthorized:
		return c.updateClientTokens()
	case client.AuthResultRedirect:
		// User needs to complete authorization, this is normal
		return nil
	default:
		return fmt.Errorf("unknown authentication result: %s", *result)
	}
}

// updateClientTokens updates HTTP headers with new tokens
func (c *Client) updateClientTokens() error {
	tokens, err := c.oauthProvider.Tokens()
	if err != nil {
		return fmt.Errorf("failed to get tokens: %w", err)
	}

	if tokens != nil && tokens.AccessToken != "" {
		c.accessToken = tokens.AccessToken
		if tokens.RefreshToken != nil {
			c.refreshToken = *tokens.RefreshToken
		}

		// Update HTTP headers
		if c.transportConfig.httpHeaders == nil {
			c.transportConfig.httpHeaders = make(http.Header)
		}
		c.transportConfig.httpHeaders.Set("Authorization", "Bearer "+c.accessToken)

		// Update existing transport
		if c.transport != nil {
			if streamableTransport, ok := c.transport.(*streamableHTTPClientTransport); ok {
				if streamableTransport.httpHeaders == nil {
					streamableTransport.httpHeaders = make(http.Header)
				}
				streamableTransport.httpHeaders.Set("Authorization", "Bearer "+c.accessToken)
			}
		}
	}

	return nil
}

// CompleteAuthFlow completes the OAuth flow with authorization code
func (c *Client) CompleteAuthFlow(ctx context.Context, authorizationCode string) error {
	if c.authFlowConfig == nil {
		return fmt.Errorf("auth flow not configured")
	}

	// Set the authorization code in options
	authOptions := auth.AuthOptions{
		ServerUrl:           c.authFlowConfig.ServerURL,
		Scope:               c.authFlowConfig.Scope,
		FetchFn:             c.authFlowConfig.CustomFetchFunc,
		ResourceMetadataUrl: c.authFlowConfig.ResourceMetadataURL,
		AuthorizationCode:   &authorizationCode,
	}

	// Execute the flow with the authorization code
	result, err := c.authInternal(authOptions)
	if err != nil {
		return fmt.Errorf("failed to complete auth flow: %w", err)
	}

	return c.handleAuthResult(result)
}
