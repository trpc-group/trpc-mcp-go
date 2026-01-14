// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestSSEServer_UnregisterTools(t *testing.T) {
	// Create an SSE server
	server := NewSSEServer("test-server", "1.0.0")

	// Create test tools
	tool1 := &Tool{
		Name:        "test-tool-1",
		Description: "Test tool 1",
	}
	tool2 := &Tool{
		Name:        "test-tool-2",
		Description: "Test tool 2",
	}
	tool3 := &Tool{
		Name:        "test-tool-3",
		Description: "Test tool 3",
	}

	// Register tools
	server.RegisterTool(tool1, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("result1"), nil
	})
	server.RegisterTool(tool2, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("result2"), nil
	})
	server.RegisterTool(tool3, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("result3"), nil
	})

	// Verify all tools are registered
	tools := server.toolManager.getTools()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}

	// Test unregistering multiple tools
	err := server.UnregisterTools("test-tool-1", "test-tool-2")
	if err != nil {
		t.Errorf("UnregisterTools failed: %v", err)
	}

	// Verify tools were unregistered
	tools = server.toolManager.getTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after unregistering 2, got %d", len(tools))
	}
	if tools[0].Name != "test-tool-3" {
		t.Errorf("Expected remaining tool to be test-tool-3, got %s", tools[0].Name)
	}

	// Test unregistering single tool
	err = server.UnregisterTools("test-tool-3")
	if err != nil {
		t.Errorf("UnregisterTools failed for single tool: %v", err)
	}

	// Verify all tools are unregistered
	tools = server.toolManager.getTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools after unregistering all, got %d", len(tools))
	}

	// Test error cases
	t.Run("No tool names provided", func(t *testing.T) {
		err := server.UnregisterTools()
		if err == nil {
			t.Error("Expected error when no tool names provided")
		}
		if err.Error() != "no tool names provided" {
			t.Errorf("Expected 'no tool names provided' error, got: %v", err)
		}
	})

	t.Run("None of the specified tools found", func(t *testing.T) {
		err := server.UnregisterTools("non-existent-tool")
		if err == nil {
			t.Error("Expected error when trying to unregister non-existent tool")
		}
		if err.Error() != "none of the specified tools were found" {
			t.Errorf("Expected 'none of the specified tools were found' error, got: %v", err)
		}
	})
}

func TestSSEServer_RegisterAndUnregisterTool(t *testing.T) {
	// Create an SSE server
	server := NewSSEServer("test-server", "1.0.0")

	// Create a test tool
	tool := &Tool{
		Name:        "dynamic-tool",
		Description: "A dynamically managed tool",
	}

	// Register the tool
	server.RegisterTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("dynamic result"), nil
	})

	// Verify tool is registered
	tools := server.toolManager.getTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after registration, got %d", len(tools))
	}
	if tools[0].Name != "dynamic-tool" {
		t.Errorf("Expected tool name to be dynamic-tool, got %s", tools[0].Name)
	}

	// Unregister the tool
	err := server.UnregisterTools("dynamic-tool")
	if err != nil {
		t.Errorf("UnregisterTools failed: %v", err)
	}

	// Verify tool is unregistered
	tools = server.toolManager.getTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools after unregistration, got %d", len(tools))
	}

	// Register the tool again to verify it can be re-registered
	server.RegisterTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return NewTextResult("dynamic result again"), nil
	})

	// Verify tool is registered again
	tools = server.toolManager.getTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after re-registration, got %d", len(tools))
	}
}

