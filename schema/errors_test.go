package schema

import (
	"errors"
	"fmt"
	"testing"
)

func TestMCPError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *MCPError
		expected string
	}{
		{
			name:     "ErrorWithoutCause",
			err:      NewError(ErrUnknown, "Unknown error"),
			expected: "[1000] Unknown error",
		},
		{
			name:     "ErrorWithCause",
			err:      WrapError(ErrConnectionFailed, "Connection failed", errors.New("Network timeout")),
			expected: "[1101] Connection failed: Network timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("MCPError.Error() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestMCPError_WithDetails(t *testing.T) {
	err := NewError(ErrInvalidArgument, "Invalid parameter")
	err.WithDetails("param", "username").WithDetails("reason", "too short")

	if len(err.Details) != 2 {
		t.Errorf("Expected 2 details, got %d", len(err.Details))
	}

	if v, ok := err.Details["param"]; !ok || v != "username" {
		t.Errorf("Expected detail 'param' to be 'username', got %v", v)
	}

	if v, ok := err.Details["reason"]; !ok || v != "too short" {
		t.Errorf("Expected detail 'reason' to be 'too short', got %v", v)
	}
}

func TestMCPError_Unwrap(t *testing.T) {
	cause := errors.New("Original error")
	err := WrapError(ErrTimeout, "Operation timeout", cause)

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() = %v, expected %v", unwrapped, cause)
	}
}

func TestMCPError_Is(t *testing.T) {
	err1 := NewError(ErrInvalidArgument, "Invalid parameter")
	err2 := NewError(ErrInvalidArgument, "Another invalid parameter")
	err3 := NewError(ErrTimeout, "Operation timeout")

	if !err1.Is(err2) {
		t.Error("err1.Is(err2) should be true, got false")
	}

	if err1.Is(err3) {
		t.Error("err1.Is(err3) should be false, got true")
	}
}

func TestToFromJSONRPCError(t *testing.T) {
	originalErr := NewError(ErrInvalidRequestMsg, "Invalid request").WithDetails("field", "method")

	// Convert to JSON-RPC error
	rpcErr := originalErr.ToJSONRPCError()

	if rpcErr.Code != -32600 {
		t.Errorf("Expected RPC error code -32600, got %d", rpcErr.Code)
	}

	if rpcErr.Message != "Invalid request" {
		t.Errorf("Expected RPC error message 'Invalid request', got '%s'", rpcErr.Message)
	}

	// Convert back to MCP error
	reconvertedErr := FromJSONRPCError(rpcErr)

	if reconvertedErr.Code != ErrInvalidRequestMsg {
		t.Errorf("Expected reconverted error code %d, got %d", ErrInvalidRequestMsg, reconvertedErr.Code)
	}

	if reconvertedErr.Message != "Invalid request" {
		t.Errorf("Expected reconverted error message 'Invalid request', got '%s'", reconvertedErr.Message)
	}

	// Check detail preservation
	if details, ok := rpcErr.Data.(map[string]interface{}); !ok {
		t.Error("RPC error data should be a map")
	} else if v, ok := details["field"]; !ok || v != "method" {
		t.Errorf("Expected detail 'field' to be 'method', got %v", v)
	}
}

func TestIsErrorCode(t *testing.T) {
	err := NewError(ErrTimeout, "Operation timeout")

	if !IsErrorCode(err, ErrTimeout) {
		t.Error("IsErrorCode(err, ErrTimeout) should be true, got false")
	}

	if IsErrorCode(err, ErrUnknown) {
		t.Error("IsErrorCode(err, ErrUnknown) should be false, got true")
	}

	// Test non-MCPError
	plainErr := errors.New("Regular error")
	if IsErrorCode(plainErr, ErrUnknown) {
		t.Error("IsErrorCode(plainErr, ErrUnknown) should be false, got true")
	}
}

func TestAs(t *testing.T) {
	var mcpErr *MCPError
	originalErr := NewError(ErrTimeout, "Operation timeout")

	if !As(originalErr, &mcpErr) {
		t.Error("As(originalErr, &mcpErr) should be true, got false")
	}

	if mcpErr.Code != ErrTimeout {
		t.Errorf("Expected extracted error code %d, got %d", ErrTimeout, mcpErr.Code)
	}

	// Test non-MCPError
	plainErr := errors.New("Regular error")
	mcpErr = nil

	if As(plainErr, &mcpErr) {
		t.Error("As(plainErr, &mcpErr) should be false, got true")
	}

	if mcpErr != nil {
		t.Error("mcpErr should remain nil after failing As")
	}
}

func ExampleNewError() {
	err := NewError(ErrInvalidArgument, "Invalid username")
	fmt.Println(err)
	// Output: [1001] Invalid username
}

func ExampleWrapError() {
	cause := errors.New("Value too short")
	err := WrapError(ErrInvalidArgument, "Invalid username", cause)
	fmt.Println(err)
	// Output: [1001] Invalid username: Value too short
}

func ExampleMCPError_WithDetails() {
	err := NewError(ErrInvalidArgument, "Invalid parameter").
		WithDetails("param", "username").
		WithDetails("min_length", 8)
	fmt.Printf("%s (Details: %v)\n", err, err.Details)
	// Output: [1001] Invalid parameter (Details: map[min_length:8 param:username])
}
