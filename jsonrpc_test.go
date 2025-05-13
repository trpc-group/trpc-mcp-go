package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewJSONRPCRequest(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		id       interface{}
		method   string
		params   map[string]interface{}
		expected *JSONRPCRequest
	}{
		{
			name:   "Request without parameters",
			id:     1,
			method: "test.method",
			params: nil,
			expected: &JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Request: Request{
					Method: "test.method",
				},
				Params: nil,
			},
		},
		{
			name:   "Request with parameters",
			id:     "request-1",
			method: "tools/call",
			params: map[string]interface{}{
				"name":      "test-tool",
				"arguments": map[string]interface{}{"param1": "value1"},
			},
			expected: &JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      "request-1",
				Request: Request{
					Method: "tools/call",
				},
				Params: map[string]interface{}{
					"name":      "test-tool",
					"arguments": map[string]interface{}{"param1": "value1"},
				},
			},
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := NewJSONRPCRequest(tc.id, tc.method, tc.params)

			assert.Equal(t, tc.expected.JSONRPC, result.JSONRPC)
			assert.Equal(t, tc.expected.ID, result.ID)
			assert.Equal(t, tc.expected.Method, result.Method)
			assert.Equal(t, tc.expected.Params, result.Params)
		})
	}
}

func TestNewJSONRPCResponse(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		id       interface{}
		result   interface{}
		expected *JSONRPCResponse
	}{
		{
			name:   "Simple response",
			id:     1,
			result: "test-result",
			expected: &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Result:  "test-result",
			},
		},
		{
			name: "Complex response",
			id:   "response-1",
			result: map[string]interface{}{
				"data": []string{"item1", "item2"},
			},
			expected: &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      "response-1",
				Result: map[string]interface{}{
					"data": []string{"item1", "item2"},
				},
			},
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := NewJSONRPCResponse(tc.id, tc.result)

			assert.Equal(t, tc.expected.JSONRPC, result.JSONRPC)
			assert.Equal(t, tc.expected.ID, result.ID)
			assert.Equal(t, tc.expected.Result, result.Result)
		})
	}
}

func TestNewJSONRPCErrorResponse(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		id       interface{}
		code     int
		message  string
		data     interface{}
		expected *JSONRPCError
	}{
		{
			name:    "Simple error",
			id:      1,
			code:    ErrCodeInvalidRequest,
			message: "Invalid request",
			data:    nil,
			expected: &JSONRPCError{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Error: struct {
					Code    int         `json:"code"`
					Message string      `json:"message"`
					Data    interface{} `json:"data,omitempty"`
				}{
					Code:    ErrCodeInvalidRequest,
					Message: "Invalid request",
					Data:    nil,
				},
			},
		},
		{
			name:    "Error with data",
			id:      "error-1",
			code:    ErrCodeMethodNotFound,
			message: "Method not found",
			data:    "Requested method not found",
			expected: &JSONRPCError{
				JSONRPC: JSONRPCVersion,
				ID:      "error-1",
				Error: struct {
					Code    int         `json:"code"`
					Message string      `json:"message"`
					Data    interface{} `json:"data,omitempty"`
				}{
					Code:    ErrCodeMethodNotFound,
					Message: "Method not found",
					Data:    "Requested method not found",
				},
			},
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := NewJSONRPCErrorResponse(tc.id, tc.code, tc.message, tc.data)

			assert.Equal(t, tc.expected.JSONRPC, result.JSONRPC)
			assert.Equal(t, tc.expected.ID, result.ID)
			assert.Equal(t, tc.expected.Error.Code, result.Error.Code)
			assert.Equal(t, tc.expected.Error.Message, result.Error.Message)
			assert.Equal(t, tc.expected.Error.Data, result.Error.Data)
		})
	}

	// Test backward compatibility function
	t.Run("Backward compatibility", func(t *testing.T) {
		errResp1 := NewJSONRPCErrorResponse(1, ErrCodeInvalidRequest, "Invalid request", nil)
		errResp2 := NewJSONRPCErrorResponse(1, ErrCodeInvalidRequest, "Invalid request", nil)

		assert.Equal(t, errResp1.JSONRPC, errResp2.JSONRPC)
		assert.Equal(t, errResp1.ID, errResp2.ID)
		assert.Equal(t, errResp1.Error.Code, errResp2.Error.Code)
		assert.Equal(t, errResp1.Error.Message, errResp2.Error.Message)
	})
}

func TestNewJSONRPCNotification(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		method   string
		params   NotificationParams
		expected *JSONRPCNotification
	}{
		{
			name:   "Notification without parameters",
			method: "notifications/initialized",
			params: NotificationParams{
				Meta: nil,
			},
			expected: &JSONRPCNotification{
				JSONRPC: JSONRPCVersion,
				Notification: Notification{
					Method: "notifications/initialized",
					Params: NotificationParams{
						Meta:             nil,
						AdditionalFields: nil,
					},
				},
			},
		},
		{
			name:   "Notification with parameters",
			method: "utilities/logging",
			params: NotificationParams{
				Meta: nil,
				AdditionalFields: map[string]interface{}{
					"level": "info",
					"data":  "Test log message",
				},
			},
			expected: &JSONRPCNotification{
				JSONRPC: JSONRPCVersion,
				Notification: Notification{
					Method: "utilities/logging",
					Params: NotificationParams{
						Meta: nil,
						AdditionalFields: map[string]interface{}{
							"level": "info",
							"data":  "Test log message",
						},
					},
				},
			},
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputMap := make(map[string]interface{})
			if tc.params.Meta != nil {
				inputMap["_meta"] = tc.params.Meta
			}
			for k, v := range tc.params.AdditionalFields {
				inputMap[k] = v
			}
			result := NewJSONRPCNotificationFromMap(tc.method, inputMap)

			assert.Equal(t, tc.expected.JSONRPC, result.JSONRPC)
			assert.Equal(t, tc.expected.Method, result.Method)
			assert.Equal(t, tc.expected.Params, result.Params)
		})
	}
}
