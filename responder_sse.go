// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"trpc.group/trpc-go/trpc-mcp-go/internal/httputil"
	"trpc.group/trpc-go/trpc-mcp-go/internal/sseutil"
)

// sseResponder implements the SSE response handler
type sseResponder struct {
	// Current event ID (Note: this field seems unused, consider removal if truly not needed for other logic)
	eventID string

	// Whether to use stateless mode
	isStateless bool

	// SSE utility writer
	sseWriter *sseutil.Writer
}

// newSSEResponder creates a new SSE response handler
func newSSEResponder(options ...func(*sseResponder)) *sseResponder {
	responder := &sseResponder{
		isStateless: false, // Default to stateful mode
		sseWriter:   sseutil.NewWriter(),
	}

	// Apply options
	for _, option := range options {
		option(responder)
	}

	return responder
}

// withSSEStatelessMode sets whether to use stateless mode
func withSSEStatelessMode(isStateless bool) func(*sseResponder) {
	return func(r *sseResponder) {
		r.isStateless = isStateless
	}
}

// withEventID sets the event ID
func withEventID(eventID string) func(*sseResponder) {
	return func(r *sseResponder) {
		if eventID != "" {
			r.eventID = eventID
		}
	}
}

// respond sends an SSE response
func (r *sseResponder) respond(
	ctx context.Context,
	w http.ResponseWriter,
	req *http.Request,
	resp interface{},
	session Session,
) error {
	r.setSSEHeaders(w, session)
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return nil
	}
	respBytes, err := r.marshalResponse(resp)
	if err != nil {
		return err
	}
	return r.sendSSEEvent(w, respBytes)
}

// setSSEHeaders sets standard SSE headers and session header if needed
func (r *sseResponder) setSSEHeaders(w http.ResponseWriter, session Session) {
	sseutil.SetStandardHeaders(w)
	if !r.isStateless && session != nil {
		w.Header().Set(httputil.SessionIDHeader, session.GetID())
	}
}

// marshalResponse serializes the response to JSON
func (r *sseResponder) marshalResponse(resp interface{}) ([]byte, error) {
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseSerialization, err)
	}
	return respBytes, nil
}

// sendSSEEvent sends the SSE event with generated event ID
func (r *sseResponder) sendSSEEvent(w http.ResponseWriter, respBytes []byte) error {
	eventID := r.sseWriter.GenerateEventID()
	return r.sseWriter.WriteEvent(w, sseutil.Event{ID: eventID, Data: respBytes})
}

// supportsContentType checks if the specified content type is supported
func (r *sseResponder) supportsContentType(accepts []string) bool {
	return httputil.ContainsContentType(accepts, httputil.ContentTypeSSE)
}

// containsRequest determines if the request might contain a request (not a notification)
func (r *sseResponder) containsRequest(body []byte) bool {
	// When SSE is supported, we can handle any request containing an "id" field
	return true
}

// sendNotification sends a notification event
// Note: Standard SSE headers should be set by the caller (e.g. handleGet in httpServerHandler)
// if this is used for GET SSE streams.
func (r *sseResponder) sendNotification(w http.ResponseWriter, notification interface{}) (string, error) {
	// Check if it's a response type, which should be sent using the respond method
	if _, ok := notification.(*JSONRPCResponse); ok {
		return "", ErrInvalidResponseType
	}

	// Ensure notification object is a mcp.Notification type with correct jsonrpc field
	var notifBytes []byte
	var err error

	// Try to convert to mcp.Notification to validate format and set JSONRPCVersion
	if n, ok := notification.(*JSONRPCNotification); ok {
		// Ensure jsonrpc field is set correctly
		if n.JSONRPC == "" {
			n.JSONRPC = JSONRPCVersion // JSONRPCVersion is a const in mcp package
		}
		// Serialize notification
		notifBytes, err = json.Marshal(n)
	} else {
		notifBytes, err = json.Marshal(notification)
	}

	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotificationSerialization, err)
	}

	return r.sendSSEMessage(w, notifBytes)
}

// sendSSEMessage sends a message via SSE (common logic for requests and notifications).
func (r *sseResponder) sendSSEMessage(w http.ResponseWriter, messageBytes []byte) (string, error) {
	// Generate event ID
	eventID := r.sseWriter.GenerateEventID()

	err := r.sseWriter.WriteEvent(w, sseutil.Event{ID: eventID, Data: messageBytes})
	if err != nil {
		return "", err
	}

	return eventID, nil
}

// sendRequest sends a JSON-RPC request via SSE.
func (r *sseResponder) sendRequest(w http.ResponseWriter, request *JSONRPCRequest) (string, error) {
	// Ensure request has proper jsonrpc field.
	if request.JSONRPC == "" {
		request.JSONRPC = JSONRPCVersion
	}

	// Serialize request.
	reqBytes, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to serialize request: %w", err)
	}

	return r.sendSSEMessage(w, reqBytes)
}

// Generate the next event ID - This method is now effectively a proxy to sseWriter.
func (r *sseResponder) nextEventID() string {
	return r.sseWriter.GenerateEventID()
}
