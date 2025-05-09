package protocol

import (
	"context"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/transport"
)

// ServerProvider interface defines components that can provide server instances
type ServerProvider interface {
	// WithContext injects server instance into the context
	WithContext(ctx context.Context) context.Context
}

// ToolManager is responsible for managing MCP tools
type ToolManager struct {
	// Registered tools
	tools map[string]*schema.Tool

	// Mutex for concurrent access
	mu sync.RWMutex

	// Server provider for injecting server instance into context
	serverProvider ServerProvider
}

// NewToolManager creates a tool manager
func NewToolManager() *ToolManager {
	return &ToolManager{
		tools: make(map[string]*schema.Tool),
	}
}

// WithServerProvider sets the server provider
func (m *ToolManager) WithServerProvider(provider ServerProvider) *ToolManager {
	m.serverProvider = provider
	return m
}

// RegisterTool registers a tool
func (m *ToolManager) RegisterTool(tool *schema.Tool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if _, exists := m.tools[tool.Name]; exists {
		return fmt.Errorf("tool %s is already registered", tool.Name)
	}

	m.tools[tool.Name] = tool
	return nil
}

// GetTool retrieves a tool by name
func (m *ToolManager) GetTool(name string) (*schema.Tool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tool, ok := m.tools[name]
	return tool, ok
}

// GetTools gets all registered tools
func (m *ToolManager) GetTools(protocolVersion string) []*schema.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make([]*schema.Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}

	return tools
}

// HandleListTools handles tools/list requests
func (m *ToolManager) HandleListTools(ctx context.Context, req *schema.Request, session *transport.Session) (*schema.Response, error) {
	// Get all tools
	tools := m.GetTools("")

	// Create response
	return schema.NewResponse(req.ID, map[string]interface{}{
		"tools": tools,
	}), nil
}

// HandleCallTool handles tools/call requests
func (m *ToolManager) HandleCallTool(ctx context.Context, req *schema.Request, session *transport.Session) (*schema.Response, error) {
	// Parse request parameters
	if req.Params == nil {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "parameters are empty", nil), nil
	}

	// Get tool name
	toolName, ok := req.Params["name"].(string)
	if !ok || toolName == "" {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing tool name", nil), nil
	}

	// Get tool
	tool, ok := m.GetTool(toolName)
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrMethodNotFound, fmt.Sprintf("tool %s not found", toolName), nil), nil
	}

	// Create tool call request according to specification
	toolReq := &schema.CallToolRequest{
		Method: "tools/call",
		Params: schema.CallToolParams{
			Name: toolName,
		},
	}

	// Get tool arguments
	if args, ok := req.Params["arguments"]; ok && args != nil {
		if argsMap, ok := args.(map[string]interface{}); ok {
			toolReq.Params.Arguments = argsMap
		}
	}

	// Progress notification token (if any)
	if progressToken, ok := req.Params["progressToken"]; ok {
		// Note: Current version of CallToolRequest doesn't fully implement Meta field
		// Future implementation should add toolReq.Meta = ... code
		// meta := &schema.RequestMeta{
		//	ProgressToken: progressToken,
		// }
		_ = progressToken // Ignore progress token for now
	}

	// Before calling the tool, inject server instance into context if server provider exists
	if m.serverProvider != nil {
		ctx = m.serverProvider.WithContext(ctx)
	}

	// Execute tool
	result, err := tool.ExecuteFunc(ctx, toolReq)
	if err != nil {
		return schema.NewErrorResponse(req.ID, schema.ErrInternal, fmt.Sprintf("failed to execute tool %s: %v", toolName, err), nil), nil
	}

	// Create response using the dedicated tool result response function
	return schema.NewToolResultResponse(req.ID, result), nil
}
