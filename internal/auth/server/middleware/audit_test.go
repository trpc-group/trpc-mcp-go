// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
)

// contains checks whether a slice of strings contains a specific item
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// captureLogger is a mock implementation of AuditLogger
// It captures the last logged AuditEvent for inspection in tests
type captureLogger struct {
	last AuditEvent
}

// LogEvent stores the provided event in captureLogger
func (c *captureLogger) LogEvent(e AuditEvent) error {
	c.last = e
	return nil
}

// LogError stores the event along with the error message in captureLogger
func (c *captureLogger) LogError(e AuditEvent, err error) error {
	e.ErrorMessage = err.Error()
	c.last = e
	return nil
}

func TestAuditLevelConstants(t *testing.T) {
	// Test audit level constant value
	if AuditLevelNone != 0 {
		t.Errorf("Expected AuditLevelNone to be 0, got %d", AuditLevelNone)
	}
	if AuditLevelBasic != 1 {
		t.Errorf("Expected AuditLevelBasic to be 1, got %d", AuditLevelBasic)
	}
	if AuditLevelDetailed != 2 {
		t.Errorf("Expected AuditLevelDetailed to be 2, got %d", AuditLevelDetailed)
	}
	if AuditLevelFull != 3 {
		t.Errorf("Expected AuditLevelFull to be 3, got %d", AuditLevelFull)
	}
}

func TestNewAuditLogger(t *testing.T) {
	// Test create default logger
	logger := NewAuditLogger(nil)
	if logger == nil {
		t.Fatal("Expected logger to be created")
	}

	// Test to get the underlying zap logger
	zapLogger := logger.GetZapLogger()
	if zapLogger == nil {
		t.Fatal("Expected underlying zap logger to exist")
	}

	// Testing using a custom zap logger
	testLogger := zaptest.NewLogger(t)
	customLogger := NewAuditLogger(testLogger)
	if customLogger == nil {
		t.Fatal("Expected custom logger to be created")
	}

	if customLogger.GetZapLogger() != testLogger {
		t.Fatal("Expected custom zap logger to be used")
	}
}

