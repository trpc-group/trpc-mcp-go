package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/transport"
)

// Client state constants.
const (
	StateDisconnected = "disconnected"
	StateConnected    = "connected"
	StateInitialized  = "initialized"
)

// Client represents an MCP client.
type Client struct {
	// Transport layer.
	transport transport.HTTPTransport

	// Client information.
	clientInfo mcp.Implementation

	// Protocol version.
	protocolVersion string

	// Whether the client is initialized.
	initialized bool

	// Capabilities.
	capabilities map[string]interface{}

	// State.
	state string
}

// ClientOption client option function
type ClientOption func(*Client)

// NewClient creates a new MCP client.
func NewClient(serverURL string, clientInfo mcp.Implementation, options ...ClientOption) (*Client, error) {
	// Parse the server URL.
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %v", err)
	}

	// Create client
	client := &Client{
		clientInfo:      clientInfo,
		protocolVersion: mcp.ProtocolVersion_2024_11_05, // Default compatible version
		capabilities:    make(map[string]interface{}),
		state:           StateDisconnected,
	}

	// Apply options
	for _, option := range options {
		option(client)
	}

	// Create transport layer if not previously set via options
	if client.transport == nil {
		// Create default transport layer, supporting both JSON and SSE
		client.transport = transport.NewStreamableHTTPClientTransport(parsedURL)
	}

	return client, nil
}

// WithProtocolVersion sets the protocol version.
func WithProtocolVersion(version string) ClientOption {
	return func(c *Client) {
		c.protocolVersion = version
	}
}

// WithTransport sets the custom transport layer.
func WithTransport(transport transport.HTTPTransport) ClientOption {
	return func(c *Client) {
		c.transport = transport
	}
}

// WithGetSSEEnabled sets whether to enable GET SSE.
func WithGetSSEEnabled(enabled bool) ClientOption {
	return func(c *Client) {
		if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
			// Use WithEnableGetSSE option.
			transport.WithEnableGetSSE(enabled)(httpTransport)
		}
	}
}

// Initialize initializes the client connection.
func (c *Client) Initialize(ctx context.Context) (*mcp.InitializeResult, error) {
	// Check if already initialized.
	if c.initialized {
		return nil, fmt.Errorf("client already initialized")
	}

	// Create request.
	req := mcp.NewJSONRPCRequest("initialize", mcp.MethodInitialize, map[string]interface{}{
		"protocolVersion": c.protocolVersion,
		"clientInfo":      c.clientInfo,
		"capabilities":    c.capabilities,
	})

	// Fallback to old method (for cases that don't support StreamableHTTPClientTransport)
	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("initialization request failed: %v", err)
	}

	// Check for error response
	if mcp.IsErrorResponse(rawResp) {
		errResp, err := mcp.ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("initialization error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse the response using our specialized parser
	initResult, err := mcp.ParseInitializeResultFromJSON(rawResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse initialization response: %v", err)
	}

	// Send initialized notification.
	if err := c.SendInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %v", err)
	}

	c.initialized = true
	return initResult, nil
}

// SendInitialized sends an initialized notification.
func (c *Client) SendInitialized(ctx context.Context) error {
	notification := mcp.NewInitializedNotification()
	return c.transport.SendNotification(ctx, notification)
}

// ListTools lists available tools.
func (c *Client) ListTools(ctx context.Context) (*mcp.ListToolsResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := mcp.NewJSONRPCRequest("tools-list", mcp.MethodToolsList, nil)

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list tools: %v", err)
		}

		// Type assert.
		if toolsResult, ok := result.(*mcp.ListToolsResult); ok {
			return toolsResult, nil
		}
		return nil, fmt.Errorf("failed to parse list tools result: type assertion error")
	}

	// Fallback to old method
	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list tools request failed: %v", err)
	}

	// Check for error response
	if mcp.IsErrorResponse(rawResp) {
		errResp, err := mcp.ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list tools error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return mcp.ParseListToolsResultFromJSON(rawResp)
}

// CallTool calls a tool.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request
	req := mcp.NewJSONRPCRequest("tool-call", mcp.MethodToolsCall, map[string]interface{}{
		"name":      name,
		"arguments": args,
	})

	//// Send request and parse response.
	//if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
	//	result, err := httpTransport.SendRequestAndParse(ctx, req)
	//	if err != nil {
	//		return nil, fmt.Errorf("tool call failed: %v", err)
	//	}
	//
	//	// Type assert.
	//	if toolResult, ok := result.(*mcp.CallToolResult); ok {
	//		return toolResult, nil
	//	}
	//	return nil, fmt.Errorf("failed to parse tool call result: type assertion error")
	//}

	// Fallback to old method
	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tool call request failed: %v", err)
	}

	// Check for error response
	if mcp.IsErrorResponse(rawResp) {
		errResp, err := mcp.ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("tool call error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	return mcp.ParseCallToolResult(rawResp)

	//// Parse response as success response
	//var resp struct {
	//	Result mcp.CallToolResult `json:"result"`
	//}
	//if err := json.Unmarshal(*rawResp, &resp); err != nil {
	//	return nil, fmt.Errorf("failed to unmarshal tool call response: %w", err)
	//}
	//
	//return &resp.Result, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	return c.transport.Close()
}

// GetSessionID gets the session ID.
func (c *Client) GetSessionID() string {
	return c.transport.GetSessionID()
}

// TerminateSession terminates the session.
func (c *Client) TerminateSession(ctx context.Context) error {
	return c.transport.TerminateSession(ctx)
}

// RegisterNotificationHandler registers a notification handler.
func (c *Client) RegisterNotificationHandler(method string, handler transport.NotificationHandler) {
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		httpTransport.RegisterNotificationHandler(method, handler)
	}
}

