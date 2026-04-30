// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTool is a test tool for testing
type TestTool struct {
	*Tool
}

// handleTestTool handles the test tool
func handleTestTool(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
	name := "World"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	return NewTextResult("Hello, " + name + "!"), nil
}

// NewTestTool creates a new test tool
func NewTestTool() *Tool {
	return NewTool("test-tool",
		WithDescription("Test Tool"),
		WithString("name",
			Description("Name to greet"),
		),
	)
}

// Create test environment including server and client
func setupTestEnvironment(t *testing.T) (*Client, *httptest.Server, func()) {
	// Create MCP server
	mcpServer := NewServer(
		"Test-Server",          // Server name
		"1.0.0",                // Server version
		WithServerPath("/mcp"), // Set API path
	)

	// Register test tool
	tool := NewTestTool()
	mcpServer.RegisterTool(tool, handleTestTool)

	// Create HTTP test server
	httpServer := httptest.NewServer(mcpServer.HTTPHandler())

	// Create client
	client, err := NewClient(httpServer.URL+"/mcp", Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	})
	require.NoError(t, err)

	// Return cleanup function
	cleanup := func() {
		client.Close()
		httpServer.Close()
	}

	return client, httpServer, cleanup
}

func TestNewClient(t *testing.T) {
	// Test client creation
	client, err := NewClient("http://localhost:3000/mcp", Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	})

	// Verify successful object creation
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "Test-Client", client.clientInfo.Name)
	assert.Equal(t, "1.0.0", client.clientInfo.Version)
	assert.Equal(t, ProtocolVersion_2025_03_26, client.protocolVersion) // Update to current default version.
	assert.False(t, client.initialized)
}

func TestClient_WithProtocolVersion(t *testing.T) {
	// Test creating client with custom protocol version
	client, err := NewClient("http://localhost:3000/mcp", Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	}, WithProtocolVersion(ProtocolVersion_2024_11_05))

	// Verify protocol version
	assert.NoError(t, err)
	assert.Equal(t, ProtocolVersion_2024_11_05, client.protocolVersion)
}

func TestClient_Initialize(t *testing.T) {
	// Create test environment
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test initialization
	ctx := context.Background()
	resp, err := client.Initialize(ctx, &InitializeRequest{})

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Test-Server", resp.ServerInfo.Name)
	assert.Equal(t, "1.0.0", resp.ServerInfo.Version)
	assert.Equal(t, ProtocolVersion_2025_03_26, resp.ProtocolVersion)
	assert.NotNil(t, resp.Capabilities)

	// Verify client state
	assert.True(t, client.initialized)
	assert.NotEmpty(t, client.GetSessionID())
}

func TestClient_ListTools(t *testing.T) {
	// Create test environment
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Initialize client
	ctx := context.Background()
	_, err := client.Initialize(ctx, &InitializeRequest{})
	require.NoError(t, err)

	// Test listing tools
	toolsResult, err := client.ListTools(ctx, &ListToolsRequest{})

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, toolsResult.Tools, 1)
	assert.Equal(t, "test-tool", toolsResult.Tools[0].Name)
	assert.Equal(t, "Test Tool", toolsResult.Tools[0].Description)
}

func TestClient_CallTool(t *testing.T) {
	// Create test environment
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Initialize client
	ctx := context.Background()
	_, err := client.Initialize(ctx, &InitializeRequest{})
	require.NoError(t, err)

	// Test calling tool
	toolResult, err := client.CallTool(ctx, &CallToolRequest{
		Params: CallToolParams{
			Name: "test-tool",
			Arguments: map[string]interface{}{
				"name": "Test User",
			},
		},
	})

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, toolResult.Content, 1)

	// Use type assertion to convert ToolContent interface to TextContent type
	textContent, ok := toolResult.Content[0].(TextContent)
	assert.True(t, ok, "Content should be of TextContent type")
	assert.Equal(t, "text", textContent.Type)
	assert.Equal(t, "Hello, Test User!", textContent.Text)
}

func TestClient_GetSessionID(t *testing.T) {
	// Create test environment
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Session ID should be empty in initial state
	assert.Empty(t, client.GetSessionID())

	// Initialize client
	ctx := context.Background()
	_, err := client.Initialize(ctx, &InitializeRequest{})
	require.NoError(t, err)

	// Session ID should not be empty after initialization
	assert.NotEmpty(t, client.GetSessionID())
}

