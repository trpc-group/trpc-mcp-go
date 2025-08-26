// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// StdioServer provides API for STDIO MCP servers.
type StdioServer struct {
	serverInfo           Implementation
	logger               Logger
	contextFunc          StdioContextFunc
	toolManager          *toolManager
	resourceManager      *resourceManager
	promptManager        *promptManager
	lifecycleManager     *lifecycleManager
	internal             messageHandler
	requestID            atomic.Int64                         // Request ID counter for generating unique request IDs.
	responses            map[uint64]interface{}               // Map for storing response channels.
	responsesMu          sync.RWMutex                         // Mutex for responses map.
	notificationHandlers map[string]ServerNotificationHandler // Map of notification handlers by method name.
	notificationMu       sync.RWMutex                         // Mutex for notification handlers map.

	// STDIO transport components for sending requests to client.
	outputMu sync.Mutex // Mutex for protecting stdout writer.
}

// messageHandler defines the core interface for handling JSON-RPC messages (internal use).
type messageHandler interface {
	// HandleRequest processes a JSON-RPC request and returns a response
	HandleRequest(ctx context.Context, rawMessage json.RawMessage) (interface{}, error)

	// HandleNotification processes a JSON-RPC notification
	HandleNotification(ctx context.Context, rawMessage json.RawMessage) error

	// HandleResponse processes a JSON-RPC response (for server-to-client requests).
	HandleResponse(ctx context.Context, rawMessage json.RawMessage) error
}

// stdioServerConfig contains configuration for the STDIO server.
type stdioServerConfig struct {
	logger      Logger
	contextFunc StdioContextFunc
}

// StdioServerOption defines an option function for configuring StdioServer.
type StdioServerOption func(*stdioServerConfig)

// WithStdioServerLogger sets a custom logger for the STDIO server.
func WithStdioServerLogger(logger Logger) StdioServerOption {
	return func(config *stdioServerConfig) {
		config.logger = logger
	}
}

// WithStdioContext sets a context function for the STDIO server.
func WithStdioContext(fn StdioContextFunc) StdioServerOption {
	return func(config *stdioServerConfig) {
		config.contextFunc = fn
	}
}

// StdioContextFunc defines a function that can modify the context for stdio requests.
type StdioContextFunc func(ctx context.Context) context.Context

// NewStdioServer creates a new high-level STDIO server that reuses existing managers.
func NewStdioServer(name, version string, options ...StdioServerOption) *StdioServer {
	config := &stdioServerConfig{
		logger:      GetDefaultLogger(),
		contextFunc: nil,
	}

	for _, option := range options {
		option(config)
	}

	// Create reusable managers (same as HTTP server).
	toolManager := newToolManager()
	resourceManager := newResourceManager()
	promptManager := newPromptManager()
	lifecycleManager := newLifecycleManager(Implementation{
		Name:    name,
		Version: version,
	})

	// Set up manager relationships.
	lifecycleManager.withToolManager(toolManager)
	lifecycleManager.withResourceManager(resourceManager)
	lifecycleManager.withPromptManager(promptManager)
	lifecycleManager.withLogger(config.logger)

	server := &StdioServer{
		serverInfo: Implementation{
			Name:    name,
			Version: version,
		},
		logger:               config.logger,
		contextFunc:          config.contextFunc,
		toolManager:          toolManager,
		resourceManager:      resourceManager,
		promptManager:        promptManager,
		lifecycleManager:     lifecycleManager,
		responses:            make(map[uint64]interface{}),
		notificationHandlers: make(map[string]ServerNotificationHandler),
	}

	// Set server as server provider for toolManager (to inject server context in tool calls).
	toolManager.withServerProvider(server)

	server.internal = &stdioServerInternal{
		parent: server,
	}

	return server
}

