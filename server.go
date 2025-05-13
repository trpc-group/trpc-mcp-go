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

// ServerConfig stores all server configuration options
type ServerConfig struct {
	// Basic configuration
	Addr       string
	PathPrefix string

	// Session related
	SessionManager *SessionManager
	EnableSession  bool
	IsStateless    bool

	// Response related
	POSTSSEEnabled         bool
	GetSSEEnabled          bool
	DefaultResponseMode    string
	NotificationBufferSize int
}

// Server MCP server
type Server struct {
	serverInfo      Implementation     // Server information.
	config          *ServerConfig      // Configuration.
	logger          Logger             // Logger for the server and subcomponents.
	httpHandler     *HTTPServerHandler // HTTP handler.
	mcpHandler      *MCPHandler        // MCP handler.
	toolManager     *ToolManager       // Tool manager.
	resourceManager *ResourceManager   // Resource manager.
	promptManager   *PromptManager     // Prompt manager.
	customServer    *http.Server       // Custom HTTP server.
}

// NewServer creates a new MCP server
func NewServer(addr string, serverInfo Implementation, options ...ServerOption) *Server {
	// Create default configuration
	config := &ServerConfig{
		Addr:                   addr,
		PathPrefix:             "/mcp",
		EnableSession:          true,
		IsStateless:            false,
		POSTSSEEnabled:         true,
		GetSSEEnabled:          true,
		DefaultResponseMode:    "sse",
		NotificationBufferSize: 10,
	}

	// Create server
	server := &Server{
		serverInfo: serverInfo,
		config:     config,
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
	s.toolManager = NewToolManager()

	// Create resource manager.
	s.resourceManager = NewResourceManager()

	// Create prompt manager.
	s.promptManager = NewPromptManager()

	// Create lifecycle manager, inject logger if provided.
	var lifecycleManager *LifecycleManager
	if s.logger != nil {
		lifecycleManager = NewLifecycleManager(s.serverInfo).WithLogger(s.logger)
	} else {
		lifecycleManager = NewLifecycleManager(s.serverInfo)
	}

	// Create MCP handler.
	s.mcpHandler = NewMCPHandler(
		WithToolManager(s.toolManager),
		WithLifecycleManager(lifecycleManager),
		WithResourceManager(s.resourceManager),
		WithPromptManager(s.promptManager),
	)

	// Collect HTTP handler options.
	var httpOptions []func(*HTTPServerHandler)

	// Session configuration.
	if !s.config.EnableSession {
		httpOptions = append(httpOptions, WithoutTransportSession())
	} else if s.config.SessionManager != nil {
		httpOptions = append(httpOptions, WithTransportSessionManager(s.config.SessionManager))
	}

	// State mode configuration.
	if s.config.IsStateless {
		httpOptions = append(httpOptions, WithTransportStatelessMode())
	}

	// Response type configuration.
	httpOptions = append(httpOptions,
		WithServerSSEEnabled(s.config.POSTSSEEnabled),
		WithTransportGetSSEEnabled(s.config.GetSSEEnabled),
		WithServerDefaultResponseMode(s.config.DefaultResponseMode),
		WithTransportNotificationBufferSize(s.config.NotificationBufferSize),
	)

	// Inject logger into HTTPServerHandler if provided.
	if s.logger != nil {
		httpOptions = append(httpOptions, WithServerTransportLogger(s.logger)) // This is the HTTPServerHandler option version.
	}

	// Create HTTP handler.
	s.httpHandler = NewHTTPServerHandler(s.mcpHandler, httpOptions...)

	// Set server instance as the tool manager's server provider.
	s.toolManager.WithServerProvider(s)
}

// ServerOption server option function.
type ServerOption func(*Server)

// WithServerLogger sets the logger for the server and all subcomponents.
// This is a ServerOption, and should not be confused with WithServerTransportLogger for HTTPServerHandler.
func WithServerLogger(logger Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

// WithSessionManager sets the session manager
func WithSessionManager(manager *SessionManager) ServerOption {
	return func(s *Server) {
		s.config.SessionManager = manager
		s.config.EnableSession = true
	}
}

// WithoutSession disables session
func WithoutSession() ServerOption {
	return func(s *Server) {
		s.config.EnableSession = false
		s.config.SessionManager = nil
	}
}

// WithPathPrefix sets the API path prefix
func WithPathPrefix(prefix string) ServerOption {
	return func(s *Server) {
		s.config.PathPrefix = prefix
	}
}

// WithSSEEnabled enables or disables SSE responses
func WithSSEEnabled(enabled bool) ServerOption {
	return func(s *Server) {
		s.config.POSTSSEEnabled = enabled
	}
}

// WithGetSSEEnabled enables or disables GET SSE
func WithGetSSEEnabled(enabled bool) ServerOption {
	return func(s *Server) {
		s.config.GetSSEEnabled = enabled
	}
}

// WithDefaultResponseMode sets the default response mode ("json" or "sse")
func WithDefaultResponseMode(mode string) ServerOption {
	return func(s *Server) {
		s.config.DefaultResponseMode = mode
	}
}

// WithNotificationBufferSize sets the notification buffer size
func WithNotificationBufferSize(size int) ServerOption {
	return func(s *Server) {
		s.config.NotificationBufferSize = size
	}
}

// WithStatelessMode sets whether the server uses stateless mode
// In stateless mode, the server won't generate session IDs and won't validate session IDs in client requests
// Each request will use a temporary session, which is only valid during request processing
func WithStatelessMode(enabled bool) ServerOption {
	return func(s *Server) {
		s.config.IsStateless = enabled
	}
}

// Start starts the server
func (s *Server) Start() error {
	if s.customServer != nil {
		s.customServer.Handler = s.Handler()
		return s.customServer.ListenAndServe()
	}
	return http.ListenAndServe(s.config.Addr, s.Handler())
}

// RegisterTool registers a tool
func (s *Server) RegisterTool(tool *Tool) error {
	return s.toolManager.RegisterTool(tool)
}

// RegisterResource registers a resource
//
// The resource feature is automatically enabled when the first resource is registered, no additional configuration is needed.
// When the resource feature is enabled but no resources are registered, client requests will return an empty list rather than an error.
func (s *Server) RegisterResource(resource *Resource) error {
	return s.resourceManager.RegisterResource(resource)
}

// RegisterPrompt registers a prompt
//
// The prompt feature is automatically enabled when the first prompt is registered, no additional configuration is needed.
// When the prompt feature is enabled but no prompts are registered, client requests will return an empty list rather than an error.
func (s *Server) RegisterPrompt(prompt *Prompt) error {
	return s.promptManager.RegisterPrompt(prompt)
}

// SendNotification sends a notification to a specific session
func (s *Server) SendNotification(sessionID string, method string, params map[string]interface{}) error {
	// Create notification object
	notification := NewJSONRPCNotificationFromMap(method, params)

	// Use the internal httpHandler to send
	return s.httpHandler.SendNotification(sessionID, notification)
}

// NewNotification creates a new notification object
func (s *Server) NewNotification(method string, params map[string]interface{}) *JSONRPCNotification {
	return NewJSONRPCNotificationFromMap(method, params)
}

// BroadcastNotification broadcasts a notification to all active sessions
func (s *Server) BroadcastNotification(method string, params map[string]interface{}) (int, error) {
	notification := NewJSONRPCNotificationFromMap(method, params)

	// Get active sessions
	sessions, err := s.getActiveSessions()
	if err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %w", err)
	}

	var failedCount int
	var lastError error

	// Send to each session
	for _, sessionID := range sessions {
		if err := s.httpHandler.SendNotification(sessionID, notification); err != nil {
			failedCount++
			lastError = err
		}
	}

	// If all sends failed, return the last error
	if failedCount == len(sessions) && lastError != nil {
		return 0, fmt.Errorf("%w: %w", ErrBroadcastFailed, lastError)
	}

	// Return the number of successful sends
	return len(sessions) - failedCount, nil
}

// getActiveSessions gets all active session IDs
func (s *Server) getActiveSessions() ([]string, error) {
	// Check if in stateless mode
	if s.config.IsStateless {
		return nil, ErrStatelessMode
	}

	// Use the API provided by HTTPServerHandler to get active sessions
	return s.httpHandler.GetActiveSessions(), nil
}

// SendFilteredNotification sends a notification to sessions passing a filter
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

	var successCount, failedCount int
	var lastError error

	// Filter and send to each session
	for _, sessionID := range sessions {
		// Apply filter
		if filter != nil && !filter(sessionID) {
			continue
		}

		// Send notification
		if err := s.httpHandler.SendNotification(sessionID, notification); err != nil {
			failedCount++
			lastError = err
		} else {
			successCount++
		}
	}

	// If all sends failed and we attempted at least one send, return the last error
	if failedCount > 0 && successCount == 0 && lastError != nil {
		return 0, failedCount, fmt.Errorf("%w: %w", ErrFilteredNotificationFailed, lastError)
	}

	// Return the count of successful and failed sends
	return successCount, failedCount, nil
}

// Handler returns the http.Handler for the server.
// This can be used to integrate the MCP server into existing HTTP servers.
func (s *Server) Handler() http.Handler {
	return s.httpHandler
}

// WithCustomServer sets a custom HTTP server
func WithCustomServer(srv *http.Server) ServerOption {
	return func(s *Server) {
		s.customServer = srv
	}
}

// MCPHandler returns the MCP handler
func (s *Server) MCPHandler() *MCPHandler {
	return s.mcpHandler
}

// HTTPHandler returns the HTTP handler
func (s *Server) HTTPHandler() http.Handler {
	return s.httpHandler
}

// WithContext enriches a context with server-specific information
func (s *Server) WithContext(ctx context.Context) context.Context {
	return ctx
}
