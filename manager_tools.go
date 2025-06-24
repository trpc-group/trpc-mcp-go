// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"sync"

	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// serverProvider interface defines components that can provide server instances
type serverProvider interface {
	// WithContext injects server instance into the context
	withContext(ctx context.Context) context.Context
}

// toolManager is responsible for managing MCP tools
type toolManager struct {
	// Registered tools
	tools map[string]*registeredTool

	// Mutex for concurrent access
	mu sync.RWMutex

	// Server provider for injecting server instance into context
	serverProvider serverProvider

	// Track insertion order of tools
	toolsOrder []string

	// Tool list filter function.
	toolListFilter ToolListFilter
}

// newToolManager creates a tool manager
func newToolManager() *toolManager {
	return &toolManager{
		tools: make(map[string]*registeredTool),
	}
}

// withServerProvider sets the server provider
func (m *toolManager) withServerProvider(provider serverProvider) *toolManager {
	m.serverProvider = provider
	return m
}

// withToolListFilter sets the tool list filter.
func (m *toolManager) withToolListFilter(filter ToolListFilter) *toolManager {
	m.toolListFilter = filter
	return m
}

// registerTool registers a tool
func (m *toolManager) registerTool(tool *Tool, handler toolHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tool == nil || tool.Name == "" {
		return
	}

	if _, exists := m.tools[tool.Name]; !exists {
		// Only add to order slice if it's a new tool
		m.toolsOrder = append(m.toolsOrder, tool.Name)
	}

	m.tools[tool.Name] = &registeredTool{
		Tool:    tool,
		Handler: handler,
	}
}

// getTool retrieves a tool by name
func (m *toolManager) getTool(name string) (*Tool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	registeredTool, ok := m.tools[name]
	if !ok {
		return nil, false
	}
	return registeredTool.Tool, true
}

// getTools gets all registered tools
func (m *toolManager) getTools(protocolVersion string) []*Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make([]*Tool, 0, len(m.tools))
	for _, registeredTool := range m.tools {
		if registeredTool != nil && registeredTool.Tool != nil {
			tools = append(tools, registeredTool.Tool)
		}
	}

	return tools
}

// handleListTools handles tools/list requests
func (m *toolManager) handleListTools(
	ctx context.Context,
	req *JSONRPCRequest,
	session Session,
) (JSONRPCMessage, error) {
	// Get all tools
	toolPtrs := m.getTools("")

	// Apply filter if available.
	if m.toolListFilter != nil {
		toolPtrs = m.toolListFilter(ctx, toolPtrs)
	}

	// Convert []*mcp.Tool to []mcp.Tool
	tools := make([]Tool, len(toolPtrs))
	for i, toolPtr := range toolPtrs {
		if toolPtr != nil {
			tools[i] = *toolPtr
		}
	}

	// Format and return response
	result := ListToolsResult{
		Tools: tools,
	}

	return result, nil
}

// handleCallTool handles tools/call requests
func (m *toolManager) handleCallTool(
	ctx context.Context,
	req *JSONRPCRequest,
	session Session,
) (JSONRPCMessage, error) {
	// Parse request parameters
	if req.Params == nil {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), nil
	}

	// Convert params to map for easier access
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrInvalidParams.Error(), nil), nil
	}

	// Get tool name
	toolName, ok := paramsMap["name"].(string)
	if !ok || toolName == "" {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, "missing tool name", nil), nil
	}

	// Get tool
	registeredTool, ok := m.tools[toolName]
	if !ok {
		return newJSONRPCErrorResponse(
			req.ID,
			ErrCodeMethodNotFound,
			fmt.Sprintf("%v: %s", errors.ErrToolNotFound, toolName),
			nil,
		), nil
	}

	// Create tool call request
	toolReq := &CallToolRequest{}
	toolReq.Method = MethodToolsCall // Set method manually

	// Set up CallToolParams
	params := CallToolParams{
		Name: toolName,
	}

	// Get and validate tool arguments
	if args, ok := paramsMap["arguments"]; ok && args != nil {
		argsMap, ok := args.(map[string]interface{})
		if !ok {
			errMsg := fmt.Sprintf("%v: arguments must be an object, got %T", errors.ErrInvalidParams, args)
			return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errMsg, nil), nil
		}
		params.Arguments = argsMap
	}

	toolReq.Params = params

	// Progress notification token (if any)
	if meta, ok := paramsMap["_meta"].(map[string]interface{}); ok {
		if progressToken, exists := meta["progressToken"]; exists {
			// Note: The current version of CallToolRequest doesn't fully implement Meta field
			// Future implementation should add toolReq.Meta = ... code
			_ = progressToken // Ignore progress token for now
		}
	}

	// Before calling the tool, inject server instance into context if server provider exists
	if m.serverProvider != nil {
		ctx = m.serverProvider.withContext(ctx)
	}

	// Execute tool
	result, err := registeredTool.Handler(ctx, toolReq)
	if err != nil {
		errMsg := fmt.Sprintf("tool execution failed (tool: %s): %v", registeredTool.Tool.Name, err)
		return newJSONRPCErrorResponse(req.ID, ErrCodeInternal, errMsg, nil), nil
	}

	return result, nil
}
