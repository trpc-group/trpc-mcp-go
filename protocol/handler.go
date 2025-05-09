package protocol

import (
	"context"

	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/transport"
)

// Handler interface defines the MCP protocol handler
type Handler interface {
	// HandleRequest processes requests
	HandleRequest(ctx context.Context, req *schema.Request, session *transport.Session) (*schema.Response, error)

	// HandleNotification processes notifications
	HandleNotification(ctx context.Context, notification *schema.Notification, session *transport.Session) error
}

// MCPHandler implements the default MCP protocol handler
type MCPHandler struct {
	// Tool manager
	toolManager *ToolManager

	// Lifecycle manager
	lifecycleManager *LifecycleManager

	// Resource manager
	resourceManager *ResourceManager

	// Prompt manager
	promptManager *PromptManager
}

// NewMCPHandler creates an MCP protocol handler
func NewMCPHandler(options ...func(*MCPHandler)) *MCPHandler {
	h := &MCPHandler{}

	// Apply options
	for _, option := range options {
		option(h)
	}

	// Create default managers if not set
	if h.toolManager == nil {
		h.toolManager = NewToolManager()
	}

	// Create default resource and prompt managers if not set
	if h.resourceManager == nil {
		h.resourceManager = NewResourceManager()
	}

	if h.promptManager == nil {
		h.promptManager = NewPromptManager()
	}

	if h.lifecycleManager == nil {
		h.lifecycleManager = NewLifecycleManager(schema.Implementation{
			Name:    "Go-MCP-Server",
			Version: "0.1.0",
		})
	}

	// Pass managers to lifecycle manager
	h.lifecycleManager.WithToolManager(h.toolManager)
	h.lifecycleManager.WithResourceManager(h.resourceManager)
	h.lifecycleManager.WithPromptManager(h.promptManager)

	return h
}

// WithToolManager sets the tool manager
func WithToolManager(manager *ToolManager) func(*MCPHandler) {
	return func(h *MCPHandler) {
		h.toolManager = manager
	}
}

// WithLifecycleManager sets the lifecycle manager
func WithLifecycleManager(manager *LifecycleManager) func(*MCPHandler) {
	return func(h *MCPHandler) {
		h.lifecycleManager = manager
	}
}

// WithResourceManager sets the resource manager
func WithResourceManager(manager *ResourceManager) func(*MCPHandler) {
	return func(h *MCPHandler) {
		h.resourceManager = manager
	}
}

// WithPromptManager sets the prompt manager
func WithPromptManager(manager *PromptManager) func(*MCPHandler) {
	return func(h *MCPHandler) {
		h.promptManager = manager
	}
}

// HandleRequest implements the Handler interface's HandleRequest method
//
// Resource and prompt functionality handling logic:
// 1. If no resources or prompts are registered, the corresponding functionality is disabled by default, and requests will return "method not found" error
// 2. If resources or prompts are registered (even if the list is empty), the corresponding functionality is enabled, and requests will return an empty list rather than an error
// 3. Clients can identify which functionalities the server supports through the capabilities field in the initialization response
func (h *MCPHandler) HandleRequest(ctx context.Context, req *schema.Request, session *transport.Session) (*schema.Response, error) {
	// Dispatch request based on method
	switch req.Method {
	case schema.MethodInitialize:
		return h.lifecycleManager.HandleInitialize(ctx, req, session)

	case schema.MethodPing: // Using string constant directly
		// Ping request only needs an empty success response
		return schema.NewResponse(req.ID, map[string]interface{}{}), nil

	// Tool related
	case schema.MethodToolsList:
		return h.toolManager.HandleListTools(ctx, req, session)
	case schema.MethodToolsCall:
		return h.toolManager.HandleCallTool(ctx, req, session)

	// Resource related
	case schema.MethodResourcesList:
		return h.resourceManager.HandleListResources(ctx, req)
	case schema.MethodResourcesRead:
		return h.resourceManager.HandleReadResource(ctx, req)
	case schema.MethodResourcesTemplatesList:
		return h.resourceManager.HandleListTemplates(ctx, req)
	case schema.MethodResourcesSubscribe:
		return h.resourceManager.HandleSubscribe(ctx, req)
	case schema.MethodResourcesUnsubscribe:
		return h.resourceManager.HandleUnsubscribe(ctx, req)

	// Prompt related
	case schema.MethodPromptsList:
		return h.promptManager.HandleListPrompts(ctx, req)
	case schema.MethodPromptsGet:
		return h.promptManager.HandleGetPrompt(ctx, req)
	case schema.MethodCompletionComplete:
		return h.promptManager.HandleCompletionComplete(ctx, req)

	default:
		// Unknown method
		return schema.NewErrorResponse(req.ID, schema.ErrMethodNotFound, "method not found", nil), nil
	}
}

// HandleNotification implements the Handler interface's HandleNotification method
func (h *MCPHandler) HandleNotification(ctx context.Context, notification *schema.Notification, session *transport.Session) error {
	// Dispatch notification based on method
	switch notification.Method {
	case schema.MethodNotificationsInitialized:
		return h.lifecycleManager.HandleInitialized(ctx, notification, session)
	default:
		// Ignore unknown notifications
		return nil
	}
}

// OnSessionTerminated implements the SessionEventNotifier interface's OnSessionTerminated method
func (h *MCPHandler) OnSessionTerminated(sessionID string) {
	// Notify lifecycle manager that session has terminated
	h.lifecycleManager.OnSessionTerminated(sessionID)

	// Log session termination event
	log.Infof("Protocol handler received session termination notification: %s", sessionID)
}
