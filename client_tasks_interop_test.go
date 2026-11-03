//go:build mcptasksinterop

// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent. All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// TestTasksMCPKitInterop uses the independently maintained Tasks v2 example.
// See TASKS.md for the pinned server revision and reproduction commands.
func TestTasksMCPKitInterop(t *testing.T) {
	endpoint := os.Getenv("MCP_TASKS_INTEROP_URL")
	if endpoint == "" {
		t.Skip("set MCP_TASKS_INTEROP_URL to the mcpkit Tasks v2 dual-mode endpoint")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	modern := os.Getenv("MCP_TASKS_INTEROP_PROTOCOL") == mcp.ProtocolVersion_2026_07_28
	connect := func() *mcp.Client {
		options := []mcp.ClientOption{mcp.WithTasks(true), mcp.WithSendEmptyToolArguments(true), mcp.WithClientGetSSEEnabled(false)}
		if modern {
			options = append(options, mcp.WithProtocolVersion(mcp.ProtocolVersion_2026_07_28))
		}
		client, err := mcp.NewClient(endpoint, mcp.Implementation{Name: "trpc-interop", Version: "1"},
			options...)
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })
		_, err = client.Initialize(ctx, nil)
		require.NoError(t, err)
		return client
	}
	client := connect()
	call := func(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
		req := &mcp.CallToolRequest{}
		req.Params.Name, req.Params.Arguments = name, args
		return client.CallTool(ctx, req)
	}
	result, err := call(ctx, "greet", map[string]interface{}{"name": "interop"})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	result, err = call(ctx, "slow_compute", map[string]interface{}{"seconds": 1})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	result, err = call(ctx, "failing_job", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	_, err = call(ctx, "protocol_error_job", nil)
	var taskErr *mcp.TaskError
	var rpcErr *mcp.TaskRPCError
	require.ErrorAs(t, err, &taskErr)
	require.Equal(t, "failed", taskErr.Status)
	require.NotEmpty(t, taskErr.TaskID())
	require.ErrorAs(t, err, &rpcErr)

	// The example simulates deletion; explicitly decline the synthetic request.
	_, err = call(ctx, "confirm_delete", map[string]interface{}{"filename": "interop-example.txt"})
	require.ErrorAs(t, err, &taskErr)
	require.Equal(t, "input_required", taskErr.Status)
	require.NotEmpty(t, taskErr.InputRequests)
	responses := make(map[string]interface{})
	for id, raw := range taskErr.InputRequests {
		var request struct {
			Method string `json:"method"`
		}
		require.NoError(t, json.Unmarshal(raw, &request))
		require.Equal(t, "elicitation/create", request.Method)
		responses[id] = map[string]interface{}{"result": map[string]interface{}{"action": "decline"}}
	}
	inputTaskID := taskErr.TaskID()
	require.NoError(t, client.UpdateTask(ctx, inputTaskID, responses))
	result, err = client.ResumeToolTask(ctx, inputTaskID)
	require.NoError(t, err)
	require.Contains(t, fmt.Sprint(result.Content), "kept")

	waitCtx, stop := context.WithTimeout(ctx, 150*time.Millisecond)
	_, err = call(waitCtx, "slow_compute", map[string]interface{}{"seconds": 3})
	stop()
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorAs(t, err, &taskErr)
	id := taskErr.TaskID()
	require.NotEmpty(t, id)
	// This legacy server binds tasks to the session. A new session cannot resume
	// them, even on the same endpoint; failed recovery must not recreate work.
	_, err = connect().ResumeToolTask(ctx, id)
	if modern {
		require.NoError(t, err)
	} else {
		require.ErrorAs(t, err, &rpcErr)
		require.Equal(t, -32602, rpcErr.Code)
	}
	result, err = client.ResumeToolTask(ctx, id)
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)

	waitCtx, stop = context.WithTimeout(ctx, 150*time.Millisecond)
	_, err = call(waitCtx, "slow_compute", map[string]interface{}{"seconds": 10})
	stop()
	require.ErrorAs(t, err, &taskErr)
	require.NotEmpty(t, taskErr.TaskID())
	require.NoError(t, client.CancelTask(ctx, taskErr.TaskID()))
	_, err = client.ResumeToolTask(ctx, taskErr.TaskID())
	require.ErrorAs(t, err, &taskErr)
	require.Equal(t, "cancelled", taskErr.Status)
}
