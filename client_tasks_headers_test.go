// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent. All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTasksRoutingHeadersEncodeTaskID(t *testing.T) {
	for _, protocol := range []string{ProtocolVersion_2025_03_26, ProtocolVersion_2026_07_28} {
		for _, tc := range []struct {
			name   string
			id     string
			header string
		}{
			{"ascii", "task-1", "task-1"},
			{"unicode", "task/租户", "=?base64?dGFzay/np5/miLc=?="},
			{"newline", "task\n1", "=?base64?dGFzawox?="},
			{"padded", " task-1 ", "=?base64?IHRhc2stMSA=?="},
			{"sentinel", "=?base64?literal?=", "=?base64?PT9iYXNlNjQ/bGl0ZXJhbD89?="},
		} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				methods := make(chan string, 3)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req JSONRPCRequest
					require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
					if req.Method == "notifications/initialized" {
						w.WriteHeader(http.StatusAccepted)
						return
					}
					var result interface{}
					switch req.Method {
					case MethodInitialize:
						require.Equal(t, ProtocolVersion_2025_03_26, protocol)
						result = map[string]interface{}{"protocolVersion": protocol, "capabilities": map[string]interface{}{},
							"serverInfo": Implementation{Name: "task-headers", Version: "1"}}
					case "server/discover":
						require.Equal(t, ProtocolVersion_2026_07_28, protocol)
						result = modernDiscovery()
					default:
						params := req.Params.(map[string]interface{})
						require.Equal(t, tc.id, params["taskId"])
						require.Equal(t, tc.header, r.Header.Get("Mcp-Name"))
						require.Equal(t, req.Method, r.Header.Get("Mcp-Method"))
						methods <- req.Method
						result = map[string]interface{}{"resultType": "complete"}
						if req.Method == "tasks/get" {
							result = map[string]interface{}{"resultType": "complete", "taskId": tc.id, "status": "completed",
								"result": map[string]interface{}{"content": []interface{}{}}}
						}
					}
					w.Header().Set("Content-Type", "application/json")
					require.NoError(t, json.NewEncoder(w).Encode(newJSONRPCResponse(req.ID, result)))
				}))
				defer server.Close()
				client, err := NewClient(server.URL, Implementation{Name: "task-headers", Version: "1"},
					WithProtocolVersion(protocol), WithTasks(true), WithClientGetSSEEnabled(false))
				require.NoError(t, err)
				defer client.Close()
				_, err = client.Initialize(context.Background(), nil)
				require.NoError(t, err)
				_, err = client.ResumeToolTask(context.Background(), tc.id)
				require.NoError(t, err)
				require.NoError(t, client.UpdateTask(context.Background(), tc.id, map[string]interface{}{}))
				require.NoError(t, client.CancelTask(context.Background(), tc.id))
				received := make(map[string]int)
				for i := 0; i < 3; i++ {
					received[<-methods]++
				}
				require.Equal(t, map[string]int{"tasks/get": 1, "tasks/update": 1, "tasks/cancel": 1}, received)
			})
		}
	}
}
