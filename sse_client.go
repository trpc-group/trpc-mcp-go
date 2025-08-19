// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// sseClientTransport implements SSE-based MCP transport following the 2024-11-05 spec.
// This transport allows compatibility with MCP servers that implement the older SSE protocol.
type sseClientTransport struct {
	baseURL        *url.URL                         // Server URL for SSE connection.
	endpoint       *url.URL                         // Message endpoint URL (provided by the server).
	httpClient     *http.Client                     // HTTP client.
	httpReqHandler HTTPReqHandler                   // HTTP request handler.
	httpHeaders    http.Header                      // Custom HTTP headers to be added to all requests.
	responses      map[string]chan *json.RawMessage // Map of response channels for pending requests.
	responsesMu    sync.RWMutex                     // Mutex for responses map.

	sseConn struct {
		active bool               // Flag indicating if connection is active.
		ctx    context.Context    // Context for the SSE connection.
		cancel context.CancelFunc // Function to cancel the SSE connection.
		mutex  sync.Mutex         // Mutex for synchronizing connection operations.
	}

	onNotification func(notification *JSONRPCNotification) // Notification handler.
	notificationMu sync.RWMutex                            // Mutex for notification handler.

	started      atomic.Bool   // Flag indicating if transport is started.
	closed       atomic.Bool   // Flag indicating if transport is closed.
	endpointChan chan struct{} // Channel to signal when endpoint is received.
	logger       Logger        // Logger for this client transport.

	// Fields for HTTP request handler configuration
	serviceName           string                 // Service name for custom HTTP request handlers.
	httpReqHandlerOptions []HTTPReqHandlerOption // HTTP request handler options for extensibility.

	// Client reference for accessing rootsProvider.
	client *Client // Reference to the parent client
}

// NewSSEClient creates a new client using the SSE transport from the 2024-11-05 spec.
func NewSSEClient(serverURL string, clientInfo Implementation, options ...ClientOption) (*Client, error) {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// extract transport configuration.
	config := extractTransportConfig(options)
	config.serverURL = parsedURL

	// Create client options with standard options.
	clientOptions := []ClientOption{
		WithCustomTransport(func(c *Client) {
			// create SSE transport.
			sseTransport := &sseClientTransport{
				baseURL:               parsedURL,
				httpClient:            config.httpClient,
				httpHeaders:           config.httpHeaders,
				responses:             make(map[string]chan *json.RawMessage),
				endpointChan:          make(chan struct{}),
				logger:                config.logger,
				serviceName:           config.serviceName,
				httpReqHandlerOptions: config.httpReqHandlerOptions,
			}

			// Set HTTP request handler from config if provided, otherwise create default.
			if config.httpReqHandler != nil {
				sseTransport.httpReqHandler = config.httpReqHandler
			} else if sseTransport.httpReqHandler == nil {
				sseTransport.httpReqHandler = NewHTTPReqHandler(
					sseTransport.serviceName, sseTransport.httpReqHandlerOptions...)
			}

			c.transport = sseTransport

			// Set client reference in transport for roots handling.
			sseTransport.client = c
		}),
		WithProtocolVersion(ProtocolVersion_2024_11_05), // Use the 2024-11-05 protocol version.
	}

	// Append user-provided options
	clientOptions = append(clientOptions, options...)

	// Create and return the client
	c, err := NewClient(serverURL, clientInfo, clientOptions...)
	if err != nil {
		return nil, err
	}

	err = c.transport.start(context.Background())
	if err != nil {
		return nil, err
	}

	return c, nil
}

// WithCustomTransport allows setting a custom transport implementation.
func WithCustomTransport(transportSetter func(*Client)) ClientOption {
	return func(c *Client) {
		transportSetter(c)
	}
}

