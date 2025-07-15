// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"encoding/json"
	"fmt"

	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// Constants definition
const (
	// JSONRPCVersion specifies the JSON-RPC version
	JSONRPCVersion = "2.0"

	// Standard JSON-RPC error codes
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603

	// MCP custom error code range: -32000 to -32099
)

// JSONRPCMessage represents a JSON-RPC message.
type JSONRPCMessage interface{}

// JSONRPCRequest represents a JSON-RPC request
// Conforms to the JSONRPCRequest definition in schema.json
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      RequestId   `json:"id,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Request
}

// JSONRPCResponse represents a JSON-RPC success response
// Conforms to the JSONRPCResponse definition in schema.json
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      RequestId   `json:"id"`
	Result  interface{} `json:"result,omitempty"`
}

// JSONRPCError represents a JSON-RPC error response
// Conforms to the JSONRPCError definition in schema.json
type JSONRPCError struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      RequestId `json:"id,omitempty"`
	Error   struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data,omitempty"`
	} `json:"error"`
}

// JSONRPCNotification represents a JSON-RPC notification
// Conforms to the JSONRPCNotification definition in schema.json
type JSONRPCNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Notification
}

// newJSONRPCRequest creates a new JSON-RPC request
func newJSONRPCRequest(id interface{}, method string, params map[string]interface{}) *JSONRPCRequest {
	if params == nil {
		params = map[string]interface{}{}
	}

	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Request: Request{
			Method: method,
		},
		Params: params,
	}

	return req
}

// newJSONRPCResponse creates a new JSON-RPC response
func newJSONRPCResponse(id interface{}, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
}

// newJSONRPCErrorResponse creates a new JSON-RPC error response
func newJSONRPCErrorResponse(id interface{}, code int, message string, data interface{}) *JSONRPCError {
	errResp := &JSONRPCError{
		JSONRPC: JSONRPCVersion,
		ID:      id,
	}
	errResp.Error.Code = code
	errResp.Error.Message = message
	errResp.Error.Data = data
	return errResp
}

// newJSONRPCNotification creates a new JSON-RPC notification
func newJSONRPCNotification(notification Notification) *JSONRPCNotification {
	return &JSONRPCNotification{
		JSONRPC:      JSONRPCVersion,
		Notification: notification,
	}
}

// NewJSONRPCNotificationFromMap creates a new JSON-RPC notification from a map of parameters.
// This is a convenience function that converts map[string]interface{} to NotificationParams.
func NewJSONRPCNotificationFromMap(method string, params map[string]interface{}) *JSONRPCNotification {
	notificationParams := NotificationParams{
		AdditionalFields: make(map[string]interface{}),
	}

	// Extract meta field if present
	if meta, ok := params["_meta"]; ok {
		if metaMap, ok := meta.(map[string]interface{}); ok {
			notificationParams.Meta = metaMap
		}
		delete(params, "_meta")
	}

	// Add remaining fields to AdditionalFields
	for k, v := range params {
		notificationParams.AdditionalFields[k] = v
	}

	notification := Notification{
		Method: method,
		Params: notificationParams,
	}

	return newJSONRPCNotification(notification)
}

// RequestId is the base request id struct for all MCP requests.
type RequestId interface{}

// JSONRPCMessageType represents the type of a JSON-RPC message
type JSONRPCMessageType string

const (
	// JSONRPCMessageTypeRequest represents a JSON-RPC request message
	JSONRPCMessageTypeRequest JSONRPCMessageType = "request"
	// JSONRPCMessageTypeResponse represents a JSON-RPC response message
	JSONRPCMessageTypeResponse JSONRPCMessageType = "response"
	// JSONRPCMessageTypeNotification represents a JSON-RPC notification message
	JSONRPCMessageTypeNotification JSONRPCMessageType = "notification"
	// JSONRPCMessageTypeError represents a JSON-RPC error message
	JSONRPCMessageTypeError JSONRPCMessageType = "error"
	// JSONRPCMessageTypeUnknown represents an unknown message type
	JSONRPCMessageTypeUnknown JSONRPCMessageType = "unknown"
)

