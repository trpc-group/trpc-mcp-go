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
	tools map[string]*Tool

	// Mutex for concurrent access
	mu sync.RWMutex

	// Server provider for injecting server instance into context
	serverProvider serverProvider
}

// newToolManager creates a tool manager
func newToolManager() *toolManager {
	return &toolManager{
		tools: make(map[string]*Tool),
	}
}

// withServerProvider sets the server provider
func (m *toolManager) withServerProvider(provider serverProvider) *toolManager {
	m.serverProvider = provider
	return m
}

// registerTool registers a tool
func (m *toolManager) registerTool(tool *Tool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tool.Name == "" {
		return errors.ErrEmptyToolName
	}

	if _, exists := m.tools[tool.Name]; exists {
		return fmt.Errorf("%w: %s", errors.ErrToolAlreadyRegistered, tool.Name)
	}

	m.tools[tool.Name] = tool
	return nil
}

// getTool retrieves a tool by name
func (m *toolManager) getTool(name string) (*Tool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tool, ok := m.tools[name]
	return tool, ok
}

// getTools gets all registered tools
func (m *toolManager) getTools(protocolVersion string) []*Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make([]*Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}

	return tools
}

// handleListTools handles tools/list requests
func (m *toolManager) handleListTools(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	// Get all tools
	toolPtrs := m.getTools("")

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
func (m *toolManager) handleCallTool(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
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
	tool, ok := m.getTool(toolName)
	if !ok {
		return newJSONRPCErrorResponse(req.ID, ErrCodeMethodNotFound, fmt.Sprintf("%v: %s", errors.ErrToolNotFound, toolName), nil), nil
	}

	// Create tool call request
	toolReq := &CallToolRequest{}
	toolReq.Method = MethodToolsCall // Set method manually

	// Set up CallToolParams
	params := CallToolParams{
		Name: toolName,
	}

	// Get tool arguments
	if args, ok := paramsMap["arguments"]; ok && args != nil {
		if argsMap, ok := args.(map[string]interface{}); ok {
			params.Arguments = argsMap
		}
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
	result, err := tool.ExecuteFunc(ctx, toolReq)
	if err != nil {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("%v: %v", errors.ErrToolExecutionFailed, err), nil), nil
	}

	return result, nil
}
