// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// sseSession represents an active SSE connection.
type sseSession struct {
	done                chan struct{}             // Channel for signaling when the session is done.
	eventQueue          chan string               // Queue for events to be sent.
	sessionID           string                    // Session identifier.
	notificationChannel chan *JSONRPCNotification // Channel for notifications.
	initialized         atomic.Bool               // Whether the session has been initialized.
	writeMu             sync.Mutex                // Write mutex to prevent concurrent writes.
	createdAt           time.Time                 // Session creation time.
	lastActivity        time.Time                 // Last activity time.
	data                map[string]interface{}    // Session data.
	dataMu              sync.RWMutex              // Data mutex.
}

// SessionID returns the session ID.
func (s *sseSession) SessionID() string {
	return s.sessionID
}

// GetID returns the session ID (alias for SessionID for compatibility).
func (s *sseSession) GetID() string {
	return s.sessionID
}

// GetCreatedAt returns the session creation time.
func (s *sseSession) GetCreatedAt() time.Time {
	return s.createdAt
}

// GetLastActivity returns the last activity time.
func (s *sseSession) GetLastActivity() time.Time {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()
	return s.lastActivity
}

// UpdateActivity updates the last activity time.
func (s *sseSession) UpdateActivity() {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	s.lastActivity = time.Now()
}

// GetData gets session data.
func (s *sseSession) GetData(key string) (interface{}, bool) {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()
	if s.data == nil {
		return nil, false
	}
	value, ok := s.data[key]
	return value, ok
}

// SetData sets session data.
func (s *sseSession) SetData(key string, value interface{}) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	if s.data == nil {
		s.data = make(map[string]interface{})
	}
	s.data[key] = value
}

// Initialize marks the session as initialized.
func (s *sseSession) Initialize() {
	s.initialized.Store(true)
}

// Initialized returns whether the session has been initialized.
func (s *sseSession) Initialized() bool {
	return s.initialized.Load()
}

// NotificationChannel returns the notification channel.
func (s *sseSession) NotificationChannel() chan<- *JSONRPCNotification {
	return s.notificationChannel
}

// jsonRPCEnvelope is a common structure for parsing JSON-RPC messages.
// It's used to avoid duplicate anonymous struct definitions across functions
// and can handle both success and error responses.
type jsonRPCEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"`
}

// SSEServer implements a Server-Sent Events (SSE) based MCP server.
type SSEServer struct {
	mcpHandler           *mcpHandler                                                // MCP handler.
	toolManager          *toolManager                                               // Tool manager.
	resourceManager      *resourceManager                                           // Resource manager.
	promptManager        *promptManager                                             // Prompt manager.
	serverInfo           Implementation                                             // Server information.
	basePath             string                                                     // Base path for the server (e.g., "/mcp").
	messageEndpoint      string                                                     // Path for the message endpoint (e.g., "/message").
	sseEndpoint          string                                                     // Path for the SSE endpoint (e.g., "/sse").
	sessions             sync.Map                                                   // Active sessions.
	httpServer           *http.Server                                               // HTTP server.
	contextFunc          func(ctx context.Context, r *http.Request) context.Context // HTTP context function.
	keepAlive            bool                                                       // Whether to keep the connection alive.
	keepAliveInterval    time.Duration                                              // Keep-alive interval.
	logger               Logger                                                     // Logger for this server.
	requestID            atomic.Int64                                               // Request ID counter for generating unique request IDs.
	responses            map[uint64]interface{}                                     // Map for storing response channels.
	responsesMu          sync.RWMutex                                               // Mutex for responses map.
	notificationHandlers map[string]ServerNotificationHandler                       // Map of notification handlers by method name.
	notificationMu       sync.RWMutex                                               // Mutex for notification handlers map.
}

// SSEOption defines a function type for configuring the SSE server.
type SSEOption func(*SSEServer)

