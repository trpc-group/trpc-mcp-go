package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// JSONResponder implements the JSON response handler
type JSONResponder struct {
	// Whether to use stateless mode
	isStateless bool
}

// NewJSONResponder creates a new JSON response handler
func NewJSONResponder(options ...func(*JSONResponder)) *JSONResponder {
	responder := &JSONResponder{
		isStateless: false, // Default to stateful mode
	}

	// Apply options
	for _, option := range options {
		option(responder)
	}

	return responder
}

// WithJSONStatelessMode sets whether to use stateless mode
func WithJSONStatelessMode(isStateless bool) func(*JSONResponder) {
	return func(r *JSONResponder) {
		r.isStateless = isStateless
	}
}

// Respond implements the Responder interface
func (r *JSONResponder) Respond(ctx context.Context, w http.ResponseWriter, req *http.Request, resp interface{}, session *Session) error {
	// Set response headers
	w.Header().Set(ContentTypeHeader, ContentTypeJSON)
	if !r.isStateless && session != nil {
		w.Header().Set(SessionIDHeader, session.ID)
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
func (r *JSONResponder) SupportsContentType(accepts []string) bool {
	return containsContentType(accepts, ContentTypeJSON)
}

// ContainsRequest determines if the request might contain a request (not a notification)
func (r *JSONResponder) ContainsRequest(body []byte) bool {
	// Simple check for the presence of an "id" field
	return strings.Contains(string(body), `"id"`)
}
