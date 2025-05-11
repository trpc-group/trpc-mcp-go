package scenarios

import (
	"context"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionCreation tests session creation.
func TestSessionCreation(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client.
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// Session ID should be empty initially.
	assert.Empty(t, client.GetSessionID(), "Session ID should be empty initially")

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Session ID should not be empty after initialization.
	assert.NotEmpty(t, client.GetSessionID(), "Session ID should not be empty after initialization")
}

// TestSessionTermination tests session termination.
func TestSessionTermination(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client.
	client := e2e.CreateTestClient(t, serverURL)
	defer client.Close() // Do not use CleanupClient, as we want to manually test termination.

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Get session ID.
	sessionID := client.GetSessionID()
	require.NotEmpty(t, sessionID, "Session ID should not be empty after initialization")

	// Terminate session.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.TerminateSession(ctx)
	assert.NoError(t, err, "Terminating session should not return error")
}

// TestMultipleSessions tests multiple sessions.
func TestMultipleSessions(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create and initialize first client.
	client1 := e2e.CreateTestClient(t, serverURL)
	e2e.InitializeClient(t, client1)
	sessionID1 := client1.GetSessionID()
	require.NotEmpty(t, sessionID1, "First session ID should not be empty")

	// Create and initialize second client.
	client2 := e2e.CreateTestClient(t, serverURL)
	e2e.InitializeClient(t, client2)
	sessionID2 := client2.GetSessionID()
	require.NotEmpty(t, sessionID2, "Second session ID should not be empty")

	// Verify that the two session IDs are different.
	assert.NotEqual(t, sessionID1, sessionID2, "Session IDs of different clients should be different")

	// Cleanup resources.
	e2e.CleanupClient(t, client1)
	e2e.CleanupClient(t, client2)
}

// TestSessionReconnect tests reconnection after disconnection.
func TestSessionReconnect(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create and initialize client.
	client := e2e.CreateTestClient(t, serverURL)
	e2e.InitializeClient(t, client)

	// Record original session ID.
	originalSessionID := client.GetSessionID()
	require.NotEmpty(t, originalSessionID, "Original session ID should not be empty")

	// Explicitly terminate session and close client.
	e2e.CleanupClient(t, client)

	// Create new client, connect to the same server.
	newClient := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, newClient)

	// Initialize new client.
	e2e.InitializeClient(t, newClient)

	// Get new session ID.
	newSessionID := newClient.GetSessionID()
	require.NotEmpty(t, newSessionID, "New session ID should not be empty")

	// Verify the two session IDs are different.
	assert.NotEqual(t, originalSessionID, newSessionID, "Should generate a new session ID after reconnection")
}
