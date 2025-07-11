// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// ServerOption defines a server option function.
type ServerOption func(*mcp.Server)

// WithTestTools option: register test tools.
func WithTestTools() ServerOption {
	return func(s *mcp.Server) {
		// Register test tools.
		RegisterTestTools(s)
	}
}

// WithCustomPathPrefix option: set custom path prefix.
func WithCustomPathPrefix(prefix string) ServerOption {
	return func(s *mcp.Server) {
		// Already applied at instantiation, this is just a placeholder.
	}
}

// WithServerOptions option: set multiple server options.
func WithServerOptions(opts ...mcp.ServerOption) ServerOption {
	return func(s *mcp.Server) {
		for _, opt := range opts {
			opt(s)
		}
	}
}

// StartTestServer starts a test server and returns its URL and cleanup function.
func StartTestServer(t *testing.T, opts ...ServerOption) (string, func()) {
	t.Helper()

	// Create basic server config.
	pathPrefix := "/mcp"

	// Instantiate server.
	s := mcp.NewServer(
		"E2E-Test-Server",              // Server name
		"1.0.0",                        // Server version
		mcp.WithServerPath(pathPrefix), // Set API path
	)

	// Apply options.
	for _, opt := range opts {
		opt(s)
	}

	// Create HTTP test server.
	httpServer := httptest.NewServer(s.HTTPHandler())
	serverURL := httpServer.URL + pathPrefix

	t.Logf("Started test server URL: %s", serverURL)

	// Return cleanup function.
	cleanup := func() {
		t.Log("Closing test server.")
		httpServer.Close()
	}

	return serverURL, cleanup
}

// StartSSETestServer starts a test server with SSE enabled, returns its URL and cleanup function.
func StartSSETestServer(t *testing.T, opts ...ServerOption) (string, func()) {
	t.Helper()

	// Create basic server config.
	pathPrefix := "/mcp"

	// Instantiate server and enable SSE.
	s := mcp.NewServer(
		"E2E-SSE-Test-Server", // Server name
		"1.0.0",               // Server version
		mcp.WithServerPath(pathPrefix),
		mcp.WithPostSSEEnabled(true), // Enable SSE.
		mcp.WithGetSSEEnabled(true),  // Enable GET SSE.
	)

	// Apply options.
	for _, opt := range opts {
		opt(s)
	}

	// Create custom mux for test routes.
	mux := http.NewServeMux()

	// Register MCP path.
	mux.Handle(pathPrefix, s.HTTPHandler())

	// Register test notification path.
	mux.HandleFunc(pathPrefix+"/test/notify", func(w http.ResponseWriter, r *http.Request) {
		handleTestNotify(w, r, s)
	})

	// Create HTTP test server.
	httpServer := httptest.NewServer(mux)
	serverURL := httpServer.URL + pathPrefix

	t.Logf("Started SSE test server URL: %s", serverURL)

	// Return cleanup function.
	cleanup := func() {
		t.Log("Closing SSE test server.")
		httpServer.Close()
	}

	return serverURL, cleanup
}

// handleTestNotify handles the test endpoint for sending notifications.
func handleTestNotify(w http.ResponseWriter, r *http.Request, server *mcp.Server) {
	// Get target session ID.
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Missing sessionId parameter.", http.StatusBadRequest)
		return
	}

	// Parse notification content.
	var notification mcp.JSONRPCNotification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		http.Error(w, fmt.Sprintf("Invalid notification format: %v", err), http.StatusBadRequest)
		return
	}

	// Extract params as a map for the sendNotification method
	params := make(map[string]interface{})

	// Convert the entire notification to JSON and then extract relevant fields
	data, err := json.Marshal(notification)
	if err == nil {
		var fullMap map[string]interface{}
		if err := json.Unmarshal(data, &fullMap); err == nil {
			if paramsMap, ok := fullMap["params"].(map[string]interface{}); ok {
				// Exclude _meta field from params as Server.sendNotification handles it separately
				for k, v := range paramsMap {
					if k != "_meta" {
						params[k] = v
					}
				}
			}
		}
	}

	// Use server's API to send notification
	if err := server.SendNotification(sessionID, notification.Method, params); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send notification: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusAccepted)
}

// StartLocalTestServer starts a server on a real port.
// Suitable for tests that require real network communication.
func StartLocalTestServer(t *testing.T, addr string, opts ...ServerOption) (*mcp.Server, func()) {
	t.Helper()

	// Create server.
	s := mcp.NewServer(
		"E2E-Test-Server", // Server name
		"1.0.0",           // Server version
		mcp.WithServerAddress(addr),
		mcp.WithServerPath("/mcp"),
	)

	// Apply options.
	for _, opt := range opts {
		opt(s)
	}

	// Start server.
	go func() {
		if err := s.Start(); err != nil {
			log.Printf("Server closed: %v", err)
		}
	}()

	t.Logf("Started local test server at: %s", addr)

	// Return cleanup function.
	cleanup := func() {
		t.Log("Closing local test server.")
		// Server does not provide a close method, but Start returns when server is closed.
		// No need to close explicitly here, as httptest.Server's close will handle it.
	}

	return s, cleanup
}

// ServerNotifier is a server notification sender for testing.
type ServerNotifier interface {
	// sendNotification sends a notification.
	sendNotification(sessionID string, notification *mcp.JSONRPCNotification) error
}

// GetServerNotifier gets a test server's notification sender.
func GetServerNotifier(t *testing.T, serverURL string) ServerNotifier {
	t.Helper()

	// Parse URL and create a new client pointing to the same server.
	// This client is only used to call the notification API and does not establish a real connection.
	return NewTestServerNotifier(t, serverURL)
}

// TestServerNotifier implements the ServerNotifier interface for testing.
type TestServerNotifier struct {
	t         *testing.T
	serverURL string
	client    *http.Client
}

// NewTestServerNotifier creates a new test server notification sender.
func NewTestServerNotifier(t *testing.T, serverURL string) *TestServerNotifier {
	return &TestServerNotifier{
		t:         t,
		serverURL: serverURL,
		client:    &http.Client{},
	}
}

// sendNotification sends a notification.
func (n *TestServerNotifier) sendNotification(sessionID string, notification *mcp.JSONRPCNotification) error {
	n.t.Helper()

	// Create POST request to send notification to server.
	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %v", err)
	}

	// Build request URL.
	reqURL := fmt.Sprintf("%s/test/notify?sessionId=%s", n.serverURL, sessionID)

	// Create request.
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(notificationBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set request header.
	req.Header.Set("Content-Type", "application/json")

	// Send request.
	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code.
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned non-200 status code: %d, response body: %s", resp.StatusCode, string(body))
	}

	return nil
}
