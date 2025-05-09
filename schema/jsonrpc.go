package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// JSONRPCRequest represents a JSON-RPC request
// Conforms to the JSONRPCRequest definition in schema.json
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC response
// Conforms to the JSONRPCResponse definition in schema.json
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCNotification represents a JSON-RPC notification
// Conforms to the JSONRPCNotification definition in schema.json
type JSONRPCNotification struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
// Conforms to the JSONRPCError definition in schema.json
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Backward compatibility type aliases
// Note: These type aliases will be removed in future versions, please use the types with JSONRPC prefix
type Request = JSONRPCRequest
type Response = JSONRPCResponse
type Notification = JSONRPCNotification
type Error = JSONRPCError

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

// JSONRPCMessageType represents the type of a JSON-RPC message
type JSONRPCMessageType string

const (
	JSONRPCMessageTypeRequest      JSONRPCMessageType = "request"
	JSONRPCMessageTypeResponse     JSONRPCMessageType = "response"
	JSONRPCMessageTypeNotification JSONRPCMessageType = "notification"
	JSONRPCMessageTypeError        JSONRPCMessageType = "error"
	JSONRPCMessageTypeUnknown      JSONRPCMessageType = "unknown"
)

// NewJSONRPCRequest creates a new JSON-RPC request
func NewJSONRPCRequest(id interface{}, method string, params map[string]interface{}) *JSONRPCRequest {
	return &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
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
func NewJSONRPCErrorResponse(id interface{}, code int, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// NewJSONRPCNotification creates a new JSON-RPC notification
func NewJSONRPCNotification(method string, params map[string]interface{}) *JSONRPCNotification {
	return &JSONRPCNotification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	}
}

// Backward compatibility function aliases
var (
	// NewRequest is a backward compatibility alias for NewJSONRPCRequest
	NewRequest = NewJSONRPCRequest
	// NewResponse is a backward compatibility alias for NewJSONRPCResponse
	NewResponse = NewJSONRPCResponse
	// NewErrorResponse is a backward compatibility alias for NewJSONRPCErrorResponse
	NewErrorResponse = NewJSONRPCErrorResponse
	// NewNotification is a backward compatibility alias for NewJSONRPCNotification
	NewNotification = NewJSONRPCNotification
)

// ParseJSONRPCMessageType determines the type of a JSON-RPC message
func ParseJSONRPCMessageType(data []byte) (JSONRPCMessageType, error) {
	// First try to parse as a generic map to determine message type
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		return JSONRPCMessageTypeUnknown, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
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

	return JSONRPCMessageTypeUnknown, nil
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
		var resp Response
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, msgType, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
		}
		return &resp, msgType, nil

	case JSONRPCMessageTypeError:
		var respError Response // Error also uses Response type, just with Error field set
		if err := json.Unmarshal(data, &respError); err != nil {
			return nil, msgType, fmt.Errorf("failed to parse JSON-RPC error response: %w", err)
		}
		return &respError, msgType, nil

	case JSONRPCMessageTypeNotification:
		var notification Notification
		if err := json.Unmarshal(data, &notification); err != nil {
			return nil, msgType, fmt.Errorf("failed to parse JSON-RPC notification: %w", err)
		}
		return &notification, msgType, nil

	case JSONRPCMessageTypeRequest:
		var request Request
		if err := json.Unmarshal(data, &request); err != nil {
			return nil, msgType, fmt.Errorf("failed to parse JSON-RPC request: %w", err)
		}
		return &request, msgType, nil

	default:
		return nil, JSONRPCMessageTypeUnknown, fmt.Errorf("unknown JSON-RPC message type")
	}
}

// IsResponseForRequest checks if a response corresponds to a specific request
func IsResponseForRequest(resp *Response, reqID interface{}) bool {
	// Use string comparison to handle different types (numbers or strings)
	return fmt.Sprintf("%v", resp.ID) == fmt.Sprintf("%v", reqID)
}

// FormatJSONRPCMessage returns a description of a JSON-RPC message (for logging)
func FormatJSONRPCMessage(msg interface{}) string {
	switch m := msg.(type) {
	case *Response:
		if m.Error != nil {
			return fmt.Sprintf("Error response(ID=%v, Code=%d): %s",
				m.ID, m.Error.Code, m.Error.Message)
		}
		return fmt.Sprintf("Response(ID=%v)", m.ID)
	case *Notification:
		return fmt.Sprintf("Notification(Method=%s)", m.Method)
	case *Request:
		return fmt.Sprintf("Request(ID=%v, Method=%s)", m.ID, m.Method)
	default:
		return "Unknown message type"
	}
}

// ParseJSONRequest parses a JSON-RPC request from the request body
func ParseJSONRequest(body io.ReadCloser, request *Request) error {
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
func GetMethodFromRequest(req *Request) string {
	return req.Method
}

// IsSuccessResponse checks if a response indicates success
func IsSuccessResponse(resp *Response) bool {
	return resp != nil && resp.Error == nil && resp.Result != nil
}

// IsErrorResponse checks if a response indicates error
func IsErrorResponse(resp *Response) bool {
	return resp != nil && resp.Error != nil
}

// ParseJSONRPCResponse parses JSON-RPC response based on method
func ParseJSONRPCResponse(resp *Response, method string) (interface{}, error) {
	// Check error
	if IsErrorResponse(resp) {
		return nil, fmt.Errorf("JSON-RPC error response (Code=%d): %s",
			resp.Error.Code, resp.Error.Message)
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
	case MethodToolsCall:
		return ParseCallToolResult(result)
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
func NewToolResultResponse(id interface{}, result *CallToolResult) *Response {
	responseData := map[string]interface{}{
		"content": result.Content,
	}

	// Automatically handle error flag
	if result.IsError {
		responseData["isError"] = true
	}

	// If there is metadata, include it
	if result.Meta != nil && result.Meta.AdditionalData != nil {
		responseData["_meta"] = result.Meta.AdditionalData
	}

	return NewResponse(id, responseData)
}
