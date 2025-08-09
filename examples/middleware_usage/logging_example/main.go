package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/examples/middlewares"
)

// MockLogger is a simple logger implementation for testing that satisfies the mcp.Logger interface.
type MockLogger struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

// logf is a helper to format and write log messages to the buffer.
func (m *MockLogger) logf(level string, format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	m.buf.WriteString(fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg))
}

// log is a helper for unformatted log messages.
func (m *MockLogger) log(level string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprint(args...)
	m.buf.WriteString(fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg))
}

// Implementation of mcp.Logger interface
func (m *MockLogger) Debug(args ...interface{})                 { m.log("DEBUG", args...)
}
func (m *MockLogger) Debugf(format string, args ...interface{}) { m.logf("DEBUG", format, args...)
}
func (m *MockLogger) Info(args ...interface{})                  { m.log("INFO", args...)
}
func (m *MockLogger) Infof(format string, args ...interface{})  { m.logf("INFO", format, args...)
}
func (m *MockLogger) Warn(args ...interface{})                  { m.log("WARN", args...)
}
func (m *MockLogger) Warnf(format string, args ...interface{})  { m.logf("WARN", format, args...)
}
func (m *MockLogger) Error(args ...interface{})                 { m.log("ERROR", args...)
}
func (m *MockLogger) Errorf(format string, args ...interface{}) { m.logf("ERROR", format, args...)
}
func (m *MockLogger) Fatal(args ...interface{})                 { m.log("FATAL", args...)
}
func (m *MockLogger) Fatalf(format string, args ...interface{}) { m.logf("FATAL", format, args...)
}

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

func TestTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("TestTool called with: %v", req.Params.Arguments)

	message := fmt.Sprintf("TestTool executed at: %s", time.Now().Format(time.RFC3339))
	result := mcp.NewTextResult(message)
	return result, nil
}

func ErrorTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("ErrorTool called with: %v", req.Params.Arguments)
	return nil, errors.New("something went wrong")
}

func main() {

	mockLogger := &MockLogger{}

	advancedLoggingMiddleware := middlewares.NewLoggingMiddleware(
		mockLogger,
		middlewares.WithShouldLog(func(level middlewares.Level, duration time.Duration, err error) bool {
			// logging all requests for demonstration
			return true
		}),
		middlewares.WithPayloadLogging(true),
		middlewares.WithFieldsFromContext(func(ctx context.Context) middlewares.Fields {
			return middlewares.Fields{
				"test.source", "integration-test",
				"test.timestamp", time.Now().Unix(),
			}

		}),
		middlewares.WithColor(true),
	)

	// create server
	s := mcp.NewServer(
		"middleware-test-server",
		"1.0.0",
		mcp.WithStatelessMode(true),
	)

	s.Use(advancedLoggingMiddleware)

	// register simple middleware for demonstration
	testToolDef := &mcp.Tool{
		Name:        "test_tool",
		Description: "A simple test tool",
		InputSchema: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: map[string]*openapi3.SchemaRef{
				"message": {
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Test message",
					},
				},
			},
		},
	}

	errorToolDef := &mcp.Tool{
		Name:        "error_tool",
		Description: "A tool that returns an error",
		InputSchema: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: map[string]*openapi3.SchemaRef{
				"message": {
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Error message",
					},
				},
			},
		},
	}

	s.RegisterTool(testToolDef, TestTool)
	s.RegisterTool(errorToolDef, ErrorTool)

	// start server
	fmt.Println("=== Middleware Integration Test Server with MockLogger ===")
	fmt.Println("Server listening on :8080")
	fmt.Println("")
	fmt.Println("Test scenarios:")
	fmt.Println("1. All requests will pass through 3 middleware layers")
	fmt.Println("2. Simple middleware logs entry/exit")
	fmt.Println("3. Auth middleware checks authentication")
	fmt.Println("4. Advanced middleware logs detailed info using MockLogger")
	fmt.Println("5. Test both successful and error scenarios")
	fmt.Println("")
	fmt.Println("MockLogger features:")
	fmt.Println("- Thread-safe logging with mutex protection")
	fmt.Println("- Buffer-based log storage")
	fmt.Println("- Contains() method for log verification")
	fmt.Println("- String() method for complete log output")
	fmt.Println("- Reset() method for clearing logs")
	fmt.Println("")

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			logOutput := mockLogger.String()
			if logOutput != "" {
				fmt.Printf("\n=== MockLogger Output ===\n%s\n", logOutput)
				mockLogger.Reset()
			}
		}
	}()

	fmt.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", s.Handler()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
