// Package mcp defines common error types and constants
package mcp

import "errors"

// Common errors
var (
	// Tools related errors
	ErrInvalidToolListFormat = errors.New("invalid tool list response format")
	ErrInvalidToolFormat     = errors.New("invalid tool format")
	ErrToolNotFound          = errors.New("tool not found")
	ErrInvalidToolParams     = errors.New("invalid tool parameters")

	// JSON-RPC related errors
	ErrParseJSONRPC           = errors.New("failed to parse JSON-RPC message")
	ErrInvalidJSONRPCFormat   = errors.New("invalid JSON-RPC format")
	ErrInvalidJSONRPCResponse = errors.New("invalid JSON-RPC response")
	ErrInvalidJSONRPCRequest  = errors.New("invalid JSON-RPC request")
	ErrInvalidJSONRPCParams   = errors.New("invalid JSON-RPC parameters")

	// Resource related errors
	ErrInvalidResourceFormat = errors.New("invalid resource format")
	ErrResourceNotFound      = errors.New("resource not found")

	// Prompt related errors
	ErrInvalidPromptFormat = errors.New("invalid prompt format")
	ErrPromptNotFound      = errors.New("prompt not found")
)
