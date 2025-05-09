package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/transport"
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
	clientInfo schema.Implementation

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
func NewClient(serverURL string, clientInfo schema.Implementation, options ...ClientOption) (*Client, error) {
	// Parse the server URL.
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %v", err)
	}

	// Create client
	client := &Client{
		clientInfo:      clientInfo,
		protocolVersion: schema.ProtocolVersion_2024_11_05, // Default compatible version
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
func (c *Client) Initialize(ctx context.Context) (*schema.InitializeResult, error) {
	// Create an initialization request.
	req := schema.NewInitializeRequest(
		c.protocolVersion,
		c.clientInfo,
		schema.ClientCapabilities{
			Roots: &schema.RootsCapability{
				ListChanged: true,
			},
			Sampling: &schema.SamplingCapability{},
		},
	)

	// Use SendRequestAndParse to send request and parse response
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("initialization request failed: %v", err)
		}

		// Type assert to InitializeResult.
		initResult, ok := result.(*schema.InitializeResult)
		if !ok {
			return nil, fmt.Errorf("failed to parse initialization response: type assertion error")
		}

		// Send initialized notification.
		if err := c.SendInitialized(ctx); err != nil {
			return nil, fmt.Errorf("failed to send initialized notification: %v", err)
		}

		// Ensure session ID is set, which is crucial for GET SSE connection establishment.
		// Get session ID - should already be set by the transport layer after initialization response.
		sessionID := httpTransport.GetSessionID()

		// Check if stateless mode is enabled.
		isStateless := httpTransport.IsStatelessMode()

		if sessionID == "" && !isStateless {
			// Only output warning in non-stateless mode.
			fmt.Println("warning: session ID is empty, GET SSE connection may not be established")
		} else if sessionID != "" {
			// Explicitly trigger GET SSE connection attempt
			// SetSessionID will automatically try to establish a GET SSE connection (if enabled)
			httpTransport.SetSessionID(sessionID)
		}

		c.initialized = true
		return initResult, nil
	}

	// Fallback to old method (for cases that don't support StreamableHTTPClientTransport)
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("initialization request failed: %v", err)
	}

	// Check for errors.
	if resp.Error != nil {
		return nil, fmt.Errorf("initialization error: %s", resp.Error.Message)
	}

	// Parse response.
	initResult, err := schema.ParseInitializeResult(resp.Result)
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
	notification := schema.NewInitializedNotification()
	return c.transport.SendNotification(ctx, notification)
}

// ListTools lists available tools.
func (c *Client) ListTools(ctx context.Context) ([]*schema.Tool, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := schema.NewRequest("tools-list", schema.MethodToolsList, nil)

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list tools: %v", err)
		}

		// Type assert.
		if toolsResult, ok := result.(*schema.ListToolsResult); ok {
			return toolsResult.Tools, nil
		}
		return nil, fmt.Errorf("failed to parse list tools result: type assertion error")
	}

	// Fallback to old method
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list tools request failed: %v", err)
	}

	// Check for errors.
	if resp.Error != nil {
		return nil, fmt.Errorf("list tools error: %s", resp.Error.Message)
	}

	// Use dedicated parsing function to handle response.
	toolsResult, err := schema.ParseListToolsResult(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse list tools result: %v", err)
	}

	return toolsResult.Tools, nil
}

// CallTool calls a tool.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) ([]schema.ToolContent, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request
	req := schema.NewRequest("tool-call", schema.MethodToolsCall, map[string]interface{}{
		"name":      name,
		"arguments": args,
	})

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("tool call failed: %v", err)
		}

		// Type assert.
		if toolResult, ok := result.(*schema.ToolResult); ok {
			return toolResult.Content, nil
		}
		return nil, fmt.Errorf("failed to parse tool call result: type assertion error")
	}

	// Fallback to old method
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tool call request failed: %v", err)
	}

	// Check for errors.
	if resp.Error != nil {
		return nil, fmt.Errorf("tool call error: %s", resp.Error.Message)
	}

	// Use dedicated parsing function to handle response.
	toolResult, err := schema.ParseCallToolResult(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tool call result: %v", err)
	}

	return toolResult.Content, nil
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
func (c *Client) CallToolWithStream(ctx context.Context, name string, args map[string]interface{}, streamOpts *transport.StreamOptions) ([]schema.ToolContent, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := schema.NewRequest("tool-call", schema.MethodToolsCall, map[string]interface{}{
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
			if toolResult, ok := result.(*schema.ToolResult); ok {
				return toolResult.Content, nil
			}
			return nil, fmt.Errorf("failed to parse tool call result: type assertion error")
		}

		// Use streaming request if streaming options are provided
		resp, err := httpTransport.SendRequestWithStream(ctx, req, streamOpts)
		if err != nil {
			return nil, fmt.Errorf("tool call request failed: %v", err)
		}

		// Check for errors.
		if resp.Error != nil {
			return nil, fmt.Errorf("tool call error: %s", resp.Error.Message)
		}

		// Parse result.
		toolResult, err := schema.ParseCallToolResult(resp.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tool call result: %v", err)
		}

		return toolResult.Content, nil
	}

	// Fallback to regular request
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tool call request failed: %v", err)
	}

	// Check for errors.
	if resp.Error != nil {
		return nil, fmt.Errorf("tool call error: %s", resp.Error.Message)
	}

	// Use dedicated parsing function to handle response.
	toolResult, err := schema.ParseCallToolResult(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tool call result: %v", err)
	}

	return toolResult.Content, nil
}

