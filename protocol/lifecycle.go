package protocol

import (
	"context"
	"sync"

	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/transport"
)

// LifecycleManager is responsible for managing the MCP protocol lifecycle
type LifecycleManager struct {
	// Server information
	serverInfo mcp.Implementation

	// Default protocol version
	defaultProtocolVersion string

	// Supported protocol versions
	supportedVersions []string

	// Supported capabilities
	capabilities map[string]interface{}

	// Initialization status (per session)
	sessionStates map[string]bool

	// Tool manager reference
	toolManager *ToolManager

	// Resource manager reference
	resourceManager *ResourceManager

	// Prompt manager reference
	promptManager *PromptManager

	// Mutex for concurrent access
	mu sync.RWMutex
}

// NewLifecycleManager creates a lifecycle manager
func NewLifecycleManager(serverInfo mcp.Implementation) *LifecycleManager {
	return &LifecycleManager{
		serverInfo:             serverInfo,
		defaultProtocolVersion: mcp.ProtocolVersion_2024_11_05,           // Using 2024-11-05 version, according to MCP protocol specification
		supportedVersions:      []string{mcp.ProtocolVersion_2024_11_05}, // Only supporting 2024-11-05 version
		capabilities: map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": true,
			},
		},
		sessionStates: make(map[string]bool),
	}
}

// WithProtocolVersion sets the default protocol version
func (m *LifecycleManager) WithProtocolVersion(version string) *LifecycleManager {
	m.defaultProtocolVersion = version
	return m
}

// WithSupportedVersions sets the supported protocol versions
func (m *LifecycleManager) WithSupportedVersions(versions []string) *LifecycleManager {
	m.supportedVersions = versions
	return m
}

// WithCapabilities sets the capabilities
func (m *LifecycleManager) WithCapabilities(capabilities map[string]interface{}) *LifecycleManager {
	m.capabilities = capabilities
	return m
}

// WithToolManager sets the tool manager
func (m *LifecycleManager) WithToolManager(toolManager *ToolManager) *LifecycleManager {
	m.toolManager = toolManager
	return m
}

// WithResourceManager sets the resource manager
func (m *LifecycleManager) WithResourceManager(resourceManager *ResourceManager) *LifecycleManager {
	m.resourceManager = resourceManager
	return m
}

// WithPromptManager sets the prompt manager
func (m *LifecycleManager) WithPromptManager(promptManager *PromptManager) *LifecycleManager {
	m.promptManager = promptManager
	return m
}

// updateCapabilities updates the server capability information
func (m *LifecycleManager) updateCapabilities() {
	// Use map as an intermediate variable
	capMap := map[string]interface{}{}

	// Basic tool capabilities always exist
	capMap["tools"] = map[string]interface{}{
		"listChanged": true,
	}

	// If there is a resource manager and resources are registered, add resource capabilities
	if m.resourceManager != nil && len(m.resourceManager.GetResources()) > 0 {
		capMap["resources"] = map[string]interface{}{
			"listChanged": true,
		}
	}

	// If there is a prompt manager and prompts are registered, add prompt capabilities
	if m.promptManager != nil && len(m.promptManager.GetPrompts()) > 0 {
		capMap["prompts"] = map[string]interface{}{
			"listChanged": true,
		}
	}

	// Preserve existing experimental features
	if exp, ok := m.capabilities["experimental"]; ok {
		capMap["experimental"] = exp
	}

	// Update capabilities
	m.capabilities = capMap
}

