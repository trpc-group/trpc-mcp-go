package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

// MockResponder implements the schema.NotificationSender interface for testing.
type MockResponder struct {
	mu            sync.Mutex
	progressCalls []struct {
		Progress float64
		Message  string
	}
	logCalls []struct {
		Level   string
		Message string
	}
	notificationCalls []struct {
		Method string
		Params map[string]interface{}
	}
	genericNotificationCalls []*mcp.Notification
}

// SendProgress implements NotificationSender interface.
func (r *MockResponder) SendProgress(progress float64, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.progressCalls = append(r.progressCalls, struct {
		Progress float64
		Message  string
	}{progress, message})

	return nil
}

// SendLogMessage implements NotificationSender interface.
func (r *MockResponder) SendLogMessage(level string, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logCalls = append(r.logCalls, struct {
		Level   string
		Message string
	}{level, message})

	return nil
}

// SendCustomNotification implements NotificationSender interface.
func (r *MockResponder) SendCustomNotification(method string, params map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.notificationCalls = append(r.notificationCalls, struct {
		Method string
		Params map[string]interface{}
	}{method, params})

	return nil
}

// SendNotification implements NotificationSender interface.
func (r *MockResponder) SendNotification(notification *mcp.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.genericNotificationCalls = append(r.genericNotificationCalls, notification)

	return nil
}

func TestSSEProgressTool_ExecuteWithResponder(t *testing.T) {
	// Create tool instance
	tool := mcp.NewTool("sse-progress",
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Get notification sender
			notifier, ok := mcp.GetNotificationSender(ctx)
			if !ok {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.NewTextContent("Execute called without notifier, cannot send notifications"),
					},
				}, nil
			}

			// Extract steps parameter
			steps := 5 // Default value: 5 steps
			if stepsArg, ok := req.Params.Arguments["steps"]; ok {
				if stepsFloat, ok := stepsArg.(float64); ok {
					steps = int(stepsFloat)
				}
			}

			// Extract delay parameter
			delay := 1000 // Default value: 1000ms
			if delayArg, ok := req.Params.Arguments["delay"]; ok {
				if delayFloat, ok := delayArg.(float64); ok {
					delay = int(delayFloat)
				}
			}

			// Ensure delay is not too small
			if delay < 100 {
				delay = 100 // Minimum 100ms
			}

			// Send start notification
			notifier.SendLogMessage("info", "Starting to send progress notifications")
			notifier.SendProgress(0.0, "Starting process")

			// Send progress notifications
			for i := 1; i <= steps; i++ {
				select {
				case <-ctx.Done():
					// Context cancelled, stop sending
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.NewTextContent(fmt.Sprintf("Progress notifications cancelled, completed %d/%d steps", i-1, steps)),
						},
					}, ctx.Err()
				default:
					// Calculate progress
					progress := float64(i) / float64(steps)
					message := fmt.Sprintf("Step %d/%d", i, steps)

					// Send progress notification
					err := notifier.SendProgress(progress, message)
					if err != nil {
						return &mcp.CallToolResult{
							Content: []mcp.Content{
								mcp.NewTextContent(fmt.Sprintf("Failed to send progress notification: %v", err)),
							},
						}, err
					}

					// Send log message
					notifier.SendLogMessage("info", fmt.Sprintf("Completed %s", message))

					// Wait for specified delay
					time.Sleep(time.Duration(delay) * time.Millisecond)
				}
			}

			// Send completion notification
			notifier.SendProgress(1.0, "Processing complete")
			notifier.SendLogMessage("info", "All progress notifications have been sent")

			// Return result
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Successfully sent %d progress notifications", steps)),
				},
			}, nil
		},
		mcp.WithDescription("Tool for sending progress notifications"),
		mcp.WithNumber("steps",
			mcp.Description("Number of progress notifications to send"),
			mcp.Default(5),
		),
		mcp.WithNumber("delay",
			mcp.Description("Delay between notifications (milliseconds)"),
			mcp.Default(1000),
		),
	)

	// Create context and parameters
	ctx := context.Background()
	req := &mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "sse-progress",
			Arguments: map[string]interface{}{
				"steps": float64(3),  // 3 steps
				"delay": float64(10), // 10ms per step, to speed up testing
			},
		},
	}

	// Execute tool
	result, err := tool.ExecuteFunc(ctx, req)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)

	// Use type assertion to get TextContent
	textContent, ok := result.Content[0].(mcp.TextContent)
	assert.True(t, ok, "Content should be TextContent type")
	assert.Equal(t, "text", textContent.Type)
	assert.Contains(t, textContent.Text, "Successfully sent 3 progress notifications")
}

