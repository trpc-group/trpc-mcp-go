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
)

// StreamableHTTPClientTransport implements an HTTP-based MCP transport
type StreamableHTTPClientTransport struct {
	// Server URL
	serverURL *url.URL

	// HTTP client
	httpClient *http.Client

	// HTTP request handler
	httpReqHandler HTTPReqHandler

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
	isStatelessMode bool

	// Logger for this client transport.
	logger Logger
}

// NotificationHandler is a handler for notifications
type NotificationHandler func(notification *JSONRPCNotification) error

// StreamOptions represents streaming options
type StreamOptions struct {
	// Event ID (for stream recovery)
	LastEventID string

	// Notification handlers
	NotificationHandlers map[string]NotificationHandler
}

// NewStreamableHTTPClientTransport creates a new client transport
//
// This transport implementation automatically detects if the server is in stateless mode.
// When no session ID is provided in the initialize response, the client automatically
// sets itself to stateless mode and disables GET SSE connections.
// NewStreamableHTTPClientTransport creates a new client transport.
// If logger is not set via options, uses the default logger.
func NewStreamableHTTPClientTransport(serverURL *url.URL, options ...func(*StreamableHTTPClientTransport)) *StreamableHTTPClientTransport {
	transport := &StreamableHTTPClientTransport{
		serverURL:            serverURL,
		httpClient:           &http.Client{},
		httpReqHandler:       NewDefaultHTTPReqHandler(),
		notificationHandlers: make(map[string]NotificationHandler),
		enableGetSSE:         true,               // Default: GET SSE enabled
		logger:               GetDefaultLogger(), // Use default logger if not set.
	}

	for _, option := range options {
		option(transport)
	}

	return transport
}

// WithSessionID sets the session ID option
func WithSessionID(sessionID string) func(*StreamableHTTPClientTransport) {
	return func(t *StreamableHTTPClientTransport) {
		t.sessionID = sessionID
	}
}

// WithHTTPClient sets the HTTP client option
func WithHTTPClient(client *http.Client) func(*StreamableHTTPClientTransport) {
	return func(t *StreamableHTTPClientTransport) {
		t.httpClient = client
	}
}

// WithClientTransportGetSSEEnabled sets whether GET SSE is enabled
func WithClientTransportGetSSEEnabled(enabled bool) func(*StreamableHTTPClientTransport) {
	return func(t *StreamableHTTPClientTransport) {
		t.enableGetSSE = enabled
	}
}

// WithClientTransportLogger sets the logger for the client transport.
func WithClientTransportLogger(logger Logger) func(*StreamableHTTPClientTransport) {
	return func(t *StreamableHTTPClientTransport) {
		t.logger = logger
	}
}

// WithHTTPReqHandler sets the HTTP request handler
func WithTransportHTTPReqHandler(handler HTTPReqHandler) func(*StreamableHTTPClientTransport) {
	return func(t *StreamableHTTPClientTransport) {
		if handler != nil {
			t.httpReqHandler = handler
		}
	}
}

// SendRequest sends a request and waits for a response
func (t *StreamableHTTPClientTransport) SendRequest(ctx context.Context, req *JSONRPCRequest) (*json.RawMessage, error) {
	return t.sendRequest(ctx, req, nil)
}

