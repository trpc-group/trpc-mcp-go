// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package scenarios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/e2e"
)

// TestGetSSEBasic tests the basic GET SSE connection and notification functionality.
func TestGetSSEBasic(t *testing.T) {
	// Only run if the server explicitly supports GET SSE.
	// Note: This is an optional check. If your server always supports GET SSE, you can remove this check.
	t.Logf("Running GET SSE test, server should support GET SSE connection.")

	// Create server that supports GET SSE.
	serverURL, cleanup := e2e.StartSSETestServer(t,
		e2e.WithTestTools(),
		func(s *mcp.Server) {
			// Explicitly enable GET SSE.
			mcp.WithGetSSEEnabled(true)(s)
		},
	)
	defer cleanup()

	// Create client that supports GET SSE.
	client := e2e.CreateSSETestClient(t, serverURL, e2e.WithGetSSEEnabled())
	defer e2e.CleanupClient(t, client)

	// Initialize client.
	e2e.InitializeClient(t, client)

	// Create notification collector.
	collector := e2e.NewNotificationCollector()

	// Register custom notification handler to receive GET SSE notifications.
	client.RegisterNotificationHandler("notifications/message", collector.HandleCustomNotification)

	// Wait a short time to allow GET SSE connection to establish.
	time.Sleep(1 * time.Second)

	// Get test server and session ID.
	sessionID := client.GetSessionID()
	require.NotEmpty(t, sessionID, "Session ID should not be empty")

	// Send system status notification to session.
	statusNotification := &mcp.JSONRPCNotification{}
	statusNotification.Method = "notifications/message"
	statusNotification.Params.AdditionalFields = map[string]interface{}{
		"level": "info",
		"data": map[string]interface{}{
			"cpu":        75.5,
			"memory":     60.2,
			"disk":       45.8,
			"network":    120.5,
			"conditions": "normal",
			"timestamp":  time.Now().Unix(),
		},
	}

	// Send notification directly to server using HTTP request.
	t.Logf("Sending notification directly to server, sessionID: %s", sessionID)

	// Create notification request.
	notificationBytes, err := json.Marshal(statusNotification)
	require.NoError(t, err, "Failed to serialize notification")

	// Use existing test API endpoint.
	reqURL := fmt.Sprintf("%s/test/notify?sessionId=%s", serverURL, sessionID)
	resp, err := http.Post(reqURL, "application/json", bytes.NewReader(notificationBytes))
	require.NoError(t, err, "Failed to send notification request")
	defer resp.Body.Close()

	require.Equal(t, http.StatusAccepted, resp.StatusCode, "Server should return 202 status code")

	// Wait for notification to be received.
	time.Sleep(1 * time.Second)

	// Verify notification is received correctly.
	notifications := collector.GetNotifications("notifications/message")
	require.NotEmpty(t, notifications, "Should receive system status notification")
	assert.Equal(t, "notifications/message", notifications[0].Method, "Notification method should match")

	// Verify notification content.
	params := notifications[0].Params.AdditionalFields
	assert.Equal(t, "info", params["level"], "Notification level should be info")
	data, ok := params["data"].(map[string]interface{})
	require.True(t, ok, "data field should be an object")
	assert.NotNil(t, data["cpu"], "Notification should contain CPU data")
	assert.NotNil(t, data["memory"], "Notification should contain memory data")
	assert.NotNil(t, data["timestamp"], "Notification should contain timestamp")
}