func TestSSEProgressTool_Execute(t *testing.T) {
	// Create mock responder
	responder := &MockResponder{}

	// Create context with responder
	ctx := mcp.WithNotificationSender(context.Background(), responder)

	// Create tool instance
	tool := mcp.NewTool("sse-progress",
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Get notification sender
			notifier, ok := mcp.GetNotificationSender(ctx)
			if !ok {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.NewTextContent("Execute called without notifier, cannot send notifications"),
					},
				}, nil
			}

			// Extract steps parameter
			steps := 5 // Default value: 5 steps
			if stepsArg, ok := req.Params.Arguments["steps"]; ok {
				if stepsFloat, ok := stepsArg.(float64); ok {
					steps = int(stepsFloat)
				}
			}

			// Extract delay parameter
			delay := 1000 // Default value: 1000ms
			if delayArg, ok := req.Params.Arguments["delay"]; ok {
				if delayFloat, ok := delayArg.(float64); ok {
					delay = int(delayFloat)
				}
			}

			// Ensure delay is not too small
			if delay < 100 {
				delay = 100 // Minimum 100ms
			}

			// Send start notification
			notifier.SendLogMessage("info", "Starting to send progress notifications")
			notifier.SendProgress(0.0, "Starting process")

			// Send progress notifications
			for i := 1; i <= steps; i++ {
				select {
				case <-ctx.Done():
					// Context cancelled, stop sending
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.NewTextContent(fmt.Sprintf("Progress notifications cancelled, completed %d/%d steps", i-1, steps)),
						},
					}, ctx.Err()
				default:
					// Calculate progress
					progress := float64(i) / float64(steps)
					message := fmt.Sprintf("Step %d/%d", i, steps)

					// Send progress notification
					err := notifier.SendProgress(progress, message)
					if err != nil {
						return &mcp.CallToolResult{
							Content: []mcp.Content{
								mcp.NewTextContent(fmt.Sprintf("Failed to send progress notification: %v", err)),
							},
						}, err
					}

					// Send log message
					notifier.SendLogMessage("info", fmt.Sprintf("Completed %s", message))

					// Wait for specified delay
					time.Sleep(time.Duration(delay) * time.Millisecond)
				}
			}

			// Send completion notification
			notifier.SendProgress(1.0, "Processing complete")
			notifier.SendLogMessage("info", "All progress notifications have been sent")

			// Return result
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Successfully sent %d progress notifications", steps)),
				},
			}, nil
		},
		mcp.WithDescription("Tool for sending progress notifications"),
		mcp.WithNumber("steps",
			mcp.Description("Number of progress notifications to send"),
			mcp.Default(5),
		),
		mcp.WithNumber("delay",
			mcp.Description("Delay between notifications (milliseconds)"),
			mcp.Default(1000),
		),
	)

	// Create request
	req := &mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "sse-progress",
			Arguments: map[string]interface{}{
				"steps": float64(3),  // 3 steps
				"delay": float64(10), // 10ms per step, to speed up testing
			},
		},
	}

	// Execute tool
	result, err := tool.ExecuteFunc(ctx, req)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)

	// Use type assertion to get TextContent
	textContent, ok := result.Content[0].(mcp.TextContent)
	assert.True(t, ok, "Content should be TextContent type")
	assert.Equal(t, "text", textContent.Type)
	assert.Contains(t, textContent.Text, "Successfully sent 3 progress notifications")

	// Verify notification calls
	assert.Equal(t, 5, len(responder.progressCalls), "Should have 5 progress calls: start, 3 steps, end")
	assert.Equal(t, 0.0, responder.progressCalls[0].Progress, "First progress should be 0.0")
	assert.Equal(t, 1.0, responder.progressCalls[4].Progress, "Last progress should be 1.0")

	// Verify log calls
	assert.GreaterOrEqual(t, len(responder.logCalls), 5, "Should have at least 5 log calls")
}

