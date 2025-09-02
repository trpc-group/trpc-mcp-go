// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

// Implementation describes the name and version of an MCP implementation
// Corresponds to the "Implementation" definition in schema.json
type Implementation struct {
	// Name of the implementation
	Name string `json:"name"`
	// Version of the implementation
	Version string `json:"version"`
}

// ClientCapabilities describes the capabilities supported by the client
// Corresponds to the "ClientCapabilities" definition in schema.json
type ClientCapabilities struct {
	// Roots indicates whether the client supports listing roots
	// Corresponds to schema: "roots": {"description": "Present if the client supports listing roots."}
	Roots *RootsCapability `json:"roots,omitempty"`

	// Sampling indicates whether the client supports sampling from an LLM
	// Corresponds to schema: "sampling": {"description": "Present if the client supports sampling from an LLM."}
	Sampling *SamplingCapability `json:"sampling,omitempty"`

	// Experimental indicates non-standard experimental capabilities that the client supports
	// Corresponds to schema: "experimental": {"description": "Experimental, non-standard capabilities
	// that the client supports."}
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// ServerCapabilities describes the capabilities supported by the server
// Corresponds to the "ServerCapabilities" definition in schema.json
type ServerCapabilities struct {
	// Prompts indicates whether the server offers prompt templates
	// Corresponds to schema: "prompts": {"description": "Present if the server offers any prompt templates."}
	Prompts *PromptsCapability `json:"prompts,omitempty"`

	// Resources indicates whether the server offers readable resources
	// Corresponds to schema: "resources": {"description": "Present if the server offers any resources to read."}
	Resources *ResourcesCapability `json:"resources,omitempty"`

	// Tools indicates whether the server offers callable tools
	// Corresponds to schema: "tools": {"description": "Present if the server offers any tools to call."}
	Tools *ToolsCapability `json:"tools,omitempty"`

	// Logging indicates whether the server supports sending log messages
	// Corresponds to schema: "logging": {"description": "Present if the server supports sending log
	// messages to the client."}
	Logging *LoggingCapability `json:"logging,omitempty"`

	// Completions indicates whether the server supports argument autocompletion suggestions
	// Corresponds to schema: "completions": {"description": "Present if the server supports argument
	// autocompletion suggestions."}
	Completions *CompletionsCapability `json:"completions,omitempty"`

	// Experimental indicates non-standard experimental capabilities that the server supports
	// Corresponds to schema: "experimental": {"description": "Experimental, non-standard capabilities
	// that the server supports."}
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// Capability sub-structures

// RootsCapability describes client root directory capabilities
type RootsCapability struct {
	// ListChanged indicates whether the client supports notifications for changes to the roots list
	// Corresponds to schema: "listChanged": {"description": "Whether the client supports notifications
	// for changes to the roots list."}
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability describes client sampling capabilities
type SamplingCapability struct {
	// Corresponds to schema.json definition, currently has no specific fields
}

// PromptsCapability describes server prompt capabilities
type PromptsCapability struct {
	// ListChanged indicates whether the server supports notifications for changes to the prompt list
	// Corresponds to schema: "listChanged": {"description": "Whether this server supports notifications
	// for changes to the prompt list."}
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability describes server resource capabilities
type ResourcesCapability struct {
	// Subscribe indicates whether the server supports subscribing to resource updates
	// Corresponds to schema: "subscribe": {"description": "Whether this server supports subscribing to resource updates."}
	Subscribe bool `json:"subscribe,omitempty"`

	// ListChanged indicates whether the server supports notifications for changes to the resource list
	// Corresponds to schema: "listChanged": {"description": "Whether this server supports notifications
	// for changes to the resource list."}
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolsCapability describes server tool capabilities
type ToolsCapability struct {
	// ListChanged indicates whether the server supports notifications for changes to the tool list
	// Whether this server supports notifications for changes to the tool list.
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapability describes server logging capabilities
type LoggingCapability struct {
	// Corresponds to schema.json definition, currently has no specific fields
}

// CompletionsCapability describes server autocompletion capabilities
type CompletionsCapability struct {
	// Corresponds to schema.json definition, currently has no specific fields
}

// Initialize request/response related structures

// InitializeParams describes initialization request parameters
// Corresponds to schema.json InitializeRequest.params
type InitializeParams struct {
	// ProtocolVersion is the latest version of the protocol that the client supports
	// The latest version of the Model Context Protocol that the client supports.
	ProtocolVersion string `json:"protocolVersion"`

	// ClientInfo is the client implementation information
	// Corresponds to schema: "clientInfo": {"$ref": "#/definitions/Implementation"}
	ClientInfo Implementation `json:"clientInfo"`

	// Capabilities are the client capabilities
	// Corresponds to schema: "capabilities": {"$ref": "#/definitions/ClientCapabilities"}
	Capabilities ClientCapabilities `json:"capabilities"`
}

// InitializeRequest describes an initialization request
// Corresponds to the "InitializeRequest" definition in schema.json
type InitializeRequest struct {
	Request
	Params InitializeParams `json:"params"`
}

// InitializeResult describes an initialization response result
// Corresponds to the "InitializeResult" definition in schema.json
type InitializeResult struct {
	Result

	// ProtocolVersion is the version of the protocol that the server wants to use
	// The version of the Model Context Protocol that the server wants to use.
	ProtocolVersion string `json:"protocolVersion"`

	// ServerInfo is the server implementation information
	// Corresponds to schema: "serverInfo": {"$ref": "#/definitions/Implementation"}
	ServerInfo Implementation `json:"serverInfo"`

	// Capabilities are the server capabilities
	// Corresponds to schema: "capabilities": {"$ref": "#/definitions/ServerCapabilities"}
	Capabilities ServerCapabilities `json:"capabilities"`

	// Instructions describe how to use the server and its features
	// Instructions describing how to use the server and its features.
	Instructions string `json:"instructions,omitempty"`
}

// InitializedNotification describes an initialized notification
// Corresponds to the "InitializedNotification" definition in schema.json
type InitializedNotification struct {
	Notification
}

// Method constant definitions
// Using consistent naming with schema.json
const (
	// Base protocol
	MethodInitialize               = "initialize"
	MethodNotificationsInitialized = "notifications/initialized"

	// Tool related
	MethodToolsList = "tools/list"
	MethodToolsCall = "tools/call"

	// Prompt related
	MethodPromptsList        = "prompts/list"
	MethodPromptsGet         = "prompts/get"
	MethodCompletionComplete = "completion/complete"

	// Resource related
	MethodResourcesList          = "resources/list"
	MethodResourcesRead          = "resources/read"
	MethodResourcesTemplatesList = "resources/templates/list"
	MethodResourcesSubscribe     = "resources/subscribe"
	MethodResourcesUnsubscribe   = "resources/unsubscribe"

	// Roots related
	MethodRootsList                     = "roots/list"
	MethodNotificationsRootsListChanged = "notifications/roots/list_changed"

	// Utilities
	MethodLoggingSetLevel = "logging/setLevel"
	MethodPing            = "ping"
)

// Protocol version constants
const (
	ProtocolVersion_2024_11_05 = "2024-11-05"
	ProtocolVersion_2025_03_26 = "2025-03-26"
)

// List of supported protocol versions, ordered by priority
var SupportedProtocolVersions = []string{
	ProtocolVersion_2025_03_26,
	ProtocolVersion_2024_11_05,
}

// IsProtocolVersionSupported checks if a protocol version is supported
func IsProtocolVersionSupported(version string) bool {
	for _, v := range SupportedProtocolVersions {
		if v == version {
			return true
		}
	}
	return false
}

// Helper functions

// NewInitializeRequest creates an initialization request
func NewInitializeRequest(
	protocolVersion string,
	clientInfo Implementation,
	capabilities ClientCapabilities,
) *JSONRPCRequest {
	params := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"clientInfo":      clientInfo,
		"capabilities":    capabilities,
	}
	return newJSONRPCRequest(1, MethodInitialize, params)
}

// NewInitializeResponse creates an initialization response
func NewInitializeResponse(
	reqID interface{},
	protocolVersion string,
	serverInfo Implementation,
	capabilities ServerCapabilities,
	instructions string,
) *JSONRPCResponse {
	result := InitializeResult{
		ProtocolVersion: protocolVersion,
		ServerInfo:      serverInfo,
		Capabilities:    capabilities,
		Instructions:    instructions,
	}
	return newJSONRPCResponse(reqID, result)
}

// NewInitializedNotification creates an initialized notification
func NewInitializedNotification() *JSONRPCNotification {
	return newJSONRPCNotification(Notification{Method: MethodNotificationsInitialized, Params: NotificationParams{}})
}
