// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// Common errors
var (
	ErrStatelessMode              = errors.New("cannot get active sessions in stateless mode")
	ErrBroadcastFailed            = errors.New("failed to broadcast notification")
	ErrFilteredNotificationFailed = errors.New("failed to send filtered notification")
)

const (
	// defaultServerAddress is the default address for the server
	defaultServerAddress = "localhost:3000"
	// defaultServerPath is the default API path prefix
	defaultServerPath = "/mcp"
	// defaultNotificationBufferSize is the default size of the notification buffer
	defaultNotificationBufferSize = 10
)

// HTTPContextFunc defines a function type that extracts information from HTTP request to context.
// This function is called before each HTTP request processing, allowing users to extract information
// from HTTP request and add it to the context.
// Multiple HTTPContextFunc will be executed in the order they are registered.
type HTTPContextFunc func(ctx context.Context, r *http.Request) context.Context

// serverConfig stores all server configuration options
type serverConfig struct {
	// Basic configuration
	addr string
	path string

	// Session related
	sessionManager sessionManager
	EnableSession  bool
	isStateless    bool

	// Response related
	postSSEEnabled         bool
	getSSEEnabled          bool
	notificationBufferSize int

	// HTTP context functions for extracting information from HTTP requests
	httpContextFuncs []HTTPContextFunc

	// Tool list filter function
	toolListFilter ToolListFilter

	// Method name modifier for external customization.
	methodNameModifier MethodNameModifier
}

// Server MCP server
type Server struct {
	serverInfo      Implementation     // Server information.
	config          *serverConfig      // Configuration.
	logger          Logger             // Logger for the server and subcomponents.
	httpHandler     *httpServerHandler // HTTP handler.
	mcpHandler      *mcpHandler        // MCP handler.
	toolManager     *toolManager       // Tool manager.
	resourceManager *resourceManager   // Resource manager.
	promptManager   *promptManager     // Prompt manager.
	customServer    *http.Server       // Custom HTTP server.
}

// NewServer creates a new MCP server
func NewServer(name, version string, options ...ServerOption) *Server {
	// Create default configuration
	config := &serverConfig{
		addr:                   defaultServerAddress,
		path:                   defaultServerPath,
		EnableSession:          true,
		isStateless:            false,
		postSSEEnabled:         true,
		getSSEEnabled:          true,
		notificationBufferSize: defaultNotificationBufferSize,
	}

	// Create server with provided serverInfo
	server := &Server{
		serverInfo: Implementation{
			Name:    name,
			Version: version,
		},
		config: config,
	}

	// Apply options
	for _, option := range options {
		option(server)
	}

	// Initialize components
	server.initComponents()

	return server
}

// initComponents initializes server components based on configuration.
func (s *Server) initComponents() {
	// Create tool manager.
	s.toolManager = newToolManager()

	// Set tool list filter if configured
	if s.config.toolListFilter != nil {
		s.toolManager.withToolListFilter(s.config.toolListFilter)
	}

	// Set method name modifier if configured.
	if s.config.methodNameModifier != nil {
		s.toolManager.withMethodNameModifier(s.config.methodNameModifier)
	}

	// Create resource manager.
	s.resourceManager = newResourceManager()

	// Create prompt manager.
	s.promptManager = newPromptManager()

	// Create lifecycle manager, inject logger if provided.
	var lifecycleManager *lifecycleManager
	if s.logger != nil {
		lifecycleManager = newLifecycleManager(s.serverInfo).withLogger(s.logger)
	} else {
		lifecycleManager = newLifecycleManager(s.serverInfo)
	}

	// Configure stateless mode for lifecycle manager.
	if s.config.isStateless {
		lifecycleManager = lifecycleManager.withStatelessMode(true)
	}

	// Create MCP handler.
	s.mcpHandler = newMCPHandler(
		withToolManager(s.toolManager),
		withLifecycleManager(lifecycleManager),
		withResourceManager(s.resourceManager),
		withPromptManager(s.promptManager),
	)

	// Collect HTTP handler options.
	var httpOptions []func(*httpServerHandler)

	// Session configuration.
	if !s.config.EnableSession {
		httpOptions = append(httpOptions, withoutTransportSession())
	} else if s.config.sessionManager != nil {
		httpOptions = append(httpOptions, withTransportSessionManager(s.config.sessionManager))
	}

	// State mode configuration.
	if s.config.isStateless {
		httpOptions = append(httpOptions, withTransportStatelessMode())
	}

	// Response type configuration.
	httpOptions = append(httpOptions,
		withServerPOSTSSEEnabled(s.config.postSSEEnabled),
		withTransportGetSSEEnabled(s.config.getSSEEnabled),
		withTransportNotificationBufferSize(s.config.notificationBufferSize),
	)

	// HTTP context functions configuration.
	if len(s.config.httpContextFuncs) > 0 {
		httpOptions = append(httpOptions, withTransportHTTPContextFuncs(s.config.httpContextFuncs))
	}

	// Inject logger into httpServerHandler if provided.
	if s.logger != nil {
		// This is the httpServerHandler option version.
		httpOptions = append(httpOptions, withServerTransportLogger(s.logger))
	}

	// Create HTTP handler.
	s.httpHandler = newHTTPServerHandler(s.mcpHandler, s.config.path, httpOptions...)

	// Set server instance as the tool manager's server provider.
	s.toolManager.withServerProvider(s)
}

