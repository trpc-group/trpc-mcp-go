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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStatelessSSELineEndingsAndBOM(t *testing.T) {
	for _, delimiter := range []struct {
		name  string
		value string
	}{{"LF", "\n"}, {"CRLF", "\r\n"}, {"CR", "\r"}} {
		for _, bom := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/BOM=%t", delimiter.name, bom), func(t *testing.T) {
				client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					req := modernRequest(t, r)
					switch req.Method {
					case "server/discover":
						modernReply(t, w, req, modernDiscovery())
					case MethodToolsList:
						modernReply(t, w, req, map[string]interface{}{"resultType": "complete", "tools": []interface{}{
							map[string]interface{}{"name": "export", "inputSchema": map[string]interface{}{"type": "object"}},
						}})
					case MethodToolsCall:
						w.Header().Set("Content-Type", "text/event-stream")
						flusher := w.(http.Flusher)
						prefix := ""
						if bom {
							prefix = "\ufeff"
						}
						event := prefix + strings.Join([]string{
							`data: {"jsonrpc":"2.0",`,
							fmt.Sprintf(`data: "id":%v,"result":{"resultType":"complete","content":[{"type":"text","text":"done"}]}}`, req.ID),
							"", "",
						}, delimiter.value)
						// Flush each byte so BOM and CRLF may span response reads.
						for i := 0; i < len(event); i++ {
							if _, err := w.Write([]byte{event[i]}); err != nil {
								return
							}
							flusher.Flush()
						}
						// A complete event must return without EOF or another byte.
						<-r.Context().Done()
					default:
						t.Errorf("unexpected method %s", req.Method)
					}
				})
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err := client.Initialize(ctx, nil)
				require.NoError(t, err)
				request := &CallToolRequest{}
				request.Params.Name = "export"
				result, err := client.CallTool(ctx, request)
				require.NoError(t, err)
				require.Len(t, result.Content, 1)
				require.Equal(t, "done", result.Content[0].(TextContent).Text)
			})
		}
	}
}