// Start establishes the SSE connection to the server and waits for the endpoint URL.
func (t *sseClientTransport) start(ctx context.Context) error {
	if t.closed.Load() {
		return errors.New("transport is closed")
	}

	if t.started.Load() {
		return nil // Already started.
	}

	// Create a new context with cancellation for the SSE stream.
	sseCtx, cancel := context.WithCancel(context.Background())
	t.sseConn.mutex.Lock()
	t.sseConn.ctx = sseCtx
	t.sseConn.cancel = cancel
	t.sseConn.mutex.Unlock()

	// Create request to establish SSE connection
	req, err := http.NewRequestWithContext(sseCtx, http.MethodGet, t.baseURL.String(), nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHTTPRequestCreation, err)
	}

	// Set headers for SSE connection
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("X-MCP-Version", ProtocolVersion_2024_11_05) // Explicitly specify protocol version.

	// Add custom headers.
	for key, values := range t.httpHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Send the request.
	resp, err := t.httpReqHandler.Handle(sseCtx, t.httpClient, req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHTTPRequestFailed, err)
	}

	// Check status code.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Check content type.
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		resp.Body.Close()
		return fmt.Errorf("%w: expected text/event-stream, got %s", ErrInvalidContentType, contentType)
	}

	t.sseConn.mutex.Lock()
	t.sseConn.active = true
	t.sseConn.mutex.Unlock()

	// Start reading SSE events.
	go t.readSSE(resp.Body)

	// Wait for the endpoint to be received.
	select {
	case <-t.endpointChan:
		// Endpoint received, proceed.
		t.started.Store(true)
		return nil
	case <-ctx.Done():
		t.close()
		return fmt.Errorf("context cancelled while waiting for endpoint: %w", ctx.Err())
	case <-time.After(60 * time.Second): // Add a timeout.
		t.close()
		return fmt.Errorf("timeout waiting for endpoint")
	}
}

// readSSE continuously reads the SSE stream and processes events.
func (t *sseClientTransport) readSSE(body io.ReadCloser) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var eventType, eventData string

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line signals the end of an event.
		if line == "" {
			if eventType != "" && eventData != "" {
				t.handleEvent(eventType, eventData)
				eventType, eventData = "", ""
			}
			continue
		}

		// Parse the SSE line.
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "data:") {
			eventData = strings.TrimSpace(line[5:])
		}
	}

	if err := scanner.Err(); err != nil {
		if t.logger != nil {
			t.logger.Errorf("Error reading SSE stream: %v", err)
		}
	}

	// Connection closed, clean up.
	t.close()
}

// handleEvent processes SSE events based on their type.
func (t *sseClientTransport) handleEvent(eventType, eventData string) {
	switch eventType {
	case "endpoint":
		t.handleEndpointEvent(eventData)
	case "message":
		t.handleMessageEvent(eventData)
	default:
		if t.logger != nil {
			t.logger.Debugf("Received unknown event type: %s, data: %s", eventType, eventData)
		}
	}
}

// handleEndpointEvent processes the endpoint event from the server.
func (t *sseClientTransport) handleEndpointEvent(endpointURL string) {
	parsedURL, err := url.Parse(endpointURL)
	if err != nil {
		if t.logger != nil {
			t.logger.Errorf("Invalid endpoint URL: %v", err)
		}
		return
	}

	// If the URL is relative, resolve it against the base URL.
	if !parsedURL.IsAbs() {
		parsedURL = t.baseURL.ResolveReference(parsedURL)
	}

	t.endpoint = parsedURL
	close(t.endpointChan) // Signal that the endpoint has been received.
}

// handleMessageEvent processes message events from the server.
func (t *sseClientTransport) handleMessageEvent(data string) {
	var message map[string]interface{}
	if err := json.Unmarshal([]byte(data), &message); err != nil {
		if t.logger != nil {
			t.logger.Errorf("Error parsing message event: %v", err)
		}
		return
	}

	// Check if the message is a request, response, or notification.
	hasID := false
	hasMethod := false
	if _, ok := message["id"]; ok {
		hasID = true
	}
	if _, ok := message["method"]; ok {
		hasMethod = true
	}

	if hasID && hasMethod {
		// This is a request from the server to the client.
		t.handleIncomingRequest(data)
	} else if hasID {
		// This is a response to a previous request.
		t.handleResponse(data)
	} else if hasMethod {
		// This is a notification.
		t.handleNotification(data)
	} else {
		if t.logger != nil {
			t.logger.Errorf("Received invalid message: %s", data)
		}
	}
}

// handleResponse processes response messages.
func (t *sseClientTransport) handleResponse(data string) {
	var response struct {
		ID interface{} `json:"id"`
	}

	if err := json.Unmarshal([]byte(data), &response); err != nil {
		if t.logger != nil {
			t.logger.Errorf("Error parsing response: %v", err)
		}
		return
	}

	// Get the response ID as a string.
	idStr := fmt.Sprintf("%v", response.ID)

	// Find the corresponding response channel.
	t.responsesMu.RLock()
	responseChan, ok := t.responses[idStr]
	t.responsesMu.RUnlock()

	if !ok {
		if t.logger != nil {
			t.logger.Debugf("Received response for unknown request ID: %s", idStr)
		}
		return
	}

	// Parse the raw message.
	rawMsg := json.RawMessage(data)

	// Send the response on the channel.
	select {
	case responseChan <- &rawMsg:
		// Response sent successfully.
	default:
		if t.logger != nil {
			t.logger.Errorf("Response channel for ID %s is full or closed", idStr)
		}
	}
}

