package middlewares

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/mcptest"
)

// MockLogger is a test implementation of mcp.Logger.
type MockLogger struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

// log is an internal helper to prevent concurrent writes.
func (m *MockLogger) log(level string, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf.WriteString(fmt.Sprintf("[%s] %s\n", level, message))
}

// Implement the mcp.Logger interface
func (m *MockLogger) Debug(args ...interface{}) { m.log("DEBUG", fmt.Sprint(args...)) }
func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.log("DEBUG", fmt.Sprintf(format, args...))
}
func (m *MockLogger) Info(args ...interface{}) { m.log("INFO", fmt.Sprint(args...)) }
func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.log("INFO", fmt.Sprintf(format, args...))
}
func (m *MockLogger) Warn(args ...interface{}) { m.log("WARN", fmt.Sprint(args...)) }
func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.log("WARN", fmt.Sprintf(format, args...))
}
func (m *MockLogger) Error(args ...interface{}) { m.log("ERROR", fmt.Sprint(args...)) }
func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.log("ERROR", fmt.Sprintf(format, args...))
}
func (m *MockLogger) Fatal(args ...interface{}) { m.log("FATAL", fmt.Sprint(args...)) }
func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.log("FATAL", fmt.Sprintf(format, args...))
}

// Helper methods for testing
func (m *MockLogger) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

func (m *MockLogger) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf.Reset()
}

func (m *MockLogger) Contains(sub string) bool {
	return strings.Contains(m.String(), sub)
}

func TestLoggingMiddleware_WithOptions(t *testing.T) {
	mockReq := &mcp.JSONRPCRequest{
		Request: mcp.Request{Method: "tools/call"},
	}

	mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		return &mcp.JSONRPCResponse{Result: "ok"}, nil
	}

	mockErrorHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		return nil, errors.New("something went wrong")
	}

	t.Run("ShouldLog/Default_OnlyLogsOnError", func(t *testing.T) {
		mockLogger := &MockLogger{}
		middleware := NewLoggingMiddleware(mockLogger)
		// Case 1: Success, should not log
		mcptest.RunMiddlewareTest(t, middleware, mockReq, mockFinalHandler)
		if logOutput := mockLogger.String(); logOutput != "" {
			t.Errorf("default config should not log successful requests, but got: %s", logOutput)
		}

		mockLogger.Reset()

		// Case 2: Failure, should log
		mcptest.RunMiddlewareTest(t, middleware, mockReq, mockErrorHandler)
		if !mockLogger.Contains("Request failed") || !mockLogger.Contains("something went wrong") {
			t.Errorf("default config should log failed requests, but did not. Log: %s", mockLogger.String())
		}
	})

	t.Run("ShouldLog/Custom_LogAllRequests", func(t *testing.T) {
		mockLogger := &MockLogger{}
		middleware := NewLoggingMiddleware(mockLogger,
			WithShouldLog(func(level Level, duration time.Duration, err error) bool {
				return true
			}),
		)
		mcptest.RunMiddlewareTest(t, middleware, mockReq, mockFinalHandler)
		if !mockLogger.Contains("Request completed") {
			t.Errorf("custom config to log all requests did not log a successful one. Log: %s", mockLogger.String())
		}
	})

	t.Run("PayloadLogging/Enabled", func(t *testing.T) {
		mockLogger := &MockLogger{}
		middleware := NewLoggingMiddleware(mockLogger,
			WithShouldLog(func(level Level, duration time.Duration, err error) bool { return true }),
			WithPayloadLogging(true),
		)

		reqWithParams := &mcp.JSONRPCRequest{
			Request: mcp.Request{Method: "tools/call"},
			Params:  map[string]interface{}{"user": "alice"},
		}

		mcptest.RunMiddlewareTest(t, middleware, reqWithParams, mockFinalHandler)

		logOutput := mockLogger.String()
		// ULTIMATE FIX: Check for exact formatting from the logger.
		if !strings.Contains(logOutput, "params: map[user:alice]") {
			t.Errorf("PayloadLogging enabled but request payload not found. Log: %s", logOutput)
		}
		if !strings.Contains(logOutput, "result: ok") {
			t.Errorf("PayloadLogging enabled but response payload not found. Log: %s", logOutput)
		}
	})

	t.Run("PayloadLogging/Disabled", func(t *testing.T) {
		mockLogger := &MockLogger{}
		middleware := NewLoggingMiddleware(mockLogger,
			WithShouldLog(func(level Level, duration time.Duration, err error) bool { return true }),
			WithPayloadLogging(false),
		)
		reqWithParams := &mcp.JSONRPCRequest{
			Request: mcp.Request{Method: "tools/call"},
			Params:  map[string]interface{}{"user": "alice"},
		}

		mcptest.RunMiddlewareTest(t, middleware, reqWithParams, mockFinalHandler)

		logOutput := mockLogger.String()
		if strings.Contains(logOutput, "request: {") {
			t.Errorf("PayloadLogging disabled but request payload was found. Log: %s", logOutput)
		}
	})

	t.Run("FieldsFromContext", func(t *testing.T) {
		mockLogger := &MockLogger{}
		middleware := NewLoggingMiddleware(mockLogger,
			WithShouldLog(func(level Level, duration time.Duration, err error) bool { return true }),
			WithFieldsFromContext(func(ctx context.Context) Fields {
				if requestID, ok := ctx.Value("request_id").(string); ok {
					return Fields{"request_id", requestID}
				}
				return nil
			}),
		)

		ctxWithField := context.WithValue(context.Background(), "request_id", "xyz-123")

		finalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			if ctx.Value("request_id") != "xyz-123" {
				t.Error("context was not passed correctly to the final handler")
			}
			return &mcp.JSONRPCResponse{Result: "ok"}, nil
		}

		middleware(ctxWithField, mockReq, nil, finalHandler)

		if !mockLogger.Contains("request_id") || !mockLogger.Contains("xyz-123") {
			t.Errorf("expected log to contain field from context, but it was not found. Log: %s", mockLogger.String())
		}
	})
}