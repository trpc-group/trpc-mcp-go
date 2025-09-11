package middlewares

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	. "trpc.group/trpc-go/trpc-mcp-go"
)

type RecoveryConfig struct {
	// Logger
	// Optional. Default value GetDefaultLogger().
	Logger Logger

	// EnableStack
	// Optional. Default value true.
	EnableStack bool

	// StackSkip is the number of stack frames to skip when printing the stack trace.
	// Optional. Default value 3.
	StackSkip int

	// MaxStackSize is size of the stack to be printed
	// Optional. Default value 8KB.
	MaxStackSize int

	// PanicFilter filters specific types of panics (return true means need to handle this panic)
	// Optional. Default value nil (handle all panics).
	PanicFilter func(panicErr interface{}) bool

	// CustomErrorResponse allows customizing the error response format
	// Optional. Default value nil (use default error response).
	CustomErrorResponse func(ctx context.Context, req *JSONRPCRequest, panicErr interface{}) JSONRPCMessage
}

// Recovery creates recovery middleware with default configuration
/*
Usage example:
	server.Use(Recovery())
*/
func Recovery() MiddlewareFunc {
	middleware := newRecoveryMiddleware(nil)
	return middleware.HandleFunc
}

// RecoveryWithOptions creates recovery middleware with options
/*
Usage example:
	server.Use(RecoveryWithOptions(
		WithLogger(customLogger),
		WithStackTrace(true),
		WithMaxStackSize(4096),
		WithPanicFilter(OnlyHandleRuntimeErrors()),
		WithCustomErrorResponse(func(ctx context.Context, req *JSONRPCRequest, panicErr interface{}) JSONRPCMessage {
			return NewJSONRPCErrorResponse(
				req.ID,
				ErrCodeInternal,
				"Service temporarily unavailable",
				nil,
			)
		}),
	))
*/
func RecoveryWithOptions(opts ...RecoveryOption) MiddlewareFunc {
	config := DefaultRecoveryConfig()
	for _, opt := range opts {
		opt(config)
	}
	middleware := newRecoveryMiddleware(config)

	return middleware.HandleFunc
}

// DefaultRecoveryConfig
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		Logger:              GetDefaultLogger(),
		EnableStack:         true,
		StackSkip:           3,
		MaxStackSize:        8192, // 8KB
		PanicFilter:         nil,  // handle all panics
		CustomErrorResponse: nil,  // use default error response
	}
}

// RecoveryMiddleware captures panics during request processing and returns a standard JSON-RPC error response
type RecoveryMiddleware struct {
	config *RecoveryConfig
}

// newRecoveryMiddleware creates a new recovery middleware instance
func newRecoveryMiddleware(config *RecoveryConfig) *RecoveryMiddleware {
	if config == nil {
		config = DefaultRecoveryConfig()
	}
	return &RecoveryMiddleware{
		config: config,
	}
}

// HandleFunc implements MiddlewareFunc
func (m *RecoveryMiddleware) HandleFunc(ctx context.Context, req *JSONRPCRequest, session Session, next HandleFunc) (resp JSONRPCMessage, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			// apply panic filter if configured
			if m.config.PanicFilter != nil && !m.config.PanicFilter(panicErr) {
				// filter returns false, re-panic to let upper level handle it
				panic(panicErr)
			}

			// log panic information
			m.logPanic(ctx, req, panicErr)

			// generate error response
			resp = m.generateErrorResponse(ctx, req, panicErr)
			err = nil
		}
	}()

	return next(ctx, req, session)
}

// logPanic records panic information to log
func (m *RecoveryMiddleware) logPanic(ctx context.Context, req *JSONRPCRequest, panicErr interface{}) {
	// get request information
	requestID := fmt.Sprintf("%v", req.ID)
	method := req.Method

	// build log message
	var logMsg strings.Builder
	logMsg.WriteString(fmt.Sprintf("[%s | RequestID: %s | Method: %s] Recover panic: %v\n", time.Now().Format(time.RFC3339), requestID, method, panicErr))

	// log stack trace if enabled
	if m.config.EnableStack {
		stack := m.getStackTrace()
		if stack != "" {
			m.config.Logger.Errorf("Stack trace:\n%s", stack)
		}
	}

	// log panic message
	m.config.Logger.Error(logMsg.String())
}