func TestSSEServer_PathMethods(t *testing.T) {
	t.Run("DefaultPaths", func(t *testing.T) {
		// Create SSE server with default configuration
		server := NewSSEServer("test-server", "1.0.0")

		// Test default paths
		if server.BasePath() != "" {
			t.Errorf("Expected default BasePath to be empty, got %q", server.BasePath())
		}
		if server.SSEEndpoint() != "/sse" {
			t.Errorf("Expected default SSEEndpoint to be '/sse', got %q", server.SSEEndpoint())
		}
		if server.MessageEndpoint() != "/message" {
			t.Errorf("Expected default MessageEndpoint to be '/message', got %q", server.MessageEndpoint())
		}
		if server.SSEPath() != "/sse" {
			t.Errorf("Expected default SSEPath to be '/sse', got %q", server.SSEPath())
		}
		if server.MessagePath() != "/message" {
			t.Errorf("Expected default MessagePath to be '/message', got %q", server.MessagePath())
		}
	})

	t.Run("CustomBasePath", func(t *testing.T) {
		// Create SSE server with custom base path
		server := NewSSEServer("test-server", "1.0.0", WithBasePath("/api/v1"))

		// Test custom paths
		if server.BasePath() != "/api/v1" {
			t.Errorf("Expected BasePath to be '/api/v1', got %q", server.BasePath())
		}
		if server.SSEEndpoint() != "/sse" {
			t.Errorf("Expected SSEEndpoint to be '/sse', got %q", server.SSEEndpoint())
		}
		if server.MessageEndpoint() != "/message" {
			t.Errorf("Expected MessageEndpoint to be '/message', got %q", server.MessageEndpoint())
		}
		if server.SSEPath() != "/api/v1/sse" {
			t.Errorf("Expected SSEPath to be '/api/v1/sse', got %q", server.SSEPath())
		}
		if server.MessagePath() != "/api/v1/message" {
			t.Errorf("Expected MessagePath to be '/api/v1/message', got %q", server.MessagePath())
		}
	})

	t.Run("CustomEndpoints", func(t *testing.T) {
		// Create SSE server with custom endpoints
		server := NewSSEServer("test-server", "1.0.0",
			WithBasePath("/mcp"),
			WithSSEEndpoint("/events"),
			WithMessageEndpoint("/msgs"))

		// Test custom endpoint paths
		if server.BasePath() != "/mcp" {
			t.Errorf("Expected BasePath to be '/mcp', got %q", server.BasePath())
		}
		if server.SSEEndpoint() != "/events" {
			t.Errorf("Expected SSEEndpoint to be '/events', got %q", server.SSEEndpoint())
		}
		if server.MessageEndpoint() != "/msgs" {
			t.Errorf("Expected MessageEndpoint to be '/msgs', got %q", server.MessageEndpoint())
		}
		if server.SSEPath() != "/mcp/events" {
			t.Errorf("Expected SSEPath to be '/mcp/events', got %q", server.SSEPath())
		}
		if server.MessagePath() != "/mcp/msgs" {
			t.Errorf("Expected MessagePath to be '/mcp/msgs', got %q", server.MessagePath())
		}
	})

	t.Run("EmptyBasePath", func(t *testing.T) {
		// Create SSE server with empty base path
		server := NewSSEServer("test-server", "1.0.0", WithBasePath(""))

		// Test empty base path
		if server.BasePath() != "" {
			t.Errorf("Expected BasePath to be empty, got %q", server.BasePath())
		}
		if server.SSEPath() != "/sse" {
			t.Errorf("Expected SSEPath to be '/sse', got %q", server.SSEPath())
		}
		if server.MessagePath() != "/message" {
			t.Errorf("Expected MessagePath to be '/message', got %q", server.MessagePath())
		}
	})

	t.Run("RootBasePath", func(t *testing.T) {
		// Create SSE server with root base path
		server := NewSSEServer("test-server", "1.0.0", WithBasePath("/"))

		// Test root base path - "/" might be normalized to empty string
		basePath := server.BasePath()
		if basePath != "/" && basePath != "" {
			t.Errorf("Expected BasePath to be '/' or empty, got %q", basePath)
		}

		// SSE and Message paths should still be correct
		if server.SSEPath() != "/sse" {
			t.Errorf("Expected SSEPath to be '/sse', got %q", server.SSEPath())
		}
		if server.MessagePath() != "/message" {
			t.Errorf("Expected MessagePath to be '/message', got %q", server.MessagePath())
		}
	})
}