// ServerOption server option function.
type ServerOption func(*Server)

// WithServerLogger sets the logger for the server and all subcomponents.
// This is a ServerOption and should not be confused with withServerTransportLogger for httpServerHandler.
func WithServerLogger(logger Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

// WithoutSession disables session.
func WithoutSession() ServerOption {
	return func(s *Server) {
		s.config.EnableSession = false
		s.config.sessionManager = nil
	}
}

// WithServerPath sets the API path prefix
func WithServerPath(prefix string) ServerOption {
	return func(s *Server) {
		s.config.path = prefix
	}
}

// WithPostSSEEnabled enables or disables SSE responses.
func WithPostSSEEnabled(enabled bool) ServerOption {
	return func(s *Server) {
		s.config.postSSEEnabled = enabled
	}
}

// WithGetSSEEnabled enables or disables GET SSE.
func WithGetSSEEnabled(enabled bool) ServerOption {
	return func(s *Server) {
		s.config.getSSEEnabled = enabled
	}
}

// WithNotificationBufferSize sets the notification buffer size
func WithNotificationBufferSize(size int) ServerOption {
	return func(s *Server) {
		s.config.notificationBufferSize = size
	}
}

// WithHTTPContextFunc adds an HTTP context function that will be called for each HTTP request.
// Multiple functions can be registered and they will be executed in the order they are added.
// Each function receives the current context and HTTP request, and returns a potentially modified context.
func WithHTTPContextFunc(fn HTTPContextFunc) ServerOption {
	return func(s *Server) {
		s.config.httpContextFuncs = append(s.config.httpContextFuncs, fn)
	}
}

// WithStatelessMode sets whether the server uses stateless mode
// In stateless mode, the server won't generate session IDs and won't validate session IDs in client requests
// Each request will use a temporary session, which is only valid during request processing
func WithStatelessMode(enabled bool) ServerOption {
	return func(s *Server) {
		s.config.isStateless = enabled
	}
}

// WithToolListFilter sets a tool list filter that will be applied to tools/list requests.
// The filter function receives the request context and all registered tools, and should
// return a filtered list of tools that should be visible to the client.
//
// The context may contain user information extracted from HTTP headers (in stateless mode)
// or session information (in stateful mode). Use helper functions like GetUserRole()
// and GetClientVersion() to extract relevant information.
//
// Example:
//
//	server := mcp.NewServer("my-server", "1.0",
//	    mcp.WithHTTPContextFunc(extractUserInfo), // Extract user info from headers
//	    mcp.WithToolListFilter(func(ctx context.Context, tools []*mcp.Tool) []*mcp.Tool {
//	        userRole := mcp.GetUserRole(ctx)
//	        if userRole == "admin" {
//	            return tools // Admin sees all tools
//	        }
//	        return filterUserTools(tools) // Filter for regular users
//	    }),
//	)
func WithToolListFilter(filter ToolListFilter) ServerOption {
	return func(s *Server) {
		s.config.toolListFilter = filter
	}
}

// WithServerAddress sets the server address
func WithServerAddress(addr string) ServerOption {
	return func(s *Server) {
		s.config.addr = addr
	}
}

// Start starts the server
func (s *Server) Start() error {
	if s.customServer != nil {
		s.customServer.Handler = s.Handler()
		return s.customServer.ListenAndServe()
	}
	return http.ListenAndServe(s.config.addr, s.Handler())
}

// RegisterTool registers a tool with its handler function
func (s *Server) RegisterTool(tool *Tool, handler toolHandler) {
	s.toolManager.registerTool(tool, handler)
}

// UnregisterTools removes multiple tools by names and returns an error if no tools were unregistered
func (s *Server) UnregisterTools(names ...string) error {
	if len(names) == 0 {
		return fmt.Errorf("no tool names provided")
	}

	unregisteredCount := s.toolManager.unregisterTools(names...)
	if unregisteredCount == 0 {
		return fmt.Errorf("none of the specified tools were found")
	}

	return nil
}

// RegisterResource registers a resource with its handler function
func (s *Server) RegisterResource(resource *Resource, handler resourceHandler) {
	s.resourceManager.registerResource(resource, handler)
}

// RegisterResourceTemplate registers a resource template with its handler function.
func (s *Server) RegisterResourceTemplate(
	template *ResourceTemplate,
	handler resourceTemplateHandler,
) {
	s.resourceManager.registerTemplate(template, handler)
}

// RegisterPrompt registers a prompt with its handler function
//
// The prompt feature is automatically enabled when the first prompt is registered,
// no additional configuration is needed.
// When the prompt feature is enabled but no prompts are registered, client requests
// will return an empty list rather than an error.
func (s *Server) RegisterPrompt(prompt *Prompt, handler promptHandler) {
	s.promptManager.registerPrompt(prompt, handler)
}

// SendNotification sends a notification to a specific session
func (s *Server) SendNotification(sessionID string, method string, params map[string]interface{}) error {
	// Create a notification object
	notification := NewJSONRPCNotificationFromMap(method, params)

	// Use the internal httpHandler to send
	return s.httpHandler.sendNotification(sessionID, notification)
}

// NewNotification creates a new notification object
func (s *Server) NewNotification(method string, params map[string]interface{}) *JSONRPCNotification {
	return NewJSONRPCNotificationFromMap(method, params)
}

// Send notification to multiple sessions and count failures
func (s *Server) sendNotificationToSessions(sessions []string, notification *JSONRPCNotification) (successCount, failedCount int, lastError error) {
	for _, sessionID := range sessions {
		if err := s.httpHandler.sendNotification(sessionID, notification); err != nil {
			failedCount++
			lastError = err
		} else {
			successCount++
		}
	}
	return
}

// BroadcastNotification with logic unchanged, now using helper
func (s *Server) BroadcastNotification(method string, params map[string]interface{}) (int, error) {
	notification := NewJSONRPCNotificationFromMap(method, params)

	// Get active sessions
	sessions, err := s.getActiveSessions()
	if err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %w", err)
	}

	successCount, failedCount, lastError := s.sendNotificationToSessions(sessions, notification)

	// If all sending failed, return the last error
	if failedCount == len(sessions) && lastError != nil {
		return 0, fmt.Errorf("%w: %w", ErrBroadcastFailed, lastError)
	}

	// Return the number of successful sends
	return successCount, nil
}