// NewSSEServer creates a new SSE server for MCP communication.
func NewSSEServer(name, version string, opts ...SSEOption) *SSEServer {
	// Create server info.
	serverInfo := Implementation{
		Name:    name,
		Version: version,
	}

	// Create managers.
	toolManager := newToolManager()
	resourceManager := newResourceManager()
	promptManager := newPromptManager()
	lifecycleManager := newLifecycleManager(serverInfo)

	// Set up manager relationships.
	lifecycleManager.withToolManager(toolManager)
	lifecycleManager.withResourceManager(resourceManager)
	lifecycleManager.withPromptManager(promptManager)

	// Create MCP handler.
	mcpHandler := newMCPHandler(
		withToolManager(toolManager),
		withLifecycleManager(lifecycleManager),
		withResourceManager(resourceManager),
		withPromptManager(promptManager),
	)

	s := &SSEServer{
		mcpHandler:           mcpHandler,
		toolManager:          toolManager,
		resourceManager:      resourceManager,
		promptManager:        promptManager,
		serverInfo:           serverInfo,
		sseEndpoint:          "/sse",
		messageEndpoint:      "/message",
		keepAlive:            true,
		keepAliveInterval:    30 * time.Second,
		logger:               GetDefaultLogger(),
		responses:            make(map[uint64]interface{}),
		notificationHandlers: make(map[string]ServerNotificationHandler),
	}

	// Apply all options.
	for _, opt := range opts {
		opt(s)
	}

	// Set logger for lifecycle manager.
	lifecycleManager.withLogger(s.logger)

	return s
}

// WithBasePath sets the base path for the server.
func WithBasePath(basePath string) SSEOption {
	return func(s *SSEServer) {
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}
		s.basePath = strings.TrimSuffix(basePath, "/")
	}
}

// WithMessageEndpoint sets the message endpoint path.
func WithMessageEndpoint(endpoint string) SSEOption {
	return func(s *SSEServer) {
		s.messageEndpoint = endpoint
	}
}

// WithSSEEndpoint sets the SSE endpoint path.
func WithSSEEndpoint(endpoint string) SSEOption {
	return func(s *SSEServer) {
		s.sseEndpoint = endpoint
	}
}

// WithHTTPServer sets the HTTP server instance.
func WithHTTPServer(srv *http.Server) SSEOption {
	return func(s *SSEServer) {
		s.httpServer = srv
	}
}

// WithKeepAlive enables or disables keep-alive for SSE connections.
func WithKeepAlive(keepAlive bool) SSEOption {
	return func(s *SSEServer) {
		s.keepAlive = keepAlive
	}
}

// WithKeepAliveInterval sets the interval for keep-alive messages.
func WithKeepAliveInterval(interval time.Duration) SSEOption {
	return func(s *SSEServer) {
		s.keepAliveInterval = interval
		s.keepAlive = true
	}
}

// WithSSEContextFunc sets a function to modify the context from the request.
func WithSSEContextFunc(fn func(ctx context.Context, r *http.Request) context.Context) SSEOption {
	return func(s *SSEServer) {
		s.contextFunc = fn
	}
}

// WithSSEServerLogger sets the logger for the SSE server.
func WithSSEServerLogger(logger Logger) SSEOption {
	return func(s *SSEServer) {
		s.logger = logger
	}
}

// Start starts the SSE server on the given address.
func (s *SSEServer) Start(addr string) error {
	return http.ListenAndServe(addr, s)
}

// Shutdown gracefully stops the SSE server.
func (s *SSEServer) Shutdown(ctx context.Context) error {
	srv := s.httpServer
	if srv != nil {
		// Close all sessions.
		s.sessions.Range(func(key, value interface{}) bool {
			if session, ok := value.(*sseSession); ok {
				close(session.done)
			}
			s.sessions.Delete(key)
			return true
		})

		return srv.Shutdown(ctx)
	}
	return nil
}

// getMessageEndpointForClient returns the message endpoint URL for a client.
// with the given session ID, using a relative path.
func (s *SSEServer) getMessageEndpointForClient(sessionID string) string {
	endpoint := s.messageEndpoint
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	// Ensure the base path is properly formatted.
	basePath := s.basePath
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	// Construct the relative path.
	fullPath := basePath + endpoint

	// Append session ID as a query parameter.
	if strings.Contains(fullPath, "?") {
		fullPath += "&sessionId=" + sessionID
	} else {
		fullPath += "?sessionId=" + sessionID
	}

	return fullPath
}