// ListPrompts lists available prompts.
func (c *Client) ListPrompts(ctx context.Context) ([]schema.Prompt, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := schema.NewRequest("prompts-list", schema.MethodPromptsList, nil)

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list prompts: %v", err)
		}

		// Type assert.
		if promptsResult, ok := result.(*schema.ListPromptsResponse); ok {
			return promptsResult.Prompts, nil
		}
		return nil, fmt.Errorf("failed to parse list prompts result: type assertion error")
	}

	// Fallback to old method
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list prompts request failed: %v", err)
	}

	// Check for errors.
	if resp.Error != nil {
		return nil, fmt.Errorf("list prompts error: %s", resp.Error.Message)
	}

	// Use dedicated parsing function to handle response.
	promptsResult, err := schema.ParseListPromptsResult(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse list prompts result: %v", err)
	}

	return promptsResult.Prompts, nil
}

// GetPrompt gets a specific prompt.
func (c *Client) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*schema.GetPromptResponse, error) {
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
	req := schema.NewRequest("prompt-get", schema.MethodPromptsGet, params)

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to get prompt: %v", err)
		}

		// Type assert.
		if promptResult, ok := result.(*schema.GetPromptResponse); ok {
			return promptResult, nil
		}
		return nil, fmt.Errorf("failed to parse get prompt result: type assertion error")
	}

	// Fallback to old method
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get prompt request failed: %v", err)
	}

	// Check for errors.
	if resp.Error != nil {
		return nil, fmt.Errorf("get prompt error: %s", resp.Error.Message)
	}

	// Use dedicated parsing function to handle response.
	promptResult, err := schema.ParseGetPromptResult(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse get prompt result: %v", err)
	}

	return promptResult, nil
}

// ListResources lists available resources.
func (c *Client) ListResources(ctx context.Context) ([]schema.Resource, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := schema.NewRequest("resources-list", schema.MethodResourcesList, nil)

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %v", err)
		}

		// Type assert.
		if resourcesResult, ok := result.(*schema.ListResourcesResponse); ok {
			return resourcesResult.Resources, nil
		}
		return nil, fmt.Errorf("failed to parse list resources result: type assertion error")
	}

	// Fallback to old method
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list resources request failed: %v", err)
	}

	// Check for errors.
	if resp.Error != nil {
		return nil, fmt.Errorf("list resources error: %s", resp.Error.Message)
	}

	// Use dedicated parsing function to handle response.
	resourcesResult, err := schema.ParseListResourcesResult(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse list resources result: %v", err)
	}

	return resourcesResult.Resources, nil
}

// ReadResource reads resource content.
func (c *Client) ReadResource(ctx context.Context, uri string) (*schema.ReadResourceResponse, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	// Create request.
	req := schema.NewRequest("resource-read", schema.MethodResourcesRead, map[string]interface{}{
		"uri": uri,
	})

	// Send request and parse response.
	if httpTransport, ok := c.transport.(*transport.StreamableHTTPClientTransport); ok {
		result, err := httpTransport.SendRequestAndParse(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to read resource: %v", err)
		}

		// Type assert.
		if resourceResult, ok := result.(*schema.ReadResourceResponse); ok {
			return resourceResult, nil
		}
		return nil, fmt.Errorf("failed to parse read resource result: type assertion error")
	}

	// Fallback to old method
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("read resource request failed: %v", err)
	}

	// Check for errors.
	if resp.Error != nil {
		return nil, fmt.Errorf("read resource error: %s", resp.Error.Message)
	}

	// Use dedicated parsing function to handle response.
	resourceResult, err := schema.ParseReadResourceResult(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse read resource result: %v", err)
	}

	return resourceResult, nil
}