func TestDefaultAuditLoggerLogEvent(t *testing.T) {
	// Creating a test logger
	testLogger := zaptest.NewLogger(t)
	auditLogger := NewAuditLogger(testLogger)

	// Creating a test event
	event := AuditEvent{
		EventID:      "test_123",
		Timestamp:    time.Now(),
		EventType:    "test_event",
		AuditLevel:   AuditLevelBasic,
		Method:       "GET",
		Path:         "/test",
		StatusCode:   200,
		ResponseTime: 100 * time.Millisecond,
		ClientID:     "test_client",
		Subject:      "test_user",
		Scopes:       []string{"read", "write"},
		RiskLevel:    "low",
		RiskFactors:  []string{"normal"},
	}

	// Test logging
	err := auditLogger.LogEvent(event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestDefaultAuditLoggerLogError(t *testing.T) {
	testLogger := zaptest.NewLogger(t)
	auditLogger := NewAuditLogger(testLogger)

	event := AuditEvent{
		EventID:   "test_123",
		Timestamp: time.Now(),
		Method:    "GET",
		Path:      "/test",
	}

	testErr := &http.MaxBytesError{}
	err := auditLogger.LogError(event, testErr)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Note: Since the event is passed by value, the ErrorMessage is not modified.
	// This test mainly verifies that the LogError method does not return an error.
}

func TestDefaultAuditMiddlewareOptions(t *testing.T) {
	options := DefaultAuditMiddlewareOptions()

	// Verify default values
	if options.Logger == nil {
		t.Error("Expected logger to be set")
	}
	if options.Level != AuditLevelDetailed {
		t.Errorf("Expected level to be Detailed, got %v", options.Level)
	}
	if !options.HashSensitiveData {
		t.Error("Expected HashSensitiveData to be true")
	}
	if len(options.EndpointPatterns) == 0 {
		t.Error("Expected endpoint patterns to be set")
	}
	if len(options.SensitiveKeys) == 0 {
		t.Error("Expected sensitive keys to be set")
	}
}

func TestAuditOptionsBuilder(t *testing.T) {
	builder := NewAuditOptionsBuilder()

	// Testing chain calls
	options := builder.
		WithLevel(AuditLevelFull).
		WithHashSensitiveData(false).
		WithRequestBody(true).
		WithResponseBody(true).
		Build()

	if options.Level != AuditLevelFull {
		t.Errorf("Expected level to be Full, got %v", options.Level)
	}
	if options.HashSensitiveData {
		t.Error("Expected HashSensitiveData to be false")
	}
	if !options.IncludeRequestBody {
		t.Error("Expected IncludeRequestBody to be true")
	}
	if !options.IncludeResponseBody {
		t.Error("Expected IncludeResponseBody to be true")
	}
}

func TestAuditOptionsBuilderWithCustomFunctions(t *testing.T) {
	builder := NewAuditOptionsBuilder()

	// Custom Risk Assessor
	customRiskAssessor := func(event AuditEvent) (string, []string) {
		return "high", []string{"custom_risk"}
	}

	// Custom metadata extractors
	customMetadataExtractor := func(r *http.Request) map[string]interface{} {
		return map[string]interface{}{
			"custom_field": "custom_value",
		}
	}

	options := builder.
		WithRiskAssessor(customRiskAssessor).
		WithMetadataExtractor(customMetadataExtractor).
		Build()

	if options.RiskAssessor == nil {
		t.Error("Expected RiskAssessor to be set")
	}
	if options.MetadataExtractor == nil {
		t.Error("Expected MetadataExtractor to be set")
	}

	// Testing custom functions
	event := AuditEvent{}
	riskLevel, riskFactors := options.RiskAssessor(event)
	if riskLevel != "high" {
		t.Errorf("Expected risk level 'high', got %s", riskLevel)
	}
	if len(riskFactors) != 1 || riskFactors[0] != "custom_risk" {
		t.Errorf("Expected risk factors ['custom_risk'], got %v", riskFactors)
	}
}

func TestValidateOptions(t *testing.T) {
	// Testing a valid configuration
	validOptions := &AuditMiddlewareOptions{
		EndpointPatterns: []string{"/test"},
	}
	if err := validateOptions(validOptions); err != nil {
		t.Errorf("Expected no error for valid options, got %v", err)
	}

	// Testing for invalid configurations
	invalidOptions := &AuditMiddlewareOptions{
		EndpointPatterns: []string{},
		ExcludePatterns:  []string{},
	}
	if err := validateOptions(invalidOptions); err == nil {
		t.Error("Expected error for invalid options")
	}
}

func TestShouldAuditPath(t *testing.T) {
	tests := []struct {
		path            string
		includePatterns []string
		excludePatterns []string
		expectedResult  bool
		description     string
	}{
		{
			path:            "/oauth2/authorize",
			includePatterns: []string{"/oauth2/.*"},
			excludePatterns: []string{},
			expectedResult:  true,
			description:     "Path matches include pattern",
		},
		{
			path:            "/health",
			includePatterns: []string{"/oauth2/.*"},
			excludePatterns: []string{},
			expectedResult:  false,
			description:     "Path doesn't match include pattern",
		},
		{
			path:            "/oauth2/token",
			includePatterns: []string{"/oauth2/.*"},
			excludePatterns: []string{"/oauth2/token"},
			expectedResult:  false,
			description:     "Path matches exclude pattern",
		},
		{
			path:            "/any/path",
			includePatterns: []string{},
			excludePatterns: []string{},
			expectedResult:  true,
			description:     "No patterns specified, audit all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := shouldAuditPath(tt.path, tt.includePatterns, tt.excludePatterns)
			if result != tt.expectedResult {
				t.Errorf("shouldAuditPath(%q, %v, %v) = %v, want %v",
					tt.path, tt.includePatterns, tt.excludePatterns, result, tt.expectedResult)
			}
		})
	}
}

func TestDetermineEventType(t *testing.T) {
	tests := []struct {
		path     string
		method   string
		expected string
	}{
		{"/oauth2/authorize", "GET", "oauth_authorization"},
		{"/oauth2/token", "POST", "oauth_token"},
		{"/oauth2/revoke", "POST", "oauth_revocation"},
		{"/oauth2/register", "POST", "oauth_registration"},
		{"/oauth2/metadata", "GET", "oauth_metadata"},
		{"/unknown/path", "GET", "oauth_request"},
	}

	for _, tt := range tests {
		result := determineEventType(tt.path, tt.method)
		if result != tt.expected {
			t.Errorf("determineEventType(%q, %q) = %q, want %q",
				tt.path, tt.method, result, tt.expected)
		}
	}
}

func TestGenerateEventID(t *testing.T) {
	id1 := generateEventID()
	id2 := generateEventID()

	if id1 == id2 {
		t.Error("Expected different event IDs")
	}

	if !strings.HasPrefix(id1, "audit_") {
		t.Errorf("Expected event ID to start with 'audit_', got %s", id1)
	}
}

func TestRandomString(t *testing.T) {
	str1 := randomString(10)
	str2 := randomString(10)

	if len(str1) != 10 {
		t.Errorf("Expected string length 10, got %d", len(str1))
	}

	if str1 == str2 {
		t.Error("Expected different random strings")
	}
}

func TestSanitizeMap(t *testing.T) {
	// Test query parameter sanitization
	queryParams := map[string][]string{
		"client_id":     {"test_client"},
		"client_secret": {"secret_value"},
		"scope":         {"read write"},
	}

	sensitiveKeys := []string{"client_secret", "password"}

	sanitized := sanitizeQueryParams(queryParams, sensitiveKeys)

	if sanitized["client_id"] != "test_client" {
		t.Errorf("Expected client_id to be preserved, got %s", sanitized["client_id"])
	}

	if sanitized["client_secret"] != "[REDACTED]" {
		t.Errorf("Expected client_secret to be redacted, got %s", sanitized["client_secret"])
	}

	if sanitized["scope"] != "read write" {
		t.Errorf("Expected scope to be preserved, got %s", sanitized["scope"])
	}
}

func TestSanitizeHeaders(t *testing.T) {
	headers := map[string][]string{
		"content-type":  {"application/json"},
		"authorization": {"Bearer token123"},
		"user-agent":    {"test-agent"},
	}

	sensitiveKeys := []string{"authorization", "cookie"}

	sanitized := sanitizeHeaders(headers, sensitiveKeys)

	if sanitized["content-type"] != "application/json" {
		t.Errorf("Expected content-type to be preserved, got %s", sanitized["content-type"])
	}

	if sanitized["authorization"] != "[REDACTED]" {
		t.Errorf("Expected authorization to be redacted, got %s", sanitized["authorization"])
	}

	if sanitized["user-agent"] != "test-agent" {
		t.Errorf("Expected user-agent to be preserved, got %s", sanitized["user-agent"])
	}
}

func TestHashSensitiveData(t *testing.T) {
	data := "sensitive_data"
	hash1 := hashSensitiveData(data)
	hash2 := hashSensitiveData(data)

	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}

	if hash1 != hash2 {
		t.Error("Expected same hash for same data")
	}

	if hashSensitiveData("") != "" {
		t.Error("Expected empty string for empty data")
	}
}