// handleSSE handles SSE connection requests.
func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.logger.Errorf("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.logger.Errorf("Streaming not supported by client")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers and immediately flush.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	// Create session.
	sessionID := generateSessionID()
	session := &sseSession{
		done:                make(chan struct{}),
		eventQueue:          make(chan string, 100),
		sessionID:           sessionID,
		notificationChannel: make(chan *JSONRPCNotification, 100),
		createdAt:           time.Now(),
		lastActivity:        time.Now(),
		data:                make(map[string]interface{}),
	}
	s.sessions.Store(sessionID, session)

	// Apply context function.
	ctx := r.Context()
	if s.contextFunc != nil {
		ctx = s.contextFunc(ctx, r)
	}

	// Set server instance to context.
	ctx = setServerToContext(ctx, s)

	// Set session information to context.
	ctx = setSessionToContext(ctx, session)

	// Send endpoint event.
	endpointURL := s.getMessageEndpointForClient(sessionID)
	if !sendSSEEvent(w, flusher, &session.writeMu, "endpoint", endpointURL) {
		return
	}

	// Send initial connection message.
	sendSSEComment(w, flusher, &session.writeMu, "connection established")

	// Start notification handler.
	go handleNotifications(s.logger, w, flusher, session)

	// Start event queue handler.
	go handleEventQueue(s.logger, w, flusher, session)

	// Start keep-alive handler.
	if s.keepAlive {
		go handleKeepAlive(s.logger, w, flusher, session, s.keepAliveInterval)
	}

	// Wait for connection to close.
	select {
	case <-ctx.Done():
		s.logger.Debugf("Context cancelled for session %s", sessionID)
	case <-r.Context().Done():
		s.logger.Debugf("Request context cancelled for session %s", sessionID)
	case <-session.done:
		s.logger.Debugf("Session %s closed", sessionID)
	}

	// Clean up resources.
	close(session.done)
	s.sessions.Delete(sessionID)
	s.logger.Debugf("Cleaned up session %s", sessionID)
}

// sendSSEEvent sends SSE event and returns whether it is successful.
func sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, mu *sync.Mutex, eventType, data string) bool {
	mu.Lock()
	defer mu.Unlock()

	event := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)
	if _, err := fmt.Fprint(w, event); err != nil {
		return false
	}
	flusher.Flush()
	return true
}

// sendSSEComment sends SSE comment.
func sendSSEComment(w http.ResponseWriter, flusher http.Flusher, mu *sync.Mutex, comment string) {
	mu.Lock()
	defer mu.Unlock()

	fmt.Fprintf(w, ": %s\n\n", comment)
	flusher.Flush()
}

// handleNotifications handles notification messages.
func handleNotifications(logger Logger, w http.ResponseWriter, flusher http.Flusher, session *sseSession) {
	for {
		select {
		case notification := <-session.notificationChannel:
			data, err := json.Marshal(notification)
			if err != nil {
				logger.Errorf("Error serializing notification: %v", err)
				continue
			}

			session.writeMu.Lock()
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
			session.writeMu.Unlock()

		case <-session.done:
			return
		}
	}
}

// handleEventQueue handles event queue.
func handleEventQueue(logger Logger, w http.ResponseWriter, flusher http.Flusher, session *sseSession) {
	for {
		select {
		case event := <-session.eventQueue:
			session.writeMu.Lock()
			fmt.Fprint(w, event)
			flusher.Flush()
			session.writeMu.Unlock()

		case <-session.done:
			logger.Debugf("Event queue handler terminated for session %s", session.sessionID)
			return
		}
	}
}

// handleKeepAlive handles keep-alive messages.
func handleKeepAlive(logger Logger, w http.ResponseWriter, flusher http.Flusher, session *sseSession, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			session.writeMu.Lock()
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
			session.writeMu.Unlock()

		case <-session.done:
			logger.Debugf("Keepalive handler terminated for session %s", session.sessionID)
			return
		}
	}
}

