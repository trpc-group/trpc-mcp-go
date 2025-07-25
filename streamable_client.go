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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/httputil"
)

// streamableHTTPClientTransport implements an HTTP-based MCP transport
type streamableHTTPClientTransport struct {
	// Server URL
	serverURL *url.URL

	path string

	// HTTP client
	httpClient *http.Client

	// HTTP request handler
	httpReqHandler HTTPReqHandler

	// Custom HTTP headers to be added to all requests
	httpHeaders http.Header

	// Session ID
	sessionID string

	// Notification handlers
	notificationHandlers map[string]NotificationHandler

	// Last event ID (for connection recovery)
	lastEventID string

	// Whether GET SSE is enabled
	enableGetSSE bool

	// GET SSE connection
	getSSEConn struct {
		active bool
		ctx    context.Context
		cancel context.CancelFunc
		mutex  sync.Mutex
	}

	// Notification handlers mutex
	handlersMutex sync.RWMutex

	// Whether in stateless mode
	// In stateless mode, the client will not send a session ID and will not attempt to establish a GET SSE connection.
	// This field is set by auto-detection when no session ID is provided in the initialize response.
	isStateless bool

	// Logger for this client transport.
	logger Logger

	// Service name for custom HTTP request handlers.
	// This field is typically not used by the default handler, but may be used by custom
	// implementations that replace the default NewHTTPReqHandler function.
	serviceName string

	// HTTP request handler options.
	// These options are typically not used by the default handler, but may be used by custom
	// implementations that replace the default NewHTTPReqHandler function for extensibility.
	httpReqHandlerOptions []HTTPReqHandlerOption

	// Client reference for accessing rootsProvider.
	client *Client
}

// NotificationHandler is a handler for notifications.
type NotificationHandler func(notification *JSONRPCNotification) error

// streamOptions represents streaming options
type streamOptions struct {
	// Event ID (for stream recovery)
	lastEventID string

	// Notification handlers
	notificationHandlers map[string]NotificationHandler
}

// newStreamableHTTPClientTransport creates a new client transport.

func newStreamableHTTPClientTransport(config *transportConfig, options ...transportOption) *streamableHTTPClientTransport {
	// use config to create transport.
	transport := &streamableHTTPClientTransport{
		serverURL:             config.serverURL,
		httpClient:            config.httpClient,
		httpHeaders:           config.httpHeaders,
		notificationHandlers:  make(map[string]NotificationHandler),
		enableGetSSE:          config.enableGetSSE,
		logger:                config.logger,
		serviceName:           config.serviceName,
		httpReqHandlerOptions: config.httpReqHandlerOptions,
		path:                  config.path,
	}

	// apply extra options.
	for _, option := range options {
		option(transport)
	}

	// create HTTP request handler only if not already set.
	if transport.httpReqHandler == nil {
		transport.httpReqHandler = NewHTTPReqHandler(
			transport.serviceName, transport.httpReqHandlerOptions...)
	}

	return transport
}

// transportOption transport option function
type transportOption func(*streamableHTTPClientTransport)

// withClientTransportGetSSEEnabled sets whether GET SSE is enabled
func withClientTransportGetSSEEnabled(enabled bool) transportOption {
	return func(t *streamableHTTPClientTransport) {
		t.enableGetSSE = enabled
	}
}

// withClientTransportLogger sets the logger for the client transport.
func withClientTransportLogger(logger Logger) transportOption {
	return func(t *streamableHTTPClientTransport) {
		t.logger = logger
	}
}

// withClientTransportPath sets the path for the client transport.
func withClientTransportPath(path string) transportOption {
	return func(t *streamableHTTPClientTransport) {
		t.path = path
	}
}

// withTransportHTTPReqHandler sets the HTTP request handler
func withTransportHTTPReqHandler(handler HTTPReqHandler) transportOption {
	return func(t *streamableHTTPClientTransport) {
		if handler != nil {
			t.httpReqHandler = handler
		}
	}
}

// withTransportHTTPHeaders sets custom HTTP headers for all requests
func withTransportHTTPHeaders(headers http.Header) transportOption {
	return func(t *streamableHTTPClientTransport) {
		if t.httpHeaders == nil {
			t.httpHeaders = make(http.Header)
		}
		for k, v := range headers {
			t.httpHeaders[k] = v
		}
	}
}