// sendRequest sends a request and handles the response
func (t *StreamableHTTPClientTransport) sendRequest(ctx context.Context, req *JSONRPCRequest, options *StreamOptions) (*json.RawMessage, error) {
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

	// Set request headers - accept both SSE and JSON responses
	httpReq.Header.Set(ContentTypeHeader, ContentTypeJSON)
	httpReq.Header.Set(AcceptHeader, ContentTypeJSON+", "+ContentTypeSSE)
	if t.sessionID != "" && !t.isStatelessMode {
		httpReq.Header.Set(SessionIDHeader, t.sessionID)
	}

	// If LastEventID is provided, attach it to the request
	if options != nil && options.LastEventID != "" {
		httpReq.Header.Set(LastEventIDHeader, options.LastEventID)
	} else if t.lastEventID != "" {
		httpReq.Header.Set(LastEventIDHeader, t.lastEventID)
	}

	// Send request using the handler
	httpResp, err := t.httpReqHandler.Handle(ctx, t.httpClient, httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequestFailed, err)
	}

	// Handle session ID
	if sessionID := httpResp.Header.Get(SessionIDHeader); sessionID != "" {
		t.sessionID = sessionID
		t.isStatelessMode = false
	} else if req.Method == MethodInitialize && !t.isStatelessMode {
		// If this is an initialize request and no session ID was received, auto-detect as stateless mode
		t.isStatelessMode = true
		t.enableGetSSE = false // Disable GET SSE in stateless mode
	}

	// Check content type
	contentType := httpResp.Header.Get(ContentTypeHeader)
	if strings.Contains(contentType, ContentTypeSSE) {
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

// Handle SSE response
func (t *StreamableHTTPClientTransport) handleSSEResponse(ctx context.Context, httpResp *http.Response, reqID interface{}, options *StreamOptions) (*json.RawMessage, error) {
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
	if options != nil && options.NotificationHandlers != nil {
		for method, handler := range options.NotificationHandlers {
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
				t.logger.Infof("Received event ID: %s", t.lastEventID)
				continue
			}

			// Process event data
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

				// Create a raw message from the data
				rawMessage := json.RawMessage(data)

				// First, check if it's a response to our request by looking at the ID
				var jsonResp map[string]interface{}
				if err := json.Unmarshal(rawMessage, &jsonResp); err == nil {
					// Check if it has an ID that matches our request ID
					if id, hasID := jsonResp["id"]; hasID && fmt.Sprintf("%v", id) == fmt.Sprintf("%v", reqID) {
						// Check if it's an error response
						if _, hasError := jsonResp["error"]; hasError {
							// Return the raw error response for error handling
							resultReceived = true
							rawResult = &rawMessage
							t.logger.Infof("Received error response for ID: %v", reqID)
							continue
						}

						// Extract result from the response
						if result, hasResult := jsonResp["result"]; hasResult {
							// Serialize just the result part
							resultBytes, err := json.Marshal(result)
							if err != nil {
								return nil, fmt.Errorf("failed to marshal result: %v", err)
							}

							resultRaw := json.RawMessage(resultBytes)
							rawResult = &resultRaw
							resultReceived = true
							t.logger.Infof("Received matching response for ID: %v", reqID)

							// If there are no other handlers, we can return early
							if len(handlers) == 0 {
								return rawResult, nil
							}
							continue
						}
					}
				}

				// Check if it's a notification
				var notification JSONRPCNotification
				if err := json.Unmarshal(rawMessage, &notification); err == nil && notification.Method != "" {
					// Process notification
					if handler, ok := handlers[notification.Method]; ok {
						if err := handler(&notification); err != nil {
							t.logger.Infof("Notification handler error: %v", err)
						}
					} else {
						t.logger.Infof("Received unhandled notification: %s", notification.Method)
					}
				}
			}
		}
	}
}

// RegisterNotificationHandler registers a notification handler
func (t *StreamableHTTPClientTransport) RegisterNotificationHandler(method string, handler NotificationHandler) {
	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	t.notificationHandlers[method] = handler
}

// UnregisterNotificationHandler unregisters a notification handler
func (t *StreamableHTTPClientTransport) UnregisterNotificationHandler(method string) {
	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	delete(t.notificationHandlers, method)
}

// SendNotification sends a notification (no response expected)
func (t *StreamableHTTPClientTransport) SendNotification(ctx context.Context, notification *JSONRPCNotification) error {
	// Serialize notification to JSON
	notifBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to serialize notification: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.serverURL.String(), bytes.NewReader(notifBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set request headers
	httpReq.Header.Set(ContentTypeHeader, ContentTypeJSON)
	httpReq.Header.Set(AcceptHeader, ContentTypeJSON)
	if t.sessionID != "" {
		httpReq.Header.Set(SessionIDHeader, t.sessionID)
	}

	// Send request
	httpResp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Handle session ID
	if sessionID := httpResp.Header.Get(SessionIDHeader); sessionID != "" {
		t.sessionID = sessionID
	}

	// Check status code
	if httpResp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("HTTP request failed with status code: %d", httpResp.StatusCode)
	}

	return nil
}

// SendResponse sends a response (clients don't need to implement this method)
func (t *StreamableHTTPClientTransport) SendResponse(ctx context.Context, resp *JSONRPCResponse) error {
	return fmt.Errorf("client transport does not support sending responses")
}

