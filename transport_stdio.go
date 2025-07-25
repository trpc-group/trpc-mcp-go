// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// StdioServerParameters defines parameters for launching a stdio MCP server.
// This matches the industry standard used by MCP Python SDK, Cursor, and other clients.
type StdioServerParameters struct {
	// Command to execute (e.g., "npx", "python", "node", "/path/to/binary").
	Command string `json:"command"`

	// Arguments to pass to the command.
	Args []string `json:"args,omitempty"`

	// Environment variables to set when launching the server.
	Env map[string]string `json:"env,omitempty"`

	// Working directory for the server process (optional).
	WorkingDir string `json:"working_dir,omitempty"`
}

// stdioClientTransport implements stdio-based MCP transport.
type stdioClientTransport struct {
	serverParams StdioServerParameters
	timeout      time.Duration

	process *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser

	encoder   *json.Encoder
	decoder   *json.Decoder
	requestID atomic.Int64

	requestMutex    sync.Mutex
	pendingRequests map[int64]chan *json.RawMessage
	pendingMutex    sync.RWMutex

	notificationHandlers map[string]NotificationHandler
	handlersMutex        sync.RWMutex

	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	closed    atomic.Bool

	sessionID string
	logger    Logger

	// Client reference for accessing rootsProvider.
	client *StdioClient
}

// stdioTransportOption defines options for stdio transport.
type stdioTransportOption func(*stdioClientTransport)

// newStdioClientTransport creates a new stdio client transport.
func newStdioClientTransport(serverParams StdioServerParameters, options ...stdioTransportOption) *stdioClientTransport {
	ctx, cancel := context.WithCancel(context.Background())

	transport := &stdioClientTransport{
		serverParams:         serverParams,
		timeout:              30 * time.Second, // Default timeout.
		pendingRequests:      make(map[int64]chan *json.RawMessage),
		notificationHandlers: make(map[string]NotificationHandler),
		ctx:                  ctx,
		cancel:               cancel,
		logger:               GetDefaultLogger(),
	}

	// Apply options.
	for _, option := range options {
		option(transport)
	}

	return transport
}

// withStdioTransportTimeout sets the timeout for stdio operations.
func withStdioTransportTimeout(timeout time.Duration) stdioTransportOption {
	return func(t *stdioClientTransport) {
		t.timeout = timeout
	}
}

// withStdioTransportLogger sets the logger for stdio transport.
func withStdioTransportLogger(logger Logger) stdioTransportOption {
	return func(t *stdioClientTransport) {
		t.logger = logger
	}
}

// start is a no-op for stdioClientTransport.
func (t *stdioClientTransport) start(ctx context.Context) error {
	return nil
}

