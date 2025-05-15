package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	"trpc.group/trpc-go/trpc-mcp-go/internal/httputil"
)

// responder defines the interface for different response handlers
type responder interface {
	// Respond to a request
	respond(ctx context.Context, w http.ResponseWriter, r *http.Request, response interface{}, session Session) error

	// Check if the specified content type is supported
	supportsContentType(accepts []string) bool

	// Determine if the request potentially contains a request (non-notification)
	containsRequest(body []byte) bool
}

// responderOption represents an option for a responder
type responderOption func(responder)

// responderFactory creates an appropriate response handler
type responderFactory struct {
	// Whether to enable SSE
	enablePOSTSSE bool

	// Whether to use stateless mode
	isStateless bool
}

// responderFactoryOption represents an option for the responder factory
type responderFactoryOption func(*responderFactory)

// newResponderFactory creates a responder factory
func newResponderFactory(options ...responderFactoryOption) *responderFactory {
	factory := &responderFactory{
		enablePOSTSSE: true,  // Default: SSE enabled
		isStateless:   false, // Default: stateful mode
	}

	// Apply options
	for _, option := range options {
		option(factory)
	}

	return factory
}

// withResponderSSEEnabled sets whether SSE is enabled
func withResponderSSEEnabled(enabled bool) responderFactoryOption {
	return func(f *responderFactory) {
		f.enablePOSTSSE = enabled
	}
}

// withFactoryStatelessMode sets whether to use stateless mode
func withFactoryStatelessMode(enabled bool) responderFactoryOption {
	return func(f *responderFactory) {
		f.isStateless = enabled
	}
}

// createResponder creates an appropriate responder based on the request and request body
func (f *responderFactory) createResponder(req *http.Request, body []byte) responder {
	shouldUseSSE := false

	// Check if conditions are met to use SSE responder
	if f.enablePOSTSSE && req != nil && body != nil && len(body) > 0 {
		var rpcReq struct {
			JSONRPC string      `json:"jsonrpc"`
			Method  string      `json:"method"`
			ID      interface{} `json:"id"`
		}
		// Try to parse as an RPC request with an ID
		if err := json.Unmarshal(body, &rpcReq); err == nil && rpcReq.ID != nil {
			// RPC request: check content types accepted by the client
			accepts := httputil.ParseAcceptHeader(req.Header.Get(httputil.AcceptHeader))
			if httputil.ContainsContentType(accepts, httputil.ContentTypeSSE) {
				shouldUseSSE = true
			}
		}
	}

	if shouldUseSSE {
		return newSSEResponder(
			withSSEStatelessMode(f.isStateless),
		)
	}
	// Default to JSON responder if SSE conditions are not met
	return newJSONResponder(
		withJSONStatelessMode(f.isStateless),
	)
}
