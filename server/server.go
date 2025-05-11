package server

import (
	"context"
	"fmt"
	"net/http"

	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/protocol"
	"trpc.group/trpc-go/trpc-mcp-go/transport"
)

// ServerConfig stores all server configuration options
type ServerConfig struct {
	// Basic configuration
	Addr       string
	PathPrefix string

	// Session related
	SessionManager *transport.SessionManager
	EnableSession  bool
	IsStateless    bool

	// Response related
	SSEEnabled             bool
	GetSSEEnabled          bool
	DefaultResponseMode    string
	NotificationBufferSize int
}

// Server MCP server
type Server struct {
	// Server information
	serverInfo mcp.Implementation

	// Configuration
	config *ServerConfig

	// HTTP handler
	httpHandler *transport.HTTPServerHandler

	// MCP handler
	mcpHandler *protocol.MCPHandler

	// Tool manager
	toolManager *protocol.ToolManager

	// Resource manager
	resourceManager *protocol.ResourceManager

	// Prompt manager
	promptManager *protocol.PromptManager
}

// ServerOption server option function
type ServerOption func(*Server)

// NewServer creates a new MCP server
func NewServer(addr string, serverInfo mcp.Implementation, options ...ServerOption) *Server {
	// Create default configuration
	config := &ServerConfig{
		Addr:                   addr,
		PathPrefix:             "/mcp",
		EnableSession:          true,
		IsStateless:            false,
		SSEEnabled:             true,
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

// initComponents initializes server components based on configuration
func (s *Server) initComponents() {
	// Create tool manager
	s.toolManager = protocol.NewToolManager()

	// Create resource manager
	s.resourceManager = protocol.NewResourceManager()

	// Create prompt manager
	s.promptManager = protocol.NewPromptManager()

	// Create lifecycle manager
	lifecycleManager := protocol.NewLifecycleManager(s.serverInfo)

	// Create MCP handler
	s.mcpHandler = protocol.NewMCPHandler(
		protocol.WithToolManager(s.toolManager),
		protocol.WithLifecycleManager(lifecycleManager),
		protocol.WithResourceManager(s.resourceManager),
		protocol.WithPromptManager(s.promptManager),
	)

	// Collect HTTP handler options
	var httpOptions []func(*transport.HTTPServerHandler)

	// Session configuration
	if !s.config.EnableSession {
		httpOptions = append(httpOptions, transport.WithoutSession())
	} else if s.config.SessionManager != nil {
		httpOptions = append(httpOptions, transport.WithSessionManager(s.config.SessionManager))
	}

	// State mode configuration
	if s.config.IsStateless {
		httpOptions = append(httpOptions, transport.WithStatelessMode())
	}

	// Response type configuration
	httpOptions = append(httpOptions,
		transport.WithServerSSEEnabled(s.config.SSEEnabled),
		transport.WithGetSSEEnabled(s.config.GetSSEEnabled),
		transport.WithServerDefaultResponseMode(s.config.DefaultResponseMode),
		transport.WithNotificationBufferSize(s.config.NotificationBufferSize),
	)

	// Create HTTP handler
	s.httpHandler = transport.NewHTTPServerHandler(s.mcpHandler, httpOptions...)

	// Set server instance as the tool manager's server provider
	s.toolManager.WithServerProvider(s)
}

// WithSessionManager sets the session manager
func WithSessionManager(manager *transport.SessionManager) ServerOption {
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
		s.config.SSEEnabled = enabled
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

// RegisterTool registers a tool
func (s *Server) RegisterTool(tool *mcp.Tool) error {
	return s.toolManager.RegisterTool(tool)
}

// RegisterResource registers a resource
//
// The resource feature is automatically enabled when the first resource is registered, no additional configuration is needed.
// When the resource feature is enabled but no resources are registered, client requests will return an empty list rather than an error.
func (s *Server) RegisterResource(resource *mcp.Resource) error {
	return s.resourceManager.RegisterResource(resource)
}

// RegisterPrompt registers a prompt
//
// The prompt feature is automatically enabled when the first prompt is registered, no additional configuration is needed.
// When the prompt feature is enabled but no prompts are registered, client requests will return an empty list rather than an error.
func (s *Server) RegisterPrompt(prompt *mcp.Prompt) error {
	return s.promptManager.RegisterPrompt(prompt)
}

// SendNotification sends a notification to a specific session
func (s *Server) SendNotification(sessionID string, method string, params map[string]interface{}) error {
	// Create notification object
	notification := mcp.NewJSONRPCNotificationFromMap(method, params)

	// Use the internal httpHandler to send
	return s.httpHandler.SendNotification(sessionID, notification)
}

// NewNotification creates a new notification object
func (s *Server) NewNotification(method string, params map[string]interface{}) *mcp.JSONRPCNotification {
	return mcp.NewJSONRPCNotificationFromMap(method, params)
}

// BroadcastNotification broadcasts a notification to all active sessions
func (s *Server) BroadcastNotification(method string, params map[string]interface{}) (int, error) {
	notification := mcp.NewJSONRPCNotificationFromMap(method, params)

	// Get active sessions
	sessions, err := s.getActiveSessions()
	if err != nil {
		return 0, err
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
		return 0, fmt.Errorf("failed to broadcast notification: %v", lastError)
	}

	// Return the number of successful sends
	return len(sessions) - failedCount, nil
}

// getActiveSessions gets all active session IDs
func (s *Server) getActiveSessions() ([]string, error) {
	// Check if in stateless mode
	if s.config.IsStateless {
		return nil, fmt.Errorf("cannot get active sessions in stateless mode")
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
	notification := mcp.NewJSONRPCNotificationFromMap(method, params)

	// Get active sessions
	sessions, err := s.getActiveSessions()
	if err != nil {
		return 0, 0, err
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
		return 0, failedCount, fmt.Errorf("failed to send filtered notification: %v", lastError)
	}

	// Return the count of successful and failed sends
	return successCount, failedCount, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Create HTTP server
	server := &http.Server{
		Addr:    s.config.Addr,
		Handler: s.httpHandler,
	}

	// Start server
	return server.ListenAndServe()
}

// MCPHandler returns the MCP handler
func (s *Server) MCPHandler() *protocol.MCPHandler {
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
