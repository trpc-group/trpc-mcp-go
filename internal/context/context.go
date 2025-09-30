// Package context provides additional context-related functionality
// that is not available in the standard context package.
package context

import (
	"context"
	"time"
)

// WithoutCancel creates a context that inherits values from the parent context
// but ignores cancellation. This is a Go 1.20 compatible implementation of
// context.WithoutCancel (introduced in Go 1.21).
//
// The returned context preserves all values from the parent context but ignores
// cancellation signals. This implementation uses the standard library pattern
// of wrapping the parent context rather than creating a new background context.
//
// Use cases:
//   - Asynchronous notification processing in SSE connections
//   - MCP request handling that should not be interrupted by HTTP connection closure
//   - Background tasks that need to retain context values but not cancellation
//   - Resource cleanup operations that must complete regardless of parent cancellation
//
// Example:
//   ctx := context.WithValue(parent, "key", "value")
//   detachedCtx := WithoutCancel(ctx)
//   go func() {
//       val := detachedCtx.Value("key") // "value" - all values are preserved
//       // This goroutine won't be canceled when parent is canceled
//   }()
func WithoutCancel(parent context.Context) context.Context {
	if parent == nil {
		panic("cannot create context from nil parent")
	}
	return withoutCancelCtx{parent}
}

// withoutCancelCtx is a context that wraps another context and only inherits its values,
// not its cancellation or deadline. This follows the standard library pattern.
type withoutCancelCtx struct {
	context.Context
}

// Deadline returns the time when work done on behalf of this context
// should be canceled. Deadline returns ok==false when no deadline is
// set. Successive calls to Deadline return the same results.
func (withoutCancelCtx) Deadline() (deadline time.Time, ok bool) {
	return
}

// Done returns a channel that's closed when work done on behalf of this
// context should be canceled. Done may return nil if this context can
// never be canceled. Successive calls to Done return the same value.
// The close of the Done channel may happen asynchronously,
// after the cancel function returns.
func (withoutCancelCtx) Done() <-chan struct{} {
	return nil
}

// Err returns a non-nil error value after Done is closed,
// successive calls to Err return the same error.
// If Done is not yet closed, Err returns nil.
// If Done is never closed, Err always returns nil.
func (withoutCancelCtx) Err() error {
	return nil
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Successive calls to Value with
// the same key returns the same result.
func (c withoutCancelCtx) Value(key any) any {
	return c.Context.Value(key)
}

// String returns a string representation of the context.
func (c withoutCancelCtx) String() string {
	return "withoutCancelCtx"
}