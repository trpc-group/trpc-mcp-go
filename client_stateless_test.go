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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func statelessTestClient(t *testing.T, handler http.HandlerFunc, options ...ClientOption) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	options = append([]ClientOption{WithProtocolVersion(ProtocolVersion_2026_07_28)}, options...)
	client, err := NewClient(server.URL, Implementation{Name: "modern-test", Version: "1"}, options...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func modernRequest(t *testing.T, r *http.Request) *JSONRPCRequest {
	t.Helper()
	require.Equal(t, http.MethodPost, r.Method)
	require.Empty(t, r.Header.Get("Mcp-Session-Id"))
	require.Empty(t, r.Header.Get("Last-Event-ID"))
	require.Equal(t, ProtocolVersion_2026_07_28, r.Header.Get("MCP-Protocol-Version"))
	var req JSONRPCRequest
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&req))
	require.Equal(t, req.Method, r.Header.Get("Mcp-Method"))
	meta := req.Params.(map[string]interface{})["_meta"].(map[string]interface{})
	require.Equal(t, ProtocolVersion_2026_07_28, meta[protocolVersionMeta])
	require.Equal(t, "modern-test", meta[clientInfoMeta].(map[string]interface{})["name"])
	require.Contains(t, meta, clientCapabilitiesMeta)
	return &req
}

func modernReply(t *testing.T, w http.ResponseWriter, req *JSONRPCRequest, result interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(newJSONRPCResponse(req.ID, result)))
}

func modernDiscovery() map[string]interface{} {
	return map[string]interface{}{"resultType": "complete", "supportedVersions": []string{ProtocolVersion_2026_07_28},
		"capabilities": map[string]interface{}{"tools": map[string]interface{}{}},
		"instructions": "use export", "_meta": map[string]interface{}{serverInfoMeta: Implementation{Name: "modern-server", Version: "2"}}}
}

func TestStatelessDiscoveryAndToolHeaders(t *testing.T) {
	var calls atomic.Int32
	client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		req := modernRequest(t, r)
		// Even an erroneous session header must not start legacy session handling.
		w.Header().Set("Mcp-Session-Id", "do-not-use")
		switch req.Method {
		case "server/discover":
			modernReply(t, w, req, modernDiscovery())
		case MethodToolsList:
			modernReply(t, w, req, map[string]interface{}{"resultType": "complete", "tools": []interface{}{
				map[string]interface{}{"name": "export", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{
					"scope": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"tenant": map[string]interface{}{"type": "string", "x-mcp-header": "Tenant"}}},
					"count": map[string]interface{}{"type": "integer", "x-mcp-header": "Count"},
				}}},
			}})
		case MethodToolsCall:
			calls.Add(1)
			require.Equal(t, "export", r.Header.Get("Mcp-Name"))
			require.Equal(t, encodeMCPHeader("租户\n甲"), r.Header.Get("Mcp-Param-Tenant"))
			require.Equal(t, "9007199254740991", r.Header.Get("Mcp-Param-Count"))
			require.Empty(t, r.Header.Get("Mcp-Param-Stale"))
			meta := req.Params.(map[string]interface{})["_meta"].(map[string]interface{})
			require.Equal(t, "trace", meta["progressToken"])
			modernReply(t, w, req, map[string]interface{}{"resultType": "complete", "content": []interface{}{}})
		default:
			t.Errorf("unexpected method %s", req.Method)
		}
	}, WithHTTPBeforeRequest(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Mcp-Method", "wrong")
		req.Header.Set("Mcp-Session-Id", "old")
		req.Header.Set("Last-Event-ID", "old")
		req.Header.Set("Mcp-Param-Stale", "old")
		return nil
	}))
	result, err := client.Initialize(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, "modern-server", result.ServerInfo.Name)
	require.Equal(t, "use export", result.Instructions)
	require.Empty(t, client.GetSessionID())
	req := taskToolRequest()
	req.Params.Name = "export"
	req.Params.Arguments = map[string]interface{}{"scope": map[string]interface{}{"tenant": "租户\n甲"}, "count": json.Number("9007199254740991")}
	req.Params.Meta = &struct {
		ProgressToken ProgressToken `json:"progressToken,omitempty"`
	}{ProgressToken: "trace"}
	_, err = client.CallTool(context.Background(), req)
	require.NoError(t, err)
	require.EqualValues(t, 1, calls.Load())
	require.NotContains(t, req.Params.Arguments, "_meta")
	require.Equal(t, json.Number("9007199254740991"), req.Params.Arguments["count"])
	require.Error(t, client.SendInitialized(context.Background()))
	require.Error(t, client.TerminateSession(context.Background()))
}

func TestStatelessProtocolErrorsDoNotFallback(t *testing.T) {
	for _, status := range []int{400, 404, 403} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				req := modernRequest(t, r)
				calls.Add(1)
				require.Equal(t, "server/discover", req.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(newJSONRPCErrorResponse(req.ID, -32022, "unsupported version", map[string]interface{}{"supported": []string{"2025-03-26"}}))
			})
			_, err := client.Initialize(context.Background(), nil)
			require.ErrorContains(t, err, "-32022")
			require.EqualValues(t, 1, calls.Load())
			require.False(t, client.initialized)
		})
	}
}