// RegisterTool registers a tool with its handler using the tool manager.
func (s *StdioServer) RegisterTool(tool *Tool, handler toolHandler) {
	if tool == nil || handler == nil {
		s.logger.Errorf("RegisterTool: tool and handler cannot be nil")
		return
	}
	s.toolManager.registerTool(tool, handler)
	s.logger.Debugf("Registered tool: %s", tool.Name)
}

// UnregisterTools removes multiple tools by names and logs the operation.
func (s *StdioServer) UnregisterTools(names ...string) error {
	if len(names) == 0 {
		err := fmt.Errorf("no tool names provided")
		return err
	}

	unregisteredCount := s.toolManager.unregisterTools(names...)
	if unregisteredCount == 0 {
		err := fmt.Errorf("none of the specified tools were found")
		return err
	}

	return nil
}

// RegisterPrompt registers a prompt with its handler using the prompt manager.
func (s *StdioServer) RegisterPrompt(prompt *Prompt, handler promptHandler) {
	if prompt == nil || handler == nil {
		s.logger.Errorf("RegisterPrompt: prompt and handler cannot be nil")
		return
	}
	s.promptManager.registerPrompt(prompt, handler)
	s.logger.Debugf("Registered prompt: %s", prompt.Name)
}

// RegisterResource registers a resource with its handler using the resource manager.
func (s *StdioServer) RegisterResource(resource *Resource, handler resourceHandler) {
	if resource == nil || handler == nil {
		s.logger.Errorf("RegisterResource: resource and handler cannot be nil")
		return
	}
	s.resourceManager.registerResource(resource, handler)
	s.logger.Debugf("Registered resource: %s", resource.URI)
}

// RegisterResources registers a resource with its handler for multiple contents using the resource manager.
func (s *StdioServer) RegisterResources(resource *Resource, handler resourcesHandler) {
	if resource == nil || handler == nil {
		s.logger.Errorf("RegisterResources: resource and handler cannot be nil")
		return
	}
	s.resourceManager.registerResources(resource, handler)
	s.logger.Debugf("Registered resources: %s", resource.URI)
}

// RegisterResourceTemplate registers a resource template with its handler.
func (s *StdioServer) RegisterResourceTemplate(template *ResourceTemplate, handler resourceTemplateHandler) {
	if template == nil || handler == nil {
		s.logger.Errorf("RegisterResourceTemplate: template and handler cannot be nil")
		return
	}
	s.resourceManager.registerTemplate(template, handler)
	s.logger.Debugf("Registered resource template: %s", template.Name)
}

// Start starts the STDIO server.
func (s *StdioServer) Start() error {
	return serveStdio(s.internal, withStdioErrorLogger(s.logger), withStdioContextFunc(s.contextFunc))
}

// StartWithContext starts the STDIO server with context.
func (s *StdioServer) StartWithContext(ctx context.Context) error {
	return serveStdioWithContext(ctx, s.internal, withStdioErrorLogger(s.logger), withStdioContextFunc(s.contextFunc))
}

// GetServerInfo returns the server information.
func (s *StdioServer) GetServerInfo() Implementation {
	return s.serverInfo
}

// withContext enriches a context with server-specific information.
// Implements serverProvider interface for injecting server context in tool calls.
func (s *StdioServer) withContext(ctx context.Context) context.Context {
	return setServerToContext(ctx, s)
}

// stdioTransport is a low-level JSON-RPC transport for STDIO communication.
type stdioTransport struct {
	server      messageHandler
	logger      Logger
	contextFunc StdioContextFunc
	session     *stdioSession
}

// stdioServerTransportOption configures a stdioTransport.
type stdioServerTransportOption func(*stdioTransport)

// withStdioErrorLogger sets the error logger for the transport.
func withStdioErrorLogger(logger Logger) stdioServerTransportOption {
	return func(s *stdioTransport) {
		s.logger = logger
	}
}

// withStdioContextFunc sets a context transformation function.
func withStdioContextFunc(fn StdioContextFunc) stdioServerTransportOption {
	return func(s *stdioTransport) {
		s.contextFunc = fn
	}
}

