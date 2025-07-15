// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"trpc.group/trpc-go/trpc-mcp-go/internal/httputil"
	"trpc.group/trpc-go/trpc-mcp-go/internal/sseutil"
)

const (
	// defaultSessionExpirySeconds is the default session expiration time (seconds)
	defaultSessionExpirySeconds = 3600 // 1 hour
)

// requestHandler interface defines a component that handles requests
type requestHandler interface {
	// handleRequest handle a request
	handleRequest(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error)

	// handleNotification handle a notification
	handleNotification(ctx context.Context, notification *JSONRPCNotification, session Session) error
}

// httpServerHandler implements an HTTP server handler
type httpServerHandler struct {
	// Logger for this server handler.
	logger Logger
	// Session manager
	sessionManager sessionManager

	// Request handler
	requestHandler requestHandler

	// Whether sessions are enabled
	enableSession bool

	// Whether stateless mode is used
	isStateless bool

	// responder factory
	responderFactory *responderFactory

	// Notification channel buffer size
	notificationBufferSize int

	// Whether POST SSE is enabled
	enablePostSSE bool

	// Whether GET SSE is enabled
	enableGetSSE bool

	// Session GET SSE connections mapping (each session ID maps to a GET SSE connection)
	getSSEConnections     map[string]*getSSEConnection
	getSSEConnectionsLock sync.RWMutex

	// HTTP context functions for extracting information from HTTP requests
	httpContextFuncs []HTTPContextFunc

	// Server path.
	serverPath string
}

// getSSEConnection represents a GET SSE connection
type getSSEConnection struct {
	writer      http.ResponseWriter
	flusher     http.Flusher
	ctx         context.Context
	cancelFunc  context.CancelFunc
	lastEventID string

	// Prevent concurrent write conflicts
	writeLock sync.Mutex

	// Event ID generator, reuses existing sseResponder
	sseResponder *sseResponder
}

// newHTTPServerHandler creates an HTTP server handler
func newHTTPServerHandler(handler requestHandler, serverPath string, options ...func(*httpServerHandler)) *httpServerHandler {
	h := &httpServerHandler{
		logger:                 GetDefaultLogger(), // Use default logger if not set.
		requestHandler:         handler,
		enableSession:          true,  // Default: sessions enabled
		isStateless:            false, // Default: stateful mode
		notificationBufferSize: defaultNotificationBufferSize,
		enablePostSSE:          true, // Default: POST SSE enabled
		enableGetSSE:           true, // Default: GET SSE enabled
		getSSEConnections:      make(map[string]*getSSEConnection),
		serverPath:             serverPath,
	}

	// Apply options
	for _, option := range options {
		option(h)
	}

	// Ensure logger is set.
	if h.logger == nil {
		h.logger = GetDefaultLogger()
	}

	// After applying all options, ensure responderFactory uses correct stateless mode setting
	h.responderFactory = newResponderFactory(
		withResponderSSEEnabled(h.enablePostSSE),
		withFactoryStatelessMode(h.isStateless),
	)

	// If sessions are enabled but no session manager is set, create a default one
	if h.enableSession && h.sessionManager == nil {
		h.sessionManager = newSessionManager(defaultSessionExpirySeconds)
	}

	return h
}

// withTransportSessionManager sets the session manager.
func withTransportSessionManager(manager sessionManager) func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.sessionManager = manager
	}
}

// withServerTransportLogger sets the logger for httpServerHandler.
func withServerTransportLogger(logger Logger) func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.logger = logger
	}
}

// withoutTransportSession disables sessions
func withoutTransportSession() func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.enableSession = false
		h.sessionManager = nil
	}
}

// withServerPOSTSSEEnabled sets whether SSE responses are enabled
func withServerPOSTSSEEnabled(enabled bool) func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.enablePostSSE = enabled
	}
}

// withTransportGetSSEEnabled sets whether GET SSE is enabled
func withTransportGetSSEEnabled(enabled bool) func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.enableGetSSE = enabled
	}
}

