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
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTasksHTTPMutationsDoNotReplay(t *testing.T) {
	for _, transport := range []string{"streamable", "sse"} {
		for _, fault := range []string{"307", "308", "EOF"} {
			for _, method := range []string{MethodToolsCall, "tasks/update", "tasks/cancel"} {
				t.Run(transport+"/"+fault+"/"+method, func(t *testing.T) {
					client, executions := taskHTTPReplayClient(t, transport, fault, method, true)
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_, err := client.Initialize(ctx, nil)
					require.NoError(t, err)
					switch method {
					case MethodToolsCall:
						_, err = client.CallTool(ctx, taskToolRequest())
					case "tasks/update":
						err = client.UpdateTask(ctx, "task-1", map[string]interface{}{"approval": map[string]interface{}{"result": "accepted"}})
					case "tasks/cancel":
						err = client.CancelTask(ctx, "task-1")
					}
					var taskErr *TaskError
					require.ErrorAs(t, err, &taskErr)
					if method == MethodToolsCall {
						require.Empty(t, taskErr.TaskID())
					} else {
						require.Equal(t, "task-1", taskErr.TaskID())
					}
					require.False(t, taskErr.ToolCallReplaySafe())
					require.EqualValues(t, 1, executions.Load())
				})
			}
		}
	}
}

func TestTasksHTTPOrdinaryRedirectUnchanged(t *testing.T) {
	client, executions := taskHTTPReplayClient(t, "streamable", "307", MethodToolsCall, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Initialize(ctx, nil)
	require.NoError(t, err)
	_, err = client.CallTool(ctx, taskToolRequest())
	require.NoError(t, err)
	require.EqualValues(t, 2, executions.Load())
}

func taskHTTPReplayClient(t *testing.T, transport, fault, method string, enabled bool) (*Client, *atomic.Int32) {
	t.Helper()
	executions := &atomic.Int32{}
	messages := make(chan []byte, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			fmt.Fprint(w, "event: endpoint\ndata: /message\n\n")
			flusher.Flush()
			for {
				select {
				case message := <-messages:
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", message)
					flusher.Flush()
				case <-r.Context().Done():
					return
				}
			}
		}
		var request JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Error(err)
			return
		}
		if request.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		reply := func(result interface{}) {
			response := newJSONRPCResponse(request.ID, result)
			if transport == "sse" {
				data, err := json.Marshal(response)
				if err != nil {
					t.Error(err)
					return
				}
				w.WriteHeader(http.StatusAccepted)
				select {
				case messages <- data:
				case <-r.Context().Done():
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Error(err)
			}
		}
		if request.Method == MethodInitialize {
			version := request.Params.(map[string]interface{})["protocolVersion"]
			reply(map[string]interface{}{"protocolVersion": version, "capabilities": map[string]interface{}{},
				"serverInfo": Implementation{Name: "replay-fixture", Version: "1"}})
			return
		}
		if request.Method != method {
			t.Errorf("unexpected method %s", request.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if executions.Add(1) == 1 {
			if fault == "EOF" {
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				_ = connection.Close()
				return
			}
			w.Header().Set("Location", "/redirected")
			if fault == "307" {
				w.WriteHeader(http.StatusTemporaryRedirect)
			} else {
				w.WriteHeader(http.StatusPermanentRedirect)
			}
			return
		}
		reply(map[string]interface{}{"resultType": "complete", "content": []interface{}{}})
	}))
	t.Cleanup(server.Close)
	options := []ClientOption{WithTasks(enabled), WithHTTPBeforeRequest(func(_ context.Context, request *http.Request) error {
		if fault == "EOF" {
			// This header makes an otherwise replayable POST eligible for
			// net/http's transparent retry on a reused connection.
			request.Header.Set("Idempotency-Key", "replay-fixture-key")
		}
		return nil
	})}
	var client *Client
	var err error
	if transport == "sse" {
		client, err = NewSSEClient(server.URL, Implementation{Name: "replay-test", Version: "1"}, options...)
	} else {
		client, err = NewClient(server.URL, Implementation{Name: "replay-test", Version: "1"}, options...)
	}
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client, executions
}
