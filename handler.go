package mcp

import (
	"context"
)

// Handler interface defines the MCP protocol handler
type Handler interface {
	// HandleRequest processes requests
	HandleRequest(ctx context.Context, req *JSONRPCRequest, session *Session) (JSONRPCMessage, error)

	// HandleNotification processes notifications
	HandleNotification(ctx context.Context, notification *JSONRPCNotification, session *Session) error
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
		h.lifecycleManager = NewLifecycleManager(Implementation{
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
func (h *MCPHandler) HandleRequest(ctx context.Context, req *JSONRPCRequest, session *Session) (JSONRPCMessage, error) {
	// Dispatch request based on method
	switch req.Method {
	case MethodInitialize:
		return h.lifecycleManager.HandleInitialize(ctx, req, session)

	case MethodPing: // Using string constant directly
		// Ping simply returns an empty result
		return map[string]interface{}{}, nil

	// Tool related
	case MethodToolsList:
		return h.toolManager.HandleListTools(ctx, req, session)
	case MethodToolsCall:
		return h.toolManager.HandleCallTool(ctx, req, session)

	// Resource related
	case MethodResourcesList:
		return h.resourceManager.HandleListResources(ctx, req)
	case MethodResourcesRead:
		return h.resourceManager.HandleReadResource(ctx, req)
	case MethodResourcesTemplatesList:
		return h.resourceManager.HandleListTemplates(ctx, req)
	case MethodResourcesSubscribe:
		return h.resourceManager.HandleSubscribe(ctx, req)
	case MethodResourcesUnsubscribe:
		return h.resourceManager.HandleUnsubscribe(ctx, req)

	// Prompt related
	case MethodPromptsList:
		return h.promptManager.HandleListPrompts(ctx, req)
	case MethodPromptsGet:
		return h.promptManager.HandleGetPrompt(ctx, req)
	case MethodCompletionComplete:
		return h.promptManager.HandleCompletionComplete(ctx, req)

	default:
		// Unknown method
		return NewJSONRPCErrorResponse(req.ID, ErrCodeMethodNotFound, "method not found", nil), nil
	}
}

// HandleNotification implements the Handler interface's HandleNotification method
func (h *MCPHandler) HandleNotification(ctx context.Context, notification *JSONRPCNotification, session *Session) error {
	// Dispatch notification based on method
	switch notification.Method {
	case MethodNotificationsInitialized:
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
}