// TestSessionIDGenerator tests the custom session ID generator functionality
func TestSessionIDGenerator(t *testing.T) {
	t.Run("defaultSessionIDGenerator", func(t *testing.T) {
		generator := &defaultSessionIDGenerator{}
		req, _ := http.NewRequest("GET", "http://localhost:8080/sse", nil)

		sessionID := generator.GenerateSessionID(req)

		// Should start with "sse-" prefix
		if !strings.HasPrefix(sessionID, "sse-") {
			t.Errorf("Expected session ID to start with 'sse-', got: %s", sessionID)
		}

		// Should be longer than just the prefix (UUID should be added)
		if len(sessionID) <= 4 {
			t.Errorf("Expected session ID to be longer than prefix, got: %s", sessionID)
		}

		// Multiple calls should generate different IDs
		sessionID2 := generator.GenerateSessionID(req)
		if sessionID == sessionID2 {
			t.Errorf("Expected different session IDs, got same: %s", sessionID)
		}
	})

	t.Run("CustomSessionIDGenerator", func(t *testing.T) {
		// Custom generator that includes client IP in session ID
		generator := customSessionIDGenerator(func(r *http.Request) string {
			return "custom-session-" + r.RemoteAddr
		})

		req, _ := http.NewRequest("GET", "http://localhost:8080/sse", nil)
		req.RemoteAddr = "192.168.1.100:54321"

		sessionID := generator.GenerateSessionID(req)
		expected := "custom-session-192.168.1.100:54321"

		if sessionID != expected {
			t.Errorf("Expected session ID '%s', got: %s", expected, sessionID)
		}
	})
}

// TestSSEServerWithCustomSessionIDGenerator tests SSE server with custom session ID generator
func TestSSEServerWithCustomSessionIDGenerator(t *testing.T) {
	// Create a custom session ID generator
	testGenerator := &testSessionIDGenerator{
		prefix: "test-session",
	}

	// Create SSE server with custom generator
	server := NewSSEServer(
		"test-server",
		"1.0.0",
		WithSSESessionIDGenerator(testGenerator),
	)

	// Verify the custom generator is set
	if server.sessionIDGenerator != testGenerator {
		t.Errorf("Expected custom session ID generator to be set")
	}

	// Test session ID generation
	req, _ := http.NewRequest("GET", "http://localhost:8080/sse", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	sessionID := server.sessionIDGenerator.GenerateSessionID(req)

	if !strings.HasPrefix(sessionID, "test-session") {
		t.Errorf("Expected session ID to start with 'test-session', got: %s", sessionID)
	}
}

// Helper types for testing
type customSessionIDGenerator func(r *http.Request) string

func (f customSessionIDGenerator) GenerateSessionID(r *http.Request) string {
	return f(r)
}

type testSessionIDGenerator struct {
	prefix string
}

func (g *testSessionIDGenerator) GenerateSessionID(r *http.Request) string {
	return g.prefix + "-" + r.RemoteAddr + "-generated"
}

// ============================================================================
// SessionPubSub Tests
// ============================================================================

// mockSessionPubSub is a mock implementation of SessionPubSub for testing.
type mockSessionPubSub struct {
	subscriptions map[string]SessionMessageHandler
	published     []mockPublishedMessage
	subscribeCalls int
	unsubscribeCalls int
	publishCalls int
	subscribeErr error
	unsubscribeErr error
	publishErr error
}

type mockPublishedMessage struct {
	sessionID string
	payload   []byte
}

func newMockSessionPubSub() *mockSessionPubSub {
	return &mockSessionPubSub{
		subscriptions: make(map[string]SessionMessageHandler),
		published:     make([]mockPublishedMessage, 0),
	}
}

func (m *mockSessionPubSub) Subscribe(ctx context.Context, sessionID string, handler SessionMessageHandler) error {
	m.subscribeCalls++
	if m.subscribeErr != nil {
		return m.subscribeErr
	}
	m.subscriptions[sessionID] = handler
	return nil
}

func (m *mockSessionPubSub) Unsubscribe(ctx context.Context, sessionID string) error {
	m.unsubscribeCalls++
	if m.unsubscribeErr != nil {
		return m.unsubscribeErr
	}
	delete(m.subscriptions, sessionID)
	return nil
}

func (m *mockSessionPubSub) Publish(ctx context.Context, sessionID string, payload []byte) error {
	m.publishCalls++
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, mockPublishedMessage{sessionID: sessionID, payload: payload})
	return nil
}