// handleMessage handles client message requests.
func (s *SSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	// Check method.
	if r.Method != http.MethodPost {
		s.logger.Errorf("Invalid method for message endpoint: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session from request
	session, err := s.getSessionFromRequest(r)
	if err != nil {
		s.handleSessionError(w, err)
		return
	}

	// Read and parse the raw message.
	var rawMessage json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		s.logger.Errorf("Error reading request body: %v", err)
		s.writeJSONRPCError(w, nil, ErrCodeParse, "Parse error")
		return
	}

	// Parse base message to determine type.
	var base baseMessage
	if err := json.Unmarshal(rawMessage, &base); err != nil {
		s.logger.Errorf("Error parsing base message: %v", err)
		s.writeJSONRPCError(w, nil, ErrCodeParse, "Invalid JSON-RPC message")
		return
	}

	// Apply context function first.
	ctx := r.Context()
	if s.contextFunc != nil {
		ctx = s.contextFunc(ctx, r)
	}

	// Create context with session.
	ctx = s.createSessionContext(ctx, session)

	// Immediately return HTTP 202 Accepted status code, indicating request has been received.
	w.WriteHeader(http.StatusAccepted)

	// Handle different message types based on presence of ID and Method.
	if base.ID != nil && base.Method != "" {
		// JSON-RPC Request (has both ID and Method).
		s.handleRequestMessage(ctx, rawMessage, session)
	} else if base.ID == nil && base.Method != "" {
		// JSON-RPC Notification (has Method but no ID).
		s.handleNotificationMessage(ctx, rawMessage, session)
	} else if base.ID != nil && base.Method == "" {
		// JSON-RPC Response (has ID but no Method) - like roots/list response.
		s.handleResponseMessage(ctx, rawMessage, session)
	} else {
		// Invalid message format.
		s.logger.Errorf("Invalid JSON-RPC message: missing required fields")
		s.writeJSONRPCError(w, nil, ErrCodeInvalidRequest, "Invalid JSON-RPC message format")
		return
	}
}

// handleRequestMessage processes JSON-RPC requests.
func (s *SSEServer) handleRequestMessage(ctx context.Context, rawMessage json.RawMessage, session *sseSession) {
	var request JSONRPCRequest
	if err := json.Unmarshal(rawMessage, &request); err != nil {
		s.logger.Errorf("Error parsing request: %v", err)
		return
	}

	// Process request in background.
	go s.processRequestAsync(ctx, &request, session)
}

// handleNotificationMessage processes JSON-RPC notifications.
func (s *SSEServer) handleNotificationMessage(ctx context.Context, rawMessage json.RawMessage, session *sseSession) {
	var notification JSONRPCNotification
	if err := json.Unmarshal(rawMessage, &notification); err != nil {
		s.logger.Errorf("Error parsing notification: %v", err)
		return
	}

	// Handle notification asynchronously.
	go func() {
		// Create a context that will not be canceled due to HTTP connection closure.
		detachedCtx := context.WithoutCancel(ctx)

		// Process notification (currently just log it, but can be extended).
		if err := s.handleNotification(detachedCtx, &notification, session); err != nil {
			s.logger.Errorf("Error handling notification %s: %v", notification.Method, err)
		}
	}()
}

// handleNotification processes notifications (can be extended for different notification types).
func (s *SSEServer) handleNotification(ctx context.Context, notification *JSONRPCNotification, session *sseSession) error {
	// Check if there's a registered handler for this notification method.
	s.notificationMu.RLock()
	handler, exists := s.notificationHandlers[notification.Method]
	s.notificationMu.RUnlock()

	if exists {
		go func() {
			// Call the handler with the notification pointer and context.
			if err := handler(ctx, notification); err != nil && s.logger != nil {
				s.logger.Errorf("Error handling notification %s: %v", notification.Method, err)
			}
		}()
	} else if s.logger != nil {
		s.logger.Warnf("Received notification with no handler registered: %s", notification.Method)
	}

	return nil
}

// handleResponseMessage processes JSON-RPC responses (like roots/list responses).
func (s *SSEServer) handleResponseMessage(ctx context.Context, rawMessage json.RawMessage, session *sseSession) {
	var response jsonRPCEnvelope

	if err := json.Unmarshal(rawMessage, &response); err != nil {
		s.logger.Errorf("Error parsing response: %v", err)
		return
	}
	// Convert ID to uint64.
	requestIDUint, ok := s.parseRequestID(response.ID)
	if !ok {
		s.logger.Errorf("Invalid request ID in response: %v", response.ID)
		return
	}

	// Get the response channel.
	s.responsesMu.RLock()
	responseChanInterface, exists := s.responses[requestIDUint]
	s.responsesMu.RUnlock()

	if !exists {
		s.logger.Debugf("Received response for unknown request ID: %d", requestIDUint)
		return
	}

	// Type assert to the correct channel type.
	responseChan, ok := responseChanInterface.(chan *json.RawMessage)
	if !ok {
		s.logger.Errorf("Invalid response channel type for request ID: %d", requestIDUint)
		return
	}

	// Prepare response data.
	var responseMessage *json.RawMessage

	// Handle error response.
	if response.Error != nil {
		// Create error result.
		errorBytes, _ := json.Marshal(map[string]interface{}{
			"error": response.Error,
		})
		errorMsg := json.RawMessage(errorBytes)
		responseMessage = &errorMsg
	} else if response.Result != nil {
		// Handle success response.
		resultBytes, err := json.Marshal(response.Result)
		if err != nil {
			s.logger.Errorf("Failed to marshal response result: %v", err)
			return
		}
		resultMsg := json.RawMessage(resultBytes)
		responseMessage = &resultMsg
	} else {
		// Invalid response - neither error nor result.
		s.logger.Errorf("Invalid JSON-RPC response: missing both result and error for ID: %d", requestIDUint)
		return
	}

	// Deliver the response.
	select {
	case responseChan <- responseMessage:
	default:
		s.logger.Errorf("Failed to deliver response: channel full or closed for request ID: %d", requestIDUint)
	}
}

// getSessionFromRequest extracts and validates the session from the request.
func (s *SSEServer) getSessionFromRequest(r *http.Request) (*sseSession, error) {
	// Get and record all query parameters.
	queryParams := r.URL.Query()

	// Get session ID (only use sessionId parameter).
	sessionID := queryParams.Get("sessionId")
	if sessionID == "" {
		return nil, fmt.Errorf("missing sessionId parameter")
	}

	// Get session.
	sessionValue, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Type assertion.
	session, ok := sessionValue.(*sseSession)
	if !ok {
		return nil, fmt.Errorf("invalid session type for session ID: %s", sessionID)
	}

	// Update session activity.
	session.UpdateActivity()

	return session, nil
}

// handleSessionError handles errors related to session retrieval.
func (s *SSEServer) handleSessionError(w http.ResponseWriter, err error) {
	errMsg := err.Error()
	s.logger.Errorf("%s", errMsg)

	if strings.Contains(errMsg, "missing sessionId") {
		http.Error(w, "Missing sessionId parameter", http.StatusBadRequest)
	} else if strings.Contains(errMsg, "session not found") {
		http.Error(w, "Session not found", http.StatusNotFound)
	} else {
		http.Error(w, "Invalid session", http.StatusInternalServerError)
	}
}

// parseJSONRPCRequest reads and parses the JSON-RPC request from the request body.
func (s *SSEServer) parseJSONRPCRequest(r *http.Request) (*JSONRPCRequest, error) {
	// Read request body content.
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body: %v", err)
	}

	// Re-create request body.
	r.Body = io.NopCloser(bytes.NewBuffer(requestBody))

	// Parse request body.
	var request JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, fmt.Errorf("error decoding request: %v", err)
	}

	return &request, nil
}