// Send notification to filtered sessions and count results
func (s *Server) sendNotificationToFilteredSessions(sessions []string, notification *JSONRPCNotification, filter func(sessionID string) bool) (successCount, failedCount int, lastError error) {
	for _, sessionID := range sessions {
		if filter != nil && !filter(sessionID) {
			continue
		}
		if err := s.httpHandler.sendNotification(sessionID, notification); err != nil {
			failedCount++
			lastError = err
		} else {
			successCount++
		}
	}
	return
}

// SendFilteredNotification with logic unchanged, now using helper
func (s *Server) SendFilteredNotification(
	method string,
	params map[string]interface{},
	filter func(sessionID string) bool,
) (int, int, error) {
	notification := NewJSONRPCNotificationFromMap(method, params)

	// Get active sessions
	sessions, err := s.getActiveSessions()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get active sessions: %w", err)
	}

	successCount, failedCount, lastError := s.sendNotificationToFilteredSessions(sessions, notification, filter)

	// If all sending failed, and we attempted at least one send, return the last error
	if failedCount > 0 && successCount == 0 && lastError != nil {
		return 0, failedCount, fmt.Errorf("%w: %w", ErrFilteredNotificationFailed, lastError)
	}

	// Return the count of successful and failed sends
	return successCount, failedCount, nil
}

// getActiveSessions gets all active session IDs
func (s *Server) getActiveSessions() ([]string, error) {
	// Check if in stateless mode
	if s.config.isStateless {
		return nil, ErrStatelessMode
	}

	// Use the API provided by httpServerHandler to get active sessions
	return s.httpHandler.getActiveSessions(), nil
}

// GetActiveSessions returns all active session IDs.
// Returns an error if the server is in stateless mode.
func (s *Server) GetActiveSessions() ([]string, error) {
	return s.getActiveSessions()
}

// Handler  returns the http.Handler for the server.
// This can be used to integrate the MCP server into existing HTTP servers.
func (s *Server) Handler() http.Handler {
	return s.httpHandler
}

func (s *Server) Path() string {
	return s.config.path
}

// WithCustomServer sets a custom HTTP server
func WithCustomServer(srv *http.Server) ServerOption {
	return func(s *Server) {
		s.customServer = srv
	}
}

// MCPHandler  returns the MCP handler
func (s *Server) MCPHandler() requestHandler {
	return s.mcpHandler
}

// HTTPHandler returns the HTTP handler
func (s *Server) HTTPHandler() http.Handler {
	return s.httpHandler
}

// WithContext enriches a context with server-specific information
func (s *Server) withContext(ctx context.Context) context.Context {
	return setServerToContext(ctx, s)
}

// GetServerInfo returns the server implementation information
func (s *Server) GetServerInfo() Implementation {
	return s.serverInfo
}

// SetMethodNameModifier sets the method name modifier for the server.
// This allows external components to configure method name modification after server creation.
func (s *Server) SetMethodNameModifier(modifier MethodNameModifier) {
	s.config.methodNameModifier = modifier
	if s.toolManager != nil {
		s.toolManager.withMethodNameModifier(modifier)
	}
}
