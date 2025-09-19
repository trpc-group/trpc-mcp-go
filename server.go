// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"

	"golang.org/x/time/rate"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	sh "trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/handler"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/middleware"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/router"
)

// Common errors
var (
	ErrStatelessMode              = errors.New("cannot get active sessions in stateless mode")
	ErrBroadcastFailed            = errors.New("failed to broadcast notification")
	ErrFilteredNotificationFailed = errors.New("failed to send filtered notification")
	ErrNoClientSession            = errors.New("no client session in context")
)

// clientSessionKey is the key for storing client session in context.
type clientSessionKey struct{}

// ClientSessionFromContext retrieves current client session from context.
func ClientSessionFromContext(ctx context.Context) Session {
	if session, ok := ctx.Value(clientSessionKey{}).(Session); ok {
		return session
	}
	return nil
}

// withClientSession adds client session to context.
func withClientSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, clientSessionKey{}, session)
}

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

// AuditConfig defines configuration for audit middleware
type AuditConfig struct {
	// Whether to enable audit logging
	Enabled bool

	// Audit level: none, basic, detailed, full
	Level string

	// Whether to hash sensitive data
	HashSensitiveData bool

	// Whether to include request/response body
	IncludeRequestBody  bool
	IncludeResponseBody bool

	// Custom endpoint patterns to audit
	EndpointPatterns []string

	// Patterns to exclude from auditing
	ExcludePatterns []string

	// Custom metadata extractor function
	MetadataExtractor func(*http.Request) map[string]interface{}

	// Custom risk assessor function
	RiskAssessor func(map[string]interface{}) (string, []string)
}

// BearerAuthConfig defines configuration for Bearer token authentication
type BearerAuthConfig struct {
	Enabled bool

	// Required: Token validator
	Verifier server.TokenVerifierInterface

	// Optional: List of required scopes
	RequiredScopes []string

	// Optional: Write resource_metadata for WWW-Authenticate
	ResourceMetadataURL *string

	// Optional: Restrict accepted issuer (extra authorization check)
	Issuer string

	// Optional: Restrict accepted audiences/resources (extra authorization check)
	Audience []string
}

// OAuthRoutesConfig defines configuration for OAuth 2.1 server routes.
type OAuthRoutesConfig struct {
	// OAuth server implementation
	Provider server.OAuthServerProvider

	// Canonical issuer identifier (iss claim)
	IssuerURL *url.URL

	// Root URL for OAuth endpoints
	BaseURL *url.URL

	// Optional link to service documentation
	ServiceDocumentationURL *url.URL

	// Supported OAuth scopes
	ScopesSupported []string

	// Optional human-readable resource name
	ResourceName *string

	// Rate limit for /authorize endpoint
	AuthorizationRateLimit *rate.Limiter

	// Rate limit for /token endpoint
	TokenRateLimit *rate.Limiter

	// Resolve client_id from refresh token
	ResolveClientIDFromRT func(rt string) (string, bool)

	// Rate limit for dynamic client registration
	RegistrationRateLimit *sh.RegisterRateLimitConfig

	// Rate limit for token revocation
	RevocationRateLimit *sh.RevocationRateLimitConfig
}

// OAuthMetadataConfig defines configuration for exposing OAuth server metadata.
type OAuthMetadataConfig struct {
	// Core OAuth server metadata
	OAuthMetadata OAuthMetadata

	// Optional resource server URL
	ResourceServerURL *url.URL

	// Optional service documentation URL
	ServiceDocumentationURL *url.URL

	// Scopes advertised in metadata
	ScopesSupported []string

	// Optional human-readable resource name
	ResourceName *string
}

type OAuthMetadata = auth.OAuthMetadata

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

	// Audit middleware configuration
	auditConfig *AuditConfig

	// Bearer authenticate configuration
	bearerAuth *BearerAuthConfig

	// Tool list filter function
	toolListFilter ToolListFilter

	// Prompt list filter function
	promptListFilter PromptListFilter

	// Resource list filter function
	resourceListFilter ResourceListFilter

	// Method name modifier for external customization.
	methodNameModifier MethodNameModifier

	// Route installers for adding extra endpoints (e.g. OAuth, metadata).
	routerInstallers []func(*http.ServeMux) error
}

// ServerNotificationHandler defines a function that handles notifications on the server side.
// It follows the same pattern as mcp-go's NotificationHandlerFunc, receiving a context
// that contains session information, allowing handlers to call methods like ListRoots.
type ServerNotificationHandler func(ctx context.Context, notification *JSONRPCNotification) error

