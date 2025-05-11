package scenarios

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-mcp-go/e2e"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

// TestBasicWorkflow tests the basic client-server workflow.
// Includes connection, initialization, tool listing, tool calling, and disconnection.
func TestBasicWorkflow(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client.
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Test basic greet tool.
	t.Run("BasicGreet", func(t *testing.T) {
		// Call greet tool.
		content := e2e.ExecuteTestTool(t, client, "basic-greet", map[string]interface{}{
			"name": "e2e-test",
		})

		// Verify result.
		require.Len(t, content, 1, "Should have only one content")

		// Type assert to TextContent.
		textContent, ok := content[0].(mcp.TextContent)
		assert.True(t, ok, "Content should be of type TextContent")
		assert.Equal(t, "text", textContent.Type, "Content type should be text")
		assert.Contains(t, textContent.Text, "Hello, e2e-test", "Greet content should contain username")
	})
}

// TestErrorHandling tests error handling logic.
func TestErrorHandling(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client.
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Test error tool.
	t.Run("ErrorTool", func(t *testing.T) {
		// Set error message.
		errorMsg := "This is an expected test error."

		// Create context.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Call error tool.
		_, err := client.CallTool(ctx, "error-tool", map[string]interface{}{
			"error_message": errorMsg,
		})

		// Verify error.
		require.Error(t, err, "Should return error")
		assert.Contains(t, err.Error(), errorMsg, "Error message should contain the provided error message")
	})
}

// TestMultipleCalls tests multiple consecutive calls.
func TestMultipleCalls(t *testing.T) {
	// Set up test server.
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// Create test client.
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Perform multiple calls.
	names := []string{"Tom", "Lucy", "John"}

	for i, name := range names {
		t.Run(fmt.Sprintf("%s/%s", t.Name(), name), func(t *testing.T) {
			// Call greet tool.
			content := e2e.ExecuteTestTool(t, client, "basic-greet", map[string]interface{}{
				"name": name,
			})

			// Verify result.
			require.Len(t, content, 1, "Should have only one content")

			// Type assert to TextContent.
			textContent, ok := content[0].(mcp.TextContent)
			assert.True(t, ok, "Content should be of type TextContent")
			assert.Equal(t, "text", textContent.Type, "Content type should be text")
			assert.Contains(t, textContent.Text, "Hello, "+name, "Greet content should contain username")

			// Add delay for subsequent calls to simulate real scenario.
			if i < len(names)-1 {
				time.Sleep(200 * time.Millisecond)
			}
		})
	}
}
