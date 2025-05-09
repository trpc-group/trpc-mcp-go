package schema

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
				Method:  "test.method",
				Params:  nil,
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
				Method:  "tools/call",
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

	// Test backward compatibility function
	t.Run("Backward compatibility", func(t *testing.T) {
		req1 := NewJSONRPCRequest(1, "test.method", nil)
		req2 := NewRequest(1, "test.method", nil)

		assert.Equal(t, req1.JSONRPC, req2.JSONRPC)
		assert.Equal(t, req1.ID, req2.ID)
		assert.Equal(t, req1.Method, req2.Method)
		assert.Equal(t, req1.Params, req2.Params)
	})
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
				Error:   nil,
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
				Error: nil,
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
			assert.Nil(t, result.Error)
		})
	}

	// Test backward compatibility function
	t.Run("Backward compatibility", func(t *testing.T) {
		resp1 := NewJSONRPCResponse(1, "test-result")
		resp2 := NewResponse(1, "test-result")

		assert.Equal(t, resp1.JSONRPC, resp2.JSONRPC)
		assert.Equal(t, resp1.ID, resp2.ID)
		assert.Equal(t, resp1.Result, resp2.Result)
	})
}

func TestNewJSONRPCErrorResponse(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		id       interface{}
		code     int
		message  string
		data     interface{}
		expected *JSONRPCResponse
	}{
		{
			name:    "Simple error",
			id:      1,
			code:    ErrInvalidRequest,
			message: "Invalid request",
			data:    nil,
			expected: &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Result:  nil,
				Error: &JSONRPCError{
					Code:    ErrInvalidRequest,
					Message: "Invalid request",
					Data:    nil,
				},
			},
		},
		{
			name:    "Error with data",
			id:      "error-1",
			code:    ErrMethodNotFound,
			message: "Method not found",
			data:    "Requested method not found",
			expected: &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      "error-1",
				Result:  nil,
				Error: &JSONRPCError{
					Code:    ErrMethodNotFound,
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
			assert.Nil(t, result.Result)
			assert.Equal(t, tc.expected.Error.Code, result.Error.Code)
			assert.Equal(t, tc.expected.Error.Message, result.Error.Message)
			assert.Equal(t, tc.expected.Error.Data, result.Error.Data)
		})
	}

	// Test backward compatibility function
	t.Run("Backward compatibility", func(t *testing.T) {
		errResp1 := NewJSONRPCErrorResponse(1, ErrInvalidRequest, "Invalid request", nil)
		errResp2 := NewErrorResponse(1, ErrInvalidRequest, "Invalid request", nil)

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
		params   map[string]interface{}
		expected *JSONRPCNotification
	}{
		{
			name:   "Notification without parameters",
			method: "notifications/initialized",
			params: nil,
			expected: &JSONRPCNotification{
				JSONRPC: JSONRPCVersion,
				Method:  "notifications/initialized",
				Params:  nil,
			},
		},
		{
			name:   "Notification with parameters",
			method: "utilities/logging",
			params: map[string]interface{}{
				"level": "info",
				"data":  "Test log message",
			},
			expected: &JSONRPCNotification{
				JSONRPC: JSONRPCVersion,
				Method:  "utilities/logging",
				Params: map[string]interface{}{
					"level": "info",
					"data":  "Test log message",
				},
			},
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := NewJSONRPCNotification(tc.method, tc.params)

			assert.Equal(t, tc.expected.JSONRPC, result.JSONRPC)
			assert.Equal(t, tc.expected.Method, result.Method)
			assert.Equal(t, tc.expected.Params, result.Params)
		})
	}

	// Test backward compatibility function
	t.Run("Backward compatibility", func(t *testing.T) {
		notif1 := NewJSONRPCNotification("notifications/initialized", nil)
		notif2 := NewNotification("notifications/initialized", nil)

		assert.Equal(t, notif1.JSONRPC, notif2.JSONRPC)
		assert.Equal(t, notif1.Method, notif2.Method)
		assert.Equal(t, notif1.Params, notif2.Params)
	})
}