// Test WithHTTPHeaders option
func TestClient_WithHTTPHeaders(t *testing.T) {
	// Create custom headers
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer test-token")
	headers.Set("User-Agent", "TestClient/1.0")
	headers.Set("X-Custom-Header", "custom-value")

	// Create client with custom headers
	client, err := NewClient("http://localhost:3000/mcp", Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	}, WithHTTPHeaders(headers))

	// Verify successful object creation
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.transport)

	// Verify headers are set in transport
	if streamableTransport, ok := client.transport.(*streamableHTTPClientTransport); ok {
		assert.NotNil(t, streamableTransport.httpHeaders)
		assert.Equal(t, "Bearer test-token", streamableTransport.httpHeaders.Get("Authorization"))
		assert.Equal(t, "TestClient/1.0", streamableTransport.httpHeaders.Get("User-Agent"))
		assert.Equal(t, "custom-value", streamableTransport.httpHeaders.Get("X-Custom-Header"))
	}
}

// TestHTTPBeforeRequest tests the HTTPBeforeRequest functionality
func TestHTTPBeforeRequest(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if custom header was added
		if r.Header.Get("X-Custom-Header") != "test-value" {
			t.Errorf("Expected X-Custom-Header to be 'test-value', got '%s'", r.Header.Get("X-Custom-Header"))
		}
		if r.Header.Get("X-Request-ID") != "12345" {
			t.Errorf("Expected X-Request-ID to be '12345', got '%s'", r.Header.Get("X-Request-ID"))
		}

		// Return a valid MCP initialize response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"protocolVersion": "2024-11-05",
				"capabilities": {},
				"serverInfo": {
					"name": "test-server",
					"version": "1.0.0"
				}
			}
		}`))
	}))
	defer server.Close()

	// User-defined composition function
	compose := func(fns ...HTTPBeforeRequestFunc) HTTPBeforeRequestFunc {
		return func(ctx context.Context, req *http.Request) error {
			for _, fn := range fns {
				if err := fn(ctx, req); err != nil {
					return err
				}
			}
			return nil
		}
	}

	// Create client with composed HTTPBeforeRequest functions
	client, err := NewClient(
		server.URL,
		Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
		WithHTTPBeforeRequest(
			compose(
				func(ctx context.Context, req *http.Request) error {
					req.Header.Set("X-Custom-Header", "test-value")
					return nil
				},
				func(ctx context.Context, req *http.Request) error {
					req.Header.Set("X-Request-ID", "12345")
					return nil
				},
			),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Initialize the client
	ctx := context.Background()
	_, err = client.Initialize(ctx, &InitializeRequest{
		Params: InitializeParams{
			ProtocolVersion: ProtocolVersion_2024_11_05,
			Capabilities:    ClientCapabilities{},
			ClientInfo: Implementation{
				Name:    "test-client",
				Version: "1.0.0",
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}
}

// TestHTTPBeforeRequestError tests that errors from HTTPBeforeRequest functions are properly handled
func TestHTTPBeforeRequestError(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Server should not be called when HTTPBeforeRequest returns an error")
	}))
	defer server.Close()

	// Create client with HTTPBeforeRequest that returns an error
	client, err := NewClient(
		server.URL,
		Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
		WithHTTPBeforeRequest(func(ctx context.Context, req *http.Request) error {
			return http.ErrAbortHandler
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Initialize the client - should fail
	ctx := context.Background()
	_, err = client.Initialize(ctx, &InitializeRequest{
		Params: InitializeParams{
			ProtocolVersion: ProtocolVersion_2024_11_05,
			Capabilities:    ClientCapabilities{},
			ClientInfo: Implementation{
				Name:    "test-client",
				Version: "1.0.0",
			},
		},
	})
	if err == nil {
		t.Fatal("Expected error from HTTPBeforeRequest, got nil")
	}
}

// ----------------------------------------------------------------------------
// Close() contract: streamable HTTP transport
//
// Regression coverage for the synchronous-Close behavior of the streamable
// HTTP client transport. Historically, Client.Close() returned as soon as it
// canceled the GET SSE context, without waiting for the background SSE
// goroutine to exit or closing the in-flight response body. The ctx-cancel
// had to propagate through http.Transport to break the blocking
// bufio.Scanner.Scan() -> resp.Body.Read(), which on CI could stall long
// enough to cause server shutdown paths (httptest.Server.Close() in
// trpc-agent-go#1721) to hang.
//
// The tests below pin down the post-fix contract:
//   1. When Client.Close() returns, the GET SSE goroutine has exited.
//   2. Close() returns regardless of whether the server has produced any SSE
//      traffic (resp.Body.Close() drives the wake-up, not ctx propagation).
//   3. Close() right after Initialize() does not leak goroutines even when
//      racing with Initialize's `go establishGetSSEConnection(ctx)` spawn.
// ----------------------------------------------------------------------------

// sseHoldingServer is a minimal MCP-like HTTP server used by the Close()
// regression tests. It does not reuse trpc-mcp-go's own server
// implementation so that the contract being exercised is purely the
// client's Close() behavior.
type sseHoldingServer struct {
	sessionID     string
	activeSSE     atomic.Int32
	handlerExited chan struct{}
	exitedOnce    atomic.Bool
	// handlerTick is how long the SSE handler blocks between event writes
	// WITHOUT checking r.Context(). This models realistic MCP event
	// producers (slow upstream / DB / LLM token stream) that cannot be
	// interrupted mid-work.
	handlerTick time.Duration
}

func newSSEHoldingServer() *sseHoldingServer {
	return &sseHoldingServer{
		sessionID:     "regression-session-1",
		handlerExited: make(chan struct{}),
		handlerTick:   50 * time.Millisecond,
	}
}

func (s *sseHoldingServer) markExited() {
	if s.exitedOnce.CompareAndSwap(false, true) {
		close(s.handlerExited)
	}
}

func (s *sseHoldingServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	case http.MethodGet:
		s.handleGetSSE(w, r)
	case http.MethodDelete:
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *sseHoldingServer) handlePost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Mcp-Session-Id", s.sessionID)

	buf := make([]byte, 4096)
	n, _ := r.Body.Read(buf)
	body := string(buf[:n])

	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(body, `"method":"initialize"`):
		id := extractJSONRPCID(body)
		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{`+
			`"protocolVersion":"2025-03-26",`+
			`"capabilities":{"tools":{}},`+
			`"serverInfo":{"name":"regression-server","version":"0.0.1"}}}`, id)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
	case strings.Contains(body, `"method":"notifications/initialized"`):
		w.WriteHeader(http.StatusAccepted)
	default:
		id := extractJSONRPCID(body)
		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{}}`, id)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
	}
}

// handleGetSSE opens an SSE stream and holds it. Between writes it sleeps
// for handlerTick WITHOUT watching r.Context() — this is the "ctx-insensitive
// work unit" that historically kept the server handler alive past
// Client.Close() and caused httptest.Server.Close() to block.
func (s *sseHoldingServer) handleGetSSE(w http.ResponseWriter, r *http.Request) {
	s.activeSSE.Add(1)
	defer s.activeSSE.Add(-1)
	defer s.markExited()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	for {
		time.Sleep(s.handlerTick)

		select {
		case <-r.Context().Done():
			return
		default:
		}

		if _, err := w.Write([]byte(": ping\n\n")); err != nil {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// extractJSONRPCID pulls the "id" field out of a JSON-RPC envelope so that
// sseHoldingServer.handlePost can echo it back in its response.
func extractJSONRPCID(body string) string {
	idx := strings.Index(body, `"id":`)
	if idx < 0 {
		return `1`
	}
	rest := body[idx+len(`"id":`):]
	end := strings.IndexAny(rest, ",}")
	if end < 0 {
		return `1`
	}
	return strings.TrimSpace(rest[:end])
}

// countGoroutinesMatching returns the number of goroutines whose stack trace
// contains the given substring. Used to detect SSE goroutine leaks after
// Client.Close().
func countGoroutinesMatching(substr string) int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return strings.Count(string(buf[:n]), substr)
}

// waitFor polls cond every 10ms until it returns true or the timeout
// elapses. Returns true if cond became true in time.
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// TestClient_Close_IsSynchronous asserts that when Client.Close() returns,
// the GET SSE goroutine has already exited. Pre-fix this was always false:
// the goroutine outlived Close() by whatever time it took the http.Transport
// to propagate ctx cancellation into the blocked read.
func TestClient_Close_IsSynchronous(t *testing.T) {
	handler := newSSEHoldingServer()
	handler.handlerTick = 500 * time.Millisecond
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.CloseClientConnections()
		ts.Close()
	})

	c, err := NewClient(ts.URL, Implementation{
		Name:    "regression-client",
		Version: "0.0.1",
	}, WithClientGetSSEEnabled(true))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = c.Initialize(ctx, &InitializeRequest{})
	require.NoError(t, err)

	if !waitFor(3*time.Second, func() bool { return handler.activeSSE.Load() >= 1 }) {
		t.Fatalf("precondition failed: GET SSE handler never became active")
	}
	before := countGoroutinesMatching("handleGetSSEEvents")
	t.Logf("precondition ok: activeSSE=%d, client SSE goroutines=%d",
		handler.activeSSE.Load(), before)
	require.GreaterOrEqual(t, before, 1, "expected >=1 client SSE goroutine before Close")

	closeStart := time.Now()
	require.NoError(t, c.Close())
	t.Logf("Client.Close() returned in %v", time.Since(closeStart))

	// Contract: when Close returns, the client-side goroutine has exited.
	if n := countGoroutinesMatching("handleGetSSEEvents"); n != 0 {
		buf := make([]byte, 1<<20)
		nb := runtime.Stack(buf, true)
		t.Errorf("Close contract violated: %d client SSE goroutine(s) still running "+
			"after Close() returned; Close() must synchronously join them.\n"+
			"Full stacks:\n%s", n, buf[:nb])
	}
}

// TestClient_Close_DoesNotWaitForServerTraffic asserts that Close() returns
// quickly even when the server produces no SSE traffic. Pre-fix, close()
// only sent a ctx.cancel that had to traverse http.Transport to break the
// read; post-fix, close() calls resp.Body.Close() directly so the read
// returns immediately with an error.
func TestClient_Close_DoesNotWaitForServerTraffic(t *testing.T) {
	handler := newSSEHoldingServer()
	// Server sends nothing for a long time after the initial 200 OK.
	handler.handlerTick = 30 * time.Second
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.CloseClientConnections()
		ts.Close()
	})

	c, err := NewClient(ts.URL, Implementation{
		Name:    "regression-client",
		Version: "0.0.1",
	}, WithClientGetSSEEnabled(true))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = c.Initialize(ctx, &InitializeRequest{})
	require.NoError(t, err)
	if !waitFor(3*time.Second, func() bool { return handler.activeSSE.Load() >= 1 }) {
		t.Fatalf("precondition failed: GET SSE handler never became active")
	}

	// Close must return essentially immediately — body.Close() wakes up the
	// blocking Read, the goroutine exits, WaitGroup unblocks Close().
	budget := 250 * time.Millisecond
	closeStart := time.Now()
	require.NoError(t, c.Close())
	closeDur := time.Since(closeStart)
	t.Logf("Client.Close() returned in %v (budget=%v, server tick=%v)",
		closeDur, budget, handler.handlerTick)

	if closeDur >= budget {
		t.Errorf("Close took %v, exceeding budget %v while server was quiet. "+
			"This indicates Close() is waiting on ctx-cancellation to "+
			"propagate through http.Transport rather than directly closing "+
			"resp.Body", closeDur, budget)
	}
}

// TestClient_Close_RightAfterInitialize covers the race between Close() and
// Initialize's background `go establishGetSSEConnection(ctx)`. If Close()
// were to return before that goroutine registered itself with sseWg, a
// leaked SSE goroutine would appear on the runtime stack. The `closed`
// guard in establishGetSSE prevents this.
func TestClient_Close_RightAfterInitialize(t *testing.T) {
	handler := newSSEHoldingServer()
	handler.handlerTick = 100 * time.Millisecond
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.CloseClientConnections()
		ts.Close()
	})

	// A handful of iterations is enough to hit the race on most machines;
	// a regression usually leaks at least one goroutine per run.
	const iterations = 50
	for i := 0; i < iterations; i++ {
		c, err := NewClient(ts.URL, Implementation{
			Name:    "regression-client",
			Version: "0.0.1",
		}, WithClientGetSSEEnabled(true))
		require.NoErrorf(t, err, "iter %d: NewClient", i)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = c.Initialize(ctx, &InitializeRequest{})
		if err != nil {
			cancel()
			t.Fatalf("iter %d: Initialize: %v", i, err)
		}
		// Deliberately do NOT wait for the background GET SSE goroutine to
		// make progress: we want to race against it.
		if err := c.Close(); err != nil {
			cancel()
			t.Fatalf("iter %d: Close: %v", i, err)
		}
		cancel()
	}

	// Give any leaked goroutine a chance to surface on the stack.
	time.Sleep(100 * time.Millisecond)

	if n := countGoroutinesMatching("handleGetSSEEvents"); n != 0 {
		buf := make([]byte, 1<<20)
		nb := runtime.Stack(buf, true)
		t.Errorf("after %d tight Initialize/Close cycles, %d handleGetSSEEvents "+
			"goroutine(s) remain on the stack. A late-spawn race is leaking "+
			"SSE goroutines past Close().\nFull stacks:\n%s",
			iterations, n, buf[:nb])
	}
}
