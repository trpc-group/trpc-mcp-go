// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
)

const (
	// defaultServerName is the default name for the server
	defaultServerName = "Go-MCP-Server"
	// defaultServerVersion is the default version for the server
	defaultServerVersion = "0.1.0"
)

// handler interface defines the MCP protocol handler
type handler interface {
	// HandleRequest processes requests
	handleRequest(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error)

	// HandleNotification processes notifications
	handleNotification(ctx context.Context, notification *JSONRPCNotification, session Session) error
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

	// Session manager
	sessionManager sessionManager

	// Logger for the handler
	logger Logger
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
			Name:    defaultServerName,
			Version: defaultServerVersion,
		})
	}

	// Create a default session manager if not set
	if h.sessionManager == nil {
		h.sessionManager = &sessionManagerAdapter{}
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

// withSessionManager sets the session manager
func withSessionManager(manager sessionManager) func(*mcpHandler) {
	return func(h *mcpHandler) {
		h.sessionManager = manager
	}
}

// withLogger sets the logger
func withLogger(logger Logger) func(*mcpHandler) {
	return func(h *mcpHandler) {
		h.logger = logger
	}
}

// Definition: request dispatch table type
type requestHandlerFunc func(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error)

// Initialization: request dispatch table
func (h *mcpHandler) requestDispatchTable() map[string]requestHandlerFunc {
	return map[string]requestHandlerFunc{
		MethodInitialize:             h.handleInitialize,
		MethodPing:                   h.handlePing,
		MethodToolsList:              h.handleToolsList,
		MethodToolsCall:              h.handleToolsCall,
		MethodResourcesList:          h.handleResourcesList,
		MethodResourcesRead:          h.handleResourcesRead,
		MethodResourcesTemplatesList: h.handleResourcesTemplatesList,
		MethodResourcesSubscribe:     h.handleResourcesSubscribe,
		MethodResourcesUnsubscribe:   h.handleResourcesUnsubscribe,
		MethodPromptsList:            h.handlePromptsList,
		MethodPromptsGet:             h.handlePromptsGet,
		MethodCompletionComplete:     h.handleCompletionComplete,
	}
}

// Refactored handleRequest
func (h *mcpHandler) handleRequest(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	dispatchTable := h.requestDispatchTable()
	handler, ok := dispatchTable[req.Method]
	if !ok {
		return newJSONRPCErrorResponse(req.ID, ErrCodeMethodNotFound, "method not found", nil), nil
	}

	reqCtx, cancel := context.WithCancel(ctx)
	session.RegisterCanceler(req.ID, cancel)

	defer func() {
		cancel()
		session.CleanupRequest(req.ID)
	}()

	return handler(reqCtx, req, session)
}

// Private methods for each case branch
func (h *mcpHandler) handleInitialize(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	// Mark this request as an initialize request in the session
	session.SetData("__initialize_request_id", req.ID)
	return h.lifecycleManager.handleInitialize(ctx, req, session)
}

func (h *mcpHandler) handlePing(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return map[string]interface{}{}, nil
}

func (h *mcpHandler) handleToolsList(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.toolManager.handleListTools(ctx, req, session)
}

func (h *mcpHandler) handleToolsCall(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.toolManager.handleCallTool(ctx, req, session)
}

func (h *mcpHandler) handleResourcesList(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.resourceManager.handleListResources(ctx, req)
}

func (h *mcpHandler) handleResourcesRead(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.resourceManager.handleReadResource(ctx, req)
}

func (h *mcpHandler) handleResourcesTemplatesList(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.resourceManager.handleListTemplates(ctx, req)
}

func (h *mcpHandler) handleResourcesSubscribe(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.resourceManager.handleSubscribe(ctx, req)
}

func (h *mcpHandler) handleResourcesUnsubscribe(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.resourceManager.handleUnsubscribe(ctx, req)
}

func (h *mcpHandler) handlePromptsList(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.promptManager.handleListPrompts(ctx, req)
}

func (h *mcpHandler) handlePromptsGet(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.promptManager.handleGetPrompt(ctx, req)
}

func (h *mcpHandler) handleCompletionComplete(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	return h.promptManager.handleCompletionComplete(ctx, req)
}

// handleNotification implements the handler interface's handleNotification method
func (h *mcpHandler) handleNotification(ctx context.Context, notification *JSONRPCNotification, session Session) error {
	// Dispatch notification based on method
	switch notification.Method {
	case MethodNotificationsInitialized:
		return h.lifecycleManager.handleInitialized(ctx, notification, session)
	case MethodCancelRequest:
		// Extract requestId directly from additionalFields
		additionalFields := notification.Params.AdditionalFields
		if additionalFields == nil {
			// No additional fields, nothing to cancel
			return nil
		}

		// Extract requestId
		requestId, exists := additionalFields["requestId"]
		if !exists {
			// Required field missing, silently ignore
			return nil
		}

		// Check if trying to cancel initialize request
		// The initialize request MUST NOT be cancelled by clients (MCP specification)
		if initReqID, exists := session.GetData("__initialize_request_id"); exists {
			if requestId == initReqID {
				// Silently ignore attempts to cancel initialize requests
				return nil
			}
		}

		// Extract optional reason
		reason := ""
		if reasonValue, ok := additionalFields["reason"].(string); ok {
			reason = reasonValue
		}

		// Log reason if available
		if reason != "" && h.logger != nil {
			h.logger.Debugf("Cancel request %v reason: %s", requestId, reason)
		}

		session.CancelRequest(requestId)
		return nil
	default:
		// Ignore unknown notifications
		return nil
	}
}

// onSessionTerminated implements the sessionEventNotifier interface's OnSessionTerminated method
func (h *mcpHandler) onSessionTerminated(sessionID string) {
	// Notify lifecycle manager that session has terminated
	h.lifecycleManager.onSessionTerminated(sessionID)

	// Retrieve the session from the session manager
	session, found := h.sessionManager.getSession(sessionID)
	if found {
		// Cancel all in-progress requests for this session
		session.CancelAll()
	}

	// Log the session termination
	if h.logger != nil {
		h.logger.Infof("Session %s terminated, all pending requests cancelled", sessionID)
	}
}
