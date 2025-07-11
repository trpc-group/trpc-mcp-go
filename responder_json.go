// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"trpc.group/trpc-go/trpc-mcp-go/internal/httputil"
)

// jsonResponder implements the JSON response handler
type jsonResponder struct {
	// Whether to use stateless mode
	isStateless bool
}

// newJSONResponder creates a new JSON response handler
func newJSONResponder(options ...func(*jsonResponder)) *jsonResponder {
	responder := &jsonResponder{
		isStateless: false, // Default to stateful mode
	}

	// Apply options
	for _, option := range options {
		option(responder)
	}

	return responder
}

// withJSONStatelessMode sets whether to use stateless mode
func withJSONStatelessMode(isStateless bool) func(*jsonResponder) {
	return func(r *jsonResponder) {
		r.isStateless = isStateless
	}
}

// Respond implements the responder interface
func (r *jsonResponder) respond(ctx context.Context, w http.ResponseWriter, req *http.Request, resp interface{}, session Session) error {
	// Set response headers
	w.Header().Set(httputil.ContentTypeHeader, httputil.ContentTypeJSON)
	if !r.isStateless && session != nil {
		w.Header().Set(httputil.SessionIDHeader, session.GetID())
	}

	// Set status code
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return nil
	}

	// Set status code and encode response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return err
	}

	return nil
}

// SupportsContentType checks if the specified content type is supported
func (r *jsonResponder) supportsContentType(accepts []string) bool {
	return httputil.ContainsContentType(accepts, httputil.ContentTypeJSON)
}

// ContainsRequest determines if the request might contain a request (not a notification)
func (r *jsonResponder) containsRequest(body []byte) bool {
	// Simple check for the presence of an "id" field
	return strings.Contains(string(body), `"id"`)
}