// handleNotification processes notification messages.
func (t *sseClientTransport) handleNotification(data string) {
	var notification JSONRPCNotification
	if err := json.Unmarshal([]byte(data), &notification); err != nil {
		if t.logger != nil {
			t.logger.Errorf("Error parsing notification: %v", err)
		}
		return
	}

	t.notificationMu.RLock()
	handler := t.onNotification
	t.notificationMu.RUnlock()

	if handler != nil {
		handler(&notification)
	}
}

// handleIncomingRequest processes request messages from the server.
func (t *sseClientTransport) handleIncomingRequest(data string) {
	var request JSONRPCRequest
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		if t.logger != nil {
			t.logger.Errorf("Error parsing incoming request: %v", err)
		}
		return
	}

	// Handle different types of requests.
	switch request.Method {
	case MethodRootsList:
		t.handleRootsListRequest(&request)
	default:
		// Send method not found error.
		t.sendErrorResponse(&request, ErrCodeMethodNotFound, fmt.Sprintf("Method not found: %s", request.Method))
	}
}

// handleRootsListRequest handles roots/list requests from the server.
func (t *sseClientTransport) handleRootsListRequest(request *JSONRPCRequest) {
	// Get roots from the client if it has a reference.
	var roots []Root
	if t.client != nil {
		t.client.rootsMu.RLock()
		provider := t.client.rootsProvider
		t.client.rootsMu.RUnlock()

		if provider != nil {
			roots = provider.GetRoots()
		}
	}

	if roots == nil {
		roots = []Root{}
	}

	result := &ListRootsResult{
		Roots: roots,
	}

	response := &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  result,
	}

	// Send response.
	t.sendResponseMessage(response)
}

// sendErrorResponse sends an error response to the server.
func (t *sseClientTransport) sendErrorResponse(request *JSONRPCRequest, code int, message string) {
	errorResp := newJSONRPCErrorResponse(request.ID, code, message, nil)
	t.sendResponseMessage(errorResp)
}

// sendResponseMessage sends a response back to the server.
func (t *sseClientTransport) sendResponseMessage(response interface{}) {
	if t.endpoint == nil {
		if t.logger != nil {
			t.logger.Errorf("Cannot send response: endpoint not available")
		}
		return
	}

	respBytes, err := json.Marshal(response)
	if err != nil {
		if t.logger != nil {
			t.logger.Errorf("Error marshaling response: %v", err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint.String(), bytes.NewReader(respBytes))
	if err != nil {
		if t.logger != nil {
			t.logger.Errorf("Error creating HTTP request for response: %v", err)
		}
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Add custom headers.
	for key, values := range t.httpHeaders {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	var resp *http.Response
	resp, err = t.httpReqHandler.Handle(ctx, t.httpClient, httpReq) // Always use httpReqHandler consistently.

	if err != nil {
		t.logger.Errorf("Error sending response to server: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted { // According to MCP protocol, response should be 202 Accepted.
		// Read response body for error details.
		bodyBytes, bodyErr := io.ReadAll(resp.Body)
		if bodyErr != nil {
			t.logger.Errorf("Unexpected response status when sending response: %d, failed to read body: %v", resp.StatusCode, bodyErr)
		} else {
			t.logger.Errorf("Unexpected response status when sending response: %d, body: %s", resp.StatusCode, string(bodyBytes))
		}
	}
}

// sendRequest sends a request and waits for a response.
func (t *sseClientTransport) sendRequest(ctx context.Context, req *JSONRPCRequest) (*json.RawMessage, error) {
	// Auto-start the transport if not already started.
	if !t.started.Load() {
		return nil, errors.New("transport not started")
	}

	if t.closed.Load() {
		return nil, errors.New("transport is closed")
	}

	if t.endpoint == nil {
		return nil, errors.New("endpoint URL not received")
	}

	// Marshal the request.
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestSerialization, err)
	}

	// Create a response channel.
	idStr := fmt.Sprintf("%v", req.ID)
	responseChan := make(chan *json.RawMessage, 1)

	// Register the response channel.
	t.responsesMu.Lock()
	t.responses[idStr] = responseChan
	t.responsesMu.Unlock()

	// Ensure we clean up the response channel when done.
	defer func() {
		t.responsesMu.Lock()
		delete(t.responses, idStr)
		t.responsesMu.Unlock()
	}()

	// Send the HTTP request.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint.String(), bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequestCreation, err)
	}

	// Set content type.
	httpReq.Header.Set("Content-Type", "application/json")

	// Add custom headers.
	for key, values := range t.httpHeaders {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Send the request.
	resp, err := t.httpReqHandler.Handle(ctx, t.httpClient, httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequestFailed, err)
	}
	defer resp.Body.Close()

	// Check response status.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status code %d, body: %s", ErrHTTPRequestFailed, resp.StatusCode, string(bodyBytes))
	}

	// In the SSE transport, the response should come via the SSE stream.
	// So here we just wait for the response on the channel.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case rawMsg, ok := <-responseChan:
		if !ok {
			return nil, errors.New("response channel closed")
		}
		// Parse the response as a JSON-RPC response.
		var jsonResp map[string]interface{}
		if err := json.Unmarshal(*rawMsg, &jsonResp); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrResponseParsing, err)
		}

		// Check if this is an error response.
		if _, hasError := jsonResp["error"]; hasError {
			// Return the raw error response for error handling.
			rawMessage := json.RawMessage(*rawMsg)
			return &rawMessage, nil
		}

		// Extract result part.
		resultData, ok := jsonResp["result"]
		if !ok {
			return nil, ErrMissingResultField
		}

		// Serialize result to JSON.
		resultBytes, err := json.Marshal(resultData)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrResponseSerialization, err)
		}

		rawMessage := json.RawMessage(resultBytes)
		return &rawMessage, nil
	}
}