// UnregisterNotificationHandler unregisters a notification handler.
func (c *Client) UnregisterNotificationHandler(method string) {
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		httpTransport.UnregisterNotificationHandler(method)
	}
}

// CallToolWithStream calls a tool with streaming support.
func (c *Client) CallToolWithStream(ctx context.Context, name string, args map[string]interface{}, streamOpts *transport.StreamOptions) (*mcp.CallToolResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := mcp.NewJSONRPCRequest("tool-call", mcp.MethodToolsCall, map[string]interface{}{
		"name":      name,
		"arguments": args,
	})

	// Check if using streaming transport.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		// If no streaming options, try using unified parsing method.
		if streamOpts == nil {
			result, err := httpTransport.SendRequestAndParse(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("tool call failed: %v", err)
			}

			// Type assert.
			if toolResult, ok := result.(*mcp.CallToolResult); ok {
				return toolResult, nil
			}
			return nil, fmt.Errorf("failed to parse tool call result: type assertion error")
		}

		// Use streaming request if streaming options are provided
		rawResp, err := httpTransport.SendRequestWithStream(ctx, req, streamOpts)
		if err != nil {
			return nil, fmt.Errorf("tool call request failed: %v", err)
		}

		// Check for error response
		if mcp.IsErrorResponse(rawResp) {
			errResp, err := mcp.ParseRawMessageToError(rawResp)
			if err != nil {
				return nil, fmt.Errorf("failed to parse error response: %w", err)
			}
			return nil, fmt.Errorf("tool call error: %s (code: %d)",
				errResp.Error.Message, errResp.Error.Code)
		}

		// Parse response as success response
		var resp struct {
			Result mcp.CallToolResult `json:"result"`
		}
		if err := json.Unmarshal(*rawResp, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool call response: %w", err)
		}

		return &resp.Result, nil
	}

	// Fall back to regular call for non-streaming transports
	return c.CallTool(ctx, name, args)
}

// ListPrompts lists available prompts.
func (c *Client) ListPrompts(ctx context.Context) (*mcp.ListPromptsResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := mcp.NewJSONRPCRequest("prompts-list", mcp.MethodPromptsList, nil)

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list prompts: %v", err)
		}

		// Type assert.
		if promptsResult, ok := result.(*mcp.ListPromptsResult); ok {
			return promptsResult, nil
		}
		return nil, fmt.Errorf("failed to parse list prompts result: type assertion error")
	}

	// Fallback to old method
	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list prompts request failed: %v", err)
	}

	// Check for error response
	if mcp.IsErrorResponse(rawResp) {
		errResp, err := mcp.ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list prompts error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return mcp.ParseListPromptsResultFromJSON(rawResp)
}

// GetPrompt gets a specific prompt.
func (c *Client) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Prepare parameters.
	params := map[string]interface{}{
		"name": name,
	}

	// If arguments are provided, add them to the request.
	if arguments != nil && len(arguments) > 0 {
		params["arguments"] = arguments
	}

	// Create request.
	req := mcp.NewJSONRPCRequest("prompt-get", mcp.MethodPromptsGet, params)

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to get prompt: %v", err)
		}

		// Type assert.
		if promptResult, ok := result.(*mcp.GetPromptResult); ok {
			return promptResult, nil
		}
		return nil, fmt.Errorf("failed to parse get prompt result: type assertion error")
	}

	// Fallback to old method
	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get prompt request failed: %v", err)
	}

	// Check for error response
	if mcp.IsErrorResponse(rawResp) {
		errResp, err := mcp.ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("get prompt error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return mcp.ParseGetPromptResultFromJSON(rawResp)
}

// ListResources lists available resources.
func (c *Client) ListResources(ctx context.Context) (*mcp.ListResourcesResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := mcp.NewJSONRPCRequest("resources-list", mcp.MethodResourcesList, nil)

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %v", err)
		}

		// Type assert.
		if resourcesResult, ok := result.(*mcp.ListResourcesResult); ok {
			return resourcesResult, nil
		}
		return nil, fmt.Errorf("failed to parse list resources result: type assertion error")
	}

	// Fallback to old method
	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list resources request failed: %v", err)
	}

	// Check for error response
	if mcp.IsErrorResponse(rawResp) {
		errResp, err := mcp.ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list resources error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return mcp.ParseListResourcesResultFromJSON(rawResp)
}

// ReadResource reads a specific resource.
func (c *Client) ReadResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := mcp.NewJSONRPCRequest("resource-read", mcp.MethodResourcesRead, map[string]interface{}{
		"uri": uri,
	})

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to read resource: %v", err)
		}

		// Type assert.
		if resourceContent, ok := result.(*mcp.ReadResourceResult); ok {
			return resourceContent, nil
		}
		return nil, fmt.Errorf("failed to parse read resource result: type assertion error")
	}

	// Fallback to old method
	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("read resource request failed: %v", err)
	}

	// Check for error response
	if mcp.IsErrorResponse(rawResp) {
		errResp, err := mcp.ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("read resource error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return mcp.ParseReadResourceResultFromJSON(rawResp)
}
