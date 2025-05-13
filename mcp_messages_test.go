package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewInitializeRequest(t *testing.T) {
	// Prepare test data
	protocolVersion := ProtocolVersion_2024_11_05
	clientInfo := Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	}

	// Create ClientCapabilities structure conforming to the new interface
	capabilities := ClientCapabilities{
		Roots: &RootsCapability{
			ListChanged: true,
		},
		Sampling: &SamplingCapability{},
	}

	// Execute function
	req := NewInitializeRequest(protocolVersion, clientInfo, capabilities)

	// Verify results
	assert.Equal(t, JSONRPCVersion, req.JSONRPC)
	assert.Equal(t, 1, req.ID)
	assert.Equal(t, MethodInitialize, req.Method)

	// Verify parameters
	initializeParams, ok := req.Params.(InitializeParams)
	assert.True(t, ok, "Params should be of InitializeParams type")
	assert.Equal(t, protocolVersion, initializeParams.ProtocolVersion)
	assert.NotNil(t, initializeParams.ClientInfo)            // Ensure ClientInfo is not nil before dereferencing
	assert.Equal(t, clientInfo, initializeParams.ClientInfo) // Dereference pointer for comparison
	assert.Equal(t, capabilities, initializeParams.Capabilities)
}

func TestNewInitializeResponse(t *testing.T) {
	// Prepare test data
	reqID := "init-1"
	protocolVersion := ProtocolVersion_2024_11_05
	serverInfo := Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	}

	// Create ServerCapabilities structure conforming to the new interface
	capabilities := ServerCapabilities{
		Tools: &ToolsCapability{
			ListChanged: true,
		},
	}
	instructions := "Server usage instructions"

	// Execute function
	resp := NewInitializeResponse(reqID, protocolVersion, serverInfo, capabilities, instructions)

	// Verify results
	assert.Equal(t, JSONRPCVersion, resp.JSONRPC)
	assert.Equal(t, reqID, resp.ID)
	// assert.Nil(t, resp.Error)

	// Verify result content
	result, ok := resp.Result.(InitializeResult)
	assert.True(t, ok, "Result should be of InitializeResult type")
	assert.Equal(t, protocolVersion, result.ProtocolVersion)
	assert.Equal(t, serverInfo, result.ServerInfo)
	assert.Equal(t, capabilities, result.Capabilities)
	assert.Equal(t, instructions, result.Instructions)
}

func TestNewInitializedNotification(t *testing.T) {
	// Execute function
	notification := NewInitializedNotification()

	// Verify results
	assert.Equal(t, JSONRPCVersion, notification.JSONRPC)
	assert.Equal(t, MethodNotificationsInitialized, notification.Method)
	assert.Nil(t, notification.Params)
}

func TestIsProtocolVersionSupported(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		version  string
		expected bool
	}{
		{
			name:     "Supported version 2025-03-26",
			version:  ProtocolVersion_2025_03_26,
			expected: true,
		},
		{
			name:     "Supported version 2024-11-05",
			version:  ProtocolVersion_2024_11_05,
			expected: true,
		},
		{
			name:     "Unsupported version",
			version:  "2023-01-01",
			expected: false,
		},
		{
			name:     "Empty version",
			version:  "",
			expected: false,
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsProtocolVersionSupported(tc.version)
			assert.Equal(t, tc.expected, result)
		})
	}
}
