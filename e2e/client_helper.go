// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// ClientOption defines a client option function.
type ClientOption func(*mcp.Client)

// WithProtocolVersion option: set protocol version.
func WithProtocolVersion(version string) ClientOption {
	return func(c *mcp.Client) {
		// Already applied at client creation, this is just a placeholder.
	}
}

// WithGetSSEEnabled option: enable GET SSE connection.
func WithGetSSEEnabled() ClientOption {
	return func(c *mcp.Client) {
		// Use WithGetSSEEnabled from the client package.
		mcp.WithClientGetSSEEnabled(true)(c)
	}
}

// WithLastEventID option: set Last-Event-ID for stream recovery.
// Note: This only saves eventID in test helpers, actual usage requires passing via streamOptions.
func WithLastEventID(eventID string) ClientOption {
	return func(c *mcp.Client) {
		// No need to set here, will be used in ExecuteSSETestTool and similar methods.
	}
}

// CreateTestClient creates a test client connected to the given URL.
func CreateTestClient(t *testing.T, url string, opts ...ClientOption) *mcp.Client {
	t.Helper()

	// Create client.
	c, err := mcp.NewClient(url, mcp.Implementation{
		Name:    "E2E-Test-Client",
		Version: "1.0.0",
	})
	require.NoError(t, err, "failed to create client")
	require.NotNil(t, c, "client should not be nil")

	// Apply options.
	for _, opt := range opts {
		opt(c)
	}

	t.Logf("Created test client URL: %s", url)

	return c
}

// InitializeClient initializes the client and verifies success.
func InitializeClient(t *testing.T, c *mcp.Client) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize client.
	resp, err := c.Initialize(ctx, &mcp.InitializeRequest{})
	require.NoError(t, err, "failed to initialize client")
	require.NotNil(t, resp, "init response should not be nil")

	// Verify key fields.
	assert.NotEmpty(t, resp.ServerInfo.Name, "server name should not be empty")
	assert.NotEmpty(t, resp.ServerInfo.Version, "server version should not be empty")
	assert.NotEmpty(t, resp.ProtocolVersion, "protocol version should not be empty")

	// Verify session ID.
	sessionID := c.GetSessionID()
	assert.NotEmpty(t, sessionID, "session ID should not be empty")

	t.Logf("Client initialized successfully, server: %s %s, sessionID: %s",
		resp.ServerInfo.Name, resp.ServerInfo.Version, sessionID)
}

// ExecuteTestTool executes a test tool and verifies the result.
func ExecuteTestTool(t *testing.T, c *mcp.Client, toolName string, args map[string]interface{}) []mcp.Content {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call tool.
	callToolReq := &mcp.CallToolRequest{}
	callToolReq.Params.Name = toolName
	callToolReq.Params.Arguments = args
	result, err := c.CallTool(ctx, callToolReq)
	require.NoError(t, err, "failed to call tool %s", toolName)
	require.NotNil(t, result, "tool call result should not be nil")

	t.Logf("Tool %s called successfully, result content count: %d", toolName, len(result.Content))

	return result.Content
}

// CleanupClient cleans up client resources.
func CleanupClient(t *testing.T, c *mcp.Client) {
	t.Helper()

	if c == nil {
		return
	}

	// Try to terminate session.
	sessionID := c.GetSessionID()
	if sessionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := c.TerminateSession(ctx)
		if err != nil {
			t.Logf("failed to terminate session %s: %v", sessionID, err)
		} else {
			t.Logf("session %s terminated", sessionID)
		}
	}

	// close client.
	c.Close()
	t.Log("client resources cleaned up")
}

// CreateSSETestClient creates a test client configured to use SSE.
func CreateSSETestClient(t *testing.T, url string, opts ...ClientOption) *mcp.Client {
	t.Helper()

	// Create client.
	c, err := mcp.NewClient(url, mcp.Implementation{
		Name:    "E2E-SSE-Test-Client",
		Version: "1.0.0",
	})
	require.NoError(t, err, "failed to create SSE client")
	require.NotNil(t, c, "SSE client should not be nil")

	// Apply options.
	for _, opt := range opts {
		opt(c)
	}

	t.Logf("Created SSE test client URL: %s", url)

	return c
}