// deliverMessage simulates message delivery to a subscribed handler.
func (m *mockSessionPubSub) deliverMessage(ctx context.Context, sessionID string, payload []byte) error {
	handler, ok := m.subscriptions[sessionID]
	if !ok {
		return nil
	}
	return handler(ctx, sessionID, payload)
}

// TestWithSessionPubSub tests the WithSessionPubSub option.
func TestWithSessionPubSub(t *testing.T) {
	mockPubSub := newMockSessionPubSub()

	server := NewSSEServer(
		"test-server",
		"1.0.0",
		WithSessionPubSub(mockPubSub),
	)

	if server.sessionPubSub != mockPubSub {
		t.Error("Expected sessionPubSub to be set")
	}
}

// TestTrySubscribeAndUnsubscribe tests the trySubscribe and tryUnsubscribe methods.
func TestTrySubscribeAndUnsubscribe(t *testing.T) {
	t.Run("Subscribe and Unsubscribe success", func(t *testing.T) {
		mockPubSub := newMockSessionPubSub()
		server := NewSSEServer("test-server", "1.0.0", WithSessionPubSub(mockPubSub))

		ctx := context.Background()
		sessionID := "test-session-123"

		// Test subscribe
		server.trySubscribe(ctx, sessionID)
		if mockPubSub.subscribeCalls != 1 {
			t.Errorf("Expected 1 subscribe call, got %d", mockPubSub.subscribeCalls)
		}
		if _, ok := mockPubSub.subscriptions[sessionID]; !ok {
			t.Error("Expected session to be subscribed")
		}

		// Test unsubscribe
		server.tryUnsubscribe(ctx, sessionID)
		if mockPubSub.unsubscribeCalls != 1 {
			t.Errorf("Expected 1 unsubscribe call, got %d", mockPubSub.unsubscribeCalls)
		}
		if _, ok := mockPubSub.subscriptions[sessionID]; ok {
			t.Error("Expected session to be unsubscribed")
		}
	})

	t.Run("Subscribe without SessionPubSub configured", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		ctx := context.Background()
		// Should not panic when sessionPubSub is nil
		server.trySubscribe(ctx, "test-session")
		server.tryUnsubscribe(ctx, "test-session")
	})

	t.Run("Subscribe with error", func(t *testing.T) {
		mockPubSub := newMockSessionPubSub()
		mockPubSub.subscribeErr = context.DeadlineExceeded
		server := NewSSEServer("test-server", "1.0.0", WithSessionPubSub(mockPubSub))

		ctx := context.Background()
		// Should not panic, just log error
		server.trySubscribe(ctx, "test-session")
		if mockPubSub.subscribeCalls != 1 {
			t.Errorf("Expected 1 subscribe call, got %d", mockPubSub.subscribeCalls)
		}
	})
}

// TestTryPublish tests the tryPublish method.
func TestTryPublish(t *testing.T) {
	t.Run("Publish success", func(t *testing.T) {
		mockPubSub := newMockSessionPubSub()
		server := NewSSEServer("test-server", "1.0.0", WithSessionPubSub(mockPubSub))

		ctx := context.Background()
		sessionID := "test-session-456"

		body := `{"jsonrpc":"2.0","id":1,"method":"test"}`
		req, _ := http.NewRequest("POST", "http://localhost/message?sessionId="+sessionID, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.1:12345"

		err := server.tryPublish(ctx, sessionID, req)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if mockPubSub.publishCalls != 1 {
			t.Errorf("Expected 1 publish call, got %d", mockPubSub.publishCalls)
		}

		if len(mockPubSub.published) != 1 {
			t.Fatalf("Expected 1 published message, got %d", len(mockPubSub.published))
		}

		if mockPubSub.published[0].sessionID != sessionID {
			t.Errorf("Expected sessionID %s, got %s", sessionID, mockPubSub.published[0].sessionID)
		}
	})

	t.Run("Publish without SessionPubSub configured", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		ctx := context.Background()
		req, _ := http.NewRequest("POST", "http://localhost/message", strings.NewReader("{}"))

		err := server.tryPublish(ctx, "test-session", req)
		if err == nil {
			t.Error("Expected error when sessionPubSub is nil")
		}
		if !strings.Contains(err.Error(), "session pubsub not configured") {
			t.Errorf("Expected 'session pubsub not configured' error, got: %v", err)
		}
	})

	t.Run("Publish with error", func(t *testing.T) {
		mockPubSub := newMockSessionPubSub()
		mockPubSub.publishErr = context.DeadlineExceeded
		server := NewSSEServer("test-server", "1.0.0", WithSessionPubSub(mockPubSub))

		ctx := context.Background()
		req, _ := http.NewRequest("POST", "http://localhost/message", strings.NewReader("{}"))

		err := server.tryPublish(ctx, "test-session", req)
		if err == nil {
			t.Error("Expected error when publish fails")
		}
	})
}