// withTransportServiceName sets the service name for custom HTTP request handlers.
// This is typically only needed when using custom implementations of HTTPReqHandler.
func withTransportServiceName(serviceName string) transportOption {
	return func(t *streamableHTTPClientTransport) {
		t.serviceName = serviceName
	}
}

// withTransportHTTPReqHandlerOption adds an option for HTTP request handler.
// This is typically only needed when using custom implementations of HTTPReqHandler
// that support additional configuration options.
func withTransportHTTPReqHandlerOption(option HTTPReqHandlerOption) transportOption {
	return func(t *streamableHTTPClientTransport) {
		t.httpReqHandlerOptions = append(t.httpReqHandlerOptions, option)
	}
}

// start is a no-op for streamableHTTPClientTransport.
func (t *streamableHTTPClientTransport) start(ctx context.Context) error {
	return nil
}

// SendRequest sends a request and waits for a response
func (t *streamableHTTPClientTransport) sendRequest(
	ctx context.Context,
	req *JSONRPCRequest,
) (*json.RawMessage, error) {
	return t.send(ctx, req, nil)
}

// send sends a request and handles the response
func (t *streamableHTTPClientTransport) send(
	ctx context.Context,
	req *JSONRPCRequest,
	options *streamOptions,
) (*json.RawMessage, error) {
	// Serialize request to JSON
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestSerialization, err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.serverURL.String(), bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequestCreation, err)
	}
	if len(t.path) != 0 {
		httpReq.URL.Path = t.path
	}

	// Set request headers - accept both SSE and JSON responses
	httpReq.Header.Set(httputil.ContentTypeHeader, httputil.ContentTypeJSON)
	httpReq.Header.Set(httputil.AcceptHeader, httputil.ContentTypeJSON+", "+httputil.ContentTypeSSE)
	if t.sessionID != "" && !t.isStateless {
		httpReq.Header.Set(httputil.SessionIDHeader, t.sessionID)
	}

	// If lastEventID is provided, attach it to the request
	if options != nil && options.lastEventID != "" {
		httpReq.Header.Set(httputil.LastEventIDHeader, options.lastEventID)
	} else if t.lastEventID != "" {
		httpReq.Header.Set(httputil.LastEventIDHeader, t.lastEventID)
	}

	// Add custom headers
	for key, values := range t.httpHeaders {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Send request using the handler
	httpResp, err := t.httpReqHandler.Handle(ctx, t.httpClient, httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequestFailed, err)
	}

	// Handle session ID
	if sessionID := httpResp.Header.Get(httputil.SessionIDHeader); sessionID != "" {
		t.setSessionID(sessionID)
		t.isStateless = false
	} else if req.Method == MethodInitialize && !t.isStateless {
		// If this is an initialize request and no session ID was received, auto-detect as stateless mode
		t.isStateless = true
		t.enableGetSSE = false // Disable GET SSE in stateless mode
	}

	// Check content type
	contentType := httpResp.Header.Get(httputil.ContentTypeHeader)
	if strings.Contains(contentType, httputil.ContentTypeSSE) {
		// Handle response as SSE
		return t.handleSSEResponse(ctx, httpResp, req.ID, options)
	}

	// If not SSE, handle as JSON
	defer httpResp.Body.Close()

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status code %d", ErrHTTPRequestFailed, httpResp.StatusCode)
	}

	// Read response body
	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the response as a JSON-RPC response
	var jsonResp map[string]interface{}
	if err := json.Unmarshal(respBytes, &jsonResp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseParsing, err)
	}

	// Check if this is an error response
	if _, hasError := jsonResp["error"]; hasError {
		// Return the raw error response for error handling
		rawMessage := json.RawMessage(respBytes)
		return &rawMessage, nil
	}

	// Extract result part
	resultData, ok := jsonResp["result"]
	if !ok {
		return nil, ErrMissingResultField
	}

	// Serialize result to JSON
	resultBytes, err := json.Marshal(resultData)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseSerialization, err)
	}

	rawMessage := json.RawMessage(resultBytes)
	return &rawMessage, nil
}

