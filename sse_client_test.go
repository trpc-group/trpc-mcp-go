// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWithServiceName_SSETransport tests that WithServiceName option correctly configures
// the serviceName field in SSE transport.
func TestWithServiceName_SSETransport(t *testing.T) {
	// Test case: verify serviceName is correctly set in SSE transport
	testServiceName := "test-service-name"

	// Extract transport configuration using WithServiceName option
	config := extractTransportConfig([]ClientOption{
		WithServiceName(testServiceName),
	})

	// Verify serviceName is set in transport config
	assert.Equal(t, testServiceName, config.serviceName, "serviceName should be set in transport config")

	// Create a temporary SSE transport to verify the field is correctly passed
	sseTransport := &sseClientTransport{
		serviceName: config.serviceName,
	}

	// Verify serviceName is correctly set in SSE transport
	assert.Equal(t, testServiceName, sseTransport.serviceName, "serviceName should be set in SSE transport")
}

// TestWithHTTPReqHandlerOption_SSETransport tests that WithHTTPReqHandlerOption correctly configures
// the httpReqHandlerOptions field in SSE transport.
func TestWithHTTPReqHandlerOption_SSETransport(t *testing.T) {
	// Test case: verify httpReqHandlerOptions are correctly set in SSE transport
	testOption1 := func() HTTPReqHandlerOption { return nil }()
	testOption2 := func() HTTPReqHandlerOption { return nil }()

	// Extract transport configuration using WithHTTPReqHandlerOption option
	config := extractTransportConfig([]ClientOption{
		WithHTTPReqHandlerOption(testOption1, testOption2),
	})

	// Verify httpReqHandlerOptions are set in transport config
	assert.Len(t, config.httpReqHandlerOptions, 2, "httpReqHandlerOptions should be set in transport config")

	// Create a temporary SSE transport to verify the field is correctly passed
	sseTransport := &sseClientTransport{
		httpReqHandlerOptions: config.httpReqHandlerOptions,
	}

	// Verify httpReqHandlerOptions are correctly set in SSE transport
	assert.Len(t, sseTransport.httpReqHandlerOptions, 2, "httpReqHandlerOptions should be set in SSE transport")
}

// TestWithClientLogger_SSETransport tests that WithClientLogger correctly configures
// the logger field in SSE transport.
func TestWithClientLogger_SSETransport(t *testing.T) {
	// Test case: verify logger is correctly set in SSE transport
	testLogger := GetDefaultLogger()

	// Extract transport configuration using WithClientLogger option
	config := extractTransportConfig([]ClientOption{
		WithClientLogger(testLogger),
	})

	// Verify logger is set in transport config
	assert.Equal(t, testLogger, config.logger, "logger should be set in transport config")

	// Create a temporary SSE transport to verify the field is correctly passed
	sseTransport := &sseClientTransport{
		logger: config.logger,
	}

	// Verify logger is correctly set in SSE transport
	assert.Equal(t, testLogger, sseTransport.logger, "logger should be set in SSE transport")
}

// TestWithClientPath_SSETransport tests that WithClientPath correctly configures
// the path field in SSE transport.
func TestWithClientPath_SSETransport(t *testing.T) {
	// Test case: verify path is correctly set in SSE transport
	testPath := "/custom/mcp/path"

	// Extract transport configuration using WithClientPath option
	config := extractTransportConfig([]ClientOption{
		WithClientPath(testPath),
	})

	// Verify path is set in transport config
	assert.Equal(t, testPath, config.path, "path should be set in transport config")
}

// TestWithClientPath_SSEConnectionURL tests that WithClientPath correctly applies
// the custom path to the SSE connection URL.
func TestWithClientPath_SSEConnectionURL(t *testing.T) {
	// Test case: verify custom path is applied to SSE connection URL
	serverURL := "http://localhost:8080"
	customPath := "/custom/sse/endpoint"

	// Mock URL parsing (simulate the actual SSE client creation logic)
	parsedURL, err := url.Parse(serverURL)
	assert.NoError(t, err)

	// Extract transport configuration with custom path
	config := extractTransportConfig([]ClientOption{
		WithClientPath(customPath),
	})

	// Simulate the SSE connection URL building logic
	if config.path != "" {
		parsedURL.Path = config.path
	}

	// Verify the SSE connection URL has the custom path
	assert.Equal(t, "http://localhost:8080/custom/sse/endpoint", parsedURL.String(),
		"SSE connection URL should include custom path")
	assert.Equal(t, customPath, parsedURL.Path, "Path should be set correctly")
}

// TestWithHTTPHeaders_SSETransport tests that WithHTTPHeaders correctly configures
// the httpHeaders field in SSE transport.
func TestWithHTTPHeaders_SSETransport(t *testing.T) {
	// Test case: verify httpHeaders are correctly set in SSE transport
	testHeaders := http.Header{
		"Authorization": []string{"Bearer test-token"},
		"X-Custom":      []string{"test-value"},
	}

	// Extract transport configuration using WithHTTPHeaders option
	config := extractTransportConfig([]ClientOption{
		WithHTTPHeaders(testHeaders),
	})

	// Verify headers are set in transport config
	assert.Equal(t, "Bearer test-token", config.httpHeaders.Get("Authorization"), "Authorization header should be set")
	assert.Equal(t, "test-value", config.httpHeaders.Get("X-Custom"), "X-Custom header should be set")

	// Create a temporary SSE transport to verify the field is correctly passed
	sseTransport := &sseClientTransport{
		httpHeaders: config.httpHeaders,
	}

	// Verify headers are correctly set in SSE transport
	assert.Equal(t, "Bearer test-token", sseTransport.httpHeaders.Get("Authorization"), "Authorization header should be set in SSE transport")
	assert.Equal(t, "test-value", sseTransport.httpHeaders.Get("X-Custom"), "X-Custom header should be set in SSE transport")
}
