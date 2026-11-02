// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent. All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatelessHTTPResponseErrorValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		valid  bool
	}{
		{"matching-error", 200, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"rejected"}}`, true},
		{"idless-rejection", 403, `{"jsonrpc":"2.0","error":{"code":-32000,"message":"rejected"}}`, true},
		{"wrong-id", 403, `{"jsonrpc":"2.0","id":2,"error":{"code":-32000,"message":"rejected"}}`, false},
		{"null-id-rejection", 403, `{"jsonrpc":"2.0","id":null,"error":{"code":-32000,"message":"rejected"}}`, true},
		{"null-id-parse-error", 400, `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"parse error"}}`, true},
		{"null-id-200", 200, `{"jsonrpc":"2.0","id":null,"error":{"code":-32000,"message":"rejected"}}`, false},
		{"null-id-201", 201, `{"jsonrpc":"2.0","id":null,"error":{"code":-32000,"message":"rejected"}}`, false},
		{"idless-200", 200, `{"jsonrpc":"2.0","error":{"code":-32000,"message":"rejected"}}`, false},
		{"idless-201", 201, `{"jsonrpc":"2.0","error":{"code":-32000,"message":"rejected"}}`, false},
		{"idless-202", 202, `{"jsonrpc":"2.0","error":{"code":-32000,"message":"rejected"}}`, false},
		{"wrong-version", 403, `{"jsonrpc":"1.0","error":{"code":-32000,"message":"rejected"}}`, false},
		{"missing-version", 403, `{"error":{"code":-32000,"message":"rejected"}}`, false},
		{"missing-error-code", 403, `{"jsonrpc":"2.0","error":{"message":"rejected"}}`, false},
		{"missing-error-message", 403, `{"jsonrpc":"2.0","error":{"code":-32000}}`, false},
		{"null-error-code", 403, `{"jsonrpc":"2.0","error":{"code":null,"message":"rejected"}}`, false},
		{"null-error-message", 403, `{"jsonrpc":"2.0","error":{"code":-32000,"message":null}}`, false},
		{"fractional-error-code", 403, `{"jsonrpc":"2.0","error":{"code":1.5,"message":"rejected"}}`, false},
		{"string-error-code", 403, `{"jsonrpc":"2.0","error":{"code":"-32000","message":"rejected"}}`, false},
		{"numeric-error-message", 403, `{"jsonrpc":"2.0","error":{"code":-32000,"message":1}}`, false},
		{"empty-error", 403, `{"jsonrpc":"2.0","error":{}}`, false},
		{"null-error", 403, `{"jsonrpc":"2.0","error":null}`, false},
		{"array-error", 403, `{"jsonrpc":"2.0","error":[]}`, false},
		{"result-and-error", 403, `{"jsonrpc":"2.0","result":{"resultType":"complete"},"error":{"code":-32000,"message":"rejected"}}`, false},
		{"null-result-and-error", 403, `{"jsonrpc":"2.0","result":null,"error":{"code":-32000,"message":"rejected"}}`, false},
		{"result-and-null-error", 200, `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"},"error":null}`, false},
		{"idless-result", 403, `{"jsonrpc":"2.0","result":{"resultType":"complete"}}`, false},
		{"null-id-result", 403, `{"jsonrpc":"2.0","id":null,"result":{"resultType":"complete"}}`, false},
		{"null-id-success", 200, `{"jsonrpc":"2.0","id":null,"result":{"resultType":"complete"}}`, false},
		{"malformed-json", 403, `{"jsonrpc":"2.0","error":`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := statelessHTTPResponse([]byte(tc.body), 1, tc.status)
			if !tc.valid {
				require.Error(t, err)
				require.Nil(t, raw)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tc.body, string(*raw))
		})
	}
}

func TestStatelessHTTPErrorIDHandling(t *testing.T) {
	for _, status := range []int{200, 201, 202, 400, 403, 404, 500} {
		for _, idField := range []string{"", `"id":null,`} {
			t.Run(fmt.Sprintf("%d/%s", status, idField), func(t *testing.T) {
				client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					req := modernRequest(t, r)
					if req.Method == "server/discover" {
						modernReply(t, w, req, modernDiscovery())
						return
					}
					require.Equal(t, "tasks/get", req.Method)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					fmt.Fprintf(w, `{"jsonrpc":"2.0",%s"error":{"code":-32000,"message":"request rejected","data":{"reason":"origin"}}}`, idField)
				}, WithTasks(true))
				_, err := client.Initialize(context.Background(), nil)
				require.NoError(t, err)
				_, err = client.ResumeToolTask(context.Background(), "existing-task")
				var taskErr *TaskError
				var rpcErr *TaskRPCError
				require.ErrorAs(t, err, &taskErr)
				require.Equal(t, "existing-task", taskErr.TaskID())
				if status < 300 {
					require.False(t, errors.As(err, &rpcErr))
					return
				}
				require.ErrorAs(t, err, &rpcErr)
				require.Equal(t, -32000, rpcErr.Code)
				require.Equal(t, "request rejected", rpcErr.Message)
				require.JSONEq(t, `{"reason":"origin"}`, string(rpcErr.Data))
			})
		}
	}
}

func TestStatelessStrictResponsesRejectUnknownID(t *testing.T) {
	for _, idField := range []string{"", `"id":null,`} {
		t.Run(idField, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"jsonrpc":"2.0",%s"error":{"code":-32000,"message":"rejected"}}`, idField))
			_, err := statelessResponse(body, 1)
			require.Error(t, err)

			client := statelessTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				req := modernRequest(t, r)
				if req.Method == "server/discover" {
					modernReply(t, w, req, modernDiscovery())
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\n", body)
			}, WithTasks(true))
			_, err = client.Initialize(context.Background(), nil)
			require.NoError(t, err)
			_, err = client.ResumeToolTask(context.Background(), "existing-task")
			require.Error(t, err)
			var rpcErr *TaskRPCError
			require.False(t, errors.As(err, &rpcErr))
		})
	}
}