// processEventData processes SSE event data and returns the processed message
func (t *streamableHTTPClientTransport) processEventData(
	data string,
	reqID interface{},
	handlers map[string]NotificationHandler,
) (*json.RawMessage, error) {
	// Create a raw message from the data
	rawMessage := json.RawMessage(data)

	// First, check if it's a response to our request by looking at the ID
	var jsonResp map[string]interface{}
	if err := json.Unmarshal(rawMessage, &jsonResp); err == nil {
		// Check if it has an ID that matches our request ID
		if id, hasID := jsonResp["id"]; hasID && fmt.Sprintf("%v", id) == fmt.Sprintf("%v", reqID) {
			return t.handleResponseMessage(jsonResp, &rawMessage)
		}
	}

	// Check if it's a notification
	return t.handleNotificationMessage(rawMessage, handlers)
}

// handleResponseMessage processes a response message and returns the result
func (t *streamableHTTPClientTransport) handleResponseMessage(
	jsonResp map[string]interface{},
	rawMessage *json.RawMessage,
) (*json.RawMessage, error) {
	// Check if it's an error response
	if _, hasError := jsonResp["error"]; hasError {
		t.logger.Infof("Received error response for ID: %v", jsonResp["id"])
		return rawMessage, nil
	}

	// Extract result from the response
	if result, hasResult := jsonResp["result"]; hasResult {
		// Serialize just the result part
		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %v", err)
		}

		resultRaw := json.RawMessage(resultBytes)
		return &resultRaw, nil
	}

	return nil, nil
}

// handleNotificationMessage processes a notification message
func (t *streamableHTTPClientTransport) handleNotificationMessage(
	rawMessage json.RawMessage,
	handlers map[string]NotificationHandler,
) (*json.RawMessage, error) {
	var notification JSONRPCNotification
	if err := json.Unmarshal(rawMessage, &notification); err != nil {
		return nil, err
	}

	if handler, ok := handlers[notification.Method]; ok {
		if err := handler(&notification); err != nil {
			t.logger.Debugf("Failed to handle notification: %s, error: %v",
				formatJSONRPCMessage(notification), err)
		}
	}
	return nil, nil
}

// Handle SSE response
func (t *streamableHTTPClientTransport) handleSSEResponse(
	ctx context.Context,
	httpResp *http.Response,
	reqID interface{},
	options *streamOptions,
) (*json.RawMessage, error) {
	reader := bufio.NewReader(httpResp.Body)
	var rawResult *json.RawMessage
	var resultReceived bool

	// Merge notification handlers
	handlers := make(map[string]NotificationHandler)

	// Add global handlers first
	t.handlersMutex.RLock()
	for method, handler := range t.notificationHandlers {
		handlers[method] = handler
	}
	t.handlersMutex.RUnlock()

	// Add request-specific handlers (if any)
	if options != nil && options.notificationHandlers != nil {
		for method, handler := range options.notificationHandlers {
			handlers[method] = handler
		}
	}

	for {
		select {
		case <-ctx.Done():
			return rawResult, ctx.Err()
		default:
			// Read SSE event
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if resultReceived {
						return rawResult, nil
					}
					return nil, fmt.Errorf("connection closed but no final response received")
				}
				return nil, fmt.Errorf("failed to read SSE event: %w", err)
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue // Empty line is event delimiter
			}

			// Process event ID
			if strings.HasPrefix(line, "id:") {
				t.lastEventID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
				continue
			}

			// Process event data
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				result, err := t.processEventData(data, reqID, handlers)
				if err != nil {
					return nil, err
				}
				if result != nil {
					rawResult = result
					resultReceived = true
					if len(handlers) == 0 {
						return rawResult, nil
					}
				}
			}
		}
	}
}

// registerNotificationHandler registers a notification handler
func (t *streamableHTTPClientTransport) registerNotificationHandler(method string, handler NotificationHandler) {
	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	t.notificationHandlers[method] = handler
}

// unregisterNotificationHandler unregisters a notification handler
func (t *streamableHTTPClientTransport) unregisterNotificationHandler(method string) {
	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	delete(t.notificationHandlers, method)
}

