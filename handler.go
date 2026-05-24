// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"

	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
	"trpc.group/trpc-go/trpc-mcp-go/internal/utils"
)

const (
	// defaultServerName is the default name for the server
	defaultServerName = "Go-MCP-Server"
	// defaultServerVersion is the default version for the server
	defaultServerVersion = "0.1.0"
)

// HandlerFunc defines a simplified handler function for middleware.
// It uses only ctx and req parameters, session can be retrieved from ctx using ClientSessionFromContext.
type HandlerFunc func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error)

// Middleware defines a function that wraps a HandlerFunc to add cross-cutting concerns.
// Middlewares can be chained together to form a processing pipeline.
type Middleware func(next HandlerFunc) HandlerFunc

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

	// Middleware chain for request processing.
	middlewares []Middleware
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

// handleRequest processes a JSON-RPC request with optional middleware support.
// If middlewares are registered, it adapts the request to use the simplified HandlerFunc signature.
func (h *mcpHandler) handleRequest(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	// If middlewares are registered, use the middleware chain.
	if len(h.middlewares) > 0 {
		// Create core handler that adapts from HandlerFunc (2 params) to internal handler (3 params).
		coreHandler := func(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
			// Try to get session from context (should already be injected by outer layer).
			sessionFromCtx := ClientSessionFromContext(ctx)
			if sessionFromCtx != nil {
				// Use session from context.
				return h.dispatchRequest(ctx, req, sessionFromCtx)
			}
			// Fallback: use session parameter (for backward compatibility).
			return h.dispatchRequest(ctx, req, session)
		}

		// Apply middleware chain (with read lock).
		wrappedHandler := h.applyMiddlewares(coreHandler)

		// Execute with simplified signature (only ctx and req).
		return wrappedHandler(ctx, req)
	}

	// No middlewares: use original dispatch logic directly.
	return h.dispatchRequest(ctx, req, session)
}

// dispatchRequest is the core request dispatcher (original logic).
func (h *mcpHandler) dispatchRequest(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	dispatchTable := h.requestDispatchTable()
	if handler, ok := dispatchTable[req.Method]; ok {
		return handler(ctx, req, session)
	}
	return newJSONRPCErrorResponse(req.ID, ErrCodeMethodNotFound, "method not found", nil), nil
}

// applyMiddlewares applies the middleware chain to a handler.
// Middlewares are applied in reverse order (last registered = outermost layer).
func (h *mcpHandler) applyMiddlewares(handler HandlerFunc) HandlerFunc {
	// Apply from last to first (onion model).
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		handler = h.middlewares[i](handler)
	}
	return handler
}

// use registers a middleware to the handler.
// This is only called during initialization (via WithMiddleware option),
// so no locking is needed.
func (h *mcpHandler) use(middleware Middleware) {
	h.middlewares = append(h.middlewares, middleware)
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
	ref := utils.ExtractMap(req.Params.(map[string]interface{}), "ref")
	if ref == nil {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), nil
	}
	switch utils.ExtractString(ref, "type") {
	case "ref/prompt":
		return h.promptManager.handleCompletionComplete(ctx, req)
	case "ref/resource":
		return h.resourceManager.handleCompletionComplete(ctx, req)
	default:
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrInvalidParams.Error(), nil), nil
	}
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
