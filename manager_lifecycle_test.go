package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLifecycleManager(t *testing.T) {
	// Create lifecycle manager
	serverInfo := Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	}

	manager := newLifecycleManager(serverInfo)

	// Verify object created successfully
	assert.NotNil(t, manager)
	assert.Equal(t, serverInfo, manager.serverInfo)
}

func TestHandleInitialize(t *testing.T) {
	// Create lifecycle manager
	serverInfo := Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	}

	manager := newLifecycleManager(serverInfo)

	// Create session
	session := newSession()

	// Test cases
	testCases := []struct {
		name            string
		protocolVersion string
		expectError     bool
		errorCode       int
	}{
		{
			name:            "Valid protocol version 2024-11-05",
			protocolVersion: ProtocolVersion_2024_11_05,
			expectError:     false,
		},
		{
			name:            "Invalid protocol version",
			protocolVersion: "2023-01-01",
			expectError:     false,
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			request := NewInitializeRequest(
				tc.protocolVersion,
				Implementation{
					Name:    "Test-Client",
					Version: "1.0.0",
				},
				ClientCapabilities{
					Roots: &RootsCapability{
						ListChanged: true,
					},
					Sampling: &SamplingCapability{},
				},
			)

			// Process request
			ctx := context.Background()
			response, err := manager.handleInitialize(ctx, request, session)

			// Verify results
			require.NoError(t, err)

			if tc.expectError {
				// Type asserts to JSONRPCError
				errorResp, ok := response.(JSONRPCError)
				require.True(t, ok, "Expected JSONRPCError but got different type")
				assert.Equal(t, tc.errorCode, errorResp.Error.Code)
			} else {
				// Type asserts to JSONRPCResponse
				initResp, ok := response.(InitializeResult)
				require.True(t, ok, "Expected InitializeResult but got different type")

				if tc.protocolVersion == "2023-01-01" {
					assert.Equal(t, ProtocolVersion_2025_03_26, initResp.ProtocolVersion)
				} else {
					assert.Equal(t, tc.protocolVersion, initResp.ProtocolVersion)
				}

				assert.Equal(t, serverInfo.Name, initResp.ServerInfo.Name)
				assert.Equal(t, serverInfo.Version, initResp.ServerInfo.Version)
				assert.NotNil(t, initResp.Capabilities)

				// Verify protocol version stored in session
				storedVersion, ok := session.GetData("protocolVersion")
				require.True(t, ok)

				if tc.protocolVersion == "2023-01-01" {
					assert.Equal(t, ProtocolVersion_2025_03_26, storedVersion)
				} else {
					assert.Equal(t, tc.protocolVersion, storedVersion)
				}
			}
		})
	}
}

func TestWithCustomCapabilities(t *testing.T) {
	// Create lifecycle manager
	serverInfo := Implementation{
		Name:    "Test-Server",
		Version: "1.0.0",
	}

	manager := newLifecycleManager(serverInfo)

	// Create and set prompt manager with a dummy prompt
	promptMgr := newPromptManager()
	dummyPrompt := &Prompt{Name: "test-prompt"} // Create a dummy prompt
	promptMgr.registerPrompt(dummyPrompt)       // Register the dummy prompt
	manager.withPromptManager(promptMgr)        // Set the prompt manager

	// Skip this test, we will update it after fixing the capabilities conversion logic

	// Set custom capabilities
	capabilities := map[string]interface{}{
		"tools": map[string]interface{}{
			"listChanged": true,
		},
		"experimental": map[string]interface{}{
			"customFeature": true,
		},
	}

	// Set capabilities
	manager.withCapabilities(capabilities)

	// Create session
	session := newSession()

	// Create request
	request := NewInitializeRequest(
		ProtocolVersion_2024_11_05,
		Implementation{
			Name:    "Test-Client",
			Version: "1.0.0",
		},
		ClientCapabilities{
			Roots: &RootsCapability{
				ListChanged: true,
			},
			Sampling: &SamplingCapability{},
		},
	)

	// Process request
	ctx := context.Background()
	response, err := manager.handleInitialize(ctx, request, session)

	// Verify results
	require.NoError(t, err)

	// Verify custom capabilities in response
	initResp, ok := response.(InitializeResult)
	require.True(t, ok)

	// Check Tools capability
	assert.NotNil(t, initResp.Capabilities.Tools, "Tools capability should not be nil")
	if initResp.Capabilities.Tools != nil {
		assert.True(t, initResp.Capabilities.Tools.ListChanged, "Tools.ListChanged should be true")
	}

	// Check Prompts capability
	assert.NotNil(t, initResp.Capabilities.Prompts, "Prompts capability should not be nil")

	// Check Experimental capability
	assert.NotNil(t, initResp.Capabilities.Experimental, "Experimental capability should not be nil")
	if initResp.Capabilities.Experimental != nil {
		customFeature, ok := initResp.Capabilities.Experimental["customFeature"].(bool)
		assert.True(t, ok, "customFeature should be a boolean")
		assert.True(t, customFeature, "customFeature should be true")
	}
}
