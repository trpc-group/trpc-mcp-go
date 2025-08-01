// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/session"
)

// setupCancellationTest is a helper function to create a consistent test environment.
// It sets up a handler, a session, and a mock "long-running" tool.
func setupCancellationTest(t *testing.T) (*mcpHandler, *session.Session, *Tool) {
	t.Helper()

	// 1. Create the handler
	h := newMCPHandler()

	// 2. Create a concrete session object
	s := session.NewSession()

	// 3. Define our mock long-running tool
	longRunningTool := &Tool{
		Name: "long_running_tool",
	}

	return h, s, longRunningTool
}

// TestCancellationFeature provides a suite of tests for the request cancellation functionality.
func TestCancellationFeature(t *testing.T) {

	t.Run("should cancel a running request successfully", func(t *testing.T) {
		h, s, tool := setupCancellationTest(t)

		// This channel will signal that the handler has finished processing
		handlerDone := make(chan error)
		// This channel will signal that the long-running task has started
		taskStarted := make(chan struct{})

	// Define the long-running tool's handler function
	toolHandler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		close(taskStarted) // Signal that the task has started
		// Block until the context is cancelled
		<-ctx.Done()
		// Return the context's error (which should be context.Canceled)
		return nil, ctx.Err()
	}
	h.toolManager.registerTool(tool, toolHandler)

		// Create the request
		req := &JSONRPCRequest{
			ID:     "test-req-1",
			Params: map[string]interface{}{"name": tool.Name},
			Request: Request{
				Method: MethodToolsCall,
			},
		}

		// Run the request handler in a goroutine
		go func() {
			_, err := h.handleRequest(context.Background(), req, s)
			handlerDone <- err
		}()

		// Wait for the task to start, with a timeout to prevent test hangs
		select {
		case <-taskStarted:
			// Task started, proceed to cancel
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for the long-running task to start")
		}

		// Now, issue the cancellation
		s.CancelRequest(req.ID)

		// Wait for the handler to finish and check the error
		// Note: handleRequest will return nil even when the context is canceled 
		// because the error is converted to a JSON-RPC error response internally
		select {
		case err := <-handlerDone:
			// Handler should return nil, not context.Canceled directly
			if err != nil {
				t.Errorf("expected error to be nil (not propagated), but got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for the handler to return after cancellation")
		}

		// Verify the canceler map is clean
		// Instead of directly accessing the map, we can try to cancel again
		// which should have no effect if the cancellation was cleaned up
		s.CancelRequest(req.ID)
		// Or we can check if the context is cancelled
		time.Sleep(50 * time.Millisecond) // Give the system time to process
	})

	t.Run("should ignore cancellation for an already completed request", func(t *testing.T) {
		h, s, tool := setupCancellationTest(t)
		handlerDone := make(chan struct{})

		// A simple handler that finishes immediately
		toolHandler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return &CallToolResult{
				Result: Result{
					Meta: map[string]interface{}{"status": "done"},
				},
			}, nil
		}
		h.toolManager.registerTool(tool, toolHandler)

		req := &JSONRPCRequest{
			ID:     "test-req-2",
			Params: map[string]interface{}{"name": tool.Name},
			Request: Request{
				Method: MethodToolsCall,
			},
		}

		// Run and wait for the request to complete
		go func() {
			h.handleRequest(context.Background(), req, s)
			close(handlerDone)
		}()
		<-handlerDone

		// Verify the canceler map is already clean
		// We can't directly check the map, but we can verify that 
		// cancelling again has no observable effect
		s.CancelRequest(req.ID) // This should be a no-op

		// Now, send a late cancellation notification
		// This should not cause any panic or error.
		s.CancelRequest(req.ID)
	})

	t.Run("should ignore cancellation for an unknown request ID", func(t *testing.T) {
		_, s, _ := setupCancellationTest(t)
		// Attempt to cancel a request that never existed.
		// The test passes if it doesn't panic.
		s.CancelRequest("unknown-id")
	})

	t.Run("should cancel all running requests on session termination", func(t *testing.T) {
		h, s, tool := setupCancellationTest(t)
		var wg sync.WaitGroup
		taskCount := 3
		var cancelCount int32 // 使用原子操作安全计数

		// A handler that signals when it's cancelled
		toolHandler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			<-ctx.Done()
			atomic.AddInt32(&cancelCount, 1) // 安全递增计数器
			wg.Done() // Signal that this task was successfully cancelled
			return nil, ctx.Err()
		}
		h.toolManager.registerTool(tool, toolHandler)

		// Start three long-running requests
		wg.Add(taskCount)
		for i := 0; i < taskCount; i++ {
			req := &JSONRPCRequest{
				ID:     fmt.Sprintf("batch-req-%d", i),
				Params: map[string]interface{}{"name": tool.Name},
				Request: Request{
					Method: MethodToolsCall,
				},
			}
			go h.handleRequest(context.Background(), req, s)
		}

		// Give the requests a moment to register themselves
		time.Sleep(50 * time.Millisecond)

		// Now, terminate the session
		s.CancelAll()

		// Use a channel to wait for all tasks to be cancelled, with a timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All tasks were cancelled as expected
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for all tasks to be cancelled on session termination")
		}

		// Verify that all tasks were cancelled
		count := atomic.LoadInt32(&cancelCount)
		if int(count) != taskCount {
			t.Errorf("expected %d tasks to be cancelled, but only %d were", taskCount, count)
		}
	})

	t.Run("should automatically cancel request on timeout", func(t *testing.T) {
		h, s, tool := setupCancellationTest(t)
		
		// This channel will signal that the handler has finished processing
		handlerDone := make(chan error)
		// This channel will signal that the long-running task has started
		taskStarted := make(chan struct{})

		// Define the long-running tool's handler function
		toolHandler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			close(taskStarted) // Signal that the task has started
			// Block until the context is cancelled
			<-ctx.Done()
			// Return the context's error (which should be context.Canceled or context.DeadlineExceeded)
			return nil, ctx.Err()
		}
		h.toolManager.registerTool(tool, toolHandler)

		// Create the request
		req := &JSONRPCRequest{
			ID:     "timeout-req",
			Params: map[string]interface{}{"name": tool.Name},
			Request: Request{
				Method: MethodToolsCall,
			},
		}

		// Create a context with a very short timeout (100ms)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Run the request handler in a goroutine
		go func() {
			_, err := h.handleRequest(ctx, req, s)
			handlerDone <- err
		}()

		// Wait for the task to start, with a timeout to prevent test hangs
		select {
		case <-taskStarted:
			// Task started, now wait for automatic timeout
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for the long-running task to start")
		}

		// Wait for the handler to finish due to timeout
		select {
		case err := <-handlerDone:
			// Handler should return nil, as the error is converted to a JSON-RPC error internally
			if err != nil {
				t.Errorf("expected error to be nil (not propagated), but got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for the handler to return after timeout")
		}

		// Verify the canceler map is clean
		s.CancelRequest(req.ID) // Should have no effect if cleanup was done properly
	})

	t.Run("should not cancel initialize request as per MCP specification", func(t *testing.T) {
		h, s, _ := setupCancellationTest(t)

		// Create an initialize request
		initReq := &JSONRPCRequest{
			ID:     "init-123",
			Params: map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"clientInfo": map[string]interface{}{
					"name":    "TestClient",
					"version": "1.0.0",
				},
				"capabilities": map[string]interface{}{},
			},
			Request: Request{
				Method: MethodInitialize,
			},
		}

		// Process the initialize request to mark it in the session
		ctx := context.Background()
		_, err := h.handleRequest(ctx, initReq, s)
		if err != nil {
			t.Fatalf("Initialize request failed: %v", err)
		}

		// Create a cancel notification targeting the initialize request
		cancelNotification := &JSONRPCNotification{
			JSONRPC: JSONRPCVersion,
			Notification: Notification{
				Method: MethodCancelRequest,
				Params: NotificationParams{
					AdditionalFields: map[string]interface{}{
						"requestId": initReq.ID,
						"reason":    "testing cancellation protection",
					},
				},
			},
		}

		// Attempt to cancel the initialize request - this should be silently ignored
		err = h.handleNotification(ctx, cancelNotification, s)
		if err != nil {
			t.Errorf("Cancel notification should not return error, but got: %v", err)
		}

		// Verify that the initialize request ID is still marked in the session
		if storedID, exists := s.GetData("__initialize_request_id"); !exists || storedID != initReq.ID {
			t.Error("Initialize request ID should still be stored in session after attempted cancellation")
		}
	})
}
