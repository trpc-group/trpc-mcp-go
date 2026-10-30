// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent. All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatelessToolHeadersFollowAuthorization(t *testing.T) {
	for _, tasks := range []bool{false, true} {
		t.Run(fmt.Sprintf("tasks=%t", tasks), func(t *testing.T) {
			type identityKey struct{}
			var listings atomic.Int32
			var calls atomic.Int32
			client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				req := modernRequest(t, r)
				identity := r.Header.Get("Authorization")
				switch req.Method {
				case "server/discover":
					modernReply(t, w, req, modernDiscovery())
				case MethodToolsList:
					listings.Add(1)
					parameter := map[string]interface{}{"type": "string"}
					if identity == "Bearer alice" {
						parameter["x-mcp-header"] = "AliceSecret"
					}
					modernReply(t, w, req, map[string]interface{}{
						"resultType": "complete", "cacheScope": "private", "ttlMs": 60000,
						"tools": []interface{}{map[string]interface{}{
							"name": "export", "inputSchema": map[string]interface{}{
								"type": "object", "properties": map[string]interface{}{"password": parameter},
							},
						}},
					})
				case MethodToolsCall:
					calls.Add(1)
					if identity == "Bearer alice" {
						assert.Equal(t, "alice-password", r.Header.Get("Mcp-Param-AliceSecret"))
					} else {
						assert.Equal(t, "Bearer bob", identity)
						assert.Empty(t, r.Header.Get("Mcp-Param-AliceSecret"))
					}
					modernReply(t, w, req, map[string]interface{}{"resultType": "complete", "content": []interface{}{}})
				default:
					t.Errorf("unexpected method %s", req.Method)
				}
			}, WithTasks(tasks), WithHTTPBeforeRequest(func(ctx context.Context, req *http.Request) error {
				req.Header.Set("Authorization", "Bearer "+ctx.Value(identityKey{}).(string))
				return nil
			}))
			alice := context.WithValue(context.Background(), identityKey{}, "alice")
			bob := context.WithValue(context.Background(), identityKey{}, "bob")
			_, err := client.Initialize(alice, nil)
			require.NoError(t, err)
			_, err = client.ListTools(alice, &ListToolsRequest{})
			require.NoError(t, err)
			call := func(ctx context.Context, identity string) {
				req := taskToolRequest()
				req.Params.Name = "export"
				req.Params.Arguments = map[string]interface{}{"password": identity + "-password"}
				_, err := client.CallTool(ctx, req)
				assert.NoError(t, err)
			}
			call(bob, "bob")
			call(alice, "alice")
			var wg sync.WaitGroup
			for i := 0; i < 12; i++ {
				wg.Add(2)
				go func() { defer wg.Done(); call(alice, "alice") }()
				go func() { defer wg.Done(); call(bob, "bob") }()
			}
			wg.Wait()
			require.EqualValues(t, 26, calls.Load())
			require.EqualValues(t, 27, listings.Load())
		})
	}
}

func TestStatelessToolHeadersRefreshChangedAndRemovedTools(t *testing.T) {
	var version atomic.Int32
	var calls atomic.Int32
	var listings atomic.Int32
	client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		req := modernRequest(t, r)
		switch req.Method {
		case "server/discover":
			modernReply(t, w, req, modernDiscovery())
		case MethodToolsList:
			listings.Add(1)
			tools := []interface{}{}
			if version.Load() < 2 {
				tools = append(tools, map[string]interface{}{
					"name": "export", "inputSchema": map[string]interface{}{
						"type": "object", "properties": map[string]interface{}{
							"scope": map[string]interface{}{"type": "string", "x-mcp-header": fmt.Sprintf("Scope%d", version.Load())},
						},
					},
				})
			}
			modernReply(t, w, req, map[string]interface{}{
				"resultType": "complete", "cacheScope": "public", "ttlMs": 0, "tools": tools,
			})
		case MethodToolsCall:
			calls.Add(1)
			if version.Load() == 0 {
				assert.Equal(t, "tenant", r.Header.Get("Mcp-Param-Scope0"))
				assert.Empty(t, r.Header.Get("Mcp-Param-Scope1"))
			} else {
				assert.Equal(t, "tenant", r.Header.Get("Mcp-Param-Scope1"))
				assert.Empty(t, r.Header.Get("Mcp-Param-Scope0"))
			}
			modernReply(t, w, req, map[string]interface{}{"resultType": "complete", "content": []interface{}{}})
		default:
			t.Errorf("unexpected method %s", req.Method)
		}
	})
	_, err := client.Initialize(context.Background(), nil)
	require.NoError(t, err)
	req := taskToolRequest()
	req.Params.Name = "export"
	req.Params.Arguments = map[string]interface{}{"scope": "tenant"}
	_, err = client.CallTool(context.Background(), req)
	require.NoError(t, err)
	version.Store(1)
	_, err = client.CallTool(context.Background(), req)
	require.NoError(t, err)
	version.Store(2)
	_, err = client.CallTool(context.Background(), req)
	require.ErrorContains(t, err, "no usable definition")
	require.EqualValues(t, 2, calls.Load())
	require.EqualValues(t, 3, listings.Load())
}

func TestStatelessToolHeadersFindTargetOnLaterPage(t *testing.T) {
	var pages atomic.Int32
	client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		req := modernRequest(t, r)
		switch req.Method {
		case "server/discover":
			modernReply(t, w, req, modernDiscovery())
		case MethodToolsList:
			pages.Add(1)
			params := req.Params.(map[string]interface{})
			if params["cursor"] == nil {
				modernReply(t, w, req, map[string]interface{}{
					"resultType": "complete", "nextCursor": "page-two", "tools": []interface{}{},
				})
				return
			}
			require.Equal(t, "page-two", params["cursor"])
			modernReply(t, w, req, map[string]interface{}{
				"resultType": "complete", "tools": []interface{}{map[string]interface{}{
					"name": "export", "inputSchema": map[string]interface{}{
						"type": "object", "properties": map[string]interface{}{
							"scope": map[string]interface{}{"type": "string", "x-mcp-header": "Scope"},
						},
					},
				}},
			})
		case MethodToolsCall:
			assert.Equal(t, "tenant", r.Header.Get("Mcp-Param-Scope"))
			modernReply(t, w, req, map[string]interface{}{"resultType": "complete", "content": []interface{}{}})
		default:
			t.Errorf("unexpected method %s", req.Method)
		}
	})
	_, err := client.Initialize(context.Background(), nil)
	require.NoError(t, err)
	req := taskToolRequest()
	req.Params.Name = "export"
	req.Params.Arguments = map[string]interface{}{"scope": "tenant"}
	_, err = client.CallTool(context.Background(), req)
	require.NoError(t, err)
	require.EqualValues(t, 2, pages.Load())
}