// withTransportNotificationBufferSize sets the notification buffer size
func withTransportNotificationBufferSize(size int) func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.notificationBufferSize = size
	}
}

// withTransportStatelessMode sets the server to stateless mode
// In stateless mode, the server does not generate persistent session IDs; each request uses a temporary session
func withTransportStatelessMode() func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.isStateless = true
		// In stateless mode, sessions are still enabled but use temporary sessions
		h.enableSession = true
		// Ensure there's still a session manager for creating temporary sessions
		if h.sessionManager == nil {
			h.sessionManager = newSessionManager(defaultSessionExpirySeconds)
		}
	}
}

// withTransportHTTPContextFuncs sets the HTTP context functions
func withTransportHTTPContextFuncs(funcs []HTTPContextFunc) func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.httpContextFuncs = funcs
	}
}

// ServeHTTP implements the http.Handler interface
func (h *httpServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.isValidPath(r.URL.Path) {
		if h.serverPath == "" {
			http.Error(w, fmt.Sprintf("Path not found: %s (expected: %s)", r.URL.Path, h.serverPath), http.StatusNotFound)
		}
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handlePost(r.Context(), w, r)
	case http.MethodGet:
		if !h.enableGetSSE {
			w.Header().Set("Allow", "POST, DELETE")
			http.Error(w, "GET method not enabled", http.StatusMethodNotAllowed)
			return
		}
		h.handleGet(r.Context(), w, r)
	case http.MethodDelete:
		h.handleDelete(r.Context(), w, r)
	default:
		w.Header().Set("Allow", "POST, GET, DELETE")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type baseMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	ID      interface{} `json:"id"`
}

// handlePost handles POST requests
func (h *httpServerHandler) handlePost(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Apply HTTP context functions to enrich the context with information from HTTP request
	enrichedCtx := ctx
	for _, fn := range h.httpContextFuncs {
		enrichedCtx = fn(enrichedCtx, r)
	}

	var rawMessage json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		http.Error(w, ErrInvalidRequestBody.Error(), http.StatusBadRequest)
		return
	}

	// Create response context
	cancel := context.CancelFunc(func() {}) // Placeholder to keep defer cancel() syntax consistent
	defer cancel()

	var isInitialize bool
	var base baseMessage

	if err := json.Unmarshal(rawMessage, &base); err != nil {
		http.Error(w, ErrInvalidRequestBody.Error(), http.StatusBadRequest)
		return
	}

	// First try to parse as request
	// Check if it's an initialize request
	if base.ID != nil && base.Method == MethodInitialize {
		isInitialize = true
	}

	// Get session
	var session Session
	if h.isStateless {
		// Stateless mode: create a temporary session for each request.
		session = newSession()
	} else if h.enableSession {
		// Stateful mode
		sessionIDHeader := r.Header.Get(httputil.SessionIDHeader)
		if sessionIDHeader != "" {
			var ok bool
			session, ok = h.sessionManager.getSession(sessionIDHeader)
			if !ok {
				http.Error(w, "Session not found or expired", http.StatusNotFound) // 404 if session ID provided but not found
				return
			}
		} else if isInitialize {
			// If it's an initialize request and no session ID header, create a new session
			session = h.sessionManager.createSession()
			h.logger.Infof("Created new session ID: %s for initialize request", session.GetID())
		} else {
			// Not an initialize request and no session ID header was provided.
			// According to MCP spec, server SHOULD respond with 400 Bad Request.
			http.Error(w, "Missing Mcp-Session-Id header for non-initialize request", http.StatusBadRequest)
			return
		}
	}

	// Branch: request or notification
	if base.ID != nil && base.Method != "" {
		h.handlePostRequest(enrichedCtx, w, r, rawMessage, base, session)
		return
	}
	if base.ID == nil && base.Method != "" {
		h.handlePostNotification(enrichedCtx, w, r, rawMessage, base, session)
		return
	}

	// Unable to parse request
	http.Error(w, "Invalid JSON-RPC message", http.StatusBadRequest)
}