// sendNotification sends a notification without expecting a response.
func (t *sseClientTransport) sendNotification(ctx context.Context, notification *JSONRPCNotification) error {
	// Auto-start the transport if not already started.
	if !t.started.Load() {
		return errors.New("transport not started")
	}

	if t.closed.Load() {
		return errors.New("transport is closed")
	}

	if t.endpoint == nil {
		return errors.New("endpoint URL not received")
	}

	// Marshal the notification.
	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotificationSerialization, err)
	}

	// Create HTTP request.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint.String(), bytes.NewReader(notificationBytes))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHTTPRequestCreation, err)
	}

	// Set content type.
	httpReq.Header.Set("Content-Type", "application/json")

	// Add custom headers.
	for key, values := range t.httpHeaders {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Send the request.
	resp, err := t.httpReqHandler.Handle(ctx, t.httpClient, httpReq)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHTTPRequestFailed, err)
	}
	defer resp.Body.Close()

	// Check response status (for notifications, servers typically return 202 Accepted)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: status code %d, body: %s", ErrHTTPRequestFailed, resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// sendResponse sends a response (client doesn't send responses in standard case).
func (t *sseClientTransport) sendResponse(ctx context.Context, resp *JSONRPCResponse) error {
	// In typical client usage, this shouldn't be called
	return errors.New("SSE client transport doesn't support sending responses")
}

// close closes the transport.
func (t *sseClientTransport) close() error {
	if !t.closed.CompareAndSwap(false, true) {
		return nil // Already closed.
	}

	// Cancel the SSE connection if active.
	t.sseConn.mutex.Lock()
	if t.sseConn.cancel != nil {
		t.sseConn.cancel()
		t.sseConn.active = false
	}
	t.sseConn.mutex.Unlock()

	// Close all response channels.
	t.responsesMu.Lock()
	for _, ch := range t.responses {
		close(ch)
	}
	t.responses = make(map[string]chan *json.RawMessage)
	t.responsesMu.Unlock()

	return nil
}

// getSessionID returns the session ID (not applicable for SSE transport).
func (t *sseClientTransport) getSessionID() string {
	return "" // SSE transport doesn't use session IDs in the same way as streamable-http.
}

// setSessionID sets the session ID (not applicable for SSE transport).
func (t *sseClientTransport) setSessionID(sessionID string) {
	// No-op for SSE transport.
}

// terminateSession terminates the session (not applicable for SSE transport).
func (t *sseClientTransport) terminateSession(ctx context.Context) error {
	// SSE transport doesn't have explicit session termination.
	return nil
}
