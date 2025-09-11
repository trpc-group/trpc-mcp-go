// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import "context"

// HandleFunc defines the function signature for a request handler, which is the
// final destination in a middleware chain.
type HandleFunc func(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error)

// MiddlewareFunc defines the function signature for a middleware. It processes a
// request and passes control to the next handler in the chain.
type MiddlewareFunc func(ctx context.Context, req *JSONRPCRequest, session Session, next HandleFunc) (JSONRPCMessage, error)

// MiddlewareChain manages a chain of middlewares.
type MiddlewareChain struct {
	middlewares []MiddlewareFunc
}

// NewMiddlewareChain creates a new MiddlewareChain.
func NewMiddlewareChain(middlewares ...MiddlewareFunc) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: middlewares,

	}
}

// Then chains the middlewares and returns the final HandleFunc.
// The final HandleFunc will be executed at the end of the chain.
func (c *MiddlewareChain) Then(final HandleFunc) HandleFunc {
	// If there are no middlewares, just return the final handler.
	if len(c.middlewares) == 0 {
		return final
	}

	// Start with the final handler as the innermost handler.
	handler := final

	// Wrap the handler with each middleware, starting from the last one.
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		// The current middleware.
		mw := c.middlewares[i]
		// The handler that the current middleware should call next.
		next := handler
		// Create a new handler that wraps the current middleware and the next handler.
		handler = func(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error) {
			return mw(ctx, req, session, next)
		}
	}
	return handler
}