// createSessionContext creates a context with session information.
func (s *SSEServer) createSessionContext(ctx context.Context, session *sseSession) context.Context {
	// Use the global setSessionToContext function to ensure consistent context key.
	ctx = setSessionToContext(ctx, session)

	// Set server instance to context.
	ctx = setServerToContext(ctx, s)

	// Add client session to context so ServerNotificationHandler can use ClientSessionFromContext.
	ctx = withClientSession(ctx, session)

	return ctx
}

// processRequestAsync processes the request asynchronously.
func (s *SSEServer) processRequestAsync(ctx context.Context, request *JSONRPCRequest, session *sseSession) {
	// Create a context that will not be canceled due to HTTP connection closure.
	detachedCtx := context.WithoutCancel(ctx)

	// Check if this is a response to our roots/list request.
	if s.isRootsListResponse(request) {
		s.handleRootsListResponse(request)
		return
	}

	// Process request.
	result, err := s.mcpHandler.handleRequest(detachedCtx, request, session)

	if err != nil {
		s.handleRequestError(err, request.ID, session)
		return
	}

	s.sendSuccessResponse(request.ID, result, session)
}

// isRootsListResponse checks if the request is actually a response to a roots/list request.
func (s *SSEServer) isRootsListResponse(request *JSONRPCRequest) bool {
	// Check if this looks like a response (has ID but no method).
	if request.ID != nil && request.Method == "" {
		// Check if it has either Result or Error field by examining the raw JSON.

		requestMap, ok := request.Params.(map[string]interface{})
		if ok {
			_, hasResult := requestMap["result"]
			_, hasError := requestMap["error"]
			return hasResult || hasError
		}

		// If we can't check directly, fall back to checking the raw request.
		// This is a more reliable but less efficient approach.
		if data, err := json.Marshal(request); err == nil {
			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err == nil {
				_, hasResult := m["result"]
				_, hasError := m["error"]
				return hasResult || hasError
			}
		}
	}
	return false
}

