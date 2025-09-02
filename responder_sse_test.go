// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSEResponder_Respond(t *testing.T) {
	// Create response recorder
	w := httptest.NewRecorder()

	// Create SSE responder
	responder := newSSEResponder()

	// Create test response
	testResponse := newJSONRPCResponse("test-id", map[string]interface{}{
		"result": "test-result",
	})

	// Send response
	err := responder.respond(context.Background(), w, nil, testResponse, nil)

	// Verify no error
	assert.NoError(t, err)

	// Verify response content
	responseBody := w.Body.String()
	assert.Contains(t, responseBody, "id: ")
	assert.Contains(t, responseBody, "data: ")
	assert.Contains(t, responseBody, "test-id")
	assert.Contains(t, responseBody, "test-result")
}

func TestSSEResponder_SendNotification(t *testing.T) {
	// Create response recorder
	w := httptest.NewRecorder()

	// Create SSE responder
	responder := newSSEResponder()

	// Create test notification
	testNotification := NewJSONRPCNotificationFromMap("test-method", map[string]interface{}{
		"param1": "value1",
	})

	// Send notification
	eventID, err := responder.sendNotification(w, testNotification)

	// Verify no error
	assert.NoError(t, err)
	assert.NotEmpty(t, eventID)

	// Verify notification content
	notificationBody := w.Body.String()
	assert.Contains(t, notificationBody, "id: "+eventID)
	assert.Contains(t, notificationBody, "data: ")
	assert.Contains(t, notificationBody, "test-method")
	assert.Contains(t, notificationBody, "value1")
}

func TestSSEResponder_WithResponse(t *testing.T) {
	// Create response recorder
	w := httptest.NewRecorder()

	// Create SSE responder
	responder := newSSEResponder()

	// Create test response to pass to sendNotification
	testResponse := newJSONRPCResponse("test-id", map[string]interface{}{
		"result": "test-result",
	})

	// Send response as notification (should return error)
	eventID, err := responder.sendNotification(w, testResponse)

	// Verify error occurred
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid response type")
	assert.Empty(t, eventID)
}

func TestSSEResponder_NextEventID(t *testing.T) {
	responder := newSSEResponder()

	// Generate multiple event IDs, ensure they are unique
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		id := responder.nextEventID()
		assert.False(t, ids[id], "Event IDs should be unique")
		ids[id] = true
		assert.True(t, strings.HasPrefix(id, "evt-"), "Event ID should have the correct prefix")
	}
}

// Comprehensive test of SSE responder's response and notification handling
func TestSSEResponder_ComprehensiveTest(t *testing.T) {
	// Create response recorder
	w := httptest.NewRecorder()

	// Create SSE responder
	responder := newSSEResponder()

	// Test sending response
	testResponse := newJSONRPCResponse("resp-id", map[string]interface{}{
		"result": "success",
	})
	err := responder.respond(context.Background(), w, nil, testResponse, nil)
	assert.NoError(t, err)

	// Reset recorder
	responseBody := w.Body.String()
	w = httptest.NewRecorder()

	// Test sending notification
	testNotification := NewJSONRPCNotificationFromMap("progress", map[string]interface{}{
		"percent": 50,
		"message": "halfway done",
	})
	notificationEventID, err := responder.sendNotification(w, testNotification)
	assert.NoError(t, err)
	assert.NotEmpty(t, notificationEventID)

	// Verify response and notification contain correct content
	assert.Contains(t, responseBody, "\"result\":\"success\"")

	notificationBody := w.Body.String()
	assert.Contains(t, notificationBody, "id: "+notificationEventID)
	assert.Contains(t, notificationBody, "\"method\":\"progress\"")
}
