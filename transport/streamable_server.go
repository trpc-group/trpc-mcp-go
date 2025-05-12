package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

// RequestHandler interface defines a component that handles requests
type RequestHandler interface {
	// Handle a request
	HandleRequest(ctx context.Context, req *mcp.JSONRPCRequest, session *Session) (mcp.JSONRPCMessage, error)

	// Handle a notification
	HandleNotification(ctx context.Context, notification *mcp.JSONRPCNotification, session *Session) error
}

// HTTPServerHandler implements an HTTP server handler
type HTTPServerHandler struct {
	// Session manager
	sessionManager *SessionManager

	// Request handler
	requestHandler RequestHandler

	// Whether sessions are enabled
	enableSession bool

	// Whether stateless mode is used
	isStateless bool

	// Responder factory
	responderFactory *ResponderFactory

	// Notification channel buffer size
	notificationBufferSize int

	// Whether GET SSE is enabled
	enableGetSSE bool

	// Session GET SSE connections mapping (each session ID maps to a GET SSE connection)
	getSSEConnections     map[string]*GetSSEConnection
	getSSEConnectionsLock sync.RWMutex
}

// GetSSEConnection represents a GET SSE connection
type GetSSEConnection struct {
	writer      http.ResponseWriter
	flusher     http.Flusher
	ctx         context.Context
	cancelFunc  context.CancelFunc
	lastEventID string

	// Prevent concurrent write conflicts
	writeLock sync.Mutex

	// Event ID generator, reuses existing SSEResponder
	sseResponder *SSEResponder
}

// NewHTTPServerHandler creates an HTTP server handler
func NewHTTPServerHandler(handler RequestHandler, options ...func(*HTTPServerHandler)) *HTTPServerHandler {
	h := &HTTPServerHandler{
		requestHandler:         handler,
		enableSession:          true,                  // Default: sessions enabled
		isStateless:            false,                 // Default: stateful mode
		responderFactory:       NewResponderFactory(), // Default: use JSON responses
		notificationBufferSize: 10,                    // Default notification buffer size
		enableGetSSE:           true,                  // Default: GET SSE enabled
		getSSEConnections:      make(map[string]*GetSSEConnection),
	}

	// Apply options
	for _, option := range options {
		option(h)
	}

	// After applying all options, ensure ResponderFactory uses correct stateless mode setting
	h.responderFactory = NewResponderFactory(
		WithSSEEnabled(h.responderFactory.enableSSE),
		WithDefaultResponseMode(h.responderFactory.defaultMode),
		WithFactoryStatelessMode(h.isStateless),
	)

	// If sessions are enabled but no session manager is set, create a default one
	if h.enableSession && h.sessionManager == nil {
		h.sessionManager = NewSessionManager(3600) // Default: 1 hour expiry
	}

	return h
}

// WithSessionManager sets the session manager
func WithSessionManager(manager *SessionManager) func(*HTTPServerHandler) {
	return func(h *HTTPServerHandler) {
		h.sessionManager = manager
	}
}

// WithoutSession disables sessions
func WithoutSession() func(*HTTPServerHandler) {
	return func(h *HTTPServerHandler) {
		h.enableSession = false
		h.sessionManager = nil
	}
}

// WithServerSSEEnabled sets whether SSE responses are enabled
func WithServerSSEEnabled(enabled bool) func(*HTTPServerHandler) {
	return func(h *HTTPServerHandler) {
		h.responderFactory = NewResponderFactory(WithSSEEnabled(enabled))
	}
}

// WithServerDefaultResponseMode sets the default response mode
func WithServerDefaultResponseMode(mode string) func(*HTTPServerHandler) {
	return func(h *HTTPServerHandler) {
		h.responderFactory = NewResponderFactory(WithDefaultResponseMode(mode))
	}
}

// WithGetSSEEnabled sets whether GET SSE is enabled
func WithGetSSEEnabled(enabled bool) func(*HTTPServerHandler) {
	return func(h *HTTPServerHandler) {
		h.enableGetSSE = enabled
	}
}

// WithNotificationBufferSize sets the notification buffer size
func WithNotificationBufferSize(size int) func(*HTTPServerHandler) {
	return func(h *HTTPServerHandler) {
		h.notificationBufferSize = size
	}
}

