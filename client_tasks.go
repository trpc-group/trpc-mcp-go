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
	"time"
)

const tasksExtension = "io.modelcontextprotocol/tasks"

type taskRequestKey struct{}

// WithTasks enables experimental SEP-2663 task polling for CallTool. It preserves
// the Connector interface and is disabled by default. The endpoint must support
// the selected protocol and the Tasks extension. For stateless Streamable HTTP,
// also select WithProtocolVersion(ProtocolVersion_2026_07_28). Calls should have a
// context deadline. Cancellation stops waiting, not remote execution.
func WithTasks(enabled bool) ClientOption {
	return func(c *Client) { c.tasksEnabled = enabled }
}

// TasksEnabled reports whether CallTool may wait for a task. Adapters can use
// this capability to apply an overall task deadline across connection recovery
// without changing the timeout policy for ordinary clients.
func (c *Client) TasksEnabled() bool { return c.tasksEnabled }

// TaskError preserves the handle of an interrupted tool task. TaskID is empty
// when submission may have succeeded but no handle was received. Neither case
// permits replaying the original tool. ResumeToolTask may resume a nonempty
// handle on the same endpoint under the same authorization identity.
// Legacy servers may additionally bind handles to the original session.
// Status and InputRequests are snapshots and are empty when unavailable.
type TaskError struct {
	ID            string
	Status        string
	InputRequests map[string]json.RawMessage
	Cause         error
}

// Error describes the interrupted task and its cause.
func (e *TaskError) Error() string {
	return fmt.Sprintf("tool task %q (%s): %v", e.ID, e.Status, e.Cause)
}

// Unwrap returns the underlying failure, including context cancellation.
func (e *TaskError) Unwrap() error { return e.Cause }

// TaskID returns the task handle, or an empty string for an unknown submission outcome.
func (e *TaskError) TaskID() string { return e.ID }

// ToolCallReplaySafe reports whether the original tools/call can be replayed.
func (e *TaskError) ToolCallReplaySafe() bool { return false }

// TaskRPCError retains a failed task's JSON-RPC error code and data.
type TaskRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error describes the task's JSON-RPC failure.
func (e *TaskRPCError) Error() string { return fmt.Sprintf("task rpc error %d: %s", e.Code, e.Message) }

type taskState struct {
	ResultType   string                     `json:"resultType"`
	ID           string                     `json:"taskId"`
	Status       string                     `json:"status"`
	PollInterval *int64                     `json:"pollIntervalMs"`
	Result       json.RawMessage            `json:"result"`
	Error        *TaskRPCError              `json:"error"`
	Inputs       map[string]json.RawMessage `json:"inputRequests"`
}

func taskMeta() map[string]interface{} {
	return map[string]interface{}{"io.modelcontextprotocol/clientCapabilities": map[string]interface{}{
		"extensions": map[string]interface{}{tasksExtension: map[string]interface{}{}}}}
}

func (c *Client) callToolTask(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
	if req == nil {
		return nil, fmt.Errorf("nil tool request")
	}
	params := map[string]interface{}{"name": req.Params.Name}
	if len(req.Params.Arguments) > 0 {
		params["arguments"] = req.Params.Arguments
	} else if c.sendEmptyToolArguments {
		params["arguments"] = map[string]interface{}{}
	}
	meta := taskMeta()
	if req.Params.Meta != nil {
		meta["progressToken"] = req.Params.Meta.ProgressToken
	}
	params["_meta"] = meta
	// Suppress retries per request, without mutating shared transport configuration.
	submitCtx := context.WithValue(ctx, taskRequestKey{}, true)
	raw, err := c.taskRequest(submitCtx, MethodToolsCall, params)
	if err != nil {
		return nil, &TaskError{Cause: err}
	}
	var state taskState
	if err := json.Unmarshal(*raw, &state); err != nil {
		return nil, &TaskError{Cause: err}
	}
	switch state.ResultType {
	case "", "complete":
		result, err := parseCallToolResult(raw)
		if err != nil {
			return nil, &TaskError{Cause: err}
		}
		return result, nil
	case "task":
		if state.ID == "" {
			return nil, &TaskError{Cause: errors.New("task response has no taskId")}
		}
		// Creation returns Task, not DetailedTask: always retrieve terminal payloads.
		return c.pollToolTask(ctx, state.ID, state.PollInterval)
	default:
		return nil, &TaskError{Cause: fmt.Errorf("unsupported tool resultType %q", state.ResultType)}
	}
}

// ResumeToolTask waits for an existing tool task without invoking tools/call.
// The caller must bind taskID to the original endpoint and authorization identity.
// A legacy server may also require the original session; an expired or inaccessible
// task is returned as an error and never recreated by invoking the tool again.
// Input requests are returned in TaskError for explicit application handling;
// the SDK never fabricates approval or executes sampling requests automatically.
func (c *Client) ResumeToolTask(ctx context.Context, taskID string) (*CallToolResult, error) {
	if !c.tasksEnabled || !c.initialized {
		return nil, &TaskError{ID: taskID, Cause: errors.New("tasks require an initialized, tasks-enabled client")}
	}
	if taskID == "" {
		return nil, errors.New("empty task id")
	}
	return c.pollToolTask(ctx, taskID, nil)
}

