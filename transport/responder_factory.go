package transport

import (
	"encoding/json"
	"net/http"
)

// ResponderFactory creates an appropriate response handler
type ResponderFactory struct {
	// Whether to enable SSE
	enableSSE bool

	// Default response mode ("json" or "sse")
	defaultMode string

	// Whether to use stateless mode
	isStateless bool
}

// ResponderFactoryOption represents an option for the responder factory
type ResponderFactoryOption func(*ResponderFactory)

// NewResponderFactory creates a responder factory
func NewResponderFactory(options ...ResponderFactoryOption) *ResponderFactory {
	factory := &ResponderFactory{
		enableSSE:   true,  // Default: SSE enabled
		defaultMode: "sse", // Default: use SSE response
		isStateless: false, // Default: stateful mode
	}

	// Apply options
	for _, option := range options {
		option(factory)
	}

	return factory
}

// WithSSEEnabled sets whether SSE is enabled
func WithSSEEnabled(enabled bool) ResponderFactoryOption {
	return func(f *ResponderFactory) {
		f.enableSSE = enabled
	}
}

// WithDefaultResponseMode sets the default response mode
func WithDefaultResponseMode(mode string) ResponderFactoryOption {
	return func(f *ResponderFactory) {
		if mode == "json" || mode == "sse" {
			f.defaultMode = mode
		}
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
	if !f.enableSSE {
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
			accepts := parseAcceptHeader(req.Header.Get(AcceptHeader))

			// Prefer SSE response mode (if client accepts it and it's the default mode)
			if containsContentType(accepts, ContentTypeSSE) && (f.defaultMode == "sse" || req.Header.Get("Prefer") == "respond-async") {
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