// WithStatelessMode sets the server to stateless mode
// In stateless mode, the server does not generate persistent session IDs; each request uses a temporary session
func WithStatelessMode() func(*HTTPServerHandler) {
	return func(h *HTTPServerHandler) {
		h.isStateless = true
		// In stateless mode, sessions are still enabled but use temporary sessions
		h.enableSession = true
		// Ensure there's still a session manager for creating temporary sessions
		if h.sessionManager == nil {
			h.sessionManager = NewSessionManager(3600)
		}
	}
}

// ServeHTTP implements the http.Handler interface
func (h *HTTPServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// First check the HTTP method
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
func (h *HTTPServerHandler) handlePost(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var rawMessage json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		http.Error(w, ErrInvalidRequestBody.Error(), http.StatusBadRequest)
		return
	}

	// Create response context
	respCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var isInitialize bool
	var base baseMessage

	if err := json.Unmarshal(rawMessage, &base); err != nil {
		http.Error(w, ErrInvalidRequestBody.Error(), http.StatusBadRequest)
		return
	}

	// First try to parse as request
	// Check if it's an initialize request
	if base.ID != nil && base.Method == mcp.MethodInitialize {
		isInitialize = true
	}

	// Get session
	var session *Session
	if h.isStateless {
		// Stateless mode: create a temporary session for each request.
		session = NewSession()
	} else if h.enableSession {
		// Stateful mode
		sessionIDHeader := r.Header.Get(SessionIDHeader)
		if sessionIDHeader != "" {
			var ok bool
			session, ok = h.sessionManager.GetSession(sessionIDHeader)
			if !ok {
				http.Error(w, "Session not found or expired", http.StatusNotFound) // 404 if session ID provided but not found
				return
			}
		} else if isInitialize {
			// If it's an initialize request and no session ID header, create a new session
			session = h.sessionManager.CreateSession()
			log.Infof("Created new session ID: %s for initialize request", session.ID)
		} else {
			// Not an initialize request and no session ID header was provided.
			// According to MCP spec, server SHOULD respond with 400 Bad Request.
			http.Error(w, "Missing Mcp-Session-Id header for non-initialize request", http.StatusBadRequest)
			return
		}
	}

	// Create response processor
	responder := h.responderFactory.CreateResponder(r, rawMessage)

	// This is a request
	if base.ID != nil && base.Method != "" {
		var req mcp.JSONRPCRequest
		if err := json.Unmarshal(rawMessage, &req); err != nil {
			http.Error(w, "Invalid JSON-RPC request format: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Check response processor type
		if sseResponder, ok := responder.(*SSEResponder); ok {
			// Use SSE response mode
			// Check if streaming is supported
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming not supported", http.StatusInternalServerError)
				return
			}

			// Set SSE response headers
			w.Header().Set(ContentTypeHeader, ContentTypeSSE)
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			// Only send session ID in non-stateless mode
			if !h.isStateless && session != nil {
				w.Header().Set(SessionIDHeader, session.ID)
			}

			// Create SSE notification sender, inject into request context
			var sessionID string
			if session != nil {
				sessionID = session.ID
			}
			notificationSender := NewSSENotificationSender(w, flusher, sessionID)
			reqCtx := mcp.WithNotificationSender(ctx, notificationSender)

			// Add session to context
			if session != nil {
				reqCtx = SetSessionToContext(reqCtx, session)
			}

			// Directly sync process request
			resp, err := h.requestHandler.HandleRequest(reqCtx, &req, session)
			if err != nil {
				log.Infof("Request processing failed: %v", err)
				errorResp := mcp.NewJSONRPCErrorResponse(req.ID, mcp.ErrInternal, "Internal server error", nil)
				// Send error response
				err = sseResponder.Respond(ctx, w, r, errorResp, session)
				if err != nil {
					log.Infof("Failed to send SSE error response: %v", err)
				}
				return
			}
			// Send final response
			jsonrpcResponse := mcp.JSONRPCResponse{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      req.ID,
				Result:  resp,
			}
			err = sseResponder.Respond(ctx, w, r, jsonrpcResponse, session)
			if err != nil {
				log.Infof("Failed to send SSE final response: %v", err)
			}
			return
		}

		// Use normal JSON response mode
		// Create a NoOp notification sender for JSON mode
		noopSender := &NoopNotificationSender{}
		reqCtx := mcp.WithNotificationSender(ctx, noopSender)

		// Add session to context
		if session != nil {
			reqCtx = SetSessionToContext(reqCtx, session)
		}

		resp, err := h.requestHandler.HandleRequest(reqCtx, &req, session)
		if err != nil {
			log.Infof("Request processing failed: %v", err)
			errorResp := mcp.NewJSONRPCErrorResponse(req.ID, mcp.ErrInternal, "Internal server error", nil)
			responder.Respond(respCtx, w, r, errorResp, session)
			return
		}

		jsonrpcResponse := mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion,
			ID:      req.ID,
			Result:  resp,
		}

		responder.Respond(respCtx, w, r, jsonrpcResponse, session)
		return
	}

	// Try to parse as notification
	if base.ID == nil && base.Method != "" {
		var notification mcp.JSONRPCNotification
		if err := json.Unmarshal(rawMessage, &notification); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Handle special case for initialize request
		if notification.Method == mcp.MethodNotificationsInitialized {
			// This is initialized notification, if it's the first request, we need to create a session
			if h.enableSession && session == nil {
				log.Info("Warning: Received initialized notification but no active session")
			}

			// Directly return 202 Accepted, instead of using responder.Respond
			h.sendNotificationResponse(w, session)
			return
		}

		// Add session to context
		notificationCtx := ctx
		if session != nil {
			notificationCtx = SetSessionToContext(ctx, session)
		}

		// Handle other notifications
		if err := h.requestHandler.HandleNotification(notificationCtx, &notification, session); err != nil {
			log.Infof("Notification processing failed: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return 202 Accepted, use helper function instead of responder.Respond
		h.sendNotificationResponse(w, session)
		return
	}

	// Unable to parse request
	http.Error(w, "Invalid JSON-RPC message", http.StatusBadRequest)
}

// handleDelete handles DELETE requests
func (h *HTTPServerHandler) handleDelete(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Get session ID from session ID header
	sessionID := r.Header.Get(SessionIDHeader)
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	// Get session
	if h.enableSession {
		// Terminate session
		if h.sessionManager.TerminateSession(sessionID) {
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
func (h *HTTPServerHandler) handleGet(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
	sessionID := r.Header.Get(SessionIDHeader)
	if sessionID == "" {
		http.Error(w, "No session ID provided", http.StatusBadRequest)
		return
	}

	// Get session
	session, ok := h.sessionManager.GetSession(sessionID)
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
	w.Header().Set(ContentTypeHeader, ContentTypeSSE)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set(SessionIDHeader, session.ID)
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Create context, for canceling connection
	connCtx, cancelConn := context.WithCancel(ctx)
	defer cancelConn()

	// Check if there's already a GET SSE connection
	h.getSSEConnectionsLock.Lock()
	existingConn, exists := h.getSSEConnections[session.ID]
	if exists {
		// Cancel existing connection
		existingConn.cancelFunc()
	}

	// Create new GET SSE connection
	lastEventID := r.Header.Get(LastEventIDHeader)
	conn := &GetSSEConnection{
		writer:       w,
		flusher:      flusher,
		ctx:          connCtx,
		cancelFunc:   cancelConn,
		lastEventID:  lastEventID,
		sseResponder: NewSSEResponder(),
	}
	h.getSSEConnections[session.ID] = conn
	h.getSSEConnectionsLock.Unlock()

	// Record connection information
	log.Infof("Established GET SSE connection, session ID: %s", session.ID)

	// If there's Last-Event-ID, try to resume stream
	if lastEventID != "" {
		h.handleStreamResumption(connCtx, conn, session.ID)
	}

	// Wait for connection to close
	<-connCtx.Done()

	// Clean up connection
	h.getSSEConnectionsLock.Lock()
	delete(h.getSSEConnections, session.ID)
	h.getSSEConnectionsLock.Unlock()
	log.Infof("GET SSE connection closed, session ID: %s", session.ID)
}

// Send notification through GET SSE
func (h *HTTPServerHandler) sendNotificationToGetSSE(sessionID string, notification *mcp.JSONRPCNotification) error {
	h.getSSEConnectionsLock.RLock()
	conn, ok := h.getSSEConnections[sessionID]
	h.getSSEConnectionsLock.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	conn.writeLock.Lock()
	defer conn.writeLock.Unlock()

	// Use SSE responder to send notification
	eventID, err := conn.sseResponder.SendNotification(conn.writer, conn.flusher, notification)
	if err != nil {
		return fmt.Errorf("failed to send notification via SSE: %w", err)
	}

	// Update last event ID
	conn.lastEventID = eventID
	return nil
}

// Handle SSE stream resumption
func (h *HTTPServerHandler) handleStreamResumption(ctx context.Context, conn *GetSSEConnection, sessionID string) {
	// Get session
	session, ok := h.sessionManager.GetSession(sessionID)
	if !ok {
		log.Infof("Stream resumption failed: session %s not found", sessionID)
		return
	}

	// Add session to context (this is useful for future code that might use session through context)
	// Note: Currently not used in this context, but we keep this code to ensure context has session information
	_ = SetSessionToContext(ctx, session)

	// Implement resumption logic, re-sending messages based on lastEventID
	// This needs to be handled according to the server's storage/cache mechanism
	log.Infof("Resuming session %s GET SSE stream, event ID: %s", sessionID, conn.lastEventID)

	// Create params for the notification
	params := map[string]interface{}{
		"resumedFrom": conn.lastEventID,
	}

	// Create NotificationParams struct
	notification := mcp.Notification{
		Method: "stream/resumed",
		Params: mcp.NotificationParams{
			AdditionalFields: params,
		},
	}

	// Create strictly conforming JSON-RPC 2.0 notification object
	// Use core.NewNotification to ensure jsonrpc field is set to "2.0"
	jsonNotification := mcp.NewJSONRPCNotification(notification)

	// Validate notification object format before sending
	notifBytes, _ := json.Marshal(notification)
	log.Infof("Preparing to send stream resumption notification: %s", string(notifBytes))

	// Send notification
	err := h.sendNotificationToGetSSE(sessionID, jsonNotification)
	if err != nil {
		log.Infof("Failed to send stream resumption notification: %v", err)
	}
}

// sendEmptyResponse sends an empty response (retained for backward compatibility)
func (h *HTTPServerHandler) sendEmptyResponse(w http.ResponseWriter, statusCode int, session *Session) {
	// Only send session ID in non-stateless mode
	if !h.isStateless && session != nil {
		w.Header().Set(SessionIDHeader, session.ID)
	}

	w.WriteHeader(statusCode)
}

// sendNotificationResponse sends notification response (directly returns 202 status code)
func (h *HTTPServerHandler) sendNotificationResponse(w http.ResponseWriter, session *Session) {
	// Only send session ID in non-stateless mode
	if !h.isStateless && session != nil {
		w.Header().Set(SessionIDHeader, session.ID)
	}

	w.WriteHeader(http.StatusAccepted)
}

// SessionEventNotifier defines the interface for receiving session event notifications
type SessionEventNotifier interface {
	// Called when session terminates
	OnSessionTerminated(sessionID string)
}

// SendNotification sends notification to GET SSE connection
func (h *HTTPServerHandler) SendNotification(sessionID string, notification *mcp.JSONRPCNotification) error {
	// Directly send notification through GET SSE, without distinguishing notification type
	return h.sendNotificationToGetSSE(sessionID, notification)
}

// GetActiveSessions gets all active session IDs
func (h *HTTPServerHandler) GetActiveSessions() []string {
	if h.sessionManager == nil {
		return []string{}
	}
	return h.sessionManager.GetActiveSessions()
}

// Clean up resources when session terminates
func (h *HTTPServerHandler) cleanupSession(sessionID string) {
	// Close GET SSE connection
	h.getSSEConnectionsLock.Lock()
	if conn, exists := h.getSSEConnections[sessionID]; exists {
		conn.cancelFunc()
		delete(h.getSSEConnections, sessionID)
	}
	h.getSSEConnectionsLock.Unlock()
}
