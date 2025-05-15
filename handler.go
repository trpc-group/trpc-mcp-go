package mcp

import (
	"context"
)

// handler interface defines the MCP protocol handler
type handler interface {
	// HandleRequest processes requests
	handleRequest(ctx context.Context, req *JSONRPCRequest, session *Session) (JSONRPCMessage, error)

	// HandleNotification processes notifications
	handleNotification(ctx context.Context, notification *JSONRPCNotification, session *Session) error
}

// mcpHandler implements the default MCP protocol handler
type mcpHandler struct {
	// Tool manager
	toolManager *toolManager

	// Lifecycle manager
	lifecycleManager *lifecycleManager

	// Resource manager
	resourceManager *resourceManager

	// Prompt manager
	promptManager *promptManager
}

// newMCPHandler creates an MCP protocol handler
func newMCPHandler(options ...func(*mcpHandler)) *mcpHandler {
	h := &mcpHandler{}

	// Apply options
	for _, option := range options {
		option(h)
	}

	// Create default managers if not set
	if h.toolManager == nil {
		h.toolManager = newToolManager()
	}

	// Create default resource and prompt managers if not set
	if h.resourceManager == nil {
		h.resourceManager = newResourceManager()
	}

	if h.promptManager == nil {
		h.promptManager = newPromptManager()
	}

	if h.lifecycleManager == nil {
		h.lifecycleManager = newLifecycleManager(Implementation{
			Name:    "Go-MCP-Server",
			Version: "0.1.0",
		})
	}

	// Pass managers to lifecycle manager
	h.lifecycleManager.withToolManager(h.toolManager)
	h.lifecycleManager.withResourceManager(h.resourceManager)
	h.lifecycleManager.withPromptManager(h.promptManager)

	return h
}

// withToolManager sets the tool manager
func withToolManager(manager *toolManager) func(*mcpHandler) {
	return func(h *mcpHandler) {
		h.toolManager = manager
	}
}

// withLifecycleManager sets the lifecycle manager
func withLifecycleManager(manager *lifecycleManager) func(*mcpHandler) {
	return func(h *mcpHandler) {
		h.lifecycleManager = manager
	}
}

// withResourceManager sets the resource manager
func withResourceManager(manager *resourceManager) func(*mcpHandler) {
	return func(h *mcpHandler) {
		h.resourceManager = manager
	}
}

// withPromptManager sets the prompt manager
func withPromptManager(manager *promptManager) func(*mcpHandler) {
	return func(h *mcpHandler) {
		h.promptManager = manager
	}
}

// handleRequest implements the handler interface's handleRequest method
//
// Resource and prompt functionality handling logic:
// 1. If no resources or prompts are registered, the corresponding functionality is disabled by default, and requests will return "method not found" error
// 2. If resources or prompts are registered (even if the list is empty), the corresponding functionality is enabled, and requests will return an empty list rather than an error
// 3. Clients can identify which functionalities the server supports through the capabilities field in the initialization response
func (h *mcpHandler) handleRequest(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	// Dispatch request based on method
	switch req.Method {
	case MethodInitialize:
		return h.lifecycleManager.handleInitialize(ctx, req, session)

	case MethodPing: // Using string constant directly
		// Ping simply returns an empty result
		return map[string]interface{}{}, nil

	// Tool related
	case MethodToolsList:
		return h.toolManager.handleListTools(ctx, req, session)
	case MethodToolsCall:
		return h.toolManager.handleCallTool(ctx, req, session)

	// Resource related
	case MethodResourcesList:
		return h.resourceManager.handleListResources(ctx, req)
	case MethodResourcesRead:
		return h.resourceManager.handleReadResource(ctx, req)
	case MethodResourcesTemplatesList:
		return h.resourceManager.handleListTemplates(ctx, req)
	case MethodResourcesSubscribe:
		return h.resourceManager.handleSubscribe(ctx, req)
	case MethodResourcesUnsubscribe:
		return h.resourceManager.handleUnsubscribe(ctx, req)

	// Prompt related
	case MethodPromptsList:
		return h.promptManager.handleListPrompts(ctx, req)
	case MethodPromptsGet:
		return h.promptManager.handleGetPrompt(ctx, req)
	case MethodCompletionComplete:
		return h.promptManager.handleCompletionComplete(ctx, req)

	default:
		// Unknown method
		return newJSONRPCErrorResponse(req.ID, ErrCodeMethodNotFound, "method not found", nil), nil
	}
}

// handleNotification implements the handler interface's handleNotification method
func (h *mcpHandler) handleNotification(ctx context.Context, notification *JSONRPCNotification, session Session) error {
	// Dispatch notification based on method
	switch notification.Method {
	case MethodNotificationsInitialized:
		return h.lifecycleManager.handleInitialized(ctx, notification, session)
	default:
		// Ignore unknown notifications
		return nil
	}
}

// OnSessionTerminated implements the sessionEventNotifier interface's OnSessionTerminated method
func (h *mcpHandler) onSessionTerminated(sessionID string) {
	// Notify lifecycle manager that session has terminated
	h.lifecycleManager.onSessionTerminated(sessionID)
}