func TestDefaultRiskAssessment(t *testing.T) {
	tests := []struct {
		name         string
		event        AuditEvent
		expectedRisk string
		checkFactors func([]string) bool
	}{
		{
			name: "Normal request",
			event: AuditEvent{
				StatusCode:   200,
				ResponseTime: 100 * time.Millisecond,
				ClientID:     "test_client",
			},
			expectedRisk: "low",
			checkFactors: func(factors []string) bool {
				return len(factors) == 0
			},
		},
		{
			name: "Client error",
			event: AuditEvent{
				StatusCode:   400,
				ResponseTime: 100 * time.Millisecond,
				ClientID:     "test_client",
			},
			expectedRisk: "low",
			checkFactors: func(factors []string) bool {
				return contains(factors, "client_error")
			},
		},
		{
			name: "Server error",
			event: AuditEvent{
				StatusCode:   500,
				ResponseTime: 100 * time.Millisecond,
				ClientID:     "test_client",
			},
			expectedRisk: "medium",
			checkFactors: func(factors []string) bool {
				return contains(factors, "server_error")
			},
		},
		{
			name: "Slow response",
			event: AuditEvent{
				StatusCode:   200,
				ResponseTime: 6 * time.Second,
				ClientID:     "test_client",
			},
			expectedRisk: "medium",
			checkFactors: func(factors []string) bool {
				return contains(factors, "slow_response")
			},
		},
		{
			name: "Missing client ID",
			event: AuditEvent{
				StatusCode:   200,
				ResponseTime: 100 * time.Millisecond,
				ClientID:     "",
			},
			expectedRisk: "high",
			checkFactors: func(factors []string) bool {
				return contains(factors, "missing_client_id")
			},
		},
		{
			name: "Token revocation",
			event: AuditEvent{
				StatusCode:   200,
				ResponseTime: 100 * time.Millisecond,
				ClientID:     "test_client",
				Path:         "/oauth2/revoke",
			},
			expectedRisk: "medium",
			checkFactors: func(factors []string) bool {
				return contains(factors, "token_revocation")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			riskLevel, riskFactors := defaultRiskAssessment(tt.event)
			if riskLevel != tt.expectedRisk {
				t.Errorf("Expected risk level %s, got %s", tt.expectedRisk, riskLevel)
			}
			if !tt.checkFactors(riskFactors) {
				t.Errorf("Risk factors check failed for factors: %v", riskFactors)
			}
		})
	}
}