// ExecuteSSETestTool executes a test tool and supports collecting notifications.
func ExecuteSSETestTool(t *testing.T, c *mcp.Client, toolName string, args map[string]interface{}, collector *NotificationCollector) []mcp.Content {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Register notification handler.
	c.RegisterNotificationHandler(toolName, collector.GetHandlers()[toolName])

	// Call tool with streaming method.
	callToolReq := &mcp.CallToolRequest{}
	callToolReq.Params.Name = toolName
	callToolReq.Params.Arguments = args
	result, err := c.CallTool(ctx, callToolReq)
	require.NoError(t, err, "failed to call tool %s", toolName)
	require.NotNil(t, result, "tool call stream result should not be nil")

	t.Logf("Tool %s called successfully, result content count: %d, notification count: %d",
		toolName, len(result.Content), collector.Count())

	return result.Content
}

// NotificationCollector is used for collecting and validating notifications.
type NotificationCollector struct {
	// Channel for collecting notifications.
	notifications chan *mcp.JSONRPCNotification
	// Mutex to protect counter.
	mu sync.Mutex
	// Notification counter.
	count int
	// Notification map by method.
	notificationsByMethod map[string][]*mcp.JSONRPCNotification
}

// NewNotificationCollector creates a new notification collector.
func NewNotificationCollector() *NotificationCollector {
	return &NotificationCollector{
		notifications:         make(chan *mcp.JSONRPCNotification, 50),
		notificationsByMethod: make(map[string][]*mcp.JSONRPCNotification),
	}
}

// GetHandlers returns the notification handler map.
func (nc *NotificationCollector) GetHandlers() map[string]mcp.NotificationHandler {
	// Create handler map.
	handlers := make(map[string]mcp.NotificationHandler)

	// Progress notification handler
	handlers["notifications/progress"] = func(n *mcp.JSONRPCNotification) error {
		nc.addNotification(n)
		return nil
	}

	// Log notification handler
	handlers["notifications/message"] = func(n *mcp.JSONRPCNotification) error {
		nc.addNotification(n)
		return nil
	}

	return handlers
}

// addNotification
func (nc *NotificationCollector) addNotification(n *mcp.JSONRPCNotification) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	// increase
	nc.count++

	select {
	case nc.notifications <- n:
		// Notification sent to channel
	default:
		// Channel is full, skip
	}

	// Group by method
	nc.notificationsByMethod[n.Method] = append(nc.notificationsByMethod[n.Method], n)
}

// Count returns the total number of received notifications.
func (nc *NotificationCollector) Count() int {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.count
}

// GetNotifications returns the notification list for the specified method.
func (nc *NotificationCollector) GetNotifications(method string) []*mcp.JSONRPCNotification {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.notificationsByMethod[method]
}

// GetProgressNotifications returns the progress notification list.
func (nc *NotificationCollector) GetProgressNotifications() []*mcp.JSONRPCNotification {
	return nc.GetNotifications("notifications/progress")
}

// GetLogNotifications returns the log notification list.
func (nc *NotificationCollector) GetLogNotifications() []*mcp.JSONRPCNotification {
	return nc.GetNotifications("notifications/message")
}

// AssertNotificationCount asserts the notification count for the specified method.
func (nc *NotificationCollector) AssertNotificationCount(t *testing.T, method string, expectedCount int) {
	notifications := nc.GetNotifications(method)
	assert.Equal(t, expectedCount, len(notifications),
		"Method %s notification count should be %d, actual %d", method, expectedCount, len(notifications))
}

// CreateClient creates a client connection.
func CreateClient(url string, enableGetSSE bool) (*mcp.Client, error) {
	// Create client
	c, err := mcp.NewClient(url, mcp.Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	}, mcp.WithClientGetSSEEnabled(enableGetSSE))
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %v", err)
	}

	return c, nil
}

// CreateClientWithRequestMode creates a client connection with request mode.
func CreateClientWithRequestMode(url string, mode string, enableGetSSE bool) (*mcp.Client, error) {
	c, err := mcp.NewClient(url, mcp.Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	}, mcp.WithClientGetSSEEnabled(enableGetSSE))
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %v", err)
	}

	return c, nil
}
