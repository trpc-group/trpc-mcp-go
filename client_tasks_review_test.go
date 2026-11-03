// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent. All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func taskReviewClient(t *testing.T, modern, enabled bool, handler func(*JSONRPCRequest) interface{}) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		var result interface{}
		switch req.Method {
		case MethodInitialize:
			result = map[string]interface{}{"protocolVersion": ProtocolVersion_2025_03_26,
				"capabilities": map[string]interface{}{}, "serverInfo": Implementation{Name: "review-fixture", Version: "1"}}
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
			return
		case "server/discover":
			result = modernDiscovery()
		case MethodToolsList:
			taskLifecycleTools(t, w, &req)
			return
		default:
			result = handler(&req)
		}
		taskLifecycleReply(t, w, &req, result)
	}))
	t.Cleanup(server.Close)
	options := []ClientOption{WithTasks(enabled), WithClientGetSSEEnabled(false)}
	if modern {
		options = append(options, WithProtocolVersion(ProtocolVersion_2026_07_28))
	}
	client, err := NewClient(server.URL, Implementation{Name: "review-test", Version: "1"}, options...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	_, err = client.Initialize(context.Background(), nil)
	require.NoError(t, err)
	return client
}

func TestTasksReviewProgressTokenOmission(t *testing.T) {
	for _, modern := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			for _, tc := range []struct {
				name      string
				meta      string
				wantToken interface{}
			}{
				{name: "no-meta"},
				{name: "empty-meta", meta: `,"_meta":{}`},
				{name: "numeric-zero", meta: `,"_meta":{"progressToken":0}`, wantToken: float64(0)},
				{name: "empty-string", meta: `,"_meta":{"progressToken":""}`, wantToken: ""},
				{name: "string", meta: `,"_meta":{"progressToken":"progress-1"}`, wantToken: "progress-1"},
			} {
				t.Run(fmt.Sprintf("modern=%v/tasks=%v/%s", modern, enabled, tc.name), func(t *testing.T) {
					metas := make(chan map[string]interface{}, 1)
					client := taskReviewClient(t, modern, enabled, func(req *JSONRPCRequest) interface{} {
						if req.Method != MethodToolsCall {
							t.Errorf("unexpected request %s", req.Method)
							return nil
						}
						params := req.Params.(map[string]interface{})
						meta, _ := params["_meta"].(map[string]interface{})
						metas <- meta
						return map[string]interface{}{"resultType": "complete", "content": []interface{}{
							map[string]interface{}{"type": "text", "text": "ordinary result"},
						}}
					})
					var req CallToolRequest
					require.NoError(t, json.Unmarshal([]byte(`{"params":{"name":"charge-account"`+tc.meta+`}}`), &req))
					if tc.meta != "" {
						require.NotNil(t, req.Params.Meta)
					}
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					result, err := client.CallTool(ctx, &req)
					require.NoError(t, err)
					require.Equal(t, []Content{NewTextContent("ordinary result")}, result.Content)
					meta := <-metas
					if tc.wantToken == nil {
						require.NotContains(t, meta, "progressToken")
					} else {
						require.Contains(t, meta, "progressToken")
						require.Equal(t, tc.wantToken, meta["progressToken"])
					}
					if enabled {
						capabilities, ok := meta[clientCapabilitiesMeta].(map[string]interface{})
						require.True(t, ok)
						extensions, ok := capabilities["extensions"].(map[string]interface{})
						require.True(t, ok)
						require.Contains(t, extensions, tasksExtension)
					}
				})
			}
		}
	}
}

func TestTasksReviewInitialReadyStatePollsImmediately(t *testing.T) {
	for _, modern := range []bool{false, true} {
		for _, status := range []string{"completed", "failed", "cancelled", "input_required"} {
			t.Run(fmt.Sprintf("modern=%v/%s", modern, status), func(t *testing.T) {
				var calls, polls atomic.Int32
				client := taskReviewClient(t, modern, true, func(req *JSONRPCRequest) interface{} {
					switch req.Method {
					case MethodToolsCall:
						calls.Add(1)
						return map[string]interface{}{"resultType": "task", "taskId": "task-1",
							"status": status, "pollIntervalMs": 60000}
					case "tasks/get":
						polls.Add(1)
						result := taskResult(status)
						switch status {
						case "completed":
							result["result"] = map[string]interface{}{"content": []interface{}{
								map[string]interface{}{"type": "text", "text": "retrieved result"},
							}}
						case "failed":
							result["error"] = map[string]interface{}{"code": -32000, "message": "job failed",
								"data": map[string]interface{}{"reason": "quota"}}
						case "input_required":
							result["inputRequests"] = map[string]interface{}{"approval-1": map[string]interface{}{
								"method": "elicitation/create", "params": map[string]interface{}{"message": "Approve?"},
							}}
						}
						return result
					default:
						t.Errorf("unexpected request %s", req.Method)
						return nil
					}
				})
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				result, err := client.CallTool(ctx, taskToolRequest())
				require.EqualValues(t, 1, polls.Load(), "a ready task must retrieve its payload before the long polling interval")
				require.EqualValues(t, 1, calls.Load())
				if status == "completed" {
					require.NoError(t, err)
					require.Equal(t, []Content{NewTextContent("retrieved result")}, result.Content)
					return
				}
				var taskErr *TaskError
				require.ErrorAs(t, err, &taskErr)
				require.Equal(t, "task-1", taskErr.TaskID())
				require.Equal(t, status, taskErr.Status)
				require.False(t, taskErr.ToolCallReplaySafe())
				switch status {
				case "failed":
					var rpcErr *TaskRPCError
					require.ErrorAs(t, err, &rpcErr)
					require.Equal(t, -32000, rpcErr.Code)
					require.Equal(t, "job failed", rpcErr.Message)
					require.JSONEq(t, `{"reason":"quota"}`, string(rpcErr.Data))
				case "input_required":
					require.JSONEq(t, `{"method":"elicitation/create","params":{"message":"Approve?"}}`,
						string(taskErr.InputRequests["approval-1"]))
				case "cancelled":
					require.ErrorContains(t, err, "remote task cancelled")
				}
			})
		}
	}
}

func TestTasksReviewInitialWorkingStateKeepsPollInterval(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprint(modern), func(t *testing.T) {
			var calls, polls atomic.Int32
			client := taskReviewClient(t, modern, true, func(req *JSONRPCRequest) interface{} {
				if req.Method == MethodToolsCall {
					calls.Add(1)
					return map[string]interface{}{"resultType": "task", "taskId": "task-1",
						"status": "working", "pollIntervalMs": 60000}
				}
				polls.Add(1)
				t.Errorf("unexpected request %s before the working task polling interval", req.Method)
				return nil
			})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := client.CallTool(ctx, taskToolRequest())
			require.ErrorIs(t, err, context.DeadlineExceeded)
			var taskErr *TaskError
			require.ErrorAs(t, err, &taskErr)
			require.Equal(t, "task-1", taskErr.TaskID())
			require.EqualValues(t, 1, calls.Load())
			require.Zero(t, polls.Load())
		})
	}
}