// stdioSession represents a stdio session implementing the Session interface.
type stdioSession struct {
	id            string
	createdAt     time.Time
	lastActivity  time.Time
	data          map[string]interface{}
	notifications chan JSONRPCNotification
	messages      chan JSONRPCMessage // for sending any JSON-RPC message.
	initialized   atomic.Bool
	clientInfo    atomic.Value
	mu            sync.RWMutex
}

// GetID returns the session ID.
func (s *stdioSession) GetID() string {
	return s.id
}

// GetCreatedAt returns the session creation time.
func (s *stdioSession) GetCreatedAt() time.Time {
	return s.createdAt
}

// GetLastActivity returns the last activity time.
func (s *stdioSession) GetLastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActivity
}

// UpdateActivity updates the last activity time.
func (s *stdioSession) UpdateActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActivity = time.Now()
}

// GetData returns the data for the given key.
func (s *stdioSession) GetData(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.data[key]
	return value, exists
}

// SetData sets the data for the given key.
func (s *stdioSession) SetData(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]interface{})
	}
	s.data[key] = value
}

// NotificationChannel returns a channel for sending notifications to the client.
func (s *stdioSession) NotificationChannel() chan<- JSONRPCNotification {
	return s.notifications
}

// MessageChannel returns a channel for sending any JSON-RPC message to the client.
func (s *stdioSession) MessageChannel() chan<- JSONRPCMessage {
	return s.messages
}

// Initialize initializes the session.
func (s *stdioSession) Initialize() {
	s.initialized.Store(true)
}

// Initialized returns true if the session is initialized.
func (s *stdioSession) Initialized() bool {
	return s.initialized.Load()
}

// newStdioTransport creates a new stdio transport.
func newStdioTransport(server messageHandler, options ...stdioServerTransportOption) *stdioTransport {
	now := time.Now()
	transport := &stdioTransport{
		server: server,
		logger: GetDefaultLogger(),
		session: &stdioSession{
			id:            "stdio",
			createdAt:     now,
			lastActivity:  now,
			data:          make(map[string]interface{}),
			notifications: make(chan JSONRPCNotification, 100),
			messages:      make(chan JSONRPCMessage, 100),
		},
	}

	for _, option := range options {
		option(transport)
	}

	return transport
}

// listen starts listening for JSON-RPC messages on stdin and writes responses to stdout.
func (s *stdioTransport) listen(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	if s.contextFunc != nil {
		ctx = s.contextFunc(ctx)
	}

	reader := bufio.NewReader(stdin)
	go s.handleOutgoingMessages(ctx, stdout)

	return s.processInputStream(ctx, reader, stdout)
}

// handleOutgoingMessages processes all outgoing messages (notifications and other JSON-RPC messages)
// from the session's channels and writes them to stdout.
func (s *stdioTransport) handleOutgoingMessages(ctx context.Context, stdout io.Writer) {
	for {
		select {
		case notification := <-s.session.notifications:
			if err := s.writeResponse(notification, stdout); err != nil {
				s.logger.Errorf("handleOutgoingMessages: Error writing notification: %v", err)
			}
		case message := <-s.session.messages:
			// Handle any JSON-RPC message (request, response, notification).
			if err := s.writeResponse(message, stdout); err != nil {
				s.logger.Errorf("handleOutgoingMessages: Error writing message: %v", err)
			}
		case <-ctx.Done():
			s.logger.Debugf("handleOutgoingMessages: Context done, exiting")
			return
		}
	}
}

// processInputStream reads and processes messages from the input stream.
func (s *stdioTransport) processInputStream(ctx context.Context, reader *bufio.Reader, stdout io.Writer) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		line, err := s.readNextLine(ctx, reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// Process message asynchronously to avoid blocking input reading.
		go func(line string) {
			if err := s.processMessage(ctx, line, stdout); err != nil {
				if err == io.EOF {
					return
				}
				s.logger.Errorf("Error handling message: %v", err)
			}
		}(line)
	}
}