// handleRootsListResponse processes responses from clients to our roots/list requests.
func (s *SSEServer) handleRootsListResponse(request *JSONRPCRequest) {
	var responseID interface{} = request.ID
	var responseResult json.RawMessage
	var responseError json.RawMessage

	// Try to extract result/error from the request.
	if m, ok := request.Params.(map[string]interface{}); ok {
		// Try to get result and error from params.
		if result, hasResult := m["result"]; hasResult && result != nil {
			resultBytes, _ := json.Marshal(result)
			responseResult = resultBytes
		}

		if errVal, hasError := m["error"]; hasError && errVal != nil {
			errBytes, _ := json.Marshal(errVal)
			responseError = errBytes
		}
	} else {
		// Fall back to marshal-unmarshal if direct extraction fails.
		data, err := json.Marshal(request)
		if err != nil {
			s.logger.Errorf("Error marshaling request: %v", err)
			return
		}

		var envelope jsonRPCEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			s.logger.Errorf("Error unmarshaling response: %v", err)
			return
		}

		responseResult = envelope.Result
		responseError = envelope.Error
	}

	// Convert ID to uint64.
	requestIDUint, ok := s.parseRequestID(responseID)
	if !ok {
		s.logger.Errorf("Invalid request ID in response: %v", responseID)
		return
	}

	// Get the response channel.
	s.responsesMu.RLock()
	responseChanInterface, exists := s.responses[requestIDUint]
	s.responsesMu.RUnlock()

	if !exists {
		s.logger.Debugf("Received response for unknown request ID: %d", requestIDUint)
		return
	}

	// Type assert to the correct channel type.
	responseChan, ok := responseChanInterface.(chan *json.RawMessage)
	if !ok {
		s.logger.Errorf("Invalid response channel type for request ID: %d", requestIDUint)
		return
	}

	// Handle error response.
	if len(responseError) > 0 {
		errorBytes, _ := json.Marshal(map[string]interface{}{
			"error": json.RawMessage(responseError),
		})
		errorMessage := json.RawMessage(errorBytes)

		select {
		case responseChan <- &errorMessage:
		default:
			s.logger.Errorf("Failed to deliver error response: channel full or closed")
		}
	} else if len(responseResult) > 0 {
		// Handle success response.
		select {
		case responseChan <- &responseResult:
		default:
			s.logger.Errorf("Failed to deliver response: channel full or closed")
		}
	} else {
		// Invalid response - neither error nor result.
		s.logger.Errorf("Invalid JSON-RPC response: missing both result and error for ID: %d", requestIDUint)
	}
}

// parseRequestID safely converts interface{} to uint64.
func (s *SSEServer) parseRequestID(id interface{}) (uint64, bool) {
	switch v := id.(type) {
	case int:
		return uint64(v), true
	case int64:
		return uint64(v), true
	case uint64:
		return v, true
	case float64:
		return uint64(v), true
	default:
		return 0, false
	}
}

// handleRequestError creates and sends an error response for a failed request.
func (s *SSEServer) handleRequestError(err error, requestID interface{}, session *sseSession) {
	s.logger.Errorf("Error handling request: %v", err)

	// Create error response.
	errorResponse := &JSONRPCError{
		JSONRPC: "2.0",
		ID:      requestID,
		Error: struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		}{
			Code:    -32603, // Internal error
			Message: err.Error(),
		},
	}

	// Send error response.
	responseData, _ := json.Marshal(errorResponse)
	event := formatSSEEvent("message", responseData)

	select {
	case session.eventQueue <- event:
		// Error response queued successfully.
	case <-session.done:
		s.logger.Debugf("Session closed, cannot send error response: %s", session.sessionID)
	default:
		s.logger.Errorf("Failed to queue error response: event queue full for session %s", session.sessionID)
	}
}

// sendSuccessResponse creates and sends a success response.
func (s *SSEServer) sendSuccessResponse(requestID interface{}, result interface{}, session *sseSession) {
	// Construct complete JSON-RPC response.
	response := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      requestID,
		Result:  result,
	}

	// Serialize full response.
	fullResponseData, err := json.Marshal(response)
	if err != nil {
		s.logger.Errorf("Error encoding full response: %v", err)
		return
	}

	// Send response via SSE connection.
	event := formatSSEEvent("message", fullResponseData)

	// Send to SSE connection.
	select {
	case session.eventQueue <- event:
		// Response queued successfully.
	case <-session.done:
		s.logger.Debugf("Session closed, cannot send response: %s", session.sessionID)
	default:
		s.logger.Errorf("Failed to queue response: event queue full for session %s", session.sessionID)
	}
}

