package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	transport       HTTPTransport          // Transport layer.
	clientInfo      Implementation         // Client information.
	protocolVersion string                 // Protocol version.
	initialized     bool                   // Whether the client is initialized.
	requestID       atomic.Int64           // Atomic counter for request IDs.
	capabilities    map[string]interface{} // Capabilities.
	state           State                  // State.

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
		clientInfo:      clientInfo,
		protocolVersion: ProtocolVersion_2025_03_26, // Default compatible version.
		capabilities:    make(map[string]interface{}),
		state:           StateDisconnected,
	}

	// Apply options.
	for _, option := range options {
		option(client)
	}

	// Create transport layer if not previously set via options.
	if client.transport == nil {
		// If logger is set, inject it into transport.
		if client.logger != nil {
			client.transport = NewStreamableHTTPClientTransport(parsedURL, WithClientTransportLogger(client.logger))
		} else {
			client.transport = NewStreamableHTTPClientTransport(parsedURL)
		}
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
func WithTransport(transport HTTPTransport) ClientOption {
	return func(c *Client) {
		c.transport = transport
	}
}

// WithClientLogger sets the logger for the client transport.
func WithClientLogger(logger Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithClientGetSSEEnabled sets whether to enable GET SSE.
func WithClientGetSSEEnabled(enabled bool) ClientOption {
	return func(c *Client) {
		if httpTransport, ok := c.transport.(*StreamableHTTPClientTransport); ok {
			// Use WithClientTransportGetSSEEnabled option.
			WithClientTransportGetSSEEnabled(enabled)(httpTransport)
		}
	}
}

// WithHTTPReqHandler sets a custom HTTP request handler for the client
func WithHTTPReqHandler(handler HTTPReqHandler) ClientOption {
	return func(c *Client) {
		if httpTransport, ok := c.transport.(*StreamableHTTPClientTransport); ok {
			WithTransportHTTPReqHandler(handler)(httpTransport)
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
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	// Check if already initialized.
	if c.initialized {
		return nil, errors.ErrAlreadyInitialized
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := NewJSONRPCRequest(requestID, MethodInitialize, map[string]interface{}{
		"protocolVersion": c.protocolVersion,
		"clientInfo":      c.clientInfo,
		"capabilities":    c.capabilities,
	})

	// Send request and wait for response
	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("initialization request failed: %w", err)
	}

	// Connection is established successfully at this point
	c.setState(StateConnected)

	// Check for error response
	if IsErrorResponse(rawResp) {
		errResp, err := ParseRawMessageToError(rawResp)
		if err != nil {
			c.setState(StateDisconnected)
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("initialization error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse the response using our specialized parser
	initResult, err := ParseInitializeResultFromJSON(rawResp)
	if err != nil {
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("failed to parse initialization response: %v", err)
	}

	// Send initialized notification.
	if err := c.SendInitialized(ctx); err != nil {
		c.setState(StateDisconnected)
		return nil, fmt.Errorf("failed to send initialized notification: %v", err)
	}

	// Update state and initialized flag
	c.initialized = true
	c.setState(StateInitialized)

	return initResult, nil
}

// SendInitialized sends an initialized notification.
func (c *Client) SendInitialized(ctx context.Context) error {
	notification := NewInitializedNotification()
	return c.transport.SendNotification(ctx, notification)
}

// ListTools lists available tools.
func (c *Client) ListTools(ctx context.Context) (*ListToolsResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, errors.ErrNotInitialized
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := NewJSONRPCRequest(requestID, MethodToolsList, nil)

	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list tools request failed: %v", err)
	}

	// Check for error response
	if IsErrorResponse(rawResp) {
		errResp, err := ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list tools error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return ParseListToolsResultFromJSON(rawResp)
}

// CallTool calls a tool.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, errors.ErrNotInitialized
	}

	// Create request
	requestID := c.requestID.Add(1)
	req := NewJSONRPCRequest(requestID, MethodToolsCall, map[string]interface{}{
		"name":      name,
		"arguments": args,
	})

	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tool call request failed: %w", err)
	}

	// Check for error response
	if IsErrorResponse(rawResp) {
		errResp, err := ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("tool call error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	return ParseCallToolResult(rawResp)
}

// Close closes the client connection and cleans up resources.
func (c *Client) Close() error {
	if c.transport != nil {
		err := c.transport.Close()
		c.setState(StateDisconnected)
		c.initialized = false
		return err
	}
	return nil
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
func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	if httpTransport, ok := c.transport.(*StreamableHTTPClientTransport); ok {
		httpTransport.RegisterNotificationHandler(method, handler)
	}
}

// UnregisterNotificationHandler unregisters a notification handler.
func (c *Client) UnregisterNotificationHandler(method string) {
	if httpTransport, ok := c.transport.(*StreamableHTTPClientTransport); ok {
		httpTransport.UnregisterNotificationHandler(method)
	}
}

// CallToolWithStream calls a tool with streaming support.
func (c *Client) CallToolWithStream(ctx context.Context, name string, args map[string]interface{}, streamOpts *StreamOptions) (*CallToolResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, errors.ErrNotInitialized
	}

	// Create request.
	req := NewJSONRPCRequest("tool-call", MethodToolsCall, map[string]interface{}{
		"name":      name,
		"arguments": args,
	})

	// Check if using streaming transport.
	if httpTransport, ok := c.transport.(*StreamableHTTPClientTransport); ok {
		// If no streaming options, try using unified parsing method.
		if streamOpts == nil {
			rawResp, err := c.transport.SendRequest(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("tool call request failed: %w", err)
			}

			// Check for error response
			if IsErrorResponse(rawResp) {
				errResp, err := ParseRawMessageToError(rawResp)
				if err != nil {
					return nil, fmt.Errorf("failed to parse error response: %w", err)
				}
				return nil, fmt.Errorf("tool call error: %s (code: %d)",
					errResp.Error.Message, errResp.Error.Code)
			}

			return ParseCallToolResult(rawResp)
		}

		// Use streaming request if streaming options are provided
		rawResp, err := httpTransport.SendRequestWithStream(ctx, req, streamOpts)
		if err != nil {
			return nil, fmt.Errorf("tool call request failed: %w", err)
		}

		// Check for error response
		if IsErrorResponse(rawResp) {
			errResp, err := ParseRawMessageToError(rawResp)
			if err != nil {
				return nil, fmt.Errorf("failed to parse error response: %w", err)
			}
			return nil, fmt.Errorf("tool call error: %s (code: %d)",
				errResp.Error.Message, errResp.Error.Code)
		}

		// Parse response as success response
		var resp struct {
			Result CallToolResult `json:"result"`
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
func (c *Client) ListPrompts(ctx context.Context) (*ListPromptsResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, errors.ErrNotInitialized
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := NewJSONRPCRequest(requestID, MethodPromptsList, nil)

	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list prompts request failed: %w", err)
	}

	// Check for error response
	if IsErrorResponse(rawResp) {
		errResp, err := ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list prompts error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return ParseListPromptsResultFromJSON(rawResp)
}

// GetPrompt gets a specific prompt.
func (c *Client) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*GetPromptResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, errors.ErrNotInitialized
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
	requestID := c.requestID.Add(1)
	req := NewJSONRPCRequest(requestID, MethodPromptsGet, params)

	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get prompt request failed: %v", err)
	}

	// Check for error response
	if IsErrorResponse(rawResp) {
		errResp, err := ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("get prompt error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return ParseGetPromptResultFromJSON(rawResp)
}

// ListResources lists available resources.
func (c *Client) ListResources(ctx context.Context) (*ListResourcesResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("%w", errors.ErrNotInitialized)
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := NewJSONRPCRequest(requestID, MethodResourcesList, nil)

	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list resources request failed: %v", err)
	}

	// Check for error response
	if IsErrorResponse(rawResp) {
		errResp, err := ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("list resources error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return ParseListResourcesResultFromJSON(rawResp)
}

// ReadResource reads a specific resource.
func (c *Client) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	// Check if initialized.
	if !c.initialized {
		return nil, fmt.Errorf("%w", errors.ErrNotInitialized)
	}

	// Create request.
	requestID := c.requestID.Add(1)
	req := NewJSONRPCRequest(requestID, MethodResourcesRead, map[string]interface{}{
		"uri": uri,
	})

	rawResp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("read resource request failed: %v", err)
	}

	// Check for error response
	if IsErrorResponse(rawResp) {
		errResp, err := ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %w", err)
		}
		return nil, fmt.Errorf("read resource error: %s (code: %d)",
			errResp.Error.Message, errResp.Error.Code)
	}

	// Parse response using specialized parser
	return ParseReadResourceResultFromJSON(rawResp)
}
