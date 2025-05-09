package schema

import (
	"fmt"
)

// ErrorCode defines error type codes
type ErrorCode int

// Predefined error codes
const (
	// General errors (1000-1099)
	ErrUnknown           ErrorCode = 1000
	ErrInvalidArgument   ErrorCode = 1001
	ErrTimeout           ErrorCode = 1002
	ErrInvalidState      ErrorCode = 1003
	ErrOperationCanceled ErrorCode = 1004

	// Transport errors (1100-1199)
	ErrTransportGeneral  ErrorCode = 1100
	ErrConnectionFailed  ErrorCode = 1101
	ErrSessionExpired    ErrorCode = 1102
	ErrInvalidResponse   ErrorCode = 1103
	ErrResponseTimeout   ErrorCode = 1104
	ErrInvalidStatusCode ErrorCode = 1105

	// Protocol errors (1200-1299)
	ErrProtocolGeneral   ErrorCode = 1200
	ErrInvalidMessage    ErrorCode = 1201
	ErrInvalidRequestMsg ErrorCode = 1202 // Renamed to avoid conflict with existing definition
	ErrInvalidJSON       ErrorCode = 1203
	ErrUnsupportedMethod ErrorCode = 1204
	ErrInitializeFailed  ErrorCode = 1205

	// Business logic errors (1300-1399)
	ErrResourceNotFound    ErrorCode = 1300
	ErrToolExecutionFailed ErrorCode = 1301
	ErrPromptNotFound      ErrorCode = 1302
	ErrUnsupportedFeature  ErrorCode = 1303
)

// MCPError represents an MCP error
type MCPError struct {
	Code    ErrorCode
	Message string
	Details map[string]interface{}
	Cause   error
}

// Error implements the error interface
func (e *MCPError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// WithDetails adds details to the error
func (e *MCPError) WithDetails(key string, value interface{}) *MCPError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// Unwrap supports error chaining
func (e *MCPError) Unwrap() error {
	return e.Cause
}

// Is checks error type
func (e *MCPError) Is(target error) bool {
	if t, ok := target.(*MCPError); ok {
		return e.Code == t.Code
	}
	return false
}

// ToJSONRPCError converts an MCP error to a JSON-RPC error
// Field mapping:
// - Code (ErrorCode) → code (int): Converted through mapping table or algorithm
// - Message (string) → message (string): Direct mapping
// - Details (map) → data (interface{}): Direct mapping
// - Cause (error): JSON-RPC doesn't support error chaining, this field is not converted
func (e *MCPError) ToJSONRPCError() *JSONRPCError {
	// Map MCP error code to JSON-RPC error code
	// JSON-RPC standard error codes range from -32768 to -32000
	// We use the custom error code range from -32000 to -31000
	var rpcCode int

	// Standard JSON-RPC errors
	switch e.Code {
	case ErrInvalidRequestMsg:
		rpcCode = -32600 // Invalid request
	case ErrInvalidJSON:
		rpcCode = -32700 // Parse error
	case ErrUnsupportedMethod:
		rpcCode = -32601 // Method not found
	case ErrInvalidArgument:
		rpcCode = -32602 // Invalid params
	default:
		// Custom errors - map to -32000-(-31000) range
		// We use a simple algorithm: -(31000 + (error code % 1000))
		// This ensures that different error categories map to different RPC error codes
		rpcCode = -(31000 + (int(e.Code) % 1000))
	}

	return &JSONRPCError{
		Code:    rpcCode,
		Message: e.Message,
		Data:    e.Details,
	}
}

// FromJSONRPCError creates an MCP error from a JSON-RPC error
func FromJSONRPCError(err *JSONRPCError) *MCPError {
	var code ErrorCode

	// Handle standard JSON-RPC errors
	switch err.Code {
	case -32600:
		code = ErrInvalidRequestMsg
	case -32700:
		code = ErrInvalidJSON
	case -32601:
		code = ErrUnsupportedMethod
	case -32602:
		code = ErrInvalidArgument
	default:
		// If in our custom error range
		if err.Code >= -32000 && err.Code < -31000 {
			// Reverse our mapping algorithm
			c := -(err.Code + 31000)
			// Find the closest error category (1000, 1100, 1200, 1300)
			base := (c / 100) * 100
			// Add offset
			code = ErrorCode(base + (c % 100))
		} else {
			// Unknown error
			code = ErrUnknown
		}
	}

	mcpErr := &MCPError{
		Code:    code,
		Message: err.Message,
	}

	// If there's data, add as details
	if err.Data != nil {
		if details, ok := err.Data.(map[string]interface{}); ok {
			mcpErr.Details = details
		}
	}

	return mcpErr
}

// NewError creates a new error
func NewError(code ErrorCode, message string) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WrapError wraps an existing error
func WrapError(code ErrorCode, message string, cause error) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
		Cause:   cause,
	}
}

// IsErrorCode checks if the error has a specific error code
func IsErrorCode(err error, code ErrorCode) bool {
	var mcpErr *MCPError
	if e, ok := err.(*MCPError); ok {
		mcpErr = e
	} else if As(err, &mcpErr) {
		// Use errors.As
	} else {
		return false
	}

	return mcpErr.Code == code
}

// As treats an error as the target type
func As(err error, target interface{}) bool {
	// Simplified implementation, should use standard library errors.As in practice
	if target == nil {
		return false
	}

	if e, ok := err.(*MCPError); ok {
		if t, ok := target.(**MCPError); ok {
			*t = e
			return true
		}
	}

	return false
}