func TestDetermineErrorCode(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{400, "invalid_request"},
		{401, "invalid_token"},
		{403, "insufficient_scope"},
		{404, "not_found"},
		{429, "too_many_requests"},
		{500, "server_error"},
		{999, "server_error"}, // 999 is still considered a server error
	}

	for _, tt := range tests {
		result := determineErrorCode(tt.statusCode)
		if result != tt.expected {
			t.Errorf("determineErrorCode(%d) = %s, want %s", tt.statusCode, result, tt.expected)
		}
	}
}

func TestDetermineErrorMessage(t *testing.T) {
	// Testing JSON error responses
	jsonError := `{"error": "invalid_grant", "error_description": "Invalid authorization code"}`
	message := determineErrorMessage(400, []byte(jsonError))
	if message != "Invalid authorization code" {
		t.Errorf("Expected 'Invalid authorization code', got %s", message)
	}

	// Testing responses with only an error field
	jsonErrorOnly := `{"error": "invalid_request"}`
	message = determineErrorMessage(400, []byte(jsonErrorOnly))
	if message != "invalid_request" {
		t.Errorf("Expected 'invalid_request', got %s", message)
	}

	// Testing for an empty response body
	message = determineErrorMessage(404, []byte{})
	if message != "" {
		t.Errorf("Expected empty message for empty body, got %s", message)
	}

	// Testing for invalid JSON
	invalidJSON := `{invalid json}`
	message = determineErrorMessage(500, []byte(invalidJSON))
	if message != "Internal Server Error" {
		t.Errorf("Expected 'Internal Server Error', got %s", message)
	}
}

func TestExtractOAuthInfo(t *testing.T) {
	// Creating a test request
	req := httptest.NewRequest("POST", "/oauth2/token?client_id=test_client&scope=read+write", strings.NewReader("grant_type=authorization_code&code=test_code"))
	req.Header.Set("Authorization", "Bearer test_token")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Set the authentication information in the context
	ctx := server.WithAuthInfo(req.Context(), &server.AuthInfo{
		Scopes: []string{"read", "write"},
		Extra: map[string]interface{}{
			"sub":       "test_user",
			"client_id": "ctx_client_id",
		},
	})
	req = req.WithContext(ctx)

	info := extractOAuthInfo(req)

	// Verify the extracted information
	if info.ClientID != "test_client" {
		t.Errorf("Expected client_id 'test_client', got %s", info.ClientID)
	}

	// Note: Since ParseForm may not work in some test environments, we mainly test URL parameters and headers
	if info.Token != "test_token" {
		t.Errorf("Expected token 'test_token', got %s", info.Token)
	}

	if len(info.Scopes) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(info.Scopes))
	}

	if info.Subject != "test_user" {
		t.Errorf("Expected subject 'test_user', got %s", info.Subject)
	}
}