// parseJSONRPCMessageType determines the type of a JSON-RPC message
func parseJSONRPCMessageType(data []byte) (JSONRPCMessageType, error) {
	// First try to parse as a generic map to determine message type
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		return JSONRPCMessageTypeUnknown, fmt.Errorf("%w: %v", errors.ErrParseJSONRPC, err)
	}

	// Check JSON-RPC version
	if version, ok := message["jsonrpc"].(string); !ok || version != JSONRPCVersion {
		return JSONRPCMessageTypeUnknown, fmt.Errorf("%w: invalid or missing jsonrpc version", errors.ErrInvalidJSONRPCFormat)
	}

	// Determine message type
	if _, hasID := message["id"]; hasID {
		if _, hasError := message["error"]; hasError {
			return JSONRPCMessageTypeError, nil
		} else if _, hasResult := message["result"]; hasResult {
			return JSONRPCMessageTypeResponse, nil
		}
		return JSONRPCMessageTypeRequest, nil
	} else if _, hasMethod := message["method"]; hasMethod {
		return JSONRPCMessageTypeNotification, nil
	}

	return JSONRPCMessageTypeUnknown, errors.ErrInvalidJSONRPCFormat
}

// parseJSONRPCMessage parses any type of JSON-RPC message
func parseJSONRPCMessage(data []byte) (interface{}, JSONRPCMessageType, error) {
	msgType, err := parseJSONRPCMessageType(data)
	if err != nil {
		return nil, JSONRPCMessageTypeUnknown, err
	}

	switch msgType {
	case JSONRPCMessageTypeResponse:
		return parseJSONRPCResponse(data, msgType)
	case JSONRPCMessageTypeError:
		return parseJSONRPCError(data, msgType)
	case JSONRPCMessageTypeNotification:
		return parseJSONRPCNotification(data, msgType)
	case JSONRPCMessageTypeRequest:
		return parseJSONRPCRequest(data, msgType)
	default:
		return nil, msgType, errors.ErrInvalidJSONRPCFormat
	}
}

// parseJSONRPCResponse parses a JSON-RPC response
func parseJSONRPCResponse(data []byte, msgType JSONRPCMessageType) (interface{}, JSONRPCMessageType, error) {
	var resp JSONRPCResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, msgType, fmt.Errorf("%w: %v", errors.ErrInvalidJSONRPCResponse, err)
	}
	return &resp, msgType, nil
}

// parseJSONRPCError parses a JSON-RPC error
func parseJSONRPCError(data []byte, msgType JSONRPCMessageType) (interface{}, JSONRPCMessageType, error) {
	var errResp JSONRPCError
	if err := json.Unmarshal(data, &errResp); err != nil {
		return nil, msgType, fmt.Errorf("%w: %v", errors.ErrInvalidJSONRPCResponse, err)
	}
	return &errResp, msgType, nil
}

// parseJSONRPCNotification parses a JSON-RPC notification
func parseJSONRPCNotification(data []byte, msgType JSONRPCMessageType) (interface{}, JSONRPCMessageType, error) {
	var notification JSONRPCNotification
	if err := json.Unmarshal(data, &notification); err != nil {
		return nil, msgType, fmt.Errorf("%w: %v", errors.ErrInvalidJSONRPCFormat, err)
	}
	return &notification, msgType, nil
}

// parseJSONRPCRequest parses a JSON-RPC request
func parseJSONRPCRequest(data []byte, msgType JSONRPCMessageType) (interface{}, JSONRPCMessageType, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, msgType, fmt.Errorf("%w: %v", errors.ErrInvalidJSONRPCRequest, err)
	}
	return &req, msgType, nil
}

// formatJSONRPCMessage returns a description of a JSON-RPC message (for logging)
func formatJSONRPCMessage(msg interface{}) string {
	switch m := msg.(type) {
	case *JSONRPCResponse:
		return fmt.Sprintf("Response(ID=%v)", m.ID)
	case *JSONRPCError:
		return fmt.Sprintf("Error(ID=%v, Code=%d, Message=%s)", m.ID, m.Error.Code, m.Error.Message)
	case *JSONRPCNotification:
		return fmt.Sprintf("Notification(Method=%s)", m.Method)
	case *JSONRPCRequest:
		return fmt.Sprintf("Request(ID=%v, Method=%s)", m.ID, m.Method)
	default:
		return "Unknown message type"
	}
}

// isErrorResponse checks if a response indicates error
func isErrorResponse(resp *json.RawMessage) bool {
	// Check if the raw message contains an "error" field
	var message map[string]interface{}
	if err := json.Unmarshal(*resp, &message); err != nil {
		return false
	}
	_, hasError := message["error"]
	return hasError
}

// parseRawMessageToError parses a raw message into a JSONRPCError
func parseRawMessageToError(raw *json.RawMessage) (*JSONRPCError, error) {
	var errResp JSONRPCError
	if err := json.Unmarshal(*raw, &errResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC error: %w", err)
	}
	return &errResp, nil
}