// writeJSONRPCError writes a JSON-RPC error response.
func (s *SSEServer) writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := JSONRPCError{
		JSONRPC: "2.0",
		ID:      id,
		Error: struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		s.logger.Errorf("Error encoding error response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

// SendNotification sends a notification to a specific session.
// This method provides a compatible interface with the Server interface.
func (s *SSEServer) SendNotification(sessionID string, method string, params map[string]interface{}) error {
	// Create a notification object using the helper function.
	notification := NewJSONRPCNotificationFromMap(method, params)

	// Use the existing method to send the notification.
	return s.sendNotificationToSession(sessionID, notification)
}

// SendNotificationToSession sends a notification to a specific session.
func (s *SSEServer) sendNotificationToSession(sessionID string, notification *JSONRPCNotification) error {
	// Get session
	sessionValue, ok := s.sessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session, ok := sessionValue.(*sseSession)
	if !ok {
		return fmt.Errorf("invalid session type")
	}

	// Check if session is initialized.
	if !session.Initialized() {
		return fmt.Errorf("session not initialized")
	}

	// Send notification.
	select {
	case session.notificationChannel <- notification:
		return nil
	default:
		return fmt.Errorf("notification channel full")
	}
}

// ServeHTTP implements the http.Handler interface.
func (s *SSEServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle path matching.
	path := r.URL.Path

	// If basePath is set, remove the basePath prefix for correct path matching.
	if s.basePath != "" && strings.HasPrefix(path, s.basePath) {
		path = strings.TrimPrefix(path, s.basePath)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	// Standardize SSE and message endpoint paths.
	sseEndpoint := s.sseEndpoint
	if !strings.HasPrefix(sseEndpoint, "/") {
		sseEndpoint = "/" + sseEndpoint
	}

	messageEndpoint := s.messageEndpoint
	if !strings.HasPrefix(messageEndpoint, "/") {
		messageEndpoint = "/" + messageEndpoint
	}

	// Check if it matches SSE endpoint.
	if path == sseEndpoint {
		s.handleSSE(w, r)
		return
	}

	// Check if it matches message endpoint.
	if path == messageEndpoint {
		s.handleMessage(w, r)
		return
	}

	// Return 404 Not Found.
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "text/plain")
	expectedEndpoints := fmt.Sprintf("%s%s, %s%s", s.basePath, sseEndpoint, s.basePath, messageEndpoint)
	fmt.Fprintf(w, "Path not found: %s (expected endpoints: %s)", r.URL.Path, expectedEndpoints)
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	return fmt.Sprintf("sse-%d-%d", time.Now().UnixNano(), time.Now().Unix())
}

// RegisterTool registers a tool with its handler.
func (s *SSEServer) RegisterTool(tool *Tool, handler toolHandler) {
	if tool == nil || handler == nil {
		s.logger.Errorf("RegisterTool: tool and handler cannot be nil")
		return
	}
	s.toolManager.registerTool(tool, handler)
}

// UnregisterTools removes multiple tools by names and returns an error if no tools were unregistered.
func (s *SSEServer) UnregisterTools(names ...string) error {
	if len(names) == 0 {
		return fmt.Errorf("no tool names provided")
	}

	unregisteredCount := s.toolManager.unregisterTools(names...)
	if unregisteredCount == 0 {
		return fmt.Errorf("none of the specified tools were found")
	}

	return nil
}

// RegisterResource registers a resource with its handler.
func (s *SSEServer) RegisterResource(resource *Resource, handler resourceHandler) {
	if resource == nil || handler == nil {
		s.logger.Errorf("RegisterResource: resource and handler cannot be nil")
		return
	}
	s.resourceManager.registerResource(resource, handler)
}

// RegisterResources registers a resource with its handler for multiple contents.
func (s *SSEServer) RegisterResources(resource *Resource, handler resourcesHandler) {
	if resource == nil || handler == nil {
		s.logger.Errorf("RegisterResources: resource and handler cannot be nil")
		return
	}
	s.resourceManager.registerResources(resource, handler)
}

// RegisterResourceTemplate registers a resource template with its handler.
func (s *SSEServer) RegisterResourceTemplate(template *ResourceTemplate, handler resourceTemplateHandler) {
	if template == nil || handler == nil {
		s.logger.Errorf("RegisterResourceTemplate: template and handler cannot be nil")
		return
	}
	s.resourceManager.registerTemplate(template, handler)
}

// RegisterPrompt registers a prompt with its handler.
func (s *SSEServer) RegisterPrompt(prompt *Prompt, handler promptHandler) {
	if prompt == nil || handler == nil {
		s.logger.Errorf("RegisterPrompt: prompt and handler cannot be nil")
		return
	}
	s.promptManager.registerPrompt(prompt, handler)
}

// GetServerInfo returns the server information.
func (s *SSEServer) GetServerInfo() Implementation {
	return s.serverInfo
}

// ListRoots sends a request to the client asking for its list of roots.
func (s *SSEServer) ListRoots(ctx context.Context) (*ListRootsResult, error) {
	// Get the session from context.
	session := ClientSessionFromContext(ctx)
	if session == nil {
		return nil, ErrNoClientSession
	}

	sessionID := session.GetID()
	if sessionID == "" {
		return nil, fmt.Errorf("session has no ID")
	}

	// Check if session exists in our sessions map.
	_, exists := s.sessions.Load(sessionID)
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Create standard JSON-RPC request.
	requestID := s.requestID.Add(1)
	request := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      requestID,
		Request: Request{
			Method: MethodRootsList,
		},
	}

	// Send request and wait for response using transport layer.
	response, err := s.SendRequest(ctx, sessionID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to send roots/list request: %w", err)
	}

	// Parse response as ListRootsResult.
	var listRootsResult ListRootsResult
	if err := json.Unmarshal(*response, &listRootsResult); err != nil {
		return nil, fmt.Errorf("failed to parse ListRootsResult: %w", err)
	}

	return &listRootsResult, nil
}

// SendRequest sends a JSON-RPC request to a client and waits for response.
func (s *SSEServer) SendRequest(ctx context.Context, sessionID string, request *JSONRPCRequest) (*json.RawMessage, error) {
	// Get session
	sessionValue, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session, ok := sessionValue.(*sseSession)
	if !ok {
		return nil, fmt.Errorf("invalid session type")
	}

	// Generate unique request ID if not provided.
	if request.ID == nil {
		request.ID = s.requestID.Add(1)
	}

	// Create response channel.
	requestIDUint := uint64(request.ID.(int64))
	resultChan := make(chan *json.RawMessage, 1)

	// Store the channel in the responses map.
	s.responsesMu.Lock()
	if s.responses == nil {
		s.responses = make(map[uint64]interface{})
	}
	s.responses[requestIDUint] = resultChan
	s.responsesMu.Unlock()

	// Clean up the response channel when done
	defer func() {
		s.responsesMu.Lock()
		delete(s.responses, requestIDUint)
		s.responsesMu.Unlock()
	}()

	// Send the request to the client via SSE session's event queue.
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send the request as an SSE event with proper event type.
	event := formatSSEEvent("message", requestBytes)
	select {
	case session.eventQueue <- event:
	case <-ctx.Done():
		return nil, fmt.Errorf("context done while sending request: %w", ctx.Err())
	default:
		return nil, fmt.Errorf("failed to send request: event queue full")
	}

	s.logger.Debugf("Sent request with ID: %v", request.ID)

	// Wait for the response or timeout.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-resultChan:
		return response, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	}
}