// getStackTrace gets formatted stack trace information
func (m *RecoveryMiddleware) getStackTrace() string {
	// get full stack trace
	fullStack := string(debug.Stack())

	// limit stack size
	if m.config.MaxStackSize > 0 && len(fullStack) > m.config.MaxStackSize {
		fullStack = fullStack[:m.config.MaxStackSize] + "\n... (truncated)"
	}

	// filter and format stack information
	lines := strings.Split(fullStack, "\n")
	var filtered []string

	skip := m.config.StackSkip
	for _, line := range lines {
		// skip runtime related frames
		if skip > 0 && (strings.Contains(line, "runtime/panic.go") ||
			strings.Contains(line, "recovery.go") ||
			strings.Contains(line, "middleware.go")) {
			skip--
			continue
		}

		// keep useful stack information
		if strings.TrimSpace(line) != "" {
			filtered = append(filtered, line)
		}

		// limit lines to avoid too long stack trace
		if len(filtered) > 50 {
			filtered = append(filtered, "... (more frames truncated)")
			break
		}
	}

	return strings.Join(filtered, "\n")
}

// generateErrorResponse generates error response for panic
func (m *RecoveryMiddleware) generateErrorResponse(ctx context.Context, req *JSONRPCRequest, panicErr interface{}) JSONRPCMessage {
	// use custom error response generator if configured
	if m.config.CustomErrorResponse != nil {
		return m.config.CustomErrorResponse(ctx, req, panicErr)
	}

	// generate default error response
	return m.createDefaultErrorResponse(req, panicErr)
}

// createDefaultErrorResponse creates default error response for panic
func (m *RecoveryMiddleware) createDefaultErrorResponse(req *JSONRPCRequest, panicErr interface{}) JSONRPCMessage {
	var requestID interface{}
	if req != nil {
		requestID = req.ID
	}

	// create user-friendly error message (don't expose internal panic details)
	errorMsg := "Internal server error occurred"

	return NewJSONRPCErrorResponse(
		requestID,
		ErrCodeInternal,
		errorMsg,
		nil, // don't include panic details in response for security
	)
}

// RecoveryOption defines option function type for configuring recovery middleware
type RecoveryOption func(*RecoveryConfig)

// WithLogger sets custom logger
func WithLogger(logger Logger) RecoveryOption {
	return func(config *RecoveryConfig) {
		config.Logger = logger
	}
}

// WithStackTrace enables or disables stack trace logging
func WithStackTrace(enable bool) RecoveryOption {
	return func(config *RecoveryConfig) {
		config.EnableStack = enable
	}
}

// WithStackSkip sets the number of stack frames to skip
func WithStackSkip(skip int) RecoveryOption {
	return func(config *RecoveryConfig) {
		config.StackSkip = skip
	}
}

// WithMaxStackSize sets maximum stack trace size
func WithMaxStackSize(size int) RecoveryOption {
	return func(config *RecoveryConfig) {
		config.MaxStackSize = size
	}
}

// WithPanicFilter sets panic filter function
func WithPanicFilter(filter func(interface{}) bool) RecoveryOption {
	return func(config *RecoveryConfig) {
		config.PanicFilter = filter
	}
}

// WithCustomErrorResponse sets custom error response generator
func WithCustomErrorResponse(handler func(context.Context, *JSONRPCRequest, interface{}) JSONRPCMessage) RecoveryOption {
	return func(config *RecoveryConfig) {
		config.CustomErrorResponse = handler
	}
}

// Common panic filter fun

// IgnoreStringPanics ignores string type panics
func IgnoreStringPanics() func(interface{}) bool {
	return func(panicErr interface{}) bool {
		_, isString := panicErr.(string)
		return !isString // return true means handle, false means ignore
	}
}

// OnlyHandleRuntimeErrors only handles runtime errors
func OnlyHandleRuntimeErrors() func(interface{}) bool {
	return func(panicErr interface{}) bool {
		if err, ok := panicErr.(error); ok {
			errorStr := err.Error()
			return strings.Contains(errorStr, "runtime error") ||
				strings.Contains(errorStr, "slice bounds out of range") ||
				strings.Contains(errorStr, "nil pointer dereference")
		}
		return false
	}
}