func TestExtractSubject(t *testing.T) {
	authInfo := server.AuthInfo{
		Extra: map[string]interface{}{
			"sub":   "test_user",
			"other": "value",
		},
	}

	subject := extractSubject(authInfo)
	if subject != "test_user" {
		t.Errorf("Expected subject 'test_user', got %s", subject)
	}

	// Testing without a subject
	authInfoNoSub := server.AuthInfo{
		Extra: map[string]interface{}{
			"other": "value",
		},
	}

	subject = extractSubject(authInfoNoSub)
	if subject != "" {
		t.Errorf("Expected empty subject, got %s", subject)
	}
}

func TestAuditMiddlewareBasic(t *testing.T) {
	// Create basic audit middleware
	middleware := WithBasicAudit()

	// Create a Test Handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Package handler
	wrappedHandler := middleware(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/oauth2/authorize?client_id=test_client", nil)
	w := httptest.NewRecorder()

	// Execute Request
	wrappedHandler.ServeHTTP(w, req)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuditMiddlewareDetailed(t *testing.T) {
	// Create detailed audit middleware
	middleware := WithDetailedAudit()

	// Create a Test Handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Package handler
	wrappedHandler := middleware(testHandler)

	// Create a test request
	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader("grant_type=client_credentials"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "test-agent")

	w := httptest.NewRecorder()

	// Execute Request
	wrappedHandler.ServeHTTP(w, req)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuditMiddlewareFull(t *testing.T) {
	// Create full audit middleware
	middleware := WithFullAudit()

	// Create a Test Handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Package handler
	wrappedHandler := middleware(testHandler)

	// Create a test request
	req := httptest.NewRequest("DELETE", "/oauth2/revoke", strings.NewReader("token=test_token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuditMiddlewareWithCustomZapLogger(t *testing.T) {
	// Create a custom zap logger
	testLogger := zaptest.NewLogger(t)

	// Create audit middleware with a custom logger
	options := WithCustomZapLogger(testLogger, AuditLevelDetailed, true)
	middleware := AuditMiddleware(options)

	// Create a Test Handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Package handler
	wrappedHandler := middleware(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/oauth2/metadata", nil)
	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuditMiddlewareExcludePatterns(t *testing.T) {
	// Create custom configured audit middleware
	options := NewAuditOptionsBuilder().
		WithEndpointPatterns([]string{"/oauth2/.*"}).
		WithExcludePatterns([]string{"/oauth2/health"}).
		Build()

	middleware := AuditMiddleware(options)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Package handler
	wrappedHandler := middleware(testHandler)

	// Test paths that should be audited
	req1 := httptest.NewRequest("GET", "/oauth2/authorize", nil)
	w1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 for audited path, got %d", w1.Code)
	}

	// Testing paths that should be excluded
	req2 := httptest.NewRequest("GET", "/oauth2/health", nil)
	w2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for excluded path, got %d", w2.Code)
	}
}

func TestAuditMiddlewareErrorHandling(t *testing.T) {
	// Create a test logger
	testLogger := zaptest.NewLogger(t)

	// Create test audit middleware
	options := WithZapLogger(testLogger)
	middleware := AuditMiddleware(options)

	// Creating a test handler that returns an error
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse := map[string]string{
			"error":             "invalid_request",
			"error_description": "Missing required parameter",
		}
		json.NewEncoder(w).Encode(errorResponse)
	})

	// Package handler
	wrappedHandler := middleware(testHandler)

	// Create a test request
	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Validate request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAuditMiddlewarePerformance(t *testing.T) {
	// Creating audit middleware for performance testing
	options := NewAuditOptionsBuilder().
		WithLevel(AuditLevelBasic).
		WithHashSensitiveData(false).
		Build()

	middleware := AuditMiddleware(options)

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulation processing time
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Package handler
	wrappedHandler := middleware(testHandler)

	// Creating a test request
	req := httptest.NewRequest("GET", "/oauth2/authorize", nil)
	w := httptest.NewRecorder()

	// Execute request
	start := time.Now()
	wrappedHandler.ServeHTTP(w, req)
	duration := time.Since(start)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify performance
	if duration > 200*time.Millisecond {
		t.Errorf("Expected reasonable performance, took %v", duration)
	}
}

func TestAuditResponseWriter(t *testing.T) {
	// Creating a Test Response Recorder
	recorder := httptest.NewRecorder()

	// Creating an Audit Response Writer
	auditWriter := &auditResponseWriter{
		ResponseWriter: recorder,
		body:           make([]byte, 0),
	}

	// Test write header
	auditWriter.WriteHeader(http.StatusCreated)
	if auditWriter.statusCode != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, auditWriter.statusCode)
	}

	// Test writing data
	testData := []byte("test response")
	written, err := auditWriter.Write(testData)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if written != len(testData) {
		t.Errorf("Expected written bytes %d, got %d", len(testData), written)
	}

	// Verification status code is set
	if auditWriter.statusCode == 0 {
		auditWriter.statusCode = http.StatusOK
	}

	// Verify that the response body is captured
	if len(auditWriter.body) == 0 {
		t.Error("Expected response body to be captured")
	}
}

func BenchmarkAuditMiddleware(b *testing.B) {
	// Creating basic audit middleware
	middleware := WithBasicAudit()

	// Creating a Test Handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Packaging Processor
	wrappedHandler := middleware(testHandler)

	// Creating a test request
	req := httptest.NewRequest("GET", "/oauth2/authorize", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)
	}
}

func BenchmarkHashSensitiveData(b *testing.B) {
	testData := "sensitive_test_data_that_needs_hashing"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hashSensitiveData(testData)
	}
}

