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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func taskLifecycleClient(t *testing.T, modern bool,
	handler func(http.ResponseWriter, *http.Request, *JSONRPCRequest), options ...ClientOption,
) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		switch req.Method {
		case MethodInitialize:
			taskLifecycleReply(t, w, &req, map[string]interface{}{
				"protocolVersion": ProtocolVersion_2025_03_26, "capabilities": map[string]interface{}{},
				"serverInfo": Implementation{Name: "lifecycle-fixture", Version: "1"},
			})
		case "server/discover":
			taskLifecycleReply(t, w, &req, modernDiscovery())
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		default:
			handler(w, r, &req)
		}
	}))
	t.Cleanup(server.Close)
	options = append(options, WithTasks(true), WithClientGetSSEEnabled(false))
	if modern {
		options = append(options, WithProtocolVersion(ProtocolVersion_2026_07_28))
	}
	client, err := NewClient(server.URL, Implementation{Name: "lifecycle-test", Version: "1"}, options...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	_, err = client.Initialize(context.Background(), nil)
	require.NoError(t, err)
	return client
}

func taskLifecycleReply(t *testing.T, w http.ResponseWriter, req *JSONRPCRequest, result interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(newJSONRPCResponse(req.ID, result)); err != nil {
		t.Error(err)
	}
}

func taskLifecycleTools(t *testing.T, w http.ResponseWriter, req *JSONRPCRequest) {
	t.Helper()
	taskLifecycleReply(t, w, req, map[string]interface{}{
		"resultType": "complete", "tools": []interface{}{map[string]interface{}{
			"name": taskToolRequest().Params.Name, "inputSchema": map[string]interface{}{"type": "object"},
		}},
	})
}

func taskLifecycleCall(client *Client, ctx context.Context, entry string) error {
	if entry == "resume" {
		_, err := client.ResumeToolTask(ctx, "task-1")
		return err
	}
	_, err := client.CallTool(ctx, taskToolRequest())
	return err
}

func awaitTaskLifecycle(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("task request did not reach the server")
	}
}

func requireTaskStopped(t *testing.T, done <-chan error, id string) {
	t.Helper()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
		var taskErr *TaskError
		require.ErrorAs(t, err, &taskErr)
		require.Equal(t, id, taskErr.TaskID())
		require.False(t, taskErr.ToolCallReplaySafe())
	case <-time.After(3 * time.Second):
		t.Fatal("task operation did not stop")
	}
}

func TestTasksCloseStopsPolling(t *testing.T) {
	for _, modern := range []bool{false, true} {
		for _, entry := range []string{"call", "resume"} {
			for _, phase := range []string{"interval", "request"} {
				t.Run(fmt.Sprintf("modern=%v/%s/%s", modern, entry, phase), func(t *testing.T) {
					var calls, polls, mutations atomic.Int32
					ready := make(chan struct{})
					client := taskLifecycleClient(t, modern, func(w http.ResponseWriter, r *http.Request, req *JSONRPCRequest) {
						switch req.Method {
						case MethodToolsList:
							taskLifecycleTools(t, w, req)
						case MethodToolsCall:
							calls.Add(1)
							taskLifecycleReply(t, w, req, taskHandle())
						case "tasks/get":
							if polls.Add(1) > 1 {
								t.Error("task polling continued after Close")
								return
							}
							if phase == "request" {
								close(ready)
								<-r.Context().Done()
								return
							}
							state := taskResult("working")
							state["pollIntervalMs"] = 60000
							taskLifecycleReply(t, w, req, state)
							close(ready)
						default:
							mutations.Add(1)
							t.Errorf("unexpected task method %s", req.Method)
						}
					})
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					done := make(chan error, 1)
					go func() { done <- taskLifecycleCall(client, ctx, entry) }()
					awaitTaskLifecycle(t, ready)
					require.NoError(t, client.Close())
					require.NoError(t, client.Close())
					requireTaskStopped(t, done, "task-1")
					_, err := client.ResumeToolTask(ctx, "task-1")
					require.Error(t, err)
					_, err = client.CallTool(ctx, taskToolRequest())
					require.Error(t, err)
					require.EqualValues(t, 1, polls.Load())
					if entry == "call" {
						require.EqualValues(t, 1, calls.Load())
					} else {
						require.Zero(t, calls.Load())
					}
					require.Zero(t, mutations.Load())
				})
			}
		}
	}
}

