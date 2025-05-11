package protocol

import (
	"context"

	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/transport"
)

// Handler interface defines the MCP protocol handler
type Handler interface {
	// HandleRequest processes requests
	HandleRequest(ctx context.Context, req *mcp.JSONRPCRequest, session *transport.Session) (mcp.JSONRPCMessage, error)

	// HandleNotification processes notifications
	HandleNotification(ctx context.Context, notification *mcp.JSONRPCNotification, session *transport.Session) error
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
		h.lifecycleManager = NewLifecycleManager(mcp.Implementation{
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
func (h *MCPHandler) HandleRequest(ctx context.Context, req *mcp.JSONRPCRequest, session *transport.Session) (mcp.JSONRPCMessage, error) {
	// Dispatch request based on method
	switch req.Method {
	case mcp.MethodInitialize:
		return h.lifecycleManager.HandleInitialize(ctx, req, session)

	case mcp.MethodPing: // Using string constant directly
		// Ping simply returns an empty result
		return map[string]interface{}{}, nil

	// Tool related
	case mcp.MethodToolsList:
		return h.toolManager.HandleListTools(ctx, req, session)
	case mcp.MethodToolsCall:
		return h.toolManager.HandleCallTool(ctx, req, session)

	// Resource related
	case mcp.MethodResourcesList:
		return h.resourceManager.HandleListResources(ctx, req)
	case mcp.MethodResourcesRead:
		return h.resourceManager.HandleReadResource(ctx, req)
	case mcp.MethodResourcesTemplatesList:
		return h.resourceManager.HandleListTemplates(ctx, req)
	case mcp.MethodResourcesSubscribe:
		return h.resourceManager.HandleSubscribe(ctx, req)
	case mcp.MethodResourcesUnsubscribe:
		return h.resourceManager.HandleUnsubscribe(ctx, req)

	// Prompt related
	case mcp.MethodPromptsList:
		return h.promptManager.HandleListPrompts(ctx, req)
	case mcp.MethodPromptsGet:
		return h.promptManager.HandleGetPrompt(ctx, req)
	case mcp.MethodCompletionComplete:
		return h.promptManager.HandleCompletionComplete(ctx, req)

	default:
		// Unknown method
		return mcp.NewJSONRPCErrorResponse(req.ID, mcp.ErrMethodNotFound, "method not found", nil), nil
	}
}

// HandleNotification implements the Handler interface's HandleNotification method
func (h *MCPHandler) HandleNotification(ctx context.Context, notification *mcp.JSONRPCNotification, session *transport.Session) error {
	// Dispatch notification based on method
	switch notification.Method {
	case mcp.MethodNotificationsInitialized:
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