// sendNotification sends a notification (no response expected)
func (t *streamableHTTPClientTransport) sendNotification(ctx context.Context, notification *JSONRPCNotification) error {
	// Serialize notification to JSON
	notifBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to serialize notification: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.serverURL.String(), bytes.NewReader(notifBytes))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHTTPRequestCreation, err)
	}
	if len(t.path) != 0 {
		httpReq.URL.Path = t.path
	}

	// Set request headers - must accept both JSON and SSE responses per MCP specification.
	httpReq.Header.Set(httputil.ContentTypeHeader, httputil.ContentTypeJSON)
	httpReq.Header.Set(httputil.AcceptHeader, httputil.ContentTypeJSON+", "+httputil.ContentTypeSSE)
	if t.sessionID != "" {
		httpReq.Header.Set(httputil.SessionIDHeader, t.sessionID)
	}

	// Add custom headers
	for key, values := range t.httpHeaders {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Send HTTP request.
	var httpResp *http.Response
	if t.httpReqHandler != nil {
		httpResp, err = t.httpReqHandler.Handle(ctx, t.httpClient, httpReq) // Always use httpReqHandler as there's always a default value.
	} else {
		httpResp, err = t.httpClient.Do(httpReq)
	}

	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer httpResp.Body.Close()

	// Handle session ID
	if sessionID := httpResp.Header.Get(httputil.SessionIDHeader); sessionID != "" {
		t.sessionID = sessionID
	}

	// Check status code
	if httpResp.StatusCode != http.StatusAccepted {
		bodyBytes, bodyErr := io.ReadAll(httpResp.Body)
		if bodyErr != nil {
			t.logger.Warnf("Unexpected response status when sending response: %d, failed to read body: %v",
				httpResp.StatusCode, bodyErr)
		} else {
			t.logger.Warnf("Unexpected response status when sending response: %d, body: %s",
				httpResp.StatusCode, string(bodyBytes))
		}
	}

	return nil
}

// SendResponse sends a response (clients don't need to implement this method)
func (t *streamableHTTPClientTransport) sendResponse(ctx context.Context, resp *JSONRPCResponse) error {
	return fmt.Errorf("client transport does not support sending responses")
}

// Close closes the transport connection
func (t *streamableHTTPClientTransport) close() error {
	// close GET SSE connection
	t.getSSEConn.mutex.Lock()
	if t.getSSEConn.active && t.getSSEConn.cancel != nil {
		t.getSSEConn.cancel()
		t.getSSEConn.active = false
	}
	t.getSSEConn.mutex.Unlock()

	// Clear notification handlers
	t.handlersMutex.Lock()
	t.notificationHandlers = make(map[string]NotificationHandler)
	t.handlersMutex.Unlock()

	return nil
}

// GetSessionID gets the session ID
func (t *streamableHTTPClientTransport) getSessionID() string {
	return t.sessionID
}

// SetSessionID sets the session ID
func (t *streamableHTTPClientTransport) setSessionID(sessionID string) {
	t.sessionID = sessionID
}

// Establish GET SSE connection
func (t *streamableHTTPClientTransport) establishGetSSE() {
	// Get lock to ensure only one active connection
	t.getSSEConn.mutex.Lock()
	defer t.getSSEConn.mutex.Unlock()

	// If there's already an active connection, cancel the old one
	if t.getSSEConn.active && t.getSSEConn.cancel != nil {
		t.getSSEConn.cancel()
	}

	// Create new context
	ctx, cancel := context.WithCancel(context.Background())
	t.getSSEConn.ctx = ctx
	t.getSSEConn.cancel = cancel

	// Mark as active
	t.getSSEConn.active = true

	// Release lock and establish connection in a separate goroutine
	go func() {
		// Reset connection state when function exits
		defer func() {
			t.getSSEConn.mutex.Lock()
			t.getSSEConn.active = false
			t.getSSEConn.mutex.Unlock()
		}()

		// Try to establish GET SSE connection
		if err := t.connectGetSSE(ctx); err != nil {
			t.logger.Infof("GET SSE connection failed: %v", err)
		}
	}()
}