// TestHandleSessionMessage tests the handleSessionMessage method.
func TestHandleSessionMessage(t *testing.T) {
	t.Run("Session not found", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		ctx := context.Background()
		err := server.handleSessionMessage(ctx, "non-existent-session", []byte("{}"))

		if err == nil {
			t.Error("Expected error for non-existent session")
		}
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("Expected ErrSessionNotFound, got: %v", err)
		}
	})

	t.Run("Handle serializedRequest", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		// Create a session manually
		sessionID := "test-session-789"
		session := &sseSession{
			sessionID:           sessionID,
			eventQueue:          make(chan string, 100),
			notificationChannel: make(chan *JSONRPCNotification, 100),
			done:                make(chan struct{}),
		}
		server.sessions.Store(sessionID, session)

		ctx := context.Background()

		// Create a serializedRequest payload
		payload := `{
			"method": "POST",
			"url": "http://localhost/message?sessionId=test-session-789",
			"headers": {"Content-Type": ["application/json"]},
			"body": "eyJqc29ucnBjIjoiMi4wIiwibWV0aG9kIjoibm90aWZpY2F0aW9ucy9pbml0aWFsaXplZCJ9",
			"remote_addr": "192.168.1.1:12345"
		}`

		// Should not return error (notification doesn't require response)
		err := server.handleSessionMessage(ctx, sessionID, []byte(payload))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Handle raw JSON-RPC message", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		// Create a session manually
		sessionID := "test-session-raw"
		session := &sseSession{
			sessionID:           sessionID,
			eventQueue:          make(chan string, 100),
			notificationChannel: make(chan *JSONRPCNotification, 100),
			done:                make(chan struct{}),
		}
		server.sessions.Store(sessionID, session)

		ctx := context.Background()

		// Raw JSON-RPC notification (no ID, has method)
		payload := `{"jsonrpc":"2.0","method":"notifications/initialized"}`

		err := server.handleSessionMessage(ctx, sessionID, []byte(payload))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Invalid JSON payload", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		// Create a session manually
		sessionID := "test-session-invalid"
		session := &sseSession{
			sessionID:           sessionID,
			eventQueue:          make(chan string, 100),
			notificationChannel: make(chan *JSONRPCNotification, 100),
			done:                make(chan struct{}),
		}
		server.sessions.Store(sessionID, session)

		ctx := context.Background()

		// Invalid JSON
		err := server.handleSessionMessage(ctx, sessionID, []byte("not valid json"))
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
		if !strings.Contains(err.Error(), "parse error") {
			t.Errorf("Expected 'parse error', got: %v", err)
		}
	})
}