// readNextLine reads a single line from the input reader.
func (s *stdioTransport) readNextLine(ctx context.Context, reader *bufio.Reader) (string, error) {
	readChan := make(chan string, 1)
	errChan := make(chan error, 1)
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-done:
			s.logger.Debugf("readNextLine: Goroutine cancelled\n")
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				s.logger.Debugf("readNextLine: ReadString error: %v\n", err)
				select {
				case errChan <- err:
				case <-done:
				}
				return
			}
			select {
			case readChan <- line:
			case <-done:
				s.logger.Debugf("readNextLine: Done before sending to channel\n")
			}
		}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case err := <-errChan:
		return "", err
	case line := <-readChan:
		return line, nil
	}
}

// processMessage processes a single JSON-RPC message.
func (s *stdioTransport) processMessage(ctx context.Context, line string, writer io.Writer) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	var rawMessage json.RawMessage
	if err := json.Unmarshal([]byte(line), &rawMessage); err != nil {
		s.logger.Errorf("Invalid JSON received: %v", err)
		return nil
	}

	msgType, err := parseJSONRPCMessageType(rawMessage)
	if err != nil {
		s.logger.Errorf("Error parsing message type: %v", err)
		return nil
	}

	sessionCtx := setSessionToContext(ctx, s.session)

	switch msgType {
	case JSONRPCMessageTypeRequest:
		response, err := s.server.HandleRequest(sessionCtx, rawMessage)
		if err != nil {
			s.logger.Errorf("Error handling request: %v", err)
			return nil
		}
		if response != nil {
			return s.writeResponse(response, writer)
		}
		s.logger.Debugf("processMessage: No response generated\n")

	case JSONRPCMessageTypeNotification:
		if err := s.server.HandleNotification(sessionCtx, rawMessage); err != nil {
			s.logger.Errorf("Error handling notification: %v", err)
		}

	case JSONRPCMessageTypeResponse, JSONRPCMessageTypeError:
		// Handle responses to server-initiated requests (like roots/list).
		if err := s.server.HandleResponse(sessionCtx, rawMessage); err != nil {
			s.logger.Errorf("Error handling response: %v", err)
		}
	}

	return nil
}

// writeResponse writes a response to the output writer.
func (s *stdioTransport) writeResponse(response interface{}, writer io.Writer) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error marshaling response: %w", err)
	}

	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("error writing response: %w", err)
	}

	if _, err := writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("error writing newline: %w", err)
	}

	// Force flush buffer to ensure immediate delivery.
	if file, ok := writer.(*os.File); ok {
		// Use syscall to force flush.
		if err := file.Sync(); err != nil {
			// Sync may fail for pipes, try to ignore.
			s.logger.Debugf("writeResponse: Sync failed (expected for pipes): %v\n", err)
		}
	}

	// For any writer, try to flush if it has a Flush method.
	if flusher, ok := writer.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			s.logger.Debugf("writeResponse: Error flushing: %v\n", err)
		}
	}
	return nil
}

// serveStdio is a convenience function to start a stdio server.
func serveStdio(server messageHandler, options ...stdioServerTransportOption) error {
	return serveStdioWithContext(context.Background(), server, options...)
}

// serveStdioWithContext starts a stdio server with the provided context.
func serveStdioWithContext(ctx context.Context, server messageHandler, options ...stdioServerTransportOption) error {
	transport := newStdioTransport(server, options...)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return transport.listen(ctx, os.Stdin, os.Stdout)
}

// sessionFromContext extracts the stdio session from context.
func sessionFromContext(ctx context.Context) *stdioSession {
	// Use the global GetSessionFromContext function to ensure consistent context key.
	if session, ok := GetSessionFromContext(ctx); ok {
		if stdioSession, ok := session.(*stdioSession); ok {
			return stdioSession
		}
	}
	return nil
}

// stdioServerInternal implements messageHandler.
type stdioServerInternal struct {
	parent *StdioServer
}

