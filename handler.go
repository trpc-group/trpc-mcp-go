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

	// HandleNotification processes notifications.
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

	// Server reference for notification handling.
	server serverNotificationDispatcher
}

// serverNotificationDispatcher defines the interface for dispatching notifications to handlers.
type serverNotificationDispatcher interface {
	// handleServerNotification dispatches a notification to registered handlers.
	handleServerNotification(ctx context.Context, notification *JSONRPCNotification) error
}

// withServer sets the server reference.
func withServer(server serverNotificationDispatcher) func(*mcpHandler) {
	return func(h *mcpHandler) {
		h.server = server
	}
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
	if handler, ok := dispatchTable[req.Method]; ok {
		return handler(ctx, req, session)
	}
	return newJSONRPCErrorResponse(req.ID, ErrCodeMethodNotFound, "method not found", nil), nil
}

// Private methods for each case branch
func (h *mcpHandler) handleInitialize(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
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
		if err := h.lifecycleManager.handleInitialized(ctx, notification, session); err != nil {
			return err
		}

		// Then also dispatch to any custom handler if registered.
		if h.server != nil {
			return h.server.handleServerNotification(ctx, notification)
		}
		return nil
	default:
		// For other notifications, dispatch to server if available.
		if h.server != nil {
			return h.server.handleServerNotification(ctx, notification)
		}
		return nil
	}
}

// onSessionTerminated implements the sessionEventNotifier interface's OnSessionTerminated method
func (h *mcpHandler) onSessionTerminated(sessionID string) {
	// Notify lifecycle manager that session has terminated
	h.lifecycleManager.onSessionTerminated(sessionID)
}