// Connect to GET SSE endpoint
func (t *streamableHTTPClientTransport) connectGetSSE(ctx context.Context) error {
	// Check if there's a session ID
	if t.sessionID == "" {
		return fmt.Errorf("cannot establish GET SSE connection: session ID is empty")
	}

	// Build GET request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.serverURL.String(), nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHTTPRequestCreation, err)
	}
	if len(t.path) != 0 {
		req.URL.Path = t.path
	}

	// Set necessary headers
	req.Header.Set(httputil.AcceptHeader, httputil.ContentTypeSSE)
	req.Header.Set(httputil.SessionIDHeader, t.sessionID)
	if t.lastEventID != "" {
		req.Header.Set(httputil.LastEventIDHeader, t.lastEventID)
	}

	// Add custom headers
	for key, values := range t.httpHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	t.logger.Debugf("Attempting to establish GET SSE connection, session ID: %s", t.sessionID)

	// Send request
	resp, err := t.httpReqHandler.Handle(ctx, t.httpClient, req)
	if err != nil {
		return fmt.Errorf("GET SSE connection request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// If server doesn't support GET SSE, this is acceptable
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.logger.Infof("Server does not support GET SSE, status code: %d", resp.StatusCode)
			return fmt.Errorf("server does not support GET SSE: %s", resp.Status)
		}
		return fmt.Errorf("GET SSE connection failed, status code: %d", resp.StatusCode)
	}

	// Handle response
	t.logger.Debugf("GET SSE connection established, session ID: %s", t.sessionID)

	// Handle SSE event stream
	return t.handleGetSSEEvents(ctx, resp.Body)
}

// Handle GET SSE event stream
func (t *streamableHTTPClientTransport) handleGetSSEEvents(ctx context.Context, body io.ReadCloser) error {
	scanner := bufio.NewScanner(body)
	var eventID, eventData string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Process SSE line
			line := scanner.Text()

			// Skip empty lines
			if line == "" {
				// Check if there's a complete event
				if eventData != "" {
					t.processSSEEvent(eventID, eventData)
					eventID, eventData = "", ""
				}
				continue
			}

			// Parse SSE fields
			if strings.HasPrefix(line, "id:") {
				eventID = strings.TrimPrefix(line, "id:")
				eventID = strings.TrimSpace(eventID)
				t.lastEventID = eventID
			} else if strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data:")
				data = strings.TrimSpace(data)
				eventData = data
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read SSE event stream: %w", err)
	}

	return nil
}

// Process SSE event.
func (t *streamableHTTPClientTransport) processSSEEvent(eventID, eventData string) {
	// Store the last event ID for connection recovery.
	t.lastEventID = eventID

	// Skip empty events.
	if eventData == "" {
		return
	}

	// Parse the event data as JSON.
	var rawMsg json.RawMessage
	if err := json.Unmarshal([]byte(eventData), &rawMsg); err != nil {
		t.logger.Errorf("Failed to parse SSE event data as JSON: %v", err)
		return
	}

	// Determine the message type.
	msgType, err := parseJSONRPCMessageType(rawMsg)
	if err != nil {
		t.logger.Errorf("Failed to determine JSON-RPC message type: %v", err)
		return
	}

	// Process the message based on its type.
	switch msgType {
	case JSONRPCMessageTypeNotification:
		// Parse notification.
		var notification JSONRPCNotification
		if err := json.Unmarshal(rawMsg, &notification); err != nil {
			t.logger.Errorf("Failed to parse notification: %v", err)
			return
		}

		// Get notification handlers.
		t.handlersMutex.RLock()
		handlers := make(map[string]NotificationHandler, len(t.notificationHandlers))
		for method, handler := range t.notificationHandlers {
			handlers[method] = handler
		}
		t.handlersMutex.RUnlock()

		// Handle notification.
		if handler, ok := handlers[notification.Method]; ok {
			if err := handler(&notification); err != nil {
				t.logger.Errorf("Failed to handle notification: %v", err)
			}
		} else {
			t.logger.Debugf("Received unhandled notification: %s", notification.Method)
		}
	case JSONRPCMessageTypeRequest:
		// Parse request
		var request JSONRPCRequest
		if err := json.Unmarshal(rawMsg, &request); err != nil {
			t.logger.Errorf("Failed to parse request: %v", err)
			return
		}

		// Handle server-to-client request.
		t.handleIncomingRequest(&request)
	default:
		t.logger.Debugf("Received unexpected message type via SSE: %s", msgType)
	}
}