// HandleRequest implements messageHandler.HandleRequest by delegating to existing managers.
func (s *stdioServerInternal) HandleRequest(ctx context.Context, rawMessage json.RawMessage) (interface{}, error) {
	var request JSONRPCRequest
	if err := json.Unmarshal(rawMessage, &request); err != nil {
		return newJSONRPCErrorResponse(nil, -32700, "Parse error", nil), nil
	}

	s.parent.logger.Debugf("Handling request: %s (ID: %v)", request.Method, request.ID)

	// Get session from context for managers that need it.
	session := sessionFromContext(ctx)

	var result interface{}
	var err error

	switch request.Method {
	case MethodInitialize:
		result, err = s.parent.lifecycleManager.handleInitialize(ctx, &request, session)
	case MethodToolsList:
		result, err = s.parent.toolManager.handleListTools(ctx, &request, session)
	case MethodToolsCall:
		result, err = s.parent.toolManager.handleCallTool(ctx, &request, session)
	case MethodPromptsList:
		result, err = s.parent.promptManager.handleListPrompts(ctx, &request)
	case MethodPromptsGet:
		result, err = s.parent.promptManager.handleGetPrompt(ctx, &request)
	case MethodResourcesList:
		result, err = s.parent.resourceManager.handleListResources(ctx, &request)
	case MethodResourcesRead:
		result, err = s.parent.resourceManager.handleReadResource(ctx, &request)
	case MethodPing:
		return s.handlePing(ctx, request)
	default:
		return newJSONRPCErrorResponse(request.ID, -32601, "Method not found", nil), nil
	}

	if err != nil {
		return newJSONRPCErrorResponse(request.ID, -32603, "Internal error", err.Error()), nil
	}

	// Check if result is already a JSON-RPC response or error (has jsonrpc field).
	switch result.(type) {
	case *JSONRPCResponse, *JSONRPCError, JSONRPCResponse, JSONRPCError:
		return result, nil
	}

	// Wrap the result in a proper JSON-RPC response.
	return newJSONRPCResponse(request.ID, result), nil
}

// HandleNotification implements messageHandler.HandleNotification.
func (s *stdioServerInternal) HandleNotification(ctx context.Context, rawMessage json.RawMessage) error {
	var notification JSONRPCNotification
	if err := json.Unmarshal(rawMessage, &notification); err != nil {
		return err
	}

	s.parent.logger.Debugf("Received notification: %s", notification.Method)

	// Check if there's a registered handler for this notification method.
	s.parent.notificationMu.RLock()
	handler, exists := s.parent.notificationHandlers[notification.Method]
	s.parent.notificationMu.RUnlock()

	if exists {
		go func() {
			// Call the handler with the notification pointer and context.
			if err := handler(ctx, &notification); err != nil {
				s.parent.logger.Errorf("Error handling notification %s: %v", notification.Method, err)
			}
		}()
	} else {
		s.parent.logger.Warnf("Received notification with no handler registered: %s", notification.Method)
	}

	return nil
}