// Close closes the transport connection
func (t *StreamableHTTPClientTransport) Close() error {
	// Close GET SSE connection
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
func (t *StreamableHTTPClientTransport) GetSessionID() string {
	return t.sessionID
}

// SetSessionID sets the session ID
func (t *StreamableHTTPClientTransport) SetSessionID(sessionID string) {
	t.sessionID = sessionID
	t.logger.Infof("Set session ID: %s", sessionID)

	// If GET SSE is enabled and session ID exists, try to establish GET SSE connection
	if t.enableGetSSE && t.sessionID != "" {
		t.logger.Info("GET SSE is enabled, will attempt to establish GET SSE connection")
		// Establish connection asynchronously to avoid blocking
		go t.establishGetSSEConnection()
	} else {
		if !t.enableGetSSE {
			t.logger.Info("GET SSE is not enabled, will not establish GET SSE connection")
		} else if t.sessionID == "" {
			t.logger.Info("Session ID is empty, cannot establish GET SSE connection")
		}
	}
}

// Establish GET SSE connection
func (t *StreamableHTTPClientTransport) establishGetSSEConnection() {
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
func (t *StreamableHTTPClientTransport) connectGetSSE(ctx context.Context) error {
	// Check if there's a session ID
	if t.sessionID == "" {
		return fmt.Errorf("cannot establish GET SSE connection: session ID is empty")
	}

	// Build GET request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.serverURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}

	// Set necessary headers
	req.Header.Set(AcceptHeader, ContentTypeSSE)
	req.Header.Set(SessionIDHeader, t.sessionID)
	if t.lastEventID != "" {
		req.Header.Set(LastEventIDHeader, t.lastEventID)
	}

	t.logger.Infof("Attempting to establish GET SSE connection, session ID: %s", t.sessionID)

	// Send request
	resp, err := t.httpClient.Do(req)
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
	t.logger.Infof("GET SSE connection established, session ID: %s", t.sessionID)

	// Handle SSE event stream
	return t.handleGetSSEEvents(ctx, resp.Body)
}

// Handle GET SSE event stream
func (t *StreamableHTTPClientTransport) handleGetSSEEvents(ctx context.Context, body io.ReadCloser) error {
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

// Process SSE event
func (t *StreamableHTTPClientTransport) processSSEEvent(eventID, eventData string) {
	// Ignore empty events
	if eventData == "" {
		return
	}

	t.logger.Infof("Received GET SSE event: ID=%s, data length: %d", eventID, len(eventData))

	// Use the new unified parsing function to parse the message
	message, msgType, err := ParseJSONRPCMessage([]byte(eventData))
	if err != nil {
		t.logger.Infof("Failed to parse SSE event: %v", err)
		return
	}

	// Only handle notification type messages
	if msgType == JSONRPCMessageTypeNotification {
		notification := message.(*JSONRPCNotification)

		// Call the appropriate handler
		t.handlersMutex.RLock()
		handler, ok := t.notificationHandlers[notification.Method]
		t.handlersMutex.RUnlock()

		if ok && handler != nil {
			if err := handler(notification); err != nil {
				t.logger.Infof("Failed to handle notification: %s, error: %v",
					FormatJSONRPCMessage(notification), err)
			} else {
				t.logger.Infof("Successfully handled notification: %s",
					FormatJSONRPCMessage(notification))
			}
		} else {
			t.logger.Infof("Received notification with no registered handler: %s",
				FormatJSONRPCMessage(notification))
		}
	} else {
		// In GET SSE connection, we expect to receive only notifications
		t.logger.Infof("GET SSE connection received non-notification message, type: %s, ignored", msgType)
	}
}

// TerminateSession terminates the session
func (t *StreamableHTTPClientTransport) TerminateSession(ctx context.Context) error {
	// Create HTTP DELETE request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, t.serverURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set session ID header
	if t.sessionID != "" {
		httpReq.Header.Set(SessionIDHeader, t.sessionID)
	} else {
		return fmt.Errorf("no active session")
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

// IsStatelessMode returns whether the client is in stateless mode
//
// The client automatically detects if the server is in stateless mode: when no session ID
// is provided in the initialize response, the client automatically sets itself to stateless
// mode and disables GET SSE connections.
//
// If it returns true, the client is currently running in stateless mode and will not include
// a session ID in requests or attempt to establish GET SSE connections.
func (t *StreamableHTTPClientTransport) IsStatelessMode() bool {
	return t.isStatelessMode
}

// SendRequestWithStream sends a request with streaming options
func (t *StreamableHTTPClientTransport) SendRequestWithStream(ctx context.Context, req *JSONRPCRequest, options *StreamOptions) (*json.RawMessage, error) {
	return t.sendRequest(ctx, req, options)
}