// handlePostRequest handles JSON-RPC requests
func (h *httpServerHandler) handlePostRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, rawMessage json.RawMessage, base baseMessage, session Session) {
	respCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var req JSONRPCRequest
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		http.Error(w, "Invalid JSON-RPC request format: "+err.Error(), http.StatusBadRequest)
		return
	}

	responder := h.responderFactory.createResponder(r, rawMessage)

	// Check response processor type
	if sseResponder, ok := responder.(*sseResponder); ok {
		// Use SSE response mode
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}
		sseutil.SetStandardHeaders(w)
		if !h.isStateless && session != nil {
			w.Header().Set(httputil.SessionIDHeader, session.GetID())
		}
		var sessionID string
		if session != nil {
			sessionID = session.GetID()
		}
		notificationSender := newSSENotificationSender(w, flusher, sessionID)
		reqCtx := withNotificationSender(ctx, notificationSender)
		if session != nil {
			reqCtx = setSessionToContext(reqCtx, session)
		}
		resp, err := h.requestHandler.handleRequest(reqCtx, &req, session)
		if err != nil {
			h.logger.Infof("Request processing failed: %v", err)
			errorResp := newJSONRPCErrorResponse(req.ID, ErrCodeInternal, "Internal server error", nil)
			err = sseResponder.respond(ctx, w, r, errorResp, session)
			if err != nil {
				h.logger.Infof("Failed to send SSE error response: %v", err)
			}
			return
		}
		jsonrpcResponse := JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  resp,
		}
		err = sseResponder.respond(ctx, w, r, jsonrpcResponse, session)
		if err != nil {
			h.logger.Infof("Failed to send SSE final response: %v", err)
		}
		return
	}
	// Use normal JSON response mode
	noopSender := &noopNotificationSender{}
	reqCtx := withNotificationSender(ctx, noopSender)
	if session != nil {
		reqCtx = setSessionToContext(reqCtx, session)
	}
	resp, err := h.requestHandler.handleRequest(reqCtx, &req, session)
	if err != nil {
		h.logger.Infof("Request processing failed: %v", err)
		errorResp := newJSONRPCErrorResponse(req.ID, ErrCodeInternal, "Internal server error", nil)
		responder.respond(respCtx, w, r, errorResp, session)
		return
	}
	jsonrpcResponse := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  resp,
	}
	responder.respond(respCtx, w, r, jsonrpcResponse, session)
}

