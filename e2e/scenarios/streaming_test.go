package scenarios

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/e2e"
)

// TestStreamingContent tests streaming content transmission from server to client.
func TestStreamingContent(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client.
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Test streaming greet tool.
	t.Run("StreamingGreet", func(t *testing.T) {
		// Set message count.
		messageCount := 5

		// Call streaming greet tool.
		content := e2e.ExecuteTestTool(t, client, "streaming-greet", map[string]interface{}{
			"name":  "StreamingTest",
			"count": messageCount,
		})

		// Verify result.
		require.Len(t, content, messageCount, "should have the specified number of content items")

		// Verify each content item.
		for _, item := range content {
			// Type assertion to TextContent.
			textContent, ok := item.(mcp.TextContent)
			assert.True(t, ok, "content should be of type TextContent")
			assert.Equal(t, "text", textContent.Type, "content type should be text")
			assert.Contains(t, textContent.Text, "Streaming Message", "content should contain streaming message marker")
			assert.Contains(t, textContent.Text, "StreamingTest", "content should contain username")
		}
	})
}

// TestDelayedResponse tests delayed response.
func TestDelayedResponse(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client.
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Test delay tool.
	t.Run("DelayTool", func(t *testing.T) {
		// Set delay time.
		delayMs := 1000
		message := "Delay test message"

		// Record start time.
		startTime := time.Now()

		// Call delay tool.
		content := e2e.ExecuteTestTool(t, client, "delay-tool", map[string]interface{}{
			"delay_ms": delayMs,
			"message":  message,
		})

		// Calculate elapsed time.
		elapsed := time.Since(startTime)

		// Verify result.
		require.Len(t, content, 1, "should have only one content item")

		// Type assertion to TextContent.
		textContent, ok := content[0].(mcp.TextContent)
		assert.True(t, ok, "content should be of type TextContent")
		assert.Equal(t, "text", textContent.Type, "content type should be text")
		assert.Contains(t, textContent.Text, message, "content should contain the provided message")

		// Verify at least the specified delay time has elapsed.
		assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(delayMs),
			"response time should be at least the specified delay time")
	})
}

// TestContextCancellation tests context cancellation.
func TestContextCancellation(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client.
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Test context cancellation.
	t.Run("CancelDelayTool", func(t *testing.T) {
		// Create a context with short timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Call long delay tool.
		callToolReq := &mcp.CallToolRequest{}
		callToolReq.Params.Name = "delay-tool"
		callToolReq.Params.Arguments = map[string]interface{}{
			"delay_ms": 2000, // Set delay longer than context timeout.
			"message":  "This message should not be received",
		}
		_, err := client.CallTool(ctx, callToolReq)

		// Verify error.
		require.Error(t, err, "should return timeout error")
		assert.Contains(t, err.Error(), "context", "error should be related to context cancellation")
	})
}