// formatSSEEvent formats SSE event.
func formatSSEEvent(eventType string, data []byte) string {
	var builder strings.Builder

	// Add event type.
	if eventType != "" {
		builder.WriteString("event: ")
		builder.WriteString(eventType)
		builder.WriteString("\n")
	}

	// Add data, handle multi-line data.
	if len(data) > 0 {
		builder.WriteString("data: ")

		// Replace all newline characters with "\ndata: ".
		dataStr := string(data)
		dataStr = strings.ReplaceAll(dataStr, "\n", "\ndata: ")

		builder.WriteString(dataStr)
		builder.WriteString("\n")
	}

	// End event.
	builder.WriteString("\n")

	return builder.String()
}

// RegisterNotificationHandler registers a handler for the specified notification method.
// This allows the server to respond to client notifications.
func (s *SSEServer) RegisterNotificationHandler(method string, handler ServerNotificationHandler) {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()

	s.notificationHandlers[method] = handler
	if s.logger != nil {
		s.logger.Debugf("Registered notification handler for method: %s", method)
	}
}

// UnregisterNotificationHandler removes a handler for the specified notification method.
func (s *SSEServer) UnregisterNotificationHandler(method string) {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()

	delete(s.notificationHandlers, method)
	if s.logger != nil {
		s.logger.Debugf("Unregistered notification handler for method: %s", method)
	}
}