// handlePostNotification handles JSON-RPC notifications
func (h *httpServerHandler) handlePostNotification(ctx context.Context, w http.ResponseWriter, r *http.Request, rawMessage json.RawMessage, base baseMessage, session Session) {
	var notification JSONRPCNotification
	if err := json.Unmarshal(rawMessage, &notification); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if notification.Method == MethodNotificationsInitialized {
		if h.enableSession && session == nil {
			h.logger.Info("Warning: Received initialized notification but no active session")
		}
		// In stateless mode, skip initialization state check and return success directly.
		if h.isStateless {
			h.logger.Debug("Stateless mode: Skipping initialization state check for notifications/initialized")
			h.sendNotificationResponse(w, session)
			return
		}
	}
	notificationCtx := ctx
	if session != nil {
		notificationCtx = setSessionToContext(ctx, session)
	}
	if err := h.requestHandler.handleNotification(notificationCtx, &notification, session); err != nil {
		h.logger.Infof("Notification processing failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.sendNotificationResponse(w, session)
}

// handleDelete handles DELETE requests
func (h *httpServerHandler) handleDelete(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Get session ID from session ID header
	sessionID := r.Header.Get(httputil.SessionIDHeader)
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	// Get session
	if h.enableSession {
		// Terminate session
		if h.sessionManager.terminateSession(sessionID) {
			// Clean up GET SSE connections
			h.cleanupSession(sessionID)

			// Return success response
			h.sendEmptyResponse(w, http.StatusOK, nil)
			return
		}

		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Session functionality not enabled
	http.Error(w, "Session management disabled", http.StatusNotImplemented)
}

// handleGet handles GET requests (for Server-Sent Events)
func (h *httpServerHandler) handleGet(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	sessionIDHeader := r.Header.Get(httputil.SessionIDHeader) // Get session ID early for logging
	h.logger.Infof("Session [%s]: handleGet invoked. Initial r.Context().Err(): %v", sessionIDHeader, r.Context().Err())

	// Defer full cancel log
	var localCancelFunc context.CancelFunc // To access in defer for logging state before explicit cancel by defer
	// currentConnCtxForDefer was removed as it was unused after simplifying defer logging

	defer func() {
		// It's tricky to reliably get the specific connCtx here if the function exits in multiple ways
		// or if connCtx is shadowed. We'll log based on localCancelFunc being set.
		if localCancelFunc != nil {
			h.logger.Infof("Session [%s]: handleGet defer: Calling localCancelFunc().", sessionIDHeader)
			localCancelFunc() // This is the cancelConn from WithCancel for the current handleGet instance
			h.logger.Infof("Session [%s]: handleGet defer: localCancelFunc() called.", sessionIDHeader)
		} else {
			h.logger.Infof(
				"Session [%s]: handleGet defer: localCancelFunc was nil "+
					"(should not happen if connCtx was created).",
				sessionIDHeader,
			)
		}
	}()

	// Check if GET SSE is enabled
	if !h.enableGetSSE {
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "GET method not enabled", http.StatusMethodNotAllowed)
		return
	}

	// GET method not supported in stateless mode
	if h.isStateless {
		http.Error(w, "GET method not supported in stateless mode", http.StatusMethodNotAllowed)
		return
	}

	// Check if there's a session ID
	sessionID := r.Header.Get(httputil.SessionIDHeader)
	if sessionID == "" {
		http.Error(w, "No session ID provided", http.StatusBadRequest)
		return
	}

	// Get session
	session, ok := h.sessionManager.getSession(sessionID)
	if !ok {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Check if streaming is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE response headers
	sseutil.SetStandardHeaders(w)
	w.Header().Set(httputil.SessionIDHeader, session.GetID())
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Create context, for canceling connection
	connCtx, cancelConn := context.WithCancel(ctx)
	localCancelFunc = cancelConn // Assign to the variable captured by defer

	// Check if there's already a GET SSE connection
	h.getSSEConnectionsLock.Lock()
	existingConn, exists := h.getSSEConnections[session.GetID()]
	if exists {
		// Cancel existing connection
		existingConn.cancelFunc()
	}

	// Create new GET SSE connection
	lastEventID := r.Header.Get(httputil.LastEventIDHeader)
	conn := &getSSEConnection{
		writer:       w,
		flusher:      flusher,
		ctx:          connCtx,
		cancelFunc:   cancelConn,
		lastEventID:  lastEventID,
		sseResponder: newSSEResponder(),
	}
	h.getSSEConnections[session.GetID()] = conn
	h.getSSEConnectionsLock.Unlock()

	// Record connection information
	h.logger.Infof("Established GET SSE connection, session ID: %s", session.GetID())

	// If there's Last-Event-ID, try to resume stream
	if lastEventID != "" {
		h.handleStreamResumption(connCtx, conn, session.GetID())
	}

	// Wait for connection to close
	<-connCtx.Done()

	// Clean up connection
	h.getSSEConnectionsLock.Lock()
	delete(h.getSSEConnections, session.GetID())
	h.getSSEConnectionsLock.Unlock()
	h.logger.Infof("GET SSE connection closed, session ID: %s", session.GetID())
}

// Send notification through GET SSE
func (h *httpServerHandler) sendNotificationToGetSSE(sessionID string, notification *JSONRPCNotification) error {
	h.getSSEConnectionsLock.RLock()
	conn, ok := h.getSSEConnections[sessionID]
	h.getSSEConnectionsLock.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	conn.writeLock.Lock()
	defer conn.writeLock.Unlock()

	// Use SSE responder to send notification
	eventID, err := conn.sseResponder.sendNotification(conn.writer, notification)
	if err != nil {
		return fmt.Errorf("failed to send notification via SSE: %w", err)
	}

	// Update last event ID
	conn.lastEventID = eventID
	return nil
}

// Handle SSE stream resumption
func (h *httpServerHandler) handleStreamResumption(ctx context.Context, conn *getSSEConnection, sessionID string) {
	// Get session
	session, ok := h.sessionManager.getSession(sessionID)
	if !ok {
		h.logger.Infof("Stream resumption failed: session %s not found", sessionID)
		return
	}

	// Add session to context (this is useful for future code that might use session through context)
	// Note: Currently not used in this context, but we keep this code to ensure context has session information
	_ = setSessionToContext(ctx, session)

	// Implement resumption logic, re-sending messages based on lastEventID
	// This needs to be handled according to the server's storage/cache mechanism
	h.logger.Infof("Resuming session %s GET SSE stream, event ID: %s", sessionID, conn.lastEventID)

	// Create params for the notification
	params := map[string]interface{}{
		"resumedFrom": conn.lastEventID,
	}

	// Create NotificationParams struct
	notification := Notification{
		Method: "stream/resumed",
		Params: NotificationParams{
			AdditionalFields: params,
		},
	}

	// Create strictly conforming JSON-RPC 2.0 notification object
	// Use core.NewNotification to ensure jsonrpc field is set to "2.0"
	jsonNotification := newJSONRPCNotification(notification)

	// Validate notification object format before sending
	notifBytes, _ := json.Marshal(notification)
	h.logger.Infof("Preparing to send stream resumption notification: %s", string(notifBytes))

	// Send notification
	err := h.sendNotificationToGetSSE(sessionID, jsonNotification)
	if err != nil {
		h.logger.Infof("Failed to send stream resumption notification: %v", err)
	}
}

// sendEmptyResponse sends an empty response with the specified status code
func (h *httpServerHandler) sendEmptyResponse(w http.ResponseWriter, statusCode int, session Session) {
	if !h.isStateless && session != nil {
		w.Header().Set(httputil.SessionIDHeader, session.GetID())
	}
	w.WriteHeader(statusCode)
}

// sendNotificationResponse sends a 202 Accepted response for notifications
func (h *httpServerHandler) sendNotificationResponse(w http.ResponseWriter, session Session) {
	if !h.isStateless && session != nil {
		w.Header().Set(httputil.SessionIDHeader, session.GetID())
	}
	w.WriteHeader(http.StatusAccepted)
}

// sessionEventNotifier defines the interface for receiving session event notifications
type sessionEventNotifier interface {
	// Called when session terminates
	onSessionTerminated(sessionID string)
}

// sendNotification sends notification to GET SSE connection
func (h *httpServerHandler) sendNotification(sessionID string, notification *JSONRPCNotification) error {
	// Directly send notification through GET SSE, without distinguishing notification type
	return h.sendNotificationToGetSSE(sessionID, notification)
}

// getActiveSessions gets all active session IDs
func (h *httpServerHandler) getActiveSessions() []string {
	if h.sessionManager == nil {
		return []string{}
	}
	return h.sessionManager.getActiveSessions()
}

// Clean up resources when session terminates
func (h *httpServerHandler) cleanupSession(sessionID string) {
	// close GET SSE connection
	h.getSSEConnectionsLock.Lock()
	if conn, exists := h.getSSEConnections[sessionID]; exists {
		conn.cancelFunc()
		delete(h.getSSEConnections, sessionID)
	}
	h.getSSEConnectionsLock.Unlock()
}

// isValidPath validates if the request path matches the configured server path.
func (h *httpServerHandler) isValidPath(requestPath string) bool {
	if h.serverPath == "" {
		return true
	}
	return requestPath == h.serverPath
}
