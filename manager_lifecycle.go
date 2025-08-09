// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"sync"

	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// lifecycleManager is responsible for managing the MCP protocol lifecycle
type lifecycleManager struct {
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
	toolManager *toolManager

	// Resource manager reference
	resourceManager *resourceManager

	// Prompt manager reference
	promptManager *promptManager

	// Whether in stateless mode.
	isStateless bool

	// Mutex for concurrent access
	mu sync.RWMutex
}

// newLifecycleManager creates a lifecycle manager
func newLifecycleManager(serverInfo Implementation) *lifecycleManager {
	return &lifecycleManager{
		logger:                 GetDefaultLogger(), // Use default logger if not set.
		serverInfo:             serverInfo,
		defaultProtocolVersion: ProtocolVersion_2025_03_26,
		supportedVersions:      []string{ProtocolVersion_2024_11_05, ProtocolVersion_2025_03_26},
		capabilities: map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": true,
			},
		},
		sessionStates: make(map[string]bool),
	}
}

// WithProtocolVersion sets the default protocol version
func (m *lifecycleManager) withProtocolVersion(version string) *lifecycleManager {
	m.defaultProtocolVersion = version
	return m
}

// withSupportedVersions sets the supported protocol versions
func (m *lifecycleManager) withSupportedVersions(versions []string) *lifecycleManager {
	m.supportedVersions = versions
	return m
}

// withCapabilities sets the capabilities
func (m *lifecycleManager) withCapabilities(capabilities map[string]interface{}) *lifecycleManager {
	m.capabilities = capabilities
	return m
}

// withToolManager sets the tool manager
func (m *lifecycleManager) withToolManager(toolManager *toolManager) *lifecycleManager {
	m.toolManager = toolManager
	return m
}

// withResourceManager sets the resource manager
func (m *lifecycleManager) withResourceManager(resourceManager *resourceManager) *lifecycleManager {
	m.resourceManager = resourceManager
	return m
}

// withPromptManager sets the prompt manager.
func (m *lifecycleManager) withPromptManager(promptManager *promptManager) *lifecycleManager {
	m.promptManager = promptManager
	return m
}

// withClientTransportLogger sets the logger for lifecycleManager.
func (m *lifecycleManager) withLogger(logger Logger) *lifecycleManager {
	m.logger = logger
	return m
}

// withStatelessMode sets the stateless mode flag for lifecycleManager.
func (m *lifecycleManager) withStatelessMode(isStateless bool) *lifecycleManager {
	m.isStateless = isStateless
	return m
}

// updateCapabilities updates the server capability information
func (m *lifecycleManager) updateCapabilities() {
	// Use map as an intermediate variable
	capMap := map[string]interface{}{}

	// Basic tool capabilities always exist
	capMap["tools"] = map[string]interface{}{
		"listChanged": true,
	}

	// If there is a resource manager and resources are registered, add resource capabilities
	if m.resourceManager != nil && len(m.resourceManager.getResources()) > 0 {
		capMap["resources"] = map[string]interface{}{
			"listChanged": true,
		}
	}

	// If there is a prompt manager and prompts are registered, add prompt capabilities
	if m.promptManager != nil && len(m.promptManager.getPrompts()) > 0 {
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

// handleInitialize handles initialize requests
func (m *lifecycleManager) handleInitialize(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
	if errResp := m.checkInitializeParams(req); errResp != nil {
		return errResp, nil
	}

	paramsMap := req.Params.(map[string]interface{})
	protocolVersion := paramsMap["protocolVersion"].(string)
	supportedVersion := m.selectSupportedVersion(protocolVersion)
	m.logProtocolVersion(protocolVersion, supportedVersion)
	m.saveSessionState(session, supportedVersion)
	m.updateCapabilities()
	response := m.buildInitializeResponse(supportedVersion)
	return response, nil
}

// checkInitializeParams validates the request parameters for initialization
func (m *lifecycleManager) checkInitializeParams(req *JSONRPCRequest) JSONRPCMessage {
	if req.Params == nil {
		return NewJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil)
	}
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return NewJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrInvalidParams.Error(), nil)
	}
	if _, ok := paramsMap["protocolVersion"].(string); !ok {
		return NewJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil)
	}
	return nil
}

// selectSupportedVersion returns the supported protocol version
func (m *lifecycleManager) selectSupportedVersion(protocolVersion string) string {
	for _, version := range m.supportedVersions {
		if version == protocolVersion {
			return protocolVersion
		}
	}
	return m.defaultProtocolVersion
}

// logProtocolVersion logs protocol version selection
func (m *lifecycleManager) logProtocolVersion(requested, selected string) {
	if requested != selected {
		m.logger.Debugf("Client requested protocol version %s is not supported, using %s", requested, selected)
	} else {
		m.logger.Debugf("Using protocol version: %s", selected)
	}
}

// saveSessionState marks session initialization and saves protocol version
func (m *lifecycleManager) saveSessionState(session Session, protocolVersion string) {
	if session != nil {
		m.mu.Lock()
		m.sessionStates[session.GetID()] = false // Initialization started but not completed
		m.mu.Unlock()
		// Save protocol version to session data
		session.SetData("protocolVersion", protocolVersion)
	}
}

// buildInitializeResponse creates the initialization response
func (m *lifecycleManager) buildInitializeResponse(protocolVersion string) InitializeResult {
	return InitializeResult{
		ProtocolVersion: protocolVersion,
		ServerInfo: Implementation{
			Name:    m.serverInfo.Name,
			Version: m.serverInfo.Version,
		},
		Capabilities: convertToServerCapabilities(m.capabilities),
		Instructions: "MCP server is ready",
	}
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

// handleInitialized handles initialized notifications
func (m *lifecycleManager) handleInitialized(ctx context.Context, notification *JSONRPCNotification, session Session) error {
	// In stateless mode, skip session state check.
	if m.isStateless {
		m.logger.Debug("Stateless mode: Skipping session state check for notifications/initialized")
		return nil
	}

	if session == nil {
		// Or handle as a global initialized event if applicable
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessionStates[session.GetID()]; !exists {
		// This case should ideally not happen if handleInitialize was called first for the session
		return errors.ErrSessionNotInitialized // Session not found in states, wasn't being initialized
	}
	if m.sessionStates[session.GetID()] {
		return errors.ErrSessionAlreadyInitialized
	}
	m.sessionStates[session.GetID()] = true
	m.logger.Infof("Session %s initialized.", session.GetID())
	return nil
}

// isInitialized checks if a session is initialized
func (m *lifecycleManager) isInitialized(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessionStates[sessionID]
}

// OnSessionTerminated handles session termination events
func (m *lifecycleManager) onSessionTerminated(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessionStates, sessionID)
	m.logger.Infof("Session %s terminated, removed initialization state.", sessionID)
}
