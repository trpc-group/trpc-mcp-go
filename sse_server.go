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

// SSEServer implements a Server-Sent Events (SSE) based MCP server.
type SSEServer struct {
	mcpHandler        *mcpHandler                                                // MCP handler.
	toolManager       *toolManager                                               // Tool manager.
	resourceManager   *resourceManager                                           // Resource manager.
	promptManager     *promptManager                                             // Prompt manager.
	serverInfo        Implementation                                             // Server information.
	basePath          string                                                     // Base path for the server (e.g., "/mcp").
	messageEndpoint   string                                                     // Path for the message endpoint (e.g., "/message").
	sseEndpoint       string                                                     // Path for the SSE endpoint (e.g., "/sse").
	sessions          sync.Map                                                   // Active sessions.
	httpServer        *http.Server                                               // HTTP server.
	contextFunc       func(ctx context.Context, r *http.Request) context.Context // HTTP context function.
	keepAlive         bool                                                       // Whether to keep the connection alive.
	keepAliveInterval time.Duration                                              // Keep-alive interval.
	logger            Logger                                                     // Logger for this server.
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
		mcpHandler:        mcpHandler,
		toolManager:       toolManager,
		resourceManager:   resourceManager,
		promptManager:     promptManager,
		serverInfo:        serverInfo,
		sseEndpoint:       "/sse",
		messageEndpoint:   "/message",
		keepAlive:         true,
		keepAliveInterval: 30 * time.Second,
		logger:            GetDefaultLogger(),
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

	// Parse the request
	request, err := s.parseJSONRPCRequest(r)
	if err != nil {
		s.logger.Errorf("Error parsing request: %v", err)
		s.writeJSONRPCError(w, nil, -32700, "Invalid JSON")
		return
	}

	// Create context with session.
	ctx := s.createSessionContext(r.Context(), session)

	// Immediately return HTTP 202 Accepted status code, indicating request has been received.
	w.WriteHeader(http.StatusAccepted)

	// Process request in background.
	go s.processRequestAsync(ctx, request, session)
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
	// Use sessionKey structure as context key.
	type sessionKey struct{}
	ctx = context.WithValue(ctx, sessionKey{}, session)

	// Set server instance to context.
	return setServerToContext(ctx, s)
}

// processRequestAsync processes the request asynchronously.
func (s *SSEServer) processRequestAsync(ctx context.Context, request *JSONRPCRequest, session *sseSession) {
	// Create a context that will not be canceled due to HTTP connection closure.
	detachedCtx := context.WithoutCancel(ctx)

	// Process request.
	result, err := s.mcpHandler.handleRequest(detachedCtx, request, session)

	if err != nil {
		s.handleRequestError(err, request.ID, session)
		return
	}

	s.sendSuccessResponse(request.ID, result, session)
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
	fmt.Fprintf(w, "Not Found: %s", r.URL.Path)
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

// RegisterResource registers a resource with its handler.
func (s *SSEServer) RegisterResource(resource *Resource, handler resourceHandler) {
	if resource == nil || handler == nil {
		s.logger.Errorf("RegisterResource: resource and handler cannot be nil")
		return
	}
	s.resourceManager.registerResource(resource, handler)
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
