// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
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
	serverInfo       Implementation
	logger           Logger
	contextFunc      StdioContextFunc
	toolManager      *toolManager
	resourceManager  *resourceManager
	promptManager    *promptManager
	lifecycleManager *lifecycleManager
	internal         messageHandler
}

// messageHandler defines the core interface for handling JSON-RPC messages (internal use).
type messageHandler interface {
	// HandleRequest processes a JSON-RPC request and returns a response
	HandleRequest(ctx context.Context, rawMessage json.RawMessage) (interface{}, error)

	// HandleNotification processes a JSON-RPC notification
	HandleNotification(ctx context.Context, rawMessage json.RawMessage) error
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
		logger:           config.logger,
		contextFunc:      config.contextFunc,
		toolManager:      toolManager,
		resourceManager:  resourceManager,
		promptManager:    promptManager,
		lifecycleManager: lifecycleManager,
	}

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
	s.logger.Infof("Registered tool: %s", tool.Name)
}

// RegisterPrompt registers a prompt with its handler using the prompt manager.
func (s *StdioServer) RegisterPrompt(prompt *Prompt, handler promptHandler) {
	if prompt == nil || handler == nil {
		s.logger.Errorf("RegisterPrompt: prompt and handler cannot be nil")
		return
	}
	s.promptManager.registerPrompt(prompt, handler)
	s.logger.Infof("Registered prompt: %s", prompt.Name)
}

// RegisterResource registers a resource with its handler using the resource manager.
func (s *StdioServer) RegisterResource(resource *Resource, handler resourceHandler) {
	if resource == nil || handler == nil {
		s.logger.Errorf("RegisterResource: resource and handler cannot be nil")
		return
	}
	s.resourceManager.registerResource(resource, handler)
	s.logger.Infof("Registered resource: %s", resource.URI)
}

// RegisterResourceTemplate registers a resource template with its handler.
func (s *StdioServer) RegisterResourceTemplate(template *ResourceTemplate, handler resourceTemplateHandler) {
	if template == nil || handler == nil {
		s.logger.Errorf("RegisterResourceTemplate: template and handler cannot be nil")
		return
	}
	s.resourceManager.registerTemplate(template, handler)
	s.logger.Infof("Registered resource template: %s", template.Name)
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
	initialized   atomic.Bool
	mu            sync.RWMutex
	Cancelers     sync.Map 
}

func (s *stdioSession) getID() string {
	return s.id
}

func (s *stdioSession) getCreatedAt() time.Time {
	return s.createdAt
}

func (s *stdioSession) getLastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActivity
}

func (s *stdioSession) updateActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActivity = time.Now()
}

func (s *stdioSession) getData(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.data[key]
	return value, exists
}

func (s *stdioSession) setData(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]interface{})
	}
	s.data[key] = value
}

// Session interface implementation methods (exported for interface compliance)
func (s *stdioSession) GetID() string {
	return s.getID()
}

func (s *stdioSession) GetCreatedAt() time.Time {
	return s.getCreatedAt()
}

func (s *stdioSession) GetLastActivity() time.Time {
	return s.getLastActivity()
}

func (s *stdioSession) UpdateActivity() {
	s.updateActivity()
}

func (s *stdioSession) GetData(key string) (interface{}, bool) {
	return s.getData(key)
}

func (s *stdioSession) SetData(key string, value interface{}) {
	s.setData(key, value)
}

func (s *stdioSession) RegisterCanceler(requestID any, cancelFunc context.CancelFunc) {
	s.Cancelers.Store(requestID, cancelFunc)
}

func (s *stdioSession) CancelRequest(requestID any) {
	if cancelFunc, ok := s.Cancelers.LoadAndDelete(requestID); ok {
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
		}
	}
}

func (s *stdioSession) CleanupRequest(requestID any) {
	s.Cancelers.Delete(requestID)
}

func (s *stdioSession) CancelAll() {
	s.Cancelers.Range(func(key, value interface{}) bool {
		if cf, ok := value.(context.CancelFunc); ok {
			cf()
		}
		s.Cancelers.Delete(key)
		return true
	})
}

func (s *stdioSession) SessionID() string {
	return s.id
}

func (s *stdioSession) NotificationChannel() chan<- JSONRPCNotification {
	return s.notifications
}

func (s *stdioSession) Initialize() {
	s.initialized.Store(true)
}

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
	go s.handleNotifications(ctx, stdout)

	return s.processInputStream(ctx, reader, stdout)
}

// handleNotifications processes notifications from the session's notification channel.
func (s *stdioTransport) handleNotifications(ctx context.Context, stdout io.Writer) {
	for {
		select {
		case notification := <-s.session.notifications:
			if err := s.writeResponse(notification, stdout); err != nil {
				s.logger.Errorf("Error writing notification: %v", err)
			}
		case <-ctx.Done():
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

		if err := s.processMessage(ctx, line, stdout); err != nil {
			if err == io.EOF {
				return nil
			}
			s.logger.Errorf("Error handling message: %v", err)
		}
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
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				select {
				case errChan <- err:
				case <-done:
				}
				return
			}
			select {
			case readChan <- line:
			case <-done:
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

	sessionCtx := context.WithValue(ctx, sessionKey{}, s.session)

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

	case JSONRPCMessageTypeNotification:
		if err := s.server.HandleNotification(sessionCtx, rawMessage); err != nil {
			s.logger.Errorf("Error handling notification: %v", err)
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

// sessionKey is an internal context key (unexported).
type sessionKey struct{}

// sessionFromContext extracts the stdio session from context (internal use).
func sessionFromContext(ctx context.Context) *stdioSession {
	if session, ok := ctx.Value(sessionKey{}).(*stdioSession); ok {
		return session
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

	s.parent.logger.Infof("Received notification: %s", notification.Method)
	return nil
}

func (s *stdioServerInternal) handlePing(ctx context.Context, request JSONRPCRequest) (interface{}, error) {
	return newJSONRPCResponse(request.ID, struct{}{}), nil
}