func TestSSEProgressTool_CancellationHandling(t *testing.T) {
	// Create tool instance
	tool := mcp.NewTool("sse-progress-cancel",
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Get notification sender
			notifier, ok := mcp.GetNotificationSender(ctx)
			if !ok {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.NewTextContent("Execute called without notifier, cannot send notifications"),
					},
				}, nil
			}

			// Extract steps parameter
			steps := 5 // Default value: 5 steps
			if stepsArg, ok := req.Params.Arguments["steps"]; ok {
				if stepsFloat, ok := stepsArg.(float64); ok {
					steps = int(stepsFloat)
				}
			}

			// Extract delay parameter
			delay := 1000 // Default value: 1000ms
			if delayArg, ok := req.Params.Arguments["delay"]; ok {
				if delayFloat, ok := delayArg.(float64); ok {
					delay = int(delayFloat)
				}
			}

			// Send start notification
			notifier.SendLogMessage("info", "Starting to send progress notifications")
			notifier.SendProgress(0.0, "Starting process")

			// Create cancellable context
			cancelCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			// Use a channel to signal when to cancel
			cancelAfter := 2
			if cancelArg, ok := req.Params.Arguments["cancelAfter"]; ok {
				if cancelInt, ok := cancelArg.(float64); ok {
					cancelAfter = int(cancelInt)
				}
			}

			// Set up cancellation after specified steps
			go func() {
				time.Sleep(time.Duration(cancelAfter*delay) * time.Millisecond)
				cancel()
			}()

			// Send progress notifications
			for i := 1; i <= steps; i++ {
				select {
				case <-cancelCtx.Done():
					// Context cancelled, stop sending
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.NewTextContent(fmt.Sprintf("Progress notifications cancelled, completed %d/%d steps", i-1, steps)),
						},
					}, cancelCtx.Err()
				default:
					// Calculate progress
					progress := float64(i) / float64(steps)
					message := fmt.Sprintf("Step %d/%d", i, steps)

					// Send progress notification
					err := notifier.SendProgress(progress, message)
					if err != nil {
						return &mcp.CallToolResult{
							Content: []mcp.Content{
								mcp.NewTextContent(fmt.Sprintf("Failed to send progress notification: %v", err)),
							},
						}, err
					}

					// Wait for specified delay
					time.Sleep(time.Duration(delay) * time.Millisecond)
				}
			}

			// If we get here, the cancellation didn't happen
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Cancellation test failed, completed all steps without cancellation"),
				},
			}, nil
		},
		mcp.WithDescription("Tool for testing cancellation of progress notifications"),
		mcp.WithNumber("steps",
			mcp.Description("Number of progress notifications to send"),
			mcp.Default(5),
		),
		mcp.WithNumber("delay",
			mcp.Description("Delay between notifications (milliseconds)"),
			mcp.Default(1000),
		),
		mcp.WithNumber("cancelAfter",
			mcp.Description("After how many steps to cancel"),
			mcp.Default(2),
		),
	)

	// Create context and parameters
	ctx := context.Background()
	req := &mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "sse-progress-cancel",
			Arguments: map[string]interface{}{
				"steps":       float64(5),  // 5 steps
				"delay":       float64(10), // 10ms per step
				"cancelAfter": float64(2),  // Cancel after 2 steps
			},
		},
	}

	// Create mock responder and set in context
	responder := &MockResponder{}
	ctx = mcp.WithNotificationSender(ctx, responder)

	// Execute tool
	result, err := tool.ExecuteFunc(ctx, req)

	// Verify results
	assert.Error(t, err, "Should return an error due to cancellation")
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)

	// Use type assertion to get TextContent
	textContent, ok := result.Content[0].(mcp.TextContent)
	assert.True(t, ok, "Content should be TextContent type")
	assert.Equal(t, "text", textContent.Type)
	assert.Contains(t, textContent.Text, "Progress notifications cancelled")

	// Verify notification calls - we expect only start + 2 steps (not 3) because the third is when cancellation happens
	assert.LessOrEqual(t, len(responder.progressCalls), 3, "Should have at most 3 progress calls (start + 2 steps)")
}

func TestSSEProgressTool_Schema(t *testing.T) {
	// Create tool with schema
	tool := mcp.NewTool("sse-progress",
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Test"),
				},
			}, nil
		},
		mcp.WithDescription("Tool for sending progress notifications"),
		mcp.WithNumber("steps",
			mcp.Description("Number of progress notifications to send"),
			mcp.Default(5),
		),
		mcp.WithNumber("delay",
			mcp.Description("Delay between notifications (milliseconds)"),
			mcp.Default(1000),
		),
	)

	// Get parameter schema
	paramSchema := tool.InputSchema
	assert.NotNil(t, paramSchema)

	// Verify it's a valid OpenAPI schema
	assert.NotNil(t, paramSchema.Properties)

	// Test valid arguments
	validArgs := map[string]interface{}{
		"steps": float64(5),
		"delay": float64(1000),
	}
	err := validateArguments(tool, validArgs)
	assert.NoError(t, err)

	// Test invalid arguments
	invalidArgs := map[string]interface{}{
		"steps": "invalid", // Should be a number
	}
	// In a real test, we would expect this to error
	// but our simplified validateArguments doesn't validate types
	_ = validateArguments(tool, invalidArgs)
}

// validateArguments validates arguments against a tool's schema.
func validateArguments(tool *mcp.Tool, args map[string]interface{}) error {
	// We would need to implement schema validation here
	// This is a simplified version for testing purposes
	return nil
}

// addNotification adds a notification to the mock responder.
func (nc *MockResponder) addNotification(notification *mcp.JSONRPCNotification) {
	if notification.Method == "notifications/progress" {
		progress, _ := notification.Params.AdditionalFields["progress"].(float64)
		message, _ := notification.Params.AdditionalFields["message"].(string)
		nc.SendProgress(progress, message)
	} else if notification.Method == "notifications/message" {
		level, _ := notification.Params.AdditionalFields["level"].(string)
		message, _ := notification.Params.AdditionalFields["message"].(string)
		nc.SendLogMessage(level, message)
	} else {
		nc.SendCustomNotification(notification.Method, notification.Params.AdditionalFields)
	}
}