// Server MCP server
type Server struct {
	serverInfo           Implementation                       // Server information.
	config               *serverConfig                        // Configuration.
	logger               Logger                               // Logger for the server and subcomponents.
	httpHandler          *httpServerHandler                   // HTTP handler.
	mcpHandler           *mcpHandler                          // MCP handler.
	toolManager          *toolManager                         // Tool manager.
	resourceManager      *resourceManager                     // Resource manager.
	promptManager        *promptManager                       // Prompt manager.
	customServer         *http.Server                         // Custom HTTP server.
	requestID            atomic.Int64                         // Request ID counter for generating unique request IDs.
	notificationHandlers map[string]ServerNotificationHandler // Map of notification handlers by method name.
	notificationMu       sync.RWMutex                         // Mutex for notification handlers map.
	rootHandler          http.Handler                         // Server's top-level HTTP handler including the core MCP endpoint and any extra routes.
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
		auditConfig:            nil,
		bearerAuth:             nil,
	}

	// Create server with provided serverInfo
	server := &Server{
		serverInfo: Implementation{
			Name:    name,
			Version: version,
		},
		config:               config,
		notificationHandlers: make(map[string]ServerNotificationHandler),
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
	// Create lifecycle manager with logger if provided.
	lifecycleManager := newLifecycleManager(s.serverInfo)
	if s.logger != nil {
		lifecycleManager = lifecycleManager.withLogger(s.logger)
	}

	// Configure stateless mode for lifecycle manager.
	if s.config.isStateless {
		lifecycleManager = lifecycleManager.withStatelessMode(true)
	}

	// Create tool manager.
	toolManager := newToolManager()
	toolManager.withServerProvider(s)
	if s.config.methodNameModifier != nil {
		toolManager.withMethodNameModifier(s.config.methodNameModifier)
	}
	if s.config.toolListFilter != nil {
		toolManager.withToolListFilter(s.config.toolListFilter)
	}
	s.toolManager = toolManager

	// Create resource manager.
	resourceManager := newResourceManager()
	// Only set resource list filter if not nil.
	if s.config.resourceListFilter != nil {
		resourceManager.withResourceListFilter(s.config.resourceListFilter)
	}
	s.resourceManager = resourceManager

	promptManager := newPromptManager()
	// Only set prompt list filter if not nil.
	if s.config.promptListFilter != nil {
		promptManager.withPromptListFilter(s.config.promptListFilter)
	}
	s.promptManager = promptManager

	// Create MCP handler.
	s.mcpHandler = newMCPHandler(
		withToolManager(toolManager),
		withLifecycleManager(lifecycleManager),
		withResourceManager(resourceManager),
		withPromptManager(promptManager),
		withServer(s), // Set the server reference for notification handling.
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
		httpOptions = append(httpOptions, withServerTransportLogger(s.logger))
	}

	// Enable Bearer token auth middleware if configured
	if s.config.bearerAuth != nil && s.config.bearerAuth.Enabled {
		authWrap := convertToAuthMiddleware(s.config.bearerAuth)
		httpOptions = append(httpOptions, withTransportAuthEnabled(authWrap))
	}

	// Enable audit logging if configured
	if s.config.auditConfig != nil && s.config.auditConfig.Enabled {
		auditOpts := convertToMiddlewareOptions(s.config.auditConfig)
		httpOptions = append(httpOptions, withTransportAuditEnabled(
			middleware.AuditMiddleware(auditOpts),
		))
	}

	// Create HTTP handler.
	s.httpHandler = newHTTPServerHandler(s.mcpHandler, s.config.path, httpOptions...)

	mux := http.NewServeMux()
	mux.Handle(s.config.path+"/", s.httpHandler)

	// Install additional routes (OAuth, resource metadata, .well-known, etc.)
	for _, install := range s.config.routerInstallers {
		_ = install(mux)
	}

	// Exposing mux externally
	s.customServer = &http.Server{Addr: s.config.addr, Handler: mux}

	// Expose mux as the server's root handler
	s.rootHandler = mux
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

// WithAudit enables audit logging for the server with the specified configuration.
// The audit middleware will log all HTTP requests and responses based on the configuration.
//
// Example:
//
//	server := mcp.NewServer("my-server", "1.0",
//	    mcp.WithAudit(&mcp.AuditConfig{
//	        Enabled: true,
//	        Level: "detailed",
//	        EndpointPatterns: []string{"/mcp/", "/oauth2/"},
//	        HashSensitiveData: true,
//	    }),
//	)
func WithAudit(config *AuditConfig) ServerOption {
	return func(s *Server) {
		s.config.auditConfig = config
	}
}

// WithBearerAuth configures the server to use Bearer token authentication.
// The provided BearerAuthConfig specifies how tokens are verified,
// what scopes are required, and optionally, metadata for WWW-Authenticate responses.
func WithBearerAuth(config *BearerAuthConfig) ServerOption {
	return func(s *Server) {
		s.config.bearerAuth = config
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

// WithPromptListFilter sets a prompt list filter that will be applied to prompts/list requests.
// The filter function receives the request context and all registered prompts, and should
// return a filtered list of prompts that should be visible to the client.
//
// The context may contain user information extracted from HTTP headers (in stateless mode)
// or session information (in stateful mode).
//
// Example:
//
//	server := mcp.NewServer("my-server", "1.0",
//	    mcp.WithHTTPContextFunc(extractUserInfo), // Extract user info from headers
//	    mcp.WithPromptListFilter(func(ctx context.Context, prompts []*mcp.Prompt) []*mcp.Prompt {
//	        userRole := mcp.GetUserRole(ctx)
//	        if userRole == "admin" {
//	            return prompts // Admin sees all prompts
//	        }
//	        return filterUserPrompts(prompts) // Filter for regular users
//	    }),
//	)
func WithPromptListFilter(filter PromptListFilter) ServerOption {
	return func(s *Server) {
		s.config.promptListFilter = filter
	}
}

// WithResourceListFilter sets a resource list filter that will be applied to resources/list requests.
// The filter function receives the request context and all registered resources, and should
// return a filtered list of resources that should be visible to the client.
//
// The context may contain user information extracted from HTTP headers (in stateless mode)
// or session information (in stateful mode).
//
// Example:
//
//	server := mcp.NewServer("my-server", "1.0",
//	    mcp.WithHTTPContextFunc(extractUserInfo), // Extract user info from headers
//	    mcp.WithResourceListFilter(func(ctx context.Context, resources []*mcp.Resource) []*mcp.Resource {
//	        userRole := mcp.GetUserRole(ctx)
//	        if userRole == "admin" {
//	            return resources // Admin sees all resources
//	        }
//	        return filterUserResources(resources) // Filter for regular users
//	    }),
//	)
func WithResourceListFilter(filter ResourceListFilter) ServerOption {
	return func(s *Server) {
		s.config.resourceListFilter = filter
	}
}

// WithServerAddress sets the server address
func WithServerAddress(addr string) ServerOption {
	return func(s *Server) {
		s.config.addr = addr
	}
}

// WithOAuthRoutes installs standard OAuth 2.1 endpoints into the server,
// such as /authorize, /token, /revoke, and /register, depending on the
// provided AuthRouterOptions and the provider's capabilities.
func WithOAuthRoutes(cfg OAuthRoutesConfig) ServerOption {
	return withHTTPRoutes(func(mux *http.ServeMux) error {
		base := cfg.BaseURL
		if base == nil {
			base = cfg.IssuerURL
		}

		opts := router.AuthRouterOptions{
			Provider:                cfg.Provider,
			IssuerUrl:               cfg.IssuerURL,
			BaseUrl:                 base,
			ServiceDocumentationUrl: cfg.ServiceDocumentationURL,
			ScopesSupported:         cfg.ScopesSupported,
			ResourceName:            cfg.ResourceName,

			AuthorizationOptions: &sh.AuthorizationHandlerOptions{
				Provider:  cfg.Provider,
				RateLimit: cfg.AuthorizationRateLimit,
			},
			TokenOptions: &sh.TokenHandlerOptions{
				Provider:  cfg.Provider,
				RateLimit: cfg.TokenRateLimit,
			},
			ClientRegistrationOptions: &sh.ClientRegistrationHandlerOptions{
				ClientsStore: cfg.Provider.ClientsStore(),
				RateLimit:    cfg.RegistrationRateLimit,
			},
			RevocationOptions: &sh.RevocationHandlerOptions{
				Provider:  cfg.Provider,
				RateLimit: cfg.RevocationRateLimit,
			},
		}
		return router.McpAuthRouter(mux, opts)
	})
}

// WithOAuthMetadata installs the .well-known OAuth metadata endpoints
// (e.g. /.well-known/oauth-authorization-server and
// /.well-known/oauth-protected-resource) into the server.
// The returned metadata is constructed from the given AuthMetadataOptions.
func WithOAuthMetadata(cfg OAuthMetadataConfig) ServerOption {
	return withHTTPRoutes(func(mux *http.ServeMux) error {
		opts := router.AuthMetadataOptions{
			OAuthMetadata:           cfg.OAuthMetadata,
			ResourceServerUrl:       cfg.ResourceServerURL,
			ServiceDocumentationUrl: cfg.ServiceDocumentationURL,
			ScopesSupported:         cfg.ScopesSupported,
			ResourceName:            cfg.ResourceName,
		}
		return router.McpAuthMetadataRouter(mux, opts)
	})
}

// Start starts the server
func (s *Server) Start() error {
	if s.customServer != nil {
		if s.customServer.Handler == nil {
			s.customServer.Handler = s.Handler()
		}
		return s.customServer.ListenAndServe()
	}
	return http.ListenAndServe(s.config.addr, s.Handler())
}

// RegisterTool registers a tool with its handler function
func (s *Server) RegisterTool(tool *Tool, handler toolHandler) {
	s.toolManager.registerTool(tool, handler)
}

// GetTool retrieves a registered tool by name.
// Returns the tool and true if found, otherwise returns zero value and false.
// The returned tool is a copy to prevent accidental modification.
func (s *Server) GetTool(name string) (Tool, bool) {
	if name == "" {
		return Tool{}, false
	}

	tool, exists := s.toolManager.getTool(name)
	if !exists {
		return Tool{}, false
	}
	// Return a copy to prevent modification of the original
	return *tool, true
}

// GetTools returns a copy of all registered tools.
// The returned slice contains copies of the tools to prevent accidental modification.
func (s *Server) GetTools() []Tool {
	toolPtrs := s.toolManager.getTools()

	tools := make([]Tool, 0, len(toolPtrs))
	for _, toolPtr := range toolPtrs {
		tools = append(tools, *toolPtr) // getTools() guarantees non-nil
	}
	return tools
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

// RegisterResources registers a resource with its handler function for multiple contents
func (s *Server) RegisterResources(resource *Resource, handler resourcesHandler) {
	s.resourceManager.registerResources(resource, handler)
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

// SendNotification sends a notification to a specific session.
func (s *Server) SendNotification(sessionID string, method string, params map[string]interface{}) error {
	if s.config.isStateless {
		return ErrStatelessMode
	}

	notification := s.NewNotification(method, params)

	successCount, failedCount, lastError := s.sendNotificationToSessions([]string{sessionID}, notification)
	if failedCount > 0 {
		return fmt.Errorf("failed to send notification to session %s: %w", sessionID, lastError)
	}
	if successCount == 0 {
		return fmt.Errorf("no sessions found with ID %s", sessionID)
	}

	return nil
}

// ListRoots sends a request to the client asking for its list of roots.
func (s *Server) ListRoots(ctx context.Context) (*ListRootsResult, error) {
	if s.config.isStateless {
		return nil, ErrStatelessMode
	}

	// Get the session from context.
	session := ClientSessionFromContext(ctx)
	if session == nil {
		return nil, ErrNoClientSession
	}

	sessionID := session.GetID()
	if sessionID == "" {
		return nil, fmt.Errorf("session has no ID")
	}

	// Check if session exists in session manager.
	_, exists := s.httpHandler.sessionManager.getSession(sessionID)
	if !exists {
		return nil, fmt.Errorf("session not found in session manager: %s", sessionID)
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
func (s *Server) SendRequest(ctx context.Context, sessionID string, request *JSONRPCRequest) (*json.RawMessage, error) {
	if s.config.isStateless {
		return nil, ErrStatelessMode
	}

	return s.httpHandler.SendRequest(ctx, sessionID, request)
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

// Handler returns the top-level http.Handler exposed by the server.
// This handler always includes the core MCP endpoint (e.g., /mcp).
// Depending on the configured ServerOptions, it may also include
// additional routes such as OAuth endpoints and .well-known metadata.
//
// You can pass this directly to an http.Server, or mount it into
// an existing HTTP mux as the unified entry point for MCP and
// any configured auxiliary endpoints.
func (s *Server) Handler() http.Handler {
	if s.rootHandler == nil {
		return s.rootHandler
	}
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

// withContext enriches a context with server-specific information.
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

// RegisterNotificationHandler registers a handler for the specified notification method.
// This allows the server to respond to client notifications.
func (s *Server) RegisterNotificationHandler(method string, handler ServerNotificationHandler) {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()

	s.notificationHandlers[method] = handler
	if s.logger != nil {
		s.logger.Debugf("Registered notification handler for method: %s", method)
	}
}

// UnregisterNotificationHandler removes a handler for the specified notification method.
func (s *Server) UnregisterNotificationHandler(method string) {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()

	delete(s.notificationHandlers, method)
	if s.logger != nil {
		s.logger.Debugf("Unregistered notification handler for method: %s", method)
	}
}

// handleServerNotification calls the registered ServerNotificationHandler for the given notification method.
// This method is called by the mcpHandler when it receives a notification.
func (s *Server) handleServerNotification(ctx context.Context, notification *JSONRPCNotification) error {
	s.notificationMu.RLock()
	handler, exists := s.notificationHandlers[notification.Method]
	s.notificationMu.RUnlock()

	if exists {
		// Call the handler with the notification and context.
		return handler(ctx, notification)
	}

	// No handler registered for this notification method.
	if s.logger != nil {
		s.logger.Debugf("No handler registered for notification method: %s", notification.Method)
	}
	return nil
}

// convertToMiddlewareOptions converts AuditConfig to AuditMiddlewareOptions
func convertToMiddlewareOptions(config *AuditConfig) *middleware.AuditMiddlewareOptions {
	level := middleware.AuditLevelBasic
	switch config.Level {
	case "detailed":
		level = middleware.AuditLevelDetailed
	case "full":
		level = middleware.AuditLevelFull
	}

	return &middleware.AuditMiddlewareOptions{
		Level:               level,
		HashSensitiveData:   config.HashSensitiveData,
		IncludeRequestBody:  config.IncludeRequestBody,
		IncludeResponseBody: config.IncludeResponseBody,
		EndpointPatterns:    config.EndpointPatterns,
		ExcludePatterns:     config.ExcludePatterns,
		MetadataExtractor:   config.MetadataExtractor,
	}
}

// convertToAuthMiddleware converts BearerAuthConfig to an HTTP middleware wrapper using RequireBearerAuth
// It also adapts the auth info stored by middleware into the server-level context so downstream code can use server.GetAuthInfo
func convertToAuthMiddleware(config *BearerAuthConfig) func(http.Handler) http.Handler {
	if config == nil || !config.Enabled || config.Verifier == nil {
		return nil
	}

	opts := middleware.BearerAuthMiddlewareOptions{
		Verifier:            config.Verifier,
		RequiredScopes:      config.RequiredScopes,
		ResourceMetadataURL: config.ResourceMetadataURL,
		Issuer:              config.Issuer,
		Audience:            config.Audience,
	}

	bearer := middleware.RequireBearerAuth(opts)

	// Compose an adapter that maps middleware.AuthInfoKey -> server.WithAuthInfo
	return func(next http.Handler) http.Handler {
		// Wrap the downstream handler to translate context
		adapter := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v := r.Context().Value(middleware.AuthInfoKey); v != nil {
				if ai, ok := v.(server.AuthInfo); ok {
					// Avoid token transparent transmission
					aiCopy := ai
					aiCopy.Token = ""
					r = r.WithContext(server.WithAuthInfo(r.Context(), &aiCopy))
				}
			}
			next.ServeHTTP(w, r)
		})
		return bearer(adapter)
	}
}

// withTransportAuditEnabled enables audit logging by wrapping the handler with given middleware
func withTransportAuditEnabled(wrap func(http.Handler) http.Handler) func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.auditEnabled = (wrap != nil)
		h.auditWrap = wrap
	}
}

// withTransportAuthEnabled enables bearer authentication by wrapping the handler with given middleware
func withTransportAuthEnabled(wrap func(http.Handler) http.Handler) func(*httpServerHandler) {
	return func(h *httpServerHandler) {
		h.authEnabled = (wrap != nil)
		h.authWrap = wrap
	}
}

// withHTTPRoutes registers a custom installer function that can
// attach additional HTTP routes to the server's root mux.
func withHTTPRoutes(install func(*http.ServeMux) error) ServerOption {
	return func(s *Server) {
		s.config.routerInstallers = append(s.config.routerInstallers, install)
	}
}