// HandleResponse handles JSON-RPC responses from the client (for server-to-client requests).
func (s *stdioServerInternal) HandleResponse(ctx context.Context, rawMessage json.RawMessage) error {
	var response struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   interface{} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(rawMessage, &response); err != nil {
		s.parent.logger.Errorf("HandleResponse: Error unmarshaling response: %v", err)
		return err
	}

	// Convert ID to uint64.
	requestIDUint, ok := s.parseRequestID(response.ID)
	if !ok {
		s.parent.logger.Errorf("HandleResponse: Invalid request ID in response: %v", response.ID)
		return nil
	}

	// Get the response channel.
	s.parent.responsesMu.RLock()
	responseChanInterface, exists := s.parent.responses[requestIDUint]
	s.parent.responsesMu.RUnlock()

	if !exists {
		s.parent.logger.Warnf("HandleResponse: Received response for unknown request ID: %d", requestIDUint)
		return nil
	}

	// Type assert to the correct channel type.
	responseChan, ok := responseChanInterface.(chan *json.RawMessage)
	if !ok {
		s.parent.logger.Errorf("HandleResponse: Invalid response channel type for request ID: %d", requestIDUint)
		return nil
	}

	// Handle error response.
	if response.Error != nil {
		// Create error result.
		errorBytes, _ := json.Marshal(map[string]interface{}{
			"error": response.Error,
		})
		errorMessage := json.RawMessage(errorBytes)

		select {
		case responseChan <- &errorMessage:
		default:
			s.parent.logger.Errorf("HandleResponse: Failed to deliver error response: channel full or closed")
		}
	} else if response.Result != nil {
		// Handle success response.
		resultBytes, err := json.Marshal(response.Result)
		if err != nil {
			s.parent.logger.Errorf("HandleResponse: Failed to marshal response result: %v", err)
		} else {
			resultMessage := json.RawMessage(resultBytes)
			select {
			case responseChan <- &resultMessage:
			default:
				s.parent.logger.Errorf("HandleResponse: Failed to deliver response: channel full or closed")
			}
		}
	} else {
		// Invalid response - neither error nor result.
		s.parent.logger.Errorf("HandleResponse: Invalid JSON-RPC response: missing both result and error for ID: %d", requestIDUint)
	}

	return nil
}

// parseRequestID safely converts interface{} to uint64.
func (s *stdioServerInternal) parseRequestID(id interface{}) (uint64, bool) {
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

func (s *stdioServerInternal) handlePing(ctx context.Context, request JSONRPCRequest) (interface{}, error) {
	return newJSONRPCResponse(request.ID, struct{}{}), nil
}

// ListRoots sends a request to the client asking for its list of roots.
func (s *StdioServer) ListRoots(ctx context.Context) (*ListRootsResult, error) {
	// Get the session from context.
	session := ClientSessionFromContext(ctx)
	if session == nil {
		return nil, ErrNoClientSession
	}

	sessionID := session.GetID()
	if sessionID == "" {
		return nil, fmt.Errorf("session has no ID")
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
	response, err := s.SendRequest(ctx, request)
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

// SendRequest sends a JSON-RPC request to the client and waits for response.
func (s *StdioServer) SendRequest(ctx context.Context, request *JSONRPCRequest) (*json.RawMessage, error) {
	// Generate unique request ID if not provided.
	if request.ID == nil {
		request.ID = s.requestID.Add(1)
	}
	// Get session from context.
	session := sessionFromContext(ctx)
	if session == nil {
		return nil, fmt.Errorf("no session available for request")
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

	// Clean up the response channel when done.
	defer func() {
		s.responsesMu.Lock()
		delete(s.responses, requestIDUint)
		s.responsesMu.Unlock()
	}()

	select {
	case session.MessageChannel() <- request:
		// Wait for the response or timeout.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case response := <-resultChan:
			return response, nil
		case <-time.After(30 * time.Second):
			return nil, fmt.Errorf("SendRequest: Request timeout for ID %d after 30s", requestIDUint)
		}
	default:
		return nil, fmt.Errorf("failed to send request: MessageChannel full")
	}
}

// RegisterNotificationHandler registers a handler for the specified notification method.
// This allows the server to respond to client notifications.
func (s *StdioServer) RegisterNotificationHandler(method string, handler ServerNotificationHandler) {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()

	// Initialize the map if it's nil.
	if s.notificationHandlers == nil {
		s.notificationHandlers = make(map[string]ServerNotificationHandler)
	}

	s.notificationHandlers[method] = handler
	s.logger.Debugf("Registered notification handler for method: %s", method)
}

// UnregisterNotificationHandler removes a handler for the specified notification method.
func (s *StdioServer) UnregisterNotificationHandler(method string) {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()

	if s.notificationHandlers != nil {
		delete(s.notificationHandlers, method)
		s.logger.Debugf("Unregistered notification handler for method: %s", method)
	}
}
