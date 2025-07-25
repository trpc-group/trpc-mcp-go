// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// TestServerRootsProvider_Streamable tests server-to-client roots functionality over Streamable transport.
func TestServerRootsProvider_Streamable(t *testing.T) {
	// Start a test server
	serverURL, cleanup := StartTestServer(t, WithTestTools())
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client with RootsProvider capability
	client, err := mcp.NewClient(
		serverURL,
		mcp.Implementation{
			Name:    "roots-streamable-test-client",
			Version: "1.0.0",
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	)
	require.NoError(t, err)
	defer client.Close()

	// Set up roots provider with test directories
	rootsProvider := mcp.NewDefaultRootsProvider()
	rootsProvider.AddRoot("/tmp", "Temporary Directory")
	rootsProvider.AddRoot("/home", "Home Directory")
	client.SetRootsProvider(rootsProvider)

	// Initialize with roots capability
	initResult, err := client.Initialize(ctx, &mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.ProtocolVersion_2025_03_26,
			ClientInfo: mcp.Implementation{
				Name:    "roots-streamable-test-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{
				Roots: &mcp.RootsCapability{
					ListChanged: true,
				},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, mcp.ProtocolVersion_2025_03_26, initResult.ProtocolVersion)

	// Test sending roots list changed notification
	err = client.SendRootsListChangedNotification(ctx)
	require.NoError(t, err)

	// Give the server a moment to process the notification
	time.Sleep(100 * time.Millisecond)
}

// TestServerRootsProvider_Notification tests roots notification handling.
func TestServerRootsProvider_Notification(t *testing.T) {
	// Start a test server
	serverURL, cleanup := StartTestServer(t, WithTestTools())
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client with RootsProvider capability
	client, err := mcp.NewClient(
		serverURL,
		mcp.Implementation{
			Name:    "roots-notification-test-client",
			Version: "1.0.0",
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	)
	require.NoError(t, err)
	defer client.Close()

	// Set up roots provider with test directories
	rootsProvider := mcp.NewDefaultRootsProvider()
	rootsProvider.AddRoot("/opt", "Optional Directory")
	client.SetRootsProvider(rootsProvider)

	// Initialize with roots capability
	_, err = client.Initialize(ctx, &mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.ProtocolVersion_2025_03_26,
			ClientInfo: mcp.Implementation{
				Name:    "roots-notification-test-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{
				Roots: &mcp.RootsCapability{
					ListChanged: true,
				},
			},
		},
	})
	require.NoError(t, err)

	// Register a tool that uses ListRoots
	server := mcp.GetServerFromContext(ctx)
	if server != nil {
		if s, ok := server.(*mcp.Server); ok {
			listRootsTool := mcp.NewTool("list-streamable-roots",
				mcp.WithDescription("List client's root directories via Streamable transport"),
			)

			s.RegisterTool(listRootsTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				// Call ListRoots to get client roots
				roots, err := s.ListRoots(ctx)
				if err != nil {
					return mcp.NewErrorResult(fmt.Sprintf("Failed to list roots: %v", err)), nil
				}

				// Format response
				message := fmt.Sprintf("Streamable client has %d root directories:\n", len(roots.Roots))
				for i, root := range roots.Roots {
					message += fmt.Sprintf("%d. %s (%s)\n", i+1, root.Name, root.URI)
				}

				return mcp.NewTextResult(message), nil
			})

			// Call the tool to test ListRoots functionality
			result, err := client.CallTool(ctx, &mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "list-streamable-roots",
				},
			})

			if err == nil {
				// Verify the response contains our root directories
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok)
				assert.Contains(t, textContent.Text, "Optional Directory")
			}
		}
	}

	// Send roots list changed notification
	err = client.SendRootsListChangedNotification(ctx)
	require.NoError(t, err)

	// Give the server a moment to process the notification
	time.Sleep(100 * time.Millisecond)
}