// startProcess starts the MCP server process.
func (t *stdioClientTransport) startProcess() error {
	if t.closed.Load() {
		return fmt.Errorf("transport is closed")
	}

	// Create command.
	cmd := exec.CommandContext(t.ctx, t.serverParams.Command, t.serverParams.Args...)

	// Set working directory if specified.
	if t.serverParams.WorkingDir != "" {
		cmd.Dir = t.serverParams.WorkingDir
	}

	// Set environment variables.
	if len(t.serverParams.Env) > 0 {
		cmd.Env = os.Environ() // Start with current environment.
		for key, value := range t.serverParams.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Create pipes.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process.
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Store references.
	t.process = cmd
	t.stdin = stdin
	t.stdout = stdout
	t.stderr = stderr

	// Create JSON encoder/decoder.
	t.encoder = json.NewEncoder(stdin)
	t.decoder = json.NewDecoder(stdout)

	// Start background goroutines.
	go t.readLoop()
	go t.stderrLoop()
	go t.processWatcher()

	t.logger.Infof("Started stdio process: %s %v (PID: %d)",
		t.serverParams.Command, t.serverParams.Args, cmd.Process.Pid)

	return nil
}

// sendRequest sends a request and waits for a response.
func (t *stdioClientTransport) sendRequest(ctx context.Context, req *JSONRPCRequest) (*json.RawMessage, error) {
	if t.closed.Load() {
		return nil, fmt.Errorf("transport is closed")
	}

	// Start process if isn't started.
	if t.process == nil {
		if err := t.startProcess(); err != nil {
			return nil, fmt.Errorf("failed to start process: %w", err)
		}
	}

	// Generate request ID if not set.
	if req.ID == nil {
		req.ID = t.requestID.Add(1)
	}

	// Create response channel.
	respChan := make(chan *json.RawMessage, 1)
	reqID := req.ID.(int64)

	// Register pending request.
	t.pendingMutex.Lock()
	t.pendingRequests[reqID] = respChan
	t.pendingMutex.Unlock()

	// Clean up on exit.
	defer func() {
		t.pendingMutex.Lock()
		delete(t.pendingRequests, reqID)
		t.pendingMutex.Unlock()
		close(respChan)
	}()

	// Send request.
	t.requestMutex.Lock()
	err := t.encoder.Encode(req)
	t.requestMutex.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response or timeout.
	select {
	case resp := <-respChan:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(t.timeout):
		return nil, fmt.Errorf("request timeout after %v", t.timeout)
	case <-t.ctx.Done():
		return nil, fmt.Errorf("transport closed")
	}
}

// sendNotification sends a notification.
func (t *stdioClientTransport) sendNotification(ctx context.Context, notification *JSONRPCNotification) error {
	if t.closed.Load() {
		return fmt.Errorf("transport is closed")
	}

	// Start process if not started.
	if t.process == nil {
		if err := t.startProcess(); err != nil {
			return fmt.Errorf("failed to start process: %w", err)
		}
	}

	t.requestMutex.Lock()
	err := t.encoder.Encode(notification)
	t.requestMutex.Unlock()

	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// sendResponse sends a response (for server mode, not typically used in client).
func (t *stdioClientTransport) sendResponse(ctx context.Context, resp *JSONRPCResponse) error {
	if t.closed.Load() {
		return fmt.Errorf("transport is closed")
	}

	t.requestMutex.Lock()
	err := t.encoder.Encode(resp)

	// Force flush the stdin pipe to ensure immediate delivery.
	if t.stdin != nil {
		if flusher, ok := t.stdin.(interface{ Flush() error }); ok {
			if flushErr := flusher.Flush(); flushErr != nil {
				t.logger.Warnf("Client sendResponse: Flush error: %v\n", flushErr)
			}
		}

		// Try to sync if it's a file.
		if file, ok := t.stdin.(*os.File); ok {
			if syncErr := file.Sync(); syncErr != nil {
				t.logger.Warnf("Client sendResponse: Sync error: %v\n", syncErr)
			}
		}
	}

	t.requestMutex.Unlock()

	if err != nil {
		t.logger.Errorf("Client sendResponse: Error encoding response: %v", err)
	}

	return err
}

// readLoop continuously reads messages from stdout.
func (t *stdioClientTransport) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			t.logger.Errorf("readLoop panic: %v", r)
		}
	}()

	for !t.closed.Load() {
		var rawMessage json.RawMessage
		if err := t.decoder.Decode(&rawMessage); err != nil {
			if err == io.EOF || t.closed.Load() {
				break
			}
			t.logger.Errorf("Error reading message: %v", err)
			continue
		}

		// Parse message type.
		msgType, err := parseJSONRPCMessageType(rawMessage)
		if err != nil {
			t.logger.Errorf("Error parsing message type: %v", err)
			continue
		}

		switch msgType {
		case JSONRPCMessageTypeResponse:
			t.handleResponse(rawMessage)
		case JSONRPCMessageTypeError:
			t.handleErrorResponse(rawMessage)
		case JSONRPCMessageTypeNotification:
			t.handleNotification(rawMessage)
		case JSONRPCMessageTypeRequest:
			t.handleIncomingRequest(rawMessage)
		default:
			t.logger.Warnf("Unexpected message type: %s", msgType)
		}
	}
}

// handleResponse handles JSON-RPC responses.
func (t *stdioClientTransport) handleResponse(rawMessage json.RawMessage) {
	var response JSONRPCResponse
	if err := json.Unmarshal(rawMessage, &response); err != nil {
		t.logger.Errorf("Error unmarshaling response: %v", err)
		return
	}

	// Find pending request.
	var reqID int64
	switch id := response.ID.(type) {
	case int64:
		reqID = id
	case float64:
		reqID = int64(id)
	case int:
		reqID = int64(id)
	default:
		t.logger.Errorf("Invalid response ID type: %T", response.ID)
		return
	}

	t.pendingMutex.RLock()
	respChan, exists := t.pendingRequests[reqID]
	t.pendingMutex.RUnlock()

	if !exists {
		t.logger.Warnf("No pending request for ID: %d", reqID)
		return
	}

	// Extract result.
	if response.Result != nil {
		resultBytes, err := json.Marshal(response.Result)
		if err != nil {
			t.logger.Errorf("Error marshaling result: %v", err)
			return
		}
		resultMessage := json.RawMessage(resultBytes)

		select {
		case respChan <- &resultMessage:
		default:
			t.logger.Warnf("Response channel full for request ID: %d", reqID)
		}
	} else {
		// Empty result.
		emptyResult := json.RawMessage("{}")
		select {
		case respChan <- &emptyResult:
		default:
			t.logger.Warnf("Response channel full for request ID: %d", reqID)
		}
	}
}

