// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataHandler(t *testing.T) {
	// Test metadata
	testMetadata := map[string]interface{}{
		"name":        "test-server",
		"version":     "1.0.0",
		"description": "Test MCP server",
		"capabilities": map[string]interface{}{
			"auth":  true,
			"tools": []interface{}{"test-tool-1", "test-tool-2"}, // JSON decoding converts to []interface{}
		},
	}

	// Create handler
	handler := MetadataHandler(testMetadata)

	// Verify handler is created successfully
	assert.NotNil(t, handler)

	// Test GET request
	req := httptest.NewRequest(http.MethodGet, "/metadata", nil)
	w := httptest.NewRecorder()

	// Execute request
	handler(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Verify JSON response
	var responseData map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&responseData)
	require.NoError(t, err)
	assert.Equal(t, testMetadata, responseData)
}

func TestMetadataHandler_MethodValidation(t *testing.T) {
	// Test metadata
	testMetadata := map[string]interface{}{
		"name": "test-server",
	}

	// Create handler
	handler := MetadataHandler(testMetadata)

	// Test cases for different HTTP methods
	testCases := []struct {
		name            string
		method          string
		expectedCode    int
		shouldHaveJSON  bool
		shouldHaveAllow bool
	}{
		{
			name:            "GET method allowed",
			method:          http.MethodGet,
			expectedCode:    http.StatusOK,
			shouldHaveJSON:  true,
			shouldHaveAllow: false,
		},
		{
			name:            "POST method not allowed",
			method:          http.MethodPost,
			expectedCode:    http.StatusMethodNotAllowed,
			shouldHaveJSON:  true,
			shouldHaveAllow: true,
		},
		{
			name:            "PUT method not allowed",
			method:          http.MethodPut,
			expectedCode:    http.StatusMethodNotAllowed,
			shouldHaveJSON:  true,
			shouldHaveAllow: true,
		},
		{
			name:            "DELETE method not allowed",
			method:          http.MethodDelete,
			expectedCode:    http.StatusMethodNotAllowed,
			shouldHaveJSON:  true,
			shouldHaveAllow: true,
		},
		{
			name:            "PATCH method not allowed",
			method:          http.MethodPatch,
			expectedCode:    http.StatusMethodNotAllowed,
			shouldHaveJSON:  true,
			shouldHaveAllow: true,
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/metadata", nil)
			w := httptest.NewRecorder()

			// Execute request
			handler(w, req)

			// Verify status code
			assert.Equal(t, tc.expectedCode, w.Code)

			// Verify Content-Type header
			if tc.shouldHaveJSON {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			}

			// Verify Allow header for method not allowed responses
			if tc.shouldHaveAllow {
				allowHeader := w.Header().Get("Allow")
				assert.Contains(t, allowHeader, "GET")
			}

			// For successful requests, verify metadata is returned
			if tc.expectedCode == http.StatusOK {
				var responseData map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&responseData)
				require.NoError(t, err)
				assert.Equal(t, testMetadata, responseData)
			}
		})
	}
}

func TestMetadataHandler_CORSHeaders(t *testing.T) {
	// Test metadata
	testMetadata := map[string]interface{}{
		"name": "test-server",
	}

	// Create handler
	handler := MetadataHandler(testMetadata)

	// Test cases for CORS handling
	testCases := []struct {
		name                  string
		method                string
		origin                string
		expectedCode          int
		shouldHaveCORSOrigin  bool
		shouldHaveCORSMethods bool
	}{
		{
			name:                  "Non-CORS request",
			method:                http.MethodGet,
			origin:                "",
			expectedCode:          http.StatusOK,
			shouldHaveCORSOrigin:  false,
			shouldHaveCORSMethods: false,
		},
		{
			name:                  "CORS GET request",
			method:                http.MethodGet,
			origin:                "https://example.com",
			expectedCode:          http.StatusOK,
			shouldHaveCORSOrigin:  true,
			shouldHaveCORSMethods: true,
		},
		{
			name:                  "CORS OPTIONS preflight request",
			method:                http.MethodOptions,
			origin:                "https://example.com",
			expectedCode:          http.StatusNoContent,
			shouldHaveCORSOrigin:  true,
			shouldHaveCORSMethods: true,
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/metadata", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()

			// Execute request
			handler(w, req)

			// Verify status code
			assert.Equal(t, tc.expectedCode, w.Code)

			// Verify CORS headers
			if tc.shouldHaveCORSOrigin {
				assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}

			if tc.shouldHaveCORSMethods {
				allowMethods := w.Header().Get("Access-Control-Allow-Methods")
				assert.Equal(t, "GET,HEAD,PUT,PATCH,POST,DELETE", allowMethods)
			}

			// For OPTIONS requests, verify Content-Length header
			if tc.method == http.MethodOptions {
				assert.Equal(t, "0", w.Header().Get("Content-Length"))
			}

			// For successful GET requests, verify metadata is returned
			if tc.method == http.MethodGet && tc.expectedCode == http.StatusOK {
				var responseData map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&responseData)
				require.NoError(t, err)
				assert.Equal(t, testMetadata, responseData)
			}
		})
	}
}

func TestMetadataHandler_DifferentMetadataTypes(t *testing.T) {
	// Test cases with different metadata types
	testCases := []struct {
		name     string
		metadata interface{}
	}{
		{
			name:     "String metadata",
			metadata: "simple string metadata",
		},
		{
			name:     "Number metadata",
			metadata: float64(42),
		},
		{
			name:     "Boolean metadata",
			metadata: true,
		},
		{
			name:     "Array metadata",
			metadata: []interface{}{"item1", "item2", "item3"},
		},
		{
			name: "Complex object metadata",
			metadata: map[string]interface{}{
				"server": map[string]interface{}{
					"name":    "mcp-server",
					"version": "2.0.0",
					"config": map[string]interface{}{
						"debug": true,
						"port":  float64(8080),
					},
				},
				"capabilities": []interface{}{"auth", "tools", "resources"},
				"stats": map[string]interface{}{
					"uptime":   float64(3600),
					"requests": float64(1500),
					"errors":   float64(5),
				},
			},
		},
		{
			name:     "Empty metadata",
			metadata: map[string]interface{}{},
		},
		{
			name:     "Nil metadata",
			metadata: nil,
		},
	}

	// Execute tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create handler with test metadata
			handler := MetadataHandler(tc.metadata)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/metadata", nil)
			w := httptest.NewRecorder()

			// Execute request
			handler(w, req)

			// Verify response
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			// Verify JSON response matches expected metadata
			var responseData interface{}
			err := json.NewDecoder(w.Body).Decode(&responseData)
			require.NoError(t, err)
			assert.Equal(t, tc.metadata, responseData)
		})
	}
}

func TestMetadataHandler_ConcurrentRequests(t *testing.T) {
	// Test metadata
	testMetadata := map[string]interface{}{
		"name":    "concurrent-test-server",
		"version": "1.0.0",
	}

	// Create handler
	handler := MetadataHandler(testMetadata)

	// Number of concurrent requests
	numRequests := 10
	responses := make(chan *httptest.ResponseRecorder, numRequests)

	// Launch concurrent requests
	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/metadata", nil)
			w := httptest.NewRecorder()
			handler(w, req)
			responses <- w
		}()
	}

	// Collect and verify all responses
	for i := 0; i < numRequests; i++ {
		w := <-responses

		// Verify response
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		// Verify JSON response
		var responseData map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&responseData)
		require.NoError(t, err)
		assert.Equal(t, testMetadata, responseData)
	}
}
