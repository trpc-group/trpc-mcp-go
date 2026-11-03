// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent. All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-mcp-go/internal/retry"
)

func taskTestClient(t *testing.T, handler func(*JSONRPCRequest) interface{}) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		if req.Method == MethodInitialize {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"result":{"protocolVersion":"2025-03-26","capabilities":{},"serverInfo":{"name":"fixture","version":"1"}}}`, req.ID)
			return
		}
		if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		params := req.Params.(map[string]interface{})
		if req.Method == "tasks/get" || req.Method == "tasks/update" || req.Method == "tasks/cancel" {
			require.Equal(t, params["taskId"], r.Header.Get("Mcp-Name"))
			require.Equal(t, req.Method, r.Header.Get("Mcp-Method"))
		}
		result := handler(&req)
		if result == nil {
			http.Error(w, "lost response", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if rpcErr, ok := result.(*JSONRPCError); ok {
			require.NoError(t, json.NewEncoder(w).Encode(rpcErr))
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result}))
	}))
	t.Cleanup(server.Close)
	c, err := NewClient(server.URL, Implementation{Name: "tasks-test", Version: "1"}, WithTasks(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	_, err = c.Initialize(context.Background(), &InitializeRequest{})
	require.NoError(t, err)
	return c
}

func taskResult(status string) map[string]interface{} {
	return map[string]interface{}{"resultType": "complete", "taskId": "task-1", "status": status, "pollIntervalMs": 0}
}
func taskHandle() map[string]interface{} {
	return map[string]interface{}{"resultType": "task", "taskId": "task-1", "status": "working", "pollIntervalMs": 0}
}
func taskToolRequest() *CallToolRequest {
	r := &CallToolRequest{}
	r.Params.Name = "charge-account"
	return r
}

func TestTasksCallAndResumeWithoutReplay(t *testing.T) {
	var calls, polls atomic.Int32
	c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
		if req.Method == MethodToolsCall {
			calls.Add(1)
			meta := req.Params.(map[string]interface{})["_meta"].(map[string]interface{})
			require.Contains(t, meta["io.modelcontextprotocol/clientCapabilities"].(map[string]interface{})["extensions"], tasksExtension)
			return taskHandle()
		}
		require.Equal(t, "tasks/get", req.Method)
		if polls.Add(1) == 1 {
			return nil
		}
		result := taskResult("completed")
		result["result"] = map[string]interface{}{"content": []interface{}{map[string]interface{}{"type": "text", "text": "charged"}}, "structuredContent": map[string]interface{}{"receipt": "one"}, "_meta": map[string]interface{}{"trace": "ok"}, "isError": true}
		return result
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := c.CallTool(ctx, taskToolRequest())
	var taskErr *TaskError
	require.ErrorAs(t, err, &taskErr)
	require.Equal(t, "task-1", taskErr.TaskID())
	result, err := c.ResumeToolTask(ctx, taskErr.TaskID())
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Equal(t, "ok", result.Meta["trace"])
	require.Equal(t, map[string]interface{}{"receipt": "one"}, result.StructuredContent)
	require.EqualValues(t, 1, calls.Load())
	require.EqualValues(t, 2, polls.Load())
}

func TestTasksUnknownSubmissionDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	c := taskTestClient(t, func(req *JSONRPCRequest) interface{} { calls.Add(1); return nil })
	c.transport.setRetryConfig(&retry.Config{MaxRetries: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond})
	_, err := c.CallTool(context.Background(), taskToolRequest())
	var te *TaskError
	require.ErrorAs(t, err, &te)
	require.Empty(t, te.TaskID())
	require.False(t, te.ToolCallReplaySafe())
	require.EqualValues(t, 1, calls.Load())
}

func TestTasksTerminalAndMalformed(t *testing.T) {
	for _, status := range []string{"failed", "cancelled", "input_required", "unknown", "wrong-id", "bad-result", "bad-discriminator"} {
		t.Run(status, func(t *testing.T) {
			c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
				if req.Method == MethodToolsCall {
					return taskHandle()
				}
				r := taskResult(status)
				switch status {
				case "failed":
					r["error"] = map[string]interface{}{"code": -32000, "message": "job failed", "data": map[string]interface{}{"reason": "quota"}}
				case "input_required":
					r["inputRequests"] = map[string]interface{}{"approval-1": map[string]interface{}{"method": "elicitation/create", "params": map[string]interface{}{}}}
				case "wrong-id":
					r["taskId"] = "other"
				case "bad-result":
					r["status"] = "completed"
					r["result"] = map[string]interface{}{}
				case "bad-discriminator":
					r["resultType"] = "task"
				}
				return r
			})
			_, err := c.CallTool(context.Background(), taskToolRequest())
			var te *TaskError
			require.ErrorAs(t, err, &te)
			require.Equal(t, "task-1", te.TaskID())
			if status == "failed" {
				require.Equal(t, "failed", te.Status)
				var re *TaskRPCError
				require.ErrorAs(t, err, &re)
				require.Equal(t, -32000, re.Code)
				require.JSONEq(t, `{"reason":"quota"}`, string(re.Data))
			}
			if status == "input_required" {
				require.Contains(t, te.InputRequests, "approval-1")
			}
		})
	}
}

func TestTasksDeadlineAndExplicitActions(t *testing.T) {
	var calls, polls, cancels, updates atomic.Int32
	c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
		switch req.Method {
		case MethodToolsCall:
			calls.Add(1)
			r := taskHandle()
			r["pollIntervalMs"] = 60000
			return r
		case "tasks/get":
			polls.Add(1)
			return taskResult("cancelled")
		case "tasks/cancel":
			cancels.Add(1)
		case "tasks/update":
			updates.Add(1)
			require.Contains(t, req.Params.(map[string]interface{})["inputResponses"], "approval-1")
		}
		return map[string]interface{}{"resultType": "complete"}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := c.CallTool(ctx, taskToolRequest())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.EqualValues(t, 0, polls.Load())
	require.EqualValues(t, 0, cancels.Load())
	require.NoError(t, c.UpdateTask(context.Background(), "task-1", map[string]interface{}{"approval-1": map[string]interface{}{"result": map[string]interface{}{"action": "decline"}}}))
	require.NoError(t, c.CancelTask(context.Background(), "task-1"))
	require.EqualValues(t, 1, cancels.Load())
	require.EqualValues(t, 1, updates.Load())
	_, err = c.ResumeToolTask(context.Background(), "task-1")
	require.Error(t, err)
	require.EqualValues(t, 1, calls.Load())
}

func TestTasksImmediateAndDisabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
				require.Equal(t, MethodToolsCall, req.Method)
				if !enabled {
					require.NotContains(t, req.Params.(map[string]interface{}), "_meta")
				}
				return map[string]interface{}{"resultType": "complete", "content": []interface{}{}}
			})
			c.tasksEnabled = enabled
			require.Equal(t, enabled, c.TasksEnabled())
			_, err := c.CallTool(context.Background(), taskToolRequest())
			require.NoError(t, err)
		})
	}
}

func TestTasksPollPacingAndCancellation(t *testing.T) {
	zero := int64(0)
	huge := int64(1<<63 - 1)
	require.Greater(t, taskPollDelay(&zero), time.Duration(0))
	require.Greater(t, taskPollDelay(&huge), time.Duration(0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.True(t, errors.Is(waitTask(ctx, time.Hour), context.Canceled))
}

func TestTasksDeadlineDuringRequest(t *testing.T) {
	c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
		time.Sleep(100 * time.Millisecond)
		return taskHandle()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := c.CallTool(ctx, taskToolRequest())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var te *TaskError
	require.ErrorAs(t, err, &te)
	require.Empty(t, te.TaskID())
}

func TestTasksRPCEnvelopePreservesHandle(t *testing.T) {
	c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
		return newJSONRPCErrorResponse(req.ID, -32602, "task not found", map[string]interface{}{"scope": "session"})
	})
	_, err := c.ResumeToolTask(context.Background(), "expired-task")
	var te *TaskError
	var re *TaskRPCError
	require.ErrorAs(t, err, &te)
	require.Equal(t, "expired-task", te.TaskID())
	require.Empty(t, te.Status)
	require.ErrorAs(t, err, &re)
	require.Equal(t, -32602, re.Code)
	require.JSONEq(t, `{"scope":"session"}`, string(re.Data))
}

func TestTasksFailedResponseValidatesIdentity(t *testing.T) {
	c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
		r := taskResult("failed")
		r["taskId"] = "another-task"
		r["error"] = map[string]interface{}{"code": -32000, "message": "wrong task"}
		return r
	})
	_, err := c.ResumeToolTask(context.Background(), "task-1")
	require.ErrorContains(t, err, "invalid task response identity")
	var re *TaskRPCError
	require.False(t, errors.As(err, &re))
}

func TestTasksMutationsDoNotRetry(t *testing.T) {
	for _, action := range []string{"update", "cancel"} {
		t.Run(action, func(t *testing.T) {
			var calls atomic.Int32
			c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
				require.Equal(t, "tasks/"+action, req.Method)
				calls.Add(1)
				return nil
			})
			c.transport.setRetryConfig(&retry.Config{MaxRetries: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond})
			var err error
			if action == "cancel" {
				err = c.CancelTask(context.Background(), "task-1")
			} else {
				err = c.UpdateTask(context.Background(), "task-1", map[string]interface{}{"approval": map[string]interface{}{"result": "accepted"}})
			}
			var te *TaskError
			require.ErrorAs(t, err, &te)
			require.Equal(t, "task-1", te.TaskID())
			require.EqualValues(t, 1, calls.Load())
		})
	}
}

func TestTasksCancelAcknowledgementCanRaceWithCompletion(t *testing.T) {
	var polls atomic.Int32
	c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
		if req.Method == "tasks/cancel" {
			return map[string]interface{}{"resultType": "complete"}
		}
		require.Equal(t, "tasks/get", req.Method)
		if polls.Add(1) == 1 {
			return taskResult("working")
		}
		r := taskResult("completed")
		r["result"] = map[string]interface{}{"content": []interface{}{map[string]interface{}{"type": "text", "text": "completed before cancellation"}}}
		return r
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, c.CancelTask(ctx, "task-1"))
	result, err := c.ResumeToolTask(ctx, "task-1")
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	require.EqualValues(t, 2, polls.Load())
}

func TestTasksRepeatedInputRequiresApplicationAction(t *testing.T) {
	var polls atomic.Int32
	c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
		require.Equal(t, "tasks/get", req.Method)
		polls.Add(1)
		r := taskResult("input_required")
		r["inputRequests"] = map[string]interface{}{"approval-1": map[string]interface{}{"method": "elicitation/create"}}
		return r
	})
	for i := 0; i < 2; i++ {
		_, err := c.ResumeToolTask(context.Background(), "task-1")
		var te *TaskError
		require.ErrorAs(t, err, &te)
		require.Equal(t, "input_required", te.Status)
		require.Contains(t, te.InputRequests, "approval-1")
	}
	require.EqualValues(t, 2, polls.Load())
}

func TestTasksConcurrentCallsKeepIndependentHandles(t *testing.T) {
	var calls atomic.Int32
	c := taskTestClient(t, func(req *JSONRPCRequest) interface{} {
		if req.Method == MethodToolsCall {
			r := taskHandle()
			r["taskId"] = fmt.Sprintf("task-%d", calls.Add(1))
			return r
		}
		require.Equal(t, "tasks/get", req.Method)
		id := req.Params.(map[string]interface{})["taskId"].(string)
		r := taskResult("completed")
		r["taskId"] = id
		r["result"] = map[string]interface{}{"content": []interface{}{}, "structuredContent": map[string]interface{}{"id": id}}
		return r
	})
	type outcome struct {
		result *CallToolResult
		err    error
	}
	const count = 12
	results := make(chan outcome, count)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := 0; i < count; i++ {
		go func() {
			r, err := c.CallTool(ctx, taskToolRequest())
			results <- outcome{r, err}
		}()
	}
	ids := make(map[interface{}]bool)
	for i := 0; i < count; i++ {
		r := <-results
		require.NoError(t, r.err)
		id := r.result.StructuredContent.(map[string]interface{})["id"]
		require.False(t, ids[id], "task result delivered to multiple calls")
		ids[id] = true
	}
	require.Len(t, ids, count)
	require.EqualValues(t, count, calls.Load())
}