// handleErrorResponse handles JSON-RPC error responses.
func (t *stdioClientTransport) handleErrorResponse(rawMessage json.RawMessage) {
	// For error responses, we send the raw error message.
	// This allows the client to handle errors appropriately.
	var errorResp JSONRPCError
	if err := json.Unmarshal(rawMessage, &errorResp); err != nil {
		t.logger.Errorf("Error unmarshaling error response: %v", err)
		return
	}

	// Find pending request.
	var reqID int64
	switch id := errorResp.ID.(type) {
	case int64:
		reqID = id
	case float64:
		reqID = int64(id)
	case int:
		reqID = int64(id)
	default:
		t.logger.Errorf("Invalid error response ID type: %T", errorResp.ID)
		return
	}

	t.pendingMutex.RLock()
	respChan, exists := t.pendingRequests[reqID]
	t.pendingMutex.RUnlock()

	if !exists {
		t.logger.Warnf("No pending request for error ID: %d", reqID)
		return
	}

	// Send raw error message.
	select {
	case respChan <- &rawMessage:
	default:
		t.logger.Warnf("Response channel full for error ID: %d", reqID)
	}
}

// handleNotification handles JSON-RPC notifications.
func (t *stdioClientTransport) handleNotification(rawMessage json.RawMessage) {
	var notification JSONRPCNotification
	if err := json.Unmarshal(rawMessage, &notification); err != nil {
		t.logger.Debugf("Error unmarshaling notification: %v", err)
		return
	}

	t.handlersMutex.RLock()
	handler, exists := t.notificationHandlers[notification.Method]
	t.handlersMutex.RUnlock()

	if !exists {
		t.logger.Debugf("No handler for notification method: %s", notification.Method)
		return
	}

	// Call handler in goroutine to avoid blocking.
	go func() {
		if err := handler(&notification); err != nil {
			t.logger.Debugf("Error handling notification %s: %v", notification.Method, err)
		}
	}()
}

// handleIncomingRequest handles JSON-RPC requests from the server.
func (t *stdioClientTransport) handleIncomingRequest(rawMessage json.RawMessage) {
	var request JSONRPCRequest
	if err := json.Unmarshal(rawMessage, &request); err != nil {
		t.logger.Errorf("Client handleIncomingRequest: Error unmarshaling incoming request: %v", err)
		return
	}

	// Handle different types of requests.
	switch request.Method {
	case MethodRootsList:
		t.handleRootsListRequest(&request)
	default:
		t.logger.Warnf("Client handleIncomingRequest: Unknown method: %s", request.Method)
		// Send method not found error
		t.sendErrorResponse(&request, ErrCodeMethodNotFound, fmt.Sprintf("Method not found: %s", request.Method))
	}
}

// handleRootsListRequest handles roots/list requests from the server.
func (t *stdioClientTransport) handleRootsListRequest(request *JSONRPCRequest) {
	// Get roots from the client if it has a reference.
	var roots []Root
	if t.client != nil {
		t.client.rootsMu.RLock()
		provider := t.client.rootsProvider
		t.client.rootsMu.RUnlock()

		if provider != nil {
			roots = provider.GetRoots()
		} else {
			t.logger.Debugf("Client handleRootsListRequest: No roots provider available")
		}
	} else {
		t.logger.Debugf("Client handleRootsListRequest: No client reference available")
	}

	if roots == nil {
		roots = []Root{}
	}

	result := &ListRootsResult{
		Roots: roots,
	}

	response := &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  result,
	}

	// Send response.
	if err := t.sendResponse(context.Background(), response); err != nil {
		t.logger.Errorf("Client handleRootsListRequest: Failed to send roots/list response: %v", err)
	}
}

