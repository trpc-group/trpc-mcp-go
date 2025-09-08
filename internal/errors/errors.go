// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// Package mcperrors defines common error types and constants
package errors

import (
	"errors"
)

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

	// Tool manager errors
	ErrEmptyToolName         = errors.New("tool name cannot be empty")
	ErrToolAlreadyRegistered = errors.New("tool already registered")
	ErrToolExecutionFailed   = errors.New("tool execution failed")

	// Resource manager errors
	ErrEmptyResourceURI = errors.New("resource URI cannot be empty")

	// Prompt manager errors
	ErrEmptyPromptName = errors.New("prompt name cannot be empty")

	// Lifecycle manager errors
	ErrSessionAlreadyInitialized = errors.New("session already initialized")
	ErrSessionNotInitialized     = errors.New("session not initialized")

	// Parameter errors
	ErrInvalidParams = errors.New("invalid parameters")
	ErrMissingParams = errors.New("missing required parameters")

	// Client errors
	ErrAlreadyInitialized = errors.New("client already initialized")
	ErrNotInitialized     = errors.New("client not initialized")
	ErrInvalidServerURL   = errors.New("invalid server URL")
)