// HandleInitialize handles initialize requests
func (m *LifecycleManager) HandleInitialize(ctx context.Context, req *mcp.JSONRPCRequest, session *transport.Session) (mcp.JSONRPCMessage, error) {
	// Parse request parameters
	if req.Params == nil {
		return mcp.NewJSONRPCErrorResponse(req.ID, mcp.ErrInvalidParams, "parameters are empty", nil), nil
	}

	// Convert params to map for easier access
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return mcp.NewJSONRPCErrorResponse(req.ID, mcp.ErrInvalidParams, "invalid parameters format", nil), nil
	}

	// Get protocol version
	protocolVersion, ok := paramsMap["protocolVersion"].(string)
	if !ok {
		return mcp.NewJSONRPCErrorResponse(req.ID, mcp.ErrInvalidParams, "missing protocolVersion", nil), nil
	}

	// Check if protocol version is supported
	var supportedVersion string
	isVersionSupported := false

	// Check if the requested version is in the supported version list
	for _, version := range m.supportedVersions {
		if version == protocolVersion {
			isVersionSupported = true
			supportedVersion = protocolVersion
			break
		}
	}

	// If not supported, use default version
	if !isVersionSupported {
		supportedVersion = m.defaultProtocolVersion
		log.Infof("Client requested protocol version %s is not supported, using %s", protocolVersion, supportedVersion)
	} else {
		log.Infof("Using protocol version: %s", supportedVersion)
	}

	// If there is a session, mark its initialization state and save protocol version
	if session != nil {
		m.mu.Lock()
		m.sessionStates[session.ID] = false // Initialization started but not completed
		m.mu.Unlock()

		// Save protocol version to session data
		session.SetData("protocolVersion", supportedVersion)
	}

	// Update server capability information
	m.updateCapabilities()

	// Create initialization response
	response := mcp.InitializeResult{
		ProtocolVersion: supportedVersion,
		ServerInfo: mcp.Implementation{
			Name:    m.serverInfo.Name,
			Version: m.serverInfo.Version,
		},
		Capabilities: convertToServerCapabilities(m.capabilities),
		Instructions: "MCP server is ready",
	}

	return response, nil
}

// convertToServerCapabilities converts a map to ServerCapabilities structure
func convertToServerCapabilities(capMap map[string]interface{}) mcp.ServerCapabilities {
	capabilities := mcp.ServerCapabilities{}

	// Handle tools capability
	if toolsMap, ok := capMap["tools"].(map[string]interface{}); ok {
		capabilities.Tools = &mcp.ToolsCapability{}
		if listChanged, ok := toolsMap["listChanged"].(bool); ok && listChanged {
			capabilities.Tools.ListChanged = true
		}
	}

	// Handle resources capability
	if resourcesMap, ok := capMap["resources"].(map[string]interface{}); ok {
		capabilities.Resources = &mcp.ResourcesCapability{}
		if listChanged, ok := resourcesMap["listChanged"].(bool); ok && listChanged {
			capabilities.Resources.ListChanged = true
		}
		if subscribe, ok := resourcesMap["subscribe"].(bool); ok && subscribe {
			capabilities.Resources.Subscribe = true
		}
	}

	// Handle prompts capability - stricter type checking
	if _, exists := capMap["prompts"]; exists {
		promptsMap, isMap := capMap["prompts"].(map[string]interface{})
		if isMap {
			capabilities.Prompts = &mcp.PromptsCapability{}
			if listChanged, ok := promptsMap["listChanged"].(bool); ok && listChanged {
				capabilities.Prompts.ListChanged = true
			}
		} else {
			// If not the expected map type, at least create an empty PromptsCapability instance
			capabilities.Prompts = &mcp.PromptsCapability{}
		}
	}

	// Handle logging capability
	if _, ok := capMap["logging"].(map[string]interface{}); ok {
		capabilities.Logging = &mcp.LoggingCapability{}
	}

	// Handle completions capability
	if _, ok := capMap["completions"].(map[string]interface{}); ok {
		capabilities.Completions = &mcp.CompletionsCapability{}
	}

	// Handle experimental capability
	if expMap, ok := capMap["experimental"].(map[string]interface{}); ok {
		capabilities.Experimental = expMap
	}

	return capabilities
}

// HandleInitialized handles initialized notifications
func (m *LifecycleManager) HandleInitialized(ctx context.Context, notification *mcp.JSONRPCNotification, session *transport.Session) error {
	// Mark session as initialized
	if session != nil {
		m.mu.Lock()
		m.sessionStates[session.ID] = true
		m.mu.Unlock()
		log.Infof("Session %s initialization completed", session.ID)
	}

	return nil
}

// IsInitialized checks if a session is initialized
func (m *LifecycleManager) IsInitialized(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	initialized, exists := m.sessionStates[sessionID]
	return exists && initialized
}

// OnSessionTerminated handles session termination events
func (m *LifecycleManager) OnSessionTerminated(sessionID string) {
	m.mu.Lock()
	delete(m.sessionStates, sessionID)
	m.mu.Unlock()
	log.Infof("Session terminated and removed from lifecycle manager: %s", sessionID)
}
