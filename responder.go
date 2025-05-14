package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	"trpc.group/trpc-go/trpc-mcp-go/internal/httputil"
)

// Responder defines the interface for different response handlers
type Responder interface {
	// Respond to a request
	Respond(ctx context.Context, w http.ResponseWriter, r *http.Request, response interface{}, session Session) error

	// Check if the specified content type is supported
	SupportsContentType(accepts []string) bool

	// Determine if the request potentially contains a request (non-notification)
	ContainsRequest(body []byte) bool
}

// ResponderOption represents an option for a responder
type ResponderOption func(Responder)

// ResponderFactory creates an appropriate response handler
type ResponderFactory struct {
	// Whether to enable SSE
	enablePOSTSSE bool

	// Whether to use stateless mode
	isStateless bool
}

// ResponderFactoryOption represents an option for the responder factory
type ResponderFactoryOption func(*ResponderFactory)

// NewResponderFactory creates a responder factory
func NewResponderFactory(options ...ResponderFactoryOption) *ResponderFactory {
	factory := &ResponderFactory{
		enablePOSTSSE: true,  // Default: SSE enabled
		isStateless:   false, // Default: stateful mode
	}

	// Apply options
	for _, option := range options {
		option(factory)
	}

	return factory
}

// WithResponderSSEEnabled sets whether SSE is enabled
func WithResponderSSEEnabled(enabled bool) ResponderFactoryOption {
	return func(f *ResponderFactory) {
		f.enablePOSTSSE = enabled
	}
}

// WithFactoryStatelessMode sets whether to use stateless mode
func WithFactoryStatelessMode(enabled bool) ResponderFactoryOption {
	return func(f *ResponderFactory) {
		f.isStateless = enabled
	}
}

// CreateResponder creates an appropriate responder based on the request and request body
func (f *ResponderFactory) CreateResponder(req *http.Request, body []byte) Responder {
	// If SSE is not enabled, return a JSON responder
	if !f.enablePOSTSSE {
		return NewJSONResponder(
			WithJSONStatelessMode(f.isStateless),
		)
	}

	// If the request body is an RPC request and SSE is enabled
	var rpcReq struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		ID      interface{} `json:"id"`
	}

	if req != nil && body != nil && len(body) > 0 {
		if err := json.Unmarshal(body, &rpcReq); err == nil && rpcReq.ID != nil {
			// RPC request: check content types accepted by the client
			accepts := httputil.ParseAcceptHeader(req.Header.Get(httputil.AcceptHeader))

			// Prefer SSE response mode
			if httputil.ContainsContentType(accepts, httputil.ContentTypeSSE) && f.enablePOSTSSE {
				return NewSSEResponder(
					WithSSEStatelessMode(f.isStateless),
				)
			}

			// Use JSON response mode
			return NewJSONResponder(
				WithJSONStatelessMode(f.isStateless),
			)
		}
	}

	// Default to JSON responder
	return NewJSONResponder(
		WithJSONStatelessMode(f.isStateless),
	)
}