// TestHandleRawRemoteMessage tests the handleRawRemoteMessage method.
func TestHandleRawRemoteMessage(t *testing.T) {
	t.Run("Invalid JSON-RPC message format", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		session := &sseSession{
			sessionID:           "test-session",
			eventQueue:          make(chan string, 100),
			notificationChannel: make(chan *JSONRPCNotification, 100),
			done:                make(chan struct{}),
		}

		ctx := context.Background()

		// Message with neither ID nor Method
		payload := `{"jsonrpc":"2.0"}`

		err := server.handleRawRemoteMessage(ctx, session, []byte(payload))
		if err == nil {
			t.Error("Expected error for invalid message format")
		}
		if !strings.Contains(err.Error(), "invalid JSON-RPC message format") {
			t.Errorf("Expected 'invalid JSON-RPC message format' error, got: %v", err)
		}
	})

	t.Run("Handle request message", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		session := &sseSession{
			sessionID:           "test-session",
			eventQueue:          make(chan string, 100),
			notificationChannel: make(chan *JSONRPCNotification, 100),
			done:                make(chan struct{}),
		}
		server.sessions.Store(session.sessionID, session)

		ctx := context.Background()

		// Request message (has both ID and Method)
		payload := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`

		err := server.handleRawRemoteMessage(ctx, session, []byte(payload))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Handle notification message", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		session := &sseSession{
			sessionID:           "test-session",
			eventQueue:          make(chan string, 100),
			notificationChannel: make(chan *JSONRPCNotification, 100),
			done:                make(chan struct{}),
		}

		ctx := context.Background()

		// Notification message (has Method but no ID)
		payload := `{"jsonrpc":"2.0","method":"notifications/initialized"}`

		err := server.handleRawRemoteMessage(ctx, session, []byte(payload))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Handle response message", func(t *testing.T) {
		server := NewSSEServer("test-server", "1.0.0")

		session := &sseSession{
			sessionID:           "test-session",
			eventQueue:          make(chan string, 100),
			notificationChannel: make(chan *JSONRPCNotification, 100),
			done:                make(chan struct{}),
		}

		ctx := context.Background()

		// Response message (has ID but no Method)
		payload := `{"jsonrpc":"2.0","id":1,"result":{}}`

		err := server.handleRawRemoteMessage(ctx, session, []byte(payload))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}

// TestSerializedRequestWithDeadline tests deadline handling in handleSerializedRequest.
func TestSerializedRequestWithDeadline(t *testing.T) {
	server := NewSSEServer("test-server", "1.0.0")

	session := &sseSession{
		sessionID:           "test-session",
		eventQueue:          make(chan string, 100),
		notificationChannel: make(chan *JSONRPCNotification, 100),
		done:                make(chan struct{}),
	}
	server.sessions.Store(session.sessionID, session)

	ctx := context.Background()

	// Create a serializedRequest with deadline
	deadline := "2099-12-31T23:59:59Z"
	payload := `{
		"method": "POST",
		"url": "http://localhost/message",
		"headers": {},
		"body": "eyJqc29ucnBjIjoiMi4wIiwibWV0aG9kIjoibm90aWZpY2F0aW9ucy9pbml0aWFsaXplZCJ9",
		"deadline": "` + deadline + `"
	}`

	err := server.handleSessionMessage(ctx, session.sessionID, []byte(payload))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestSerializedRequestWithContextFunc tests contextFunc handling in handleSerializedRequest.
func TestSerializedRequestWithContextFunc(t *testing.T) {
	contextFuncCalled := false
	var capturedRemoteAddr string

	server := NewSSEServer(
		"test-server",
		"1.0.0",
		WithSSEContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			contextFuncCalled = true
			capturedRemoteAddr = r.RemoteAddr
			return ctx
		}),
	)

	session := &sseSession{
		sessionID:           "test-session",
		eventQueue:          make(chan string, 100),
		notificationChannel: make(chan *JSONRPCNotification, 100),
		done:                make(chan struct{}),
	}
	server.sessions.Store(session.sessionID, session)

	ctx := context.Background()

	// Create a serializedRequest with remote_addr
	payload := `{
		"method": "POST",
		"url": "http://localhost/message",
		"headers": {"X-Custom-Header": ["test-value"]},
		"body": "eyJqc29ucnBjIjoiMi4wIiwibWV0aG9kIjoibm90aWZpY2F0aW9ucy9pbml0aWFsaXplZCJ9",
		"remote_addr": "10.0.0.1:54321"
	}`

	err := server.handleSessionMessage(ctx, session.sessionID, []byte(payload))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !contextFuncCalled {
		t.Error("Expected contextFunc to be called")
	}

	if capturedRemoteAddr != "10.0.0.1:54321" {
		t.Errorf("Expected remote addr '10.0.0.1:54321', got '%s'", capturedRemoteAddr)
	}
}