func BenchmarkSanitizeMap(b *testing.B) {
	queryParams := map[string][]string{
		"client_id":     {"test_client"},
		"client_secret": {"secret_value"},
		"scope":         {"read write"},
	}
	sensitiveKeys := []string{"client_secret", "password"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeQueryParams(queryParams, sensitiveKeys)
	}
}

func TestAuditWriter_DefaultStatusOnWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	aw := &auditResponseWriter{ResponseWriter: rec, body: make([]byte, 0)}
	_, _ = aw.Write([]byte("hi"))
	if aw.statusCode != http.StatusOK {
		t.Fatalf("status should default to 200 on Write, got %d", aw.statusCode)
	}
}

func TestSSE_DisablesCapture(t *testing.T) {
	cl := &captureLogger{}
	opts := NewAuditOptionsBuilder().Build()
	opts.Logger = cl
	opts.IncludeResponseBody = true // 即便配置为 true，SSE 也应禁用
	mw := AuditMiddleware(opts)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟 SSE 输出
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		w.Write([]byte("data: ping\n\n"))
	})

	req := httptest.NewRequest("GET", "/oauth2/authorize", nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	mw(h).ServeHTTP(w, req)

	if cl.last.ResponseBody != "" {
		t.Fatalf("SSE responses should not be captured")
	}
}

func TestRequestBody_CapturedWhenEnabled(t *testing.T) {
	cl := &captureLogger{}
	opts := NewAuditOptionsBuilder().Build()
	opts.Logger = cl
	opts.IncludeRequestBody = true

	mw := AuditMiddleware(opts)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader("grant_type=client_credentials"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	mw(h).ServeHTTP(w, req)

	if cl.last.RequestBody == "" || !strings.Contains(cl.last.RequestBody, "grant_type=client_credentials") {
		t.Fatalf("request body should be captured when IncludeRequestBody=true")
	}
}