// sendErrorResponse sends an error response to the server.
func (t *stdioClientTransport) sendErrorResponse(request *JSONRPCRequest, code int, message string) {
	errorResp := newJSONRPCErrorResponse(request.ID, code, message, nil)

	// Convert JSONRPCError to bytes and send.
	errorBytes, err := json.Marshal(errorResp)
	if err != nil {
		t.logger.Errorf("Failed to marshal error response: %v", err)
		return
	}

	if err := t.encoder.Encode(json.RawMessage(errorBytes)); err != nil {
		t.logger.Errorf("Failed to send error response: %v", err)
	}
}

// stderrLoop reads and logs stderr output.
func (t *stdioClientTransport) stderrLoop() {
	if t.stderr == nil {
		return
	}

	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() && !t.closed.Load() {
		line := scanner.Text()
		if line != "" {
			t.logger.Debugf("Server stderr: %s", line)
		}
	}
}

// processWatcher monitors the process and handles unexpected exits.
func (t *stdioClientTransport) processWatcher() {
	if t.process == nil {
		return
	}

	err := t.process.Wait()
	if !t.closed.Load() {
		if err != nil {
			t.logger.Debugf("Process exited with error: %v", err)
		} else {
			t.logger.Debugf("Process exited normally")
		}
		// Cancel context to signal shutdown.
		t.cancel()
	}
}

// registerNotificationHandler registers a handler for notifications.
func (t *stdioClientTransport) registerNotificationHandler(method string, handler NotificationHandler) {
	t.handlersMutex.Lock()
	t.notificationHandlers[method] = handler
	t.handlersMutex.Unlock()
}

// unregisterNotificationHandler removes a notification handler.
func (t *stdioClientTransport) unregisterNotificationHandler(method string) {
	t.handlersMutex.Lock()
	delete(t.notificationHandlers, method)
	t.handlersMutex.Unlock()
}

// close closes the transport and terminates the process.
func (t *stdioClientTransport) close() error {
	if !t.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	var errs []error

	// Cancel context first.
	t.cancel()

	// Close pipes
	if t.stdin != nil {
		if err := t.stdin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close stdin: %w", err))
		}
	}

	if t.stdout != nil {
		if err := t.stdout.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close stdout: %w", err))
		}
	}

	if t.stderr != nil {
		if err := t.stderr.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close stderr: %w", err))
		}
	}

	// Terminate process gracefully.
	if t.process != nil && t.process.Process != nil {
		// First try SIGTERM
		if err := t.process.Process.Signal(os.Interrupt); err != nil {
			t.logger.Debugf("Failed to send SIGTERM: %v", err)
		}

		// Wait a bit for graceful shutdown.
		done := make(chan struct{})
		go func() {
			t.process.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.logger.Debugf("Process terminated gracefully")
		case <-time.After(5 * time.Second):
			// Force kill.
			if err := t.process.Process.Kill(); err != nil {
				errs = append(errs, fmt.Errorf("failed to kill process: %w", err))
			} else {
				t.logger.Debugf("Process force-killed")
			}
		}
	}

	// Close all pending request channels.
	t.pendingMutex.Lock()
	for reqID, ch := range t.pendingRequests {
		close(ch)
		delete(t.pendingRequests, reqID)
	}
	t.pendingMutex.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

// getSessionID returns the session ID (stdio doesn't use sessions typically).
func (t *stdioClientTransport) getSessionID() string {
	return t.sessionID
}

// setSessionID sets the session ID.
func (t *stdioClientTransport) setSessionID(sessionID string) {
	t.sessionID = sessionID
}

// terminateSession terminates the session (for stdio this closes the transport).
func (t *stdioClientTransport) terminateSession(ctx context.Context) error {
	return t.close()
}

// getProcessID returns the process ID.
func (t *stdioClientTransport) getProcessID() int {
	if t.process != nil && t.process.Process != nil {
		return t.process.Process.Pid
	}
	return 0
}

// getCommandLine returns the command line.
func (t *stdioClientTransport) getCommandLine() []string {
	result := []string{t.serverParams.Command}
	result = append(result, t.serverParams.Args...)
	return result
}

// isProcessRunning checks if the process is running.
func (t *stdioClientTransport) isProcessRunning() bool {
	if t.process == nil || t.process.Process == nil {
		return false
	}

	// On Unix systems, sending signal 0 checks if process exists.
	return t.process.Process.Signal(syscall.Signal(0)) == nil
}