func TestStatelessTaskErrorBodyPreserved(t *testing.T) {
	client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		req := modernRequest(t, r)
		if req.Method == "server/discover" {
			modernReply(t, w, req, modernDiscovery())
			return
		}
		require.Equal(t, "tasks/get", req.Method)
		require.Equal(t, encodeMCPHeader("task/租户"), r.Header.Get("Mcp-Name"))
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(newJSONRPCErrorResponse(req.ID, -32602, "task not found", map[string]interface{}{"scope": "identity"}))
	}, WithTasks(true))
	_, err := client.Initialize(context.Background(), nil)
	require.NoError(t, err)
	_, err = client.ResumeToolTask(context.Background(), "task/租户")
	var te *TaskError
	var re *TaskRPCError
	require.ErrorAs(t, err, &te)
	require.Equal(t, "task/租户", te.TaskID())
	require.ErrorAs(t, err, &re)
	require.Equal(t, -32602, re.Code)
	require.JSONEq(t, `{"scope":"identity"}`, string(re.Data))
}

func TestStatelessSSEClosesAfterResult(t *testing.T) {
	closed := make(chan struct{})
	client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		req := modernRequest(t, r)
		if req.Method == "server/discover" {
			modernReply(t, w, req, modernDiscovery())
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "id: never-resume\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\ndata: \"id\":%v,\"result\":{\"resultType\":\"complete\",\"taskId\":\"task-1\",\"status\":\"completed\",\"result\":{\"content\":[]}}}\n\n", req.ID)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	}, WithTasks(true))
	_, err := client.Initialize(context.Background(), nil)
	require.NoError(t, err)
	client.RegisterNotificationHandler("notifications/progress", func(*JSONRPCNotification) error { return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = client.ResumeToolTask(ctx, "task-1")
	require.NoError(t, err)
	select {
	case <-closed:
	case <-ctx.Done():
		t.Fatal("SSE response body was not closed")
	}
	require.Empty(t, client.transport.(*streamableHTTPClientTransport).lastEventID)
}

func TestStatelessConcurrentMetadataDoesNotMutateParams(t *testing.T) {
	c := &Client{protocolVersion: ProtocolVersion_2026_07_28, tasksEnabled: true}
	params := map[string]interface{}{"_meta": map[string]interface{}{"progressToken": "one"}, "arguments": map[string]interface{}{"number": int64(9007199254740991)}}
	req := newJSONRPCRequest(1, MethodToolsCall, params)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := c.statelessRequest(req); assert.NoError(t, err) }()
	}
	wg.Wait()
	require.Len(t, params["_meta"], 1)
}

func TestStatelessRejectsMalformedResponses(t *testing.T) {
	for _, raw := range []string{
		`{"jsonrpc":"2.0","id":2,"result":{"resultType":"complete"}}`,
		`{"jsonrpc":"1.0","id":1,"result":{"resultType":"complete"}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"resultType":"input_required"}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"},"error":{"code":-32000}}`,
	} {
		_, err := statelessResponse([]byte(raw), 1)
		require.Error(t, err)
	}
	raw := json.RawMessage(`{"resultType":"task","taskId":"unexpected"}`)
	_, err := validateStatelessResult(&raw, MethodToolsList, true)
	require.Error(t, err)
	_, err = validateStatelessResult(&raw, MethodToolsCall, false)
	require.Error(t, err)
}

func TestStatelessDiscoveryRejectsUnsupportedVersion(t *testing.T) {
	client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		req := modernRequest(t, r)
		result := modernDiscovery()
		result["supportedVersions"] = []string{ProtocolVersion_2025_03_26}
		modernReply(t, w, req, result)
	})
	_, err := client.Initialize(context.Background(), nil)
	require.ErrorContains(t, err, "does not advertise")
	require.False(t, client.initialized)
}

func TestStatelessDiscoveryRejectsLegacyParameters(t *testing.T) {
	client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid initialization must not send a request")
	})
	request := &InitializeRequest{}
	request.Params.ProtocolVersion = ProtocolVersion_2025_03_26
	_, err := client.Initialize(context.Background(), request)
	require.ErrorContains(t, err, "initialize parameters")
	require.False(t, client.initialized)
	unsupported := &Client{protocolVersion: ProtocolVersion_2026_07_28}
	_, err = unsupported.Initialize(context.Background(), nil)
	require.ErrorContains(t, err, "requires streamable HTTP")
}

func TestStatelessToolListPaginationAndInvalidSchema(t *testing.T) {
	var pages atomic.Int32
	client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		req := modernRequest(t, r)
		if req.Method == "server/discover" {
			modernReply(t, w, req, modernDiscovery())
			return
		}
		require.Equal(t, MethodToolsList, req.Method, "invalid tools must never be submitted")
		pages.Add(1)
		modernReply(t, w, req, map[string]interface{}{"resultType": "complete", "nextCursor": "same", "tools": []interface{}{
			map[string]interface{}{"name": "invalid", "inputSchema": map[string]interface{}{"x-mcp-header": "invalid-root", "type": "string"}},
		}})
	})
	_, err := client.Initialize(context.Background(), nil)
	require.NoError(t, err)
	req := taskToolRequest()
	req.Params.Name = "invalid"
	_, err = client.CallTool(context.Background(), req)
	require.ErrorContains(t, err, "repeated tool-list cursor")
	require.EqualValues(t, 2, pages.Load())
}
