package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Constants definition
const (
	JSONRPCVersion = "2.0"

	// Standard JSON-RPC error codes
	ErrParse          = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603

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

// NewJSONRPCRequest creates a new JSON-RPC request
func NewJSONRPCRequest(id interface{}, method string, params map[string]interface{}) *JSONRPCRequest {
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

// NewJSONRPCResponse creates a new JSON-RPC response
func NewJSONRPCResponse(id interface{}, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
}

// NewJSONRPCErrorResponse creates a new JSON-RPC error response
func NewJSONRPCErrorResponse(id interface{}, code int, message string, data interface{}) *JSONRPCError {
	errResp := &JSONRPCError{
		JSONRPC: JSONRPCVersion,
		ID:      id,
	}
	errResp.Error.Code = code
	errResp.Error.Message = message
	errResp.Error.Data = data
	return errResp
}

// NewJSONRPCNotification creates a new JSON-RPC notification
func NewJSONRPCNotification(notification Notification) *JSONRPCNotification {
	return &JSONRPCNotification{
		JSONRPC:      JSONRPCVersion,
		Notification: notification,
	}
}

// NewJSONRPCNotificationFromMap creates a new JSON-RPC notification from a map of parameters
// This is a convenience function that converts map[string]interface{} to NotificationParams
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

	return NewJSONRPCNotification(notification)
}

// RequestId is the base request id struct for all MCP requests.
type RequestId interface{}

// JSONRPCMessageType represents the type of a JSON-RPC message
type JSONRPCMessageType string

const (
	JSONRPCMessageTypeRequest      JSONRPCMessageType = "request"
	JSONRPCMessageTypeResponse     JSONRPCMessageType = "response"
	JSONRPCMessageTypeNotification JSONRPCMessageType = "notification"
	JSONRPCMessageTypeError        JSONRPCMessageType = "error"
	JSONRPCMessageTypeUnknown      JSONRPCMessageType = "unknown"
)

// ParseJSONRPCMessageType determines the type of a JSON-RPC message
func ParseJSONRPCMessageType(data []byte) (JSONRPCMessageType, error) {
	// First try to parse as a generic map to determine message type
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		return JSONRPCMessageTypeUnknown, fmt.Errorf("%w: %v", ErrParseJSONRPC, err)
	}

	// Check JSON-RPC version
	if version, ok := message["jsonrpc"].(string); !ok || version != JSONRPCVersion {
		return JSONRPCMessageTypeUnknown, fmt.Errorf("%w: invalid or missing jsonrpc version", ErrInvalidJSONRPCFormat)
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

	return JSONRPCMessageTypeUnknown, ErrInvalidJSONRPCFormat
}

// ParseJSONRPCMessage parses any type of JSON-RPC message
func ParseJSONRPCMessage(data []byte) (interface{}, JSONRPCMessageType, error) {
	// Determine message type
	msgType, err := ParseJSONRPCMessageType(data)
	if err != nil {
		return nil, JSONRPCMessageTypeUnknown, err
	}

	// Parse into specific structure based on type
	switch msgType {
	case JSONRPCMessageTypeResponse:
		var resp JSONRPCResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, msgType, fmt.Errorf("%w: %v", ErrInvalidJSONRPCResponse, err)
		}
		return &resp, msgType, nil

	case JSONRPCMessageTypeError:
		var errResp JSONRPCError
		if err := json.Unmarshal(data, &errResp); err != nil {
			return nil, msgType, fmt.Errorf("%w: %v", ErrInvalidJSONRPCResponse, err)
		}
		return &errResp, msgType, nil

	case JSONRPCMessageTypeNotification:
		var notification JSONRPCNotification
		if err := json.Unmarshal(data, &notification); err != nil {
			return nil, msgType, fmt.Errorf("%w: %v", ErrInvalidJSONRPCFormat, err)
		}
		return &notification, msgType, nil

	case JSONRPCMessageTypeRequest:
		var req JSONRPCRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, msgType, fmt.Errorf("%w: %v", ErrInvalidJSONRPCRequest, err)
		}
		return &req, msgType, nil

	default:
		return nil, msgType, ErrInvalidJSONRPCFormat
	}
}

// IsResponseForRequest checks if a response corresponds to a specific request
func IsResponseForRequest(resp *JSONRPCResponse, reqID interface{}) bool {
	// Use string comparison to handle different types (numbers or strings)
	return fmt.Sprintf("%v", resp.ID) == fmt.Sprintf("%v", reqID)
}

