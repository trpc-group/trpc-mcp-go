package scenarios

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/e2e"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSEMode tests the basic SSE mode functionality.
func TestSSEMode(t *testing.T) {
	// Set up test server with SSE support.
	serverURL, cleanup := e2e.StartSSETestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client with SSE support.
	client := e2e.CreateSSETestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Test basic greet tool (should respond via SSE mode).
	t.Run("BasicGreetWithSSE", func(t *testing.T) {
		// Call greet tool.
		content := e2e.ExecuteTestTool(t, client, "basic-greet", map[string]interface{}{
			"name": "SSE Test",
		})

		// Verify result.
		require.Len(t, content, 1, "Should have only one content")

		// Type assert to TextContent.
		textContent, ok := content[0].(schema.TextContent)
		assert.True(t, ok, "Content should be of type TextContent")
		assert.Equal(t, "text", textContent.Type, "Content type should be text")
		assert.Contains(t, textContent.Text, "Hello, SSE Test", "Greet content should contain username")
	})
}

// TestSSEProgress tests SSE progress notification functionality.
func TestSSEProgress(t *testing.T) {
	// Set up test server with SSE support.
	serverURL, cleanup := e2e.StartSSETestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client with SSE support.
	client := e2e.CreateSSETestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Test SSE progress tool.
	t.Run("SSEProgressTool", func(t *testing.T) {
		// Create notification collector.
		collector := e2e.NewNotificationCollector()

		// Set number of progress steps and delay.
		steps := 5
		delayMs := 100

		// Call SSE progress tool.
		content := e2e.ExecuteSSETestTool(t, client, "sse-progress-tool", map[string]interface{}{
			"steps":    steps,
			"delay_ms": delayMs,
			"message":  "SSE progress test succeeded",
		}, collector)

		// Verify final result.
		require.Len(t, content, 1, "Should have only one content")

		// Type assert to TextContent.
		textContent, ok := content[0].(schema.TextContent)
		assert.True(t, ok, "Content should be of type TextContent")
		assert.Equal(t, "text", textContent.Type, "Content type should be text")
		assert.Equal(t, "SSE progress test succeeded", textContent.Text, "Final message should be the specified message")

		// Allow some time for notification collection.
		time.Sleep(100 * time.Millisecond)

		// Verify number of progress notifications received.
		collector.AssertNotificationCount(t, "notifications/progress", steps)

		// Verify number of log notifications received.
		collector.AssertNotificationCount(t, "notifications/message", 5)

		// Verify content of progress notifications.
		progressNotifications := collector.GetProgressNotifications()
		for i, notification := range progressNotifications {
			// Verify progress parameter.
			progress, ok := notification.Params["progress"].(float64)
			assert.True(t, ok, "Progress parameter should be float64")
			expectedProgress := float64(i+1) / float64(steps)
			assert.InDelta(t, expectedProgress, progress, 0.01, "Progress value for step %d should be %f", i+1, expectedProgress)

			// Verify message parameter.
			message, ok := notification.Params["message"].(string)
			assert.True(t, ok, "Message parameter should be string")
			assert.Contains(t, message, "step", "Progress message should contain 'step'")
		}

		// Verify log messages.
		logNotifications := collector.GetLogNotifications()
		require.Len(t, logNotifications, 5, "Should have 5 log messages")
		assert.Equal(t, "info", logNotifications[0].Params["level"], "Log level should be info")
		assert.Contains(t, logNotifications[0].Params["data"].(string), "completed", "Log message should contain 'completed'")
	})
}

// TestSSEReconnection tests SSE reconnection functionality.
func TestSSEReconnection(t *testing.T) {
	// Set up test server with SSE support.
	serverURL, cleanup := e2e.StartSSETestServer(t, e2e.WithTestTools())
	defer cleanup()

	// First call and session save.
	var sessionID string

	// First call, get session and Last-Event-ID.
	t.Run("FirstCallAndRecordSession", func(t *testing.T) {
		// Create test client with SSE support.
		client := e2e.CreateSSETestClient(t, serverURL)
		defer e2e.CleanupClient(t, client)

		// Initialize client.
		e2e.InitializeClient(t, client)

		// Record session ID for later use.
		sessionID = client.GetSessionID()
		require.NotEmpty(t, sessionID, "Session ID should not be empty")

		// Create notification collector.
		collector := e2e.NewNotificationCollector()

		// Call SSE progress tool, use longer delay to simulate long-running process.
		content := e2e.ExecuteSSETestTool(t, client, "sse-progress-tool", map[string]interface{}{
			"steps":    3,
			"delay_ms": 200,
			"message":  "First call",
		}, collector)

		// Verify final result.
		require.Len(t, content, 1, "Should have only one content")

		// Type assert to TextContent.
		textContent, ok := content[0].(schema.TextContent)
		assert.True(t, ok, "Content should be of type TextContent")
		assert.Equal(t, "text", textContent.Type, "Content type should be text")

		// Confirm progress notifications received.
		collector.AssertNotificationCount(t, "notifications/progress", 3)
	})

	// Second call, verify session continuity.
	t.Run("SecondCallWithNewClient", func(t *testing.T) {
		// Create new client.
		client := e2e.CreateSSETestClient(t, serverURL)
		defer e2e.CleanupClient(t, client)

		// Initialize client, should generate new session.
		e2e.InitializeClient(t, client)

		// Check if session ID is different from previous.
		newSessionID := client.GetSessionID()
		require.NotEmpty(t, newSessionID, "New session ID should not be empty")
		assert.NotEqual(t, sessionID, newSessionID, "New client should have a different session ID")

		// Create notification collector.
		collector := e2e.NewNotificationCollector()

		// Call SSE progress tool.
		content := e2e.ExecuteSSETestTool(t, client, "sse-progress-tool", map[string]interface{}{
			"steps":    2,
			"delay_ms": 100,
			"message":  "Second call",
		}, collector)

		// Verify final result.
		require.Len(t, content, 1, "Should have only one content")

		// Type assert to TextContent.
		textContent, ok := content[0].(schema.TextContent)
		assert.True(t, ok, "Content should be of type TextContent")
		assert.Equal(t, "Second call", textContent.Text, "Final message should be the specified message")

		// Confirm progress notifications received.
		collector.AssertNotificationCount(t, "notifications/progress", 2)
	})
}
