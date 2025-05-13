package mcp

import (
	"context"
	"sync"

	// logger is injected, no need to import log package.
)

// LifecycleManager is responsible for managing the MCP protocol lifecycle
type LifecycleManager struct {
	// Logger for this lifecycle manager.
	logger Logger
	// Server information
	serverInfo Implementation

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
func NewLifecycleManager(serverInfo Implementation) *LifecycleManager {
	return &LifecycleManager{
		logger:                 GetDefaultLogger(), // Use default logger if not set.
		serverInfo:             serverInfo,
		defaultProtocolVersion: ProtocolVersion_2024_11_05,           // Using 2024-11-05 version, according to MCP protocol specification
		supportedVersions:      []string{ProtocolVersion_2024_11_05}, // Only supporting 2024-11-05 version
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

// WithPromptManager sets the prompt manager.
func (m *LifecycleManager) WithPromptManager(promptManager *PromptManager) *LifecycleManager {
	m.promptManager = promptManager
	return m
}

// WithClientTransportLogger sets the logger for LifecycleManager.
func (m *LifecycleManager) WithLogger(logger Logger) *LifecycleManager {
	m.logger = logger
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
func (m *LifecycleManager) HandleInitialize(ctx context.Context, req *JSONRPCRequest, session *Session) (JSONRPCMessage, error) {
	// Parse request parameters
	if req.Params == nil {
		return NewJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, "parameters are empty", nil), nil
	}

	// Convert params to map for easier access
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return NewJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, "invalid parameters format", nil), nil
	}

	// Get protocol version
	protocolVersion, ok := paramsMap["protocolVersion"].(string)
	if !ok {
		return NewJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, "missing protocolVersion", nil), nil
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
		m.logger.Infof("Client requested protocol version %s is not supported, using %s", protocolVersion, supportedVersion)
	} else {
		m.logger.Infof("Using protocol version: %s", supportedVersion)
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
	response := InitializeResult{
		ProtocolVersion: supportedVersion,
		ServerInfo: Implementation{
			Name:    m.serverInfo.Name,
			Version: m.serverInfo.Version,
		},
		Capabilities: convertToServerCapabilities(m.capabilities),
		Instructions: "MCP server is ready",
	}

	return response, nil
}

// convertToServerCapabilities converts a map to ServerCapabilities structure
func convertToServerCapabilities(capMap map[string]interface{}) ServerCapabilities {
	capabilities := ServerCapabilities{}

	// Handle tools capability
	if toolsMap, ok := capMap["tools"].(map[string]interface{}); ok {
		capabilities.Tools = &ToolsCapability{}
		if listChanged, ok := toolsMap["listChanged"].(bool); ok && listChanged {
			capabilities.Tools.ListChanged = true
		}
	}

	// Handle resources capability
	if resourcesMap, ok := capMap["resources"].(map[string]interface{}); ok {
		capabilities.Resources = &ResourcesCapability{}
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
			capabilities.Prompts = &PromptsCapability{}
			if listChanged, ok := promptsMap["listChanged"].(bool); ok && listChanged {
				capabilities.Prompts.ListChanged = true
			}
		} else {
			// If not the expected map type, at least create an empty PromptsCapability instance
			capabilities.Prompts = &PromptsCapability{}
		}
	}

	// Handle logging capability
	if _, ok := capMap["logging"].(map[string]interface{}); ok {
		capabilities.Logging = &LoggingCapability{}
	}

	// Handle completions capability
	if _, ok := capMap["completions"].(map[string]interface{}); ok {
		capabilities.Completions = &CompletionsCapability{}
	}

	// Handle experimental capability
	if expMap, ok := capMap["experimental"].(map[string]interface{}); ok {
		capabilities.Experimental = expMap
	}

	return capabilities
}

// HandleInitialized handles initialized notifications
func (m *LifecycleManager) HandleInitialized(ctx context.Context, notification *JSONRPCNotification, session *Session) error {
	// Mark session as initialized
	if session != nil {
		m.mu.Lock()
		m.sessionStates[session.ID] = true
		m.mu.Unlock()
		m.logger.Infof("Session %s initialization completed", session.ID)
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
	m.logger.Infof("Session terminated and removed from lifecycle manager: %s", sessionID)
}