// FormatJSONRPCMessage returns a description of a JSON-RPC message (for logging)
func FormatJSONRPCMessage(msg interface{}) string {
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

// ParseJSONRequest parses a JSON-RPC request from the request body
func ParseJSONRequest(body io.ReadCloser, request *JSONRPCRequest) error {
	defer body.Close()

	// Read request body
	requestBody, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// Parse JSON-RPC request
	if err := json.Unmarshal(requestBody, request); err != nil {
		return fmt.Errorf("failed to parse JSON request: %w", err)
	}

	return nil
}

// ParseJSONParams parses JSON-RPC params from a generic map
func ParseJSONParams(rawParams interface{}, params interface{}) error {
	// Convert generic params to JSON
	jsonBytes, err := json.Marshal(rawParams)
	if err != nil {
		return fmt.Errorf("failed to serialize params: %w", err)
	}

	// Parse into specific structure
	if err := json.Unmarshal(jsonBytes, params); err != nil {
		return fmt.Errorf("failed to parse params: %w", err)
	}

	return nil
}

// WriteJSONResponse writes a JSON-RPC response to the HTTP response writer
func WriteJSONResponse(w http.ResponseWriter, response interface{}) error {
	// Serialize response
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	// Write response
	_, err = w.Write(responseBytes)
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}

// GetMethodFromRequest extracts method from request
func GetMethodFromRequest(req *JSONRPCRequest) string {
	return req.Method
}

// IsSuccessResponse checks if a response indicates success
func IsSuccessResponse(resp *json.RawMessage) bool {
	// Check if the raw message contains an "error" field
	var message map[string]interface{}
	if err := json.Unmarshal(*resp, &message); err != nil {
		return false
	}
	_, hasError := message["error"]
	return !hasError
}

// IsErrorResponse checks if a response indicates error
func IsErrorResponse(resp *json.RawMessage) bool {
	// Check if the raw message contains an "error" field
	var message map[string]interface{}
	if err := json.Unmarshal(*resp, &message); err != nil {
		return false
	}
	_, hasError := message["error"]
	return hasError
}

// ParseRawMessageToError parses a raw message into a JSONRPCError
func ParseRawMessageToError(raw *json.RawMessage) (*JSONRPCError, error) {
	var errResp JSONRPCError
	if err := json.Unmarshal(*raw, &errResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC error: %w", err)
	}
	return &errResp, nil
}

// ParseRawMessageToResponse parses a raw message into a JSONRPCResponse
func ParseRawMessageToResponse(raw *json.RawMessage) (*JSONRPCResponse, error) {
	var resp JSONRPCResponse
	if err := json.Unmarshal(*raw, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}
	return &resp, nil
}

// ParseJSONRPCResponse parses JSON-RPC response based on method
func ParseJSONRPCResponse(rawResp *json.RawMessage, method string) (interface{}, error) {
	// Check error
	if IsErrorResponse(rawResp) {
		errResp, err := ParseRawMessageToError(rawResp)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("JSON-RPC error response (Code=%d): %s",
			errResp.Error.Code, errResp.Error.Message)
	}

	// Parse success response
	resp, err := ParseRawMessageToResponse(rawResp)
	if err != nil {
		return nil, err
	}

	// Check if result exists
	if resp.Result == nil {
		return nil, fmt.Errorf("JSON-RPC response missing result")
	}

	// Call method-specific response parsing function
	return ParseMethodSpecificResponse(resp.Result, method)
}

// ParseMethodSpecificResponse parses response for specific method
func ParseMethodSpecificResponse(result interface{}, method string) (interface{}, error) {
	switch method {
	case MethodToolsList:
		return ParseListToolsResult(result)
	//case MethodToolsCall:
	//return ParseCallToolResult(result)
	case MethodPromptsList:
		return ParseListPromptsResult(result)
	case MethodPromptsGet:
		return ParseGetPromptResult(result)
	case MethodResourcesList:
		return ParseListResourcesResult(result)
	case MethodResourcesRead:
		return ParseReadResourceResult(result)
	case MethodInitialize:
		// Use dedicated function to parse Initialize response
		return ParseInitializeResult(result)
	default:
		// For unknown method, return original result
		return result, nil
	}
}

// ParseInitializeResult parses Initialize response result
func ParseInitializeResult(result interface{}) (*InitializeResult, error) {
	// Convert result to bytes
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Initialize result: %w", err)
	}

	// Parse as InitializeResult structure
	var initResult InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		return nil, fmt.Errorf("failed to parse Initialize result: %w", err)
	}

	return &initResult, nil
}

// NewToolResultResponse creates a response containing tool call result
func NewToolResultResponse(id interface{}, result *CallToolResult) *JSONRPCResponse {
	responseData := map[string]interface{}{
		"content": result.Content,
	}

	// Automatically handle an error flag
	if result.IsError {
		responseData["isError"] = true
	}

	// If there is metadata, include it
	if result.Meta != nil {
		responseData["_meta"] = result.Meta
	}

	return NewJSONRPCResponse(id, responseData)
}