func TestTasksCloseStopsRequests(t *testing.T) {
	for _, modern := range []bool{false, true} {
		for _, method := range []string{MethodToolsList, MethodToolsCall, "tasks/update", "tasks/cancel"} {
			if method == MethodToolsList && !modern {
				continue
			}
			t.Run(fmt.Sprintf("modern=%v/%s", modern, method), func(t *testing.T) {
				ready := make(chan struct{})
				var requests atomic.Int32
				client := taskLifecycleClient(t, modern, func(w http.ResponseWriter, r *http.Request, req *JSONRPCRequest) {
					if method == MethodToolsCall && req.Method == MethodToolsList {
						taskLifecycleTools(t, w, req)
						return
					}
					if req.Method != method || requests.Add(1) != 1 {
						t.Errorf("unexpected request %s", req.Method)
						return
					}
					close(ready)
					<-r.Context().Done()
				})
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				done := make(chan error, 1)
				go func() {
					switch method {
					case MethodToolsList, MethodToolsCall:
						done <- taskLifecycleCall(client, ctx, "call")
					case "tasks/update":
						done <- client.UpdateTask(ctx, "task-1", map[string]interface{}{})
					case "tasks/cancel":
						done <- client.CancelTask(ctx, "task-1")
					}
				}()
				awaitTaskLifecycle(t, ready)
				require.NoError(t, client.Close())
				if method == MethodToolsList {
					select {
					case err := <-done:
						require.ErrorIs(t, err, context.Canceled)
					case <-time.After(3 * time.Second):
						t.Fatal("tool schema discovery did not stop")
					}
				} else if method == MethodToolsCall {
					requireTaskStopped(t, done, "")
				} else {
					requireTaskStopped(t, done, "task-1")
				}
				require.EqualValues(t, 1, requests.Load())
			})
		}
	}
}

func TestTasksCallerCancellationStopsPolling(t *testing.T) {
	for _, modern := range []bool{false, true} {
		for _, entry := range []string{"call", "resume"} {
			t.Run(fmt.Sprintf("modern=%v/%s", modern, entry), func(t *testing.T) {
				ready := make(chan struct{})
				var polls atomic.Int32
				client := taskLifecycleClient(t, modern, func(w http.ResponseWriter, r *http.Request, req *JSONRPCRequest) {
					switch req.Method {
					case MethodToolsList:
						taskLifecycleTools(t, w, req)
					case MethodToolsCall:
						taskLifecycleReply(t, w, req, taskHandle())
					case "tasks/get":
						if polls.Add(1) == 1 {
							close(ready)
							<-r.Context().Done()
							return
						}
						result := taskResult("completed")
						result["result"] = map[string]interface{}{"content": []interface{}{}}
						taskLifecycleReply(t, w, req, result)
					default:
						t.Errorf("unexpected task method %s", req.Method)
					}
				})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				go func() { done <- taskLifecycleCall(client, ctx, entry) }()
				awaitTaskLifecycle(t, ready)
				cancel()
				requireTaskStopped(t, done, "task-1")
				resumeCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				_, err := client.ResumeToolTask(resumeCtx, "task-1")
				require.NoError(t, err)
			})
		}
	}
}

func TestTasksCanceledContextDoesNotSend(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprint(modern), func(t *testing.T) {
			var requests atomic.Int32
			client := taskLifecycleClient(t, modern, func(http.ResponseWriter, *http.Request, *JSONRPCRequest) {
				requests.Add(1)
			})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			require.ErrorIs(t, taskLifecycleCall(client, ctx, "call"), context.Canceled)
			require.ErrorIs(t, taskLifecycleCall(client, ctx, "resume"), context.Canceled)
			require.ErrorIs(t, client.UpdateTask(ctx, "task-1", map[string]interface{}{}), context.Canceled)
			require.ErrorIs(t, client.CancelTask(ctx, "task-1"), context.Canceled)
			require.Zero(t, requests.Load())
		})
	}
}

func TestTasksReinitializeSeparatesCallLifetimes(t *testing.T) {
	type oldCallKey struct{}
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprint(modern), func(t *testing.T) {
			oldReady, releaseOld := make(chan struct{}), make(chan struct{})
			var releaseOldOnce sync.Once
			unblockOld := func() { releaseOldOnce.Do(func() { close(releaseOld) }) }
			defer unblockOld()
			newReady := make(chan struct{})
			var polls atomic.Int32
			client := taskLifecycleClient(t, modern, func(_ http.ResponseWriter, r *http.Request, req *JSONRPCRequest) {
				if req.Method != "tasks/get" || polls.Add(1) != 1 {
					t.Errorf("unexpected request %s", req.Method)
					return
				}
				close(newReady)
				<-r.Context().Done()
			}, WithHTTPBeforeRequest(func(ctx context.Context, _ *http.Request) error {
				if ctx.Value(oldCallKey{}) == true {
					close(oldReady)
					<-releaseOld
					return ctx.Err()
				}
				return nil
			}))
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			oldDone := make(chan error, 1)
			go func() { oldDone <- taskLifecycleCall(client, context.WithValue(ctx, oldCallKey{}, true), "resume") }()
			awaitTaskLifecycle(t, oldReady)
			require.NoError(t, client.Close())
			_, err := client.Initialize(ctx, nil)
			require.NoError(t, err)
			newDone := make(chan error, 1)
			go func() { newDone <- taskLifecycleCall(client, ctx, "resume") }()
			awaitTaskLifecycle(t, newReady)
			unblockOld()
			requireTaskStopped(t, oldDone, "task-1")
			require.NoError(t, client.Close())
			requireTaskStopped(t, newDone, "task-1")
			require.EqualValues(t, 1, polls.Load())
		})
	}
}