// handleIncomingRequest handles JSON-RPC requests from the server.
func (t *streamableHTTPClientTransport) handleIncomingRequest(request *JSONRPCRequest) {
	// Handle different types of requests.
	switch request.Method {
	case MethodRootsList:
		t.handleRootsListRequest(request)
	default:
		// Send method not found error.
		t.sendErrorResponse(request, ErrCodeMethodNotFound, fmt.Sprintf("Method not found: %s", request.Method))
	}
}

// handleRootsListRequest handles roots/list requests from the server.
func (t *streamableHTTPClientTransport) handleRootsListRequest(request *JSONRPCRequest) {
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

	// Send response through HTTP POST.
	t.sendResponseToServer(response)
}

// sendErrorResponse sends an error response to the server.
func (t *streamableHTTPClientTransport) sendErrorResponse(request *JSONRPCRequest, code int, message string) {
	errorResp := newJSONRPCErrorResponse(request.ID, code, message, nil)
	t.sendResponseToServer(errorResp)
}

// sendResponseToServer sends a response back to the server via HTTP POST.
func (t *streamableHTTPClientTransport) sendResponseToServer(response interface{}) {
	respBytes, err := json.Marshal(response)
	if err != nil {
		t.logger.Errorf("Error marshaling response: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.serverURL.String(), bytes.NewReader(respBytes))
	if err != nil {
		t.logger.Errorf("Error creating HTTP request for response: %v", err)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, values := range t.httpHeaders {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Add session ID if available
	if t.sessionID != "" {
		httpReq.Header.Set(httputil.SessionIDHeader, t.sessionID) // Use correct MCP protocol header: Mcp-Session-Id.
	}

	var resp *http.Response
	resp, err = t.httpReqHandler.Handle(ctx, t.httpClient, httpReq) // Always use httpReqHandler as there's always a default value.

	if err != nil {
		t.logger.Errorf("Error sending response to server: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted { // According to MCP protocol, response should be 202 Accepted not 200 OK.
		// Read response body for error details.
		bodyBytes, bodyErr := io.ReadAll(resp.Body)
		if bodyErr != nil {
			t.logger.Errorf("Unexpected response status when sending response: %d, failed to read body: %v", resp.StatusCode, bodyErr)
		} else {
			t.logger.Errorf("Unexpected response status when sending response: %d, body: %s", resp.StatusCode, string(bodyBytes))
		}
	}
}

// terminateSession terminates the session
func (t *streamableHTTPClientTransport) terminateSession(ctx context.Context) error {
	// Create HTTP DELETE request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, t.serverURL.String(), nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHTTPRequestCreation, err)
	}
	if len(t.path) != 0 {
		httpReq.URL.Path = t.path
	}

	// Set session ID header
	if t.sessionID != "" {
		httpReq.Header.Set(httputil.SessionIDHeader, t.sessionID)
	} else {
		return fmt.Errorf("no active session")
	}

	// Add custom headers
	for key, values := range t.httpHeaders {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Send request
	httpResp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Check status code
	if httpResp.StatusCode == http.StatusMethodNotAllowed {
		return fmt.Errorf("server does not support client session termination")
	} else if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("session termination failed, status code: %d", httpResp.StatusCode)
	}

	// Session successfully terminated, clear session ID
	t.sessionID = ""

	return nil
}

// isStatelessMode returns whether the client is in stateless mode
//
// The client automatically detects if the server is in stateless mode: when no session ID
// is provided in the initialize response, the client automatically sets itself to stateless
// mode and disables GET SSE connections.
//
// If it returns true, the client is currently running in stateless mode and will not include
// a session ID in requests or attempt to establish GET SSE connections.
func (t *streamableHTTPClientTransport) isStatelessMode() bool {
	return t.isStateless
}

// sendRequestWithStream sends a request with streaming options
func (t *streamableHTTPClientTransport) sendRequestWithStream(
	ctx context.Context,
	req *JSONRPCRequest,
	options *streamOptions,
) (*json.RawMessage, error) {
	return t.send(ctx, req, options)
}

// establishGetSSEConnection attempts to establish a GET SSE connection if enabled
func (t *streamableHTTPClientTransport) establishGetSSEConnection() {
	if !t.enableGetSSE {
		t.logger.Debug("GET SSE is not enabled, will not establish GET SSE connection")
		return
	}

	if t.sessionID == "" {
		t.logger.Debug("Session ID is empty, cannot establish GET SSE connection")
		return
	}

	t.establishGetSSE()
}