func (c *Client) pollToolTask(ctx context.Context, id string, interval *int64) (*CallToolResult, error) {
	// Resume polls immediately. Subsequent polls honor server pacing; zero is
	// clamped to prevent busy loops. Context remains the lifetime authority.
	delay := time.Duration(0)
	if interval != nil {
		delay = taskPollDelay(interval)
	}
	for {
		if err := waitTask(ctx, delay); err != nil {
			return nil, &TaskError{ID: id, Cause: err}
		}
		raw, err := c.taskRequest(ctx, "tasks/get", map[string]interface{}{"taskId": id, "_meta": taskMeta()})
		if err != nil {
			return nil, &TaskError{ID: id, Cause: err}
		}
		var state taskState
		if err := json.Unmarshal(*raw, &state); err != nil {
			return nil, &TaskError{ID: id, Cause: err}
		}
		fail := func(err error) (*CallToolResult, error) {
			return nil, &TaskError{ID: id, Status: state.Status, InputRequests: state.Inputs, Cause: err}
		}
		if state.ID != id || state.ResultType != "complete" {
			return fail(errors.New("invalid task response identity or resultType"))
		}
		switch state.Status {
		case "completed":
			result, err := parseCallToolResult(&state.Result)
			if err != nil {
				return fail(err)
			}
			return result, nil
		case "failed":
			if state.Error == nil {
				return fail(errors.New("failed task has no rpc error"))
			}
			return fail(state.Error)
		case "cancelled":
			return fail(errors.New("remote task cancelled"))
		case "input_required":
			return fail(errors.New("task requires explicit application input"))
		case "working":
			delay = taskPollDelay(state.PollInterval)
		default:
			return fail(fmt.Errorf("invalid task status %q", state.Status))
		}
	}
}

func taskPollDelay(ms *int64) time.Duration {
	if ms == nil {
		return time.Second
	}
	if *ms <= 0 {
		return 10 * time.Millisecond
	}
	// Saturate rather than overflow a time.Duration into a tight loop.
	if *ms > int64((1<<63-1)/time.Millisecond) {
		return time.Duration(1<<63 - 1)
	}
	return time.Duration(*ms) * time.Millisecond
}
func waitTask(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if delay == 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// UpdateTask supplies application-approved input responses keyed by request ID.
// Responses must obey the server request schema. The caller owns authorization
// and deduplication, including after a lost acknowledgement. No automatic retry
// occurs. A successful acknowledgement does not imply task completion.
func (c *Client) UpdateTask(ctx context.Context, taskID string, responses map[string]interface{}) error {
	return c.taskAction(ctx, "tasks/update", taskID, map[string]interface{}{"inputResponses": responses})
}

// CancelTask requests remote cancellation. Success acknowledges the request only;
// use ResumeToolTask to observe terminal state. Cancelling a CallTool context
// does not call this method. Use a live, bounded context for explicit cleanup.
func (c *Client) CancelTask(ctx context.Context, taskID string) error {
	return c.taskAction(ctx, "tasks/cancel", taskID, map[string]interface{}{})
}
func (c *Client) taskAction(ctx context.Context, method, id string, params map[string]interface{}) error {
	if !c.tasksEnabled || !c.initialized || id == "" {
		return errors.New("task action requires an initialized, tasks-enabled client and task id")
	}
	params["taskId"], params["_meta"] = id, taskMeta()
	raw, err := c.taskRequest(context.WithValue(ctx, taskRequestKey{}, true), method, params)
	if err != nil {
		return &TaskError{ID: id, Cause: err}
	}
	var ack struct {
		ResultType string `json:"resultType"`
	}
	if err := json.Unmarshal(*raw, &ack); err != nil {
		return &TaskError{ID: id, Cause: err}
	}
	if ack.ResultType != "complete" {
		return &TaskError{ID: id, Cause: errors.New("invalid task acknowledgement")}
	}
	return nil
}
func (c *Client) taskRequest(ctx context.Context, method string, params map[string]interface{}) (*json.RawMessage, error) {
	raw, err := c.transport.sendRequest(ctx, newJSONRPCRequest(c.requestID.Add(1), method, params))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	if raw == nil {
		return nil, errors.New("empty task response")
	}
	// Transports unwrap successful results, but retain JSON-RPC error envelopes.
	// A failed DetailedTask also contains "error"; it must reach pollToolTask so
	// its identity and status are validated rather than mistaken for an envelope.
	var envelope struct {
		JSONRPC string        `json:"jsonrpc"`
		Error   *TaskRPCError `json:"error"`
	}
	if err := json.Unmarshal(*raw, &envelope); err != nil {
		return nil, err
	}
	if envelope.JSONRPC == "2.0" && envelope.Error != nil {
		return nil, envelope.Error
	}
	return raw, nil
}
