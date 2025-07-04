// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

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

// handleDummyPrompt handles the dummy prompt
func handleDummyPrompt(ctx context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
	return &GetPromptResult{
		Description: "Dummy prompt response",
		Messages: []PromptMessage{
			{
				Role:    RoleAssistant,
				Content: NewTextContent("Dummy prompt response"),
			},
		},
	}, nil
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
	dummyPrompt := &Prompt{Name: "test-prompt"}              // Create a dummy prompt
	promptMgr.registerPrompt(dummyPrompt, handleDummyPrompt) // Register the dummy prompt
	manager.withPromptManager(promptMgr)                     // Set the prompt manager

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

// TestStatelessCrossNodeInitialization tests the cross-node scenario in stateless mode
// where initialize request goes to node A but notifications/initialized goes to node B
func TestStatelessCrossNodeInitialization(t *testing.T) {
	// Create two servers representing different nodes.
	serverA := NewServer("Node-A", "1.0.0", WithStatelessMode(true))
	serverB := NewServer("Node-B", "1.0.0", WithStatelessMode(true))

	// Node A: Build initialize JSON-RPC request directly (stateless mode).
	jsonReq := newJSONRPCRequest(1, MethodInitialize, map[string]interface{}{
		"protocolVersion": ProtocolVersion_2025_03_26,
		"clientInfo": Implementation{
			Name:    "Test-Client",
			Version: "1.0.0",
		},
		"capabilities": ClientCapabilities{},
	})

	// Create temporary session for Node A (simulating stateless mode).
	sessionA := newSession()

	// Node A processes initialize request.
	ctx := context.Background()
	initResp, err := serverA.mcpHandler.handleRequest(ctx, jsonReq, sessionA)
	require.NoError(t, err)
	assert.NotNil(t, initResp)

	// Node B: Handle notifications/initialized (different session, simulating cross-node).
	sessionB := newSession() // Different session ID from sessionA.

	// Create initialized notification
	initNotification := NewInitializedNotification()

	// Node B processes notifications/initialized - this should NOT fail.
	err = serverB.mcpHandler.handleNotification(ctx, initNotification, sessionB)
	assert.NoError(t, err, "Cross-node notifications/initialized should succeed in stateless mode")
}

// TestStatefulCrossNodeInitialization tests that stateful mode still validates session state.
func TestStatefulCrossNodeInitialization(t *testing.T) {
	// Create server in stateful mode
	server := NewServer("Stateful-Server", "1.0.0") // Default is stateful.

	// Create session that has NOT been through initialize.
	session := newSession()

	// Create initialized notification.
	initNotification := NewInitializedNotification()

	// This should fail because session was not initialized.
	ctx := context.Background()
	err := server.mcpHandler.handleNotification(ctx, initNotification, session)
	assert.Error(t, err, "notifications/initialized should fail for uninitialized session in stateful mode")
}

// TestStatelessModeSkipsLifecycleManager tests that stateless mode skips lifecycle manager.
func TestStatelessModeSkipsLifecycleManager(t *testing.T) {
	// Create stateless server.
	server := NewServer("Stateless-Server", "1.0.0", WithStatelessMode(true))

	// Verify that the lifecycle manager is configured with stateless mode.
	assert.True(t, server.mcpHandler.lifecycleManager.isStateless, "Lifecycle manager should be in stateless mode")

	// Create temporary session.
	session := newSession()

	// Create initialized notification.
	initNotification := NewInitializedNotification()

	// This should succeed without any session state validation.
	ctx := context.Background()
	err := server.mcpHandler.lifecycleManager.handleInitialized(ctx, initNotification, session)
	assert.NoError(t, err, "handleInitialized should succeed in stateless mode")
}
