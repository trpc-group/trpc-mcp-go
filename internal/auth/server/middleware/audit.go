// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
)

// AuditLevel defines audit log verbosity levels
type AuditLevel int

const (
	AuditLevelNone AuditLevel = iota
	AuditLevelBasic
	AuditLevelDetailed
	AuditLevelFull
)

// AuditEvent represents an OAuth 2.1 operation audit record
type AuditEvent struct {
	EventID      string                 `json:"event_id"`
	Timestamp    time.Time              `json:"timestamp"`
	EventType    string                 `json:"event_type"`
	AuditLevel   AuditLevel             `json:"audit_level"`
	Method       string                 `json:"method"`
	Path         string                 `json:"path"`
	QueryParams  map[string]string      `json:"query_params,omitempty"`
	Headers      map[string]string      `json:"headers,omitempty"`
	RemoteAddr   string                 `json:"remote_addr"`
	UserAgent    string                 `json:"user_agent"`
	RequestID    string                 `json:"request_id,omitempty"`
	ClientID     string                 `json:"client_id,omitempty"`
	Subject      string                 `json:"subject,omitempty"`
	Scopes       []string               `json:"scopes,omitempty"`
	GrantType    string                 `json:"grant_type,omitempty"`
	ResponseType string                 `json:"response_type,omitempty"`
	RedirectURI  string                 `json:"redirect_uri,omitempty"`
	Resource     string                 `json:"resource,omitempty"`
	StatusCode   int                    `json:"status_code"`
	ResponseTime time.Duration          `json:"response_time"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	TokenHash    string                 `json:"token_hash,omitempty"`
	CodeHash     string                 `json:"code_hash,omitempty"`
	IPHash       string                 `json:"ip_hash,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	RiskLevel    string                 `json:"risk_level,omitempty"`
	RiskFactors  []string               `json:"risk_factors,omitempty"`
	RequestBody  string                 `json:"request_body,omitempty"`
	ResponseBody string                 `json:"response_body,omitempty"`
}

// AuditLogger defines an interface for emitting audit logs
type AuditLogger interface {
	LogEvent(event AuditEvent) error
	LogError(event AuditEvent, err error) error
}

// DefaultAuditLogger provides a zap based implementation of AuditLogger
type DefaultAuditLogger struct {
	logger *zap.Logger
}

// NewAuditLogger creates a DefaultAuditLogger using the provided zap logger or sensible defaults
func NewAuditLogger(logger *zap.Logger) *DefaultAuditLogger {
	// Build a production logger by default and fall back to development if needed
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			logger, _ = zap.NewDevelopment()
		}
	}
	return &DefaultAuditLogger{logger: logger}
}

// GetZapLogger exposes the underlying zap logger for advanced usage
func (l *DefaultAuditLogger) GetZapLogger() *zap.Logger {
	return l.logger
}

// LogEvent writes a structured audit event at info level
func (l *DefaultAuditLogger) LogEvent(event AuditEvent) error {
	// Guard against uninitialized logger
	if l.logger == nil {
		return fmt.Errorf("zap logger not initialized")
	}

	// Marshal full event payload for a single structured field
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Emit event with a compact summary sub-structure for quick filtering
	l.logger.Info("[AUDIT]",
		zap.ByteString("event", data),
		zap.Any("audit", struct {
			Method       string
			Path         string
			StatusCode   int
			ResponseTime time.Duration
			ClientID     string
			Subject      string
			Scopes       []string
			RiskLevel    string
		}{
			Method:       event.Method,
			Path:         event.Path,
			StatusCode:   event.StatusCode,
			ResponseTime: event.ResponseTime,
			ClientID:     event.ClientID,
			Subject:      event.Subject,
			Scopes:       event.Scopes,
			RiskLevel:    event.RiskLevel,
		}),
	)
	return nil
}

// LogError writes an audit event including the provided error message
func (l *DefaultAuditLogger) LogError(event AuditEvent, err error) error {
	// Attach error message then delegate to LogEvent
	event.ErrorMessage = err.Error()
	return l.LogEvent(event)
}

// AuditMiddlewareOptions configures what and how the middleware audits
type AuditMiddlewareOptions struct {
	Logger              AuditLogger
	Level               AuditLevel
	HashSensitiveData   bool
	IncludeRequestBody  bool
	IncludeResponseBody bool
	RiskAssessor        func(AuditEvent) (string, []string)
	MetadataExtractor   func(*http.Request) map[string]interface{}
	EndpointPatterns    []string
	ExcludePatterns     []string
	SensitiveKeys       []string
}

// DefaultAuditMiddlewareOptions returns a sane default configuration for OAuth endpoints
func DefaultAuditMiddlewareOptions() *AuditMiddlewareOptions {
	// Default to detailed level with hashing and common OAuth endpoint patterns
	return &AuditMiddlewareOptions{
		Logger:            NewAuditLogger(nil),
		Level:             AuditLevelDetailed,
		HashSensitiveData: true,
		EndpointPatterns: []string{
			"/oauth2/authorize",
			"/oauth2/token",
			"/oauth2/revoke",
			"/oauth2/register",
			"/oauth2/metadata",
		},
		SensitiveKeys: []string{"client_secret", "code_verifier", "password", "authorization", "cookie", "x-api-key"},
	}
}

// AuditOptionsBuilder helps compose AuditMiddlewareOptions with a fluent API
type AuditOptionsBuilder struct {
	options *AuditMiddlewareOptions
}

// NewAuditOptionsBuilder creates a builder initialized with default options
func NewAuditOptionsBuilder() *AuditOptionsBuilder {
	return &AuditOptionsBuilder{options: DefaultAuditMiddlewareOptions()}
}

// WithLogger sets a custom zap logger for audit output
func (b *AuditOptionsBuilder) WithLogger(logger *zap.Logger) *AuditOptionsBuilder {
	b.options.Logger = NewAuditLogger(logger)
	return b
}

// WithLevel sets the audit verbosity level
func (b *AuditOptionsBuilder) WithLevel(level AuditLevel) *AuditOptionsBuilder {
	b.options.Level = level
	return b
}

// WithHashSensitiveData toggles hashing of sensitive fields before logging
func (b *AuditOptionsBuilder) WithHashSensitiveData(hash bool) *AuditOptionsBuilder {
	b.options.HashSensitiveData = hash
	return b
}

// WithRequestBody toggles inclusion of request body in audit events
func (b *AuditOptionsBuilder) WithRequestBody(include bool) *AuditOptionsBuilder {
	b.options.IncludeRequestBody = include
	return b
}

// WithResponseBody toggles inclusion of response body in audit events
func (b *AuditOptionsBuilder) WithResponseBody(include bool) *AuditOptionsBuilder {
	b.options.IncludeResponseBody = include
	return b
}

// WithRiskAssessor sets a custom risk assessment function
func (b *AuditOptionsBuilder) WithRiskAssessor(assessor func(AuditEvent) (string, []string)) *AuditOptionsBuilder {
	b.options.RiskAssessor = assessor
	return b
}

// WithMetadataExtractor sets a function to extract extra metadata from requests
func (b *AuditOptionsBuilder) WithMetadataExtractor(extractor func(*http.Request) map[string]interface{}) *AuditOptionsBuilder {
	b.options.MetadataExtractor = extractor
	return b
}

// WithEndpointPatterns sets regex patterns for endpoints to include in auditing
func (b *AuditOptionsBuilder) WithEndpointPatterns(patterns []string) *AuditOptionsBuilder {
	b.options.EndpointPatterns = patterns
	return b
}

// WithExcludePatterns sets regex patterns for endpoints to exclude from auditing
func (b *AuditOptionsBuilder) WithExcludePatterns(patterns []string) *AuditOptionsBuilder {
	b.options.ExcludePatterns = patterns
	return b
}

// WithSensitiveKeys sets keys that should be redacted in headers and queries
func (b *AuditOptionsBuilder) WithSensitiveKeys(keys []string) *AuditOptionsBuilder {
	b.options.SensitiveKeys = keys
	return b
}

// Build finalizes and returns the configured options
func (b *AuditOptionsBuilder) Build() *AuditMiddlewareOptions {
	return b.options
}

// AuditMiddleware returns an HTTP middleware that emits audit events based on the provided options
func AuditMiddleware(options *AuditMiddlewareOptions) func(http.Handler) http.Handler {
	// Initialize default options and logger as needed
	if options == nil {
		options = DefaultAuditMiddlewareOptions()
	}
	if options.Logger == nil {
		options.Logger = NewAuditLogger(nil)
	}
	// Validate configuration early and fail fast for programmer errors
	if err := validateOptions(options); err != nil {
		panic(fmt.Sprintf("invalid audit middleware options: %v", err))
	}

	// Wrap the next handler with auditing behavior
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auditing if path does not match include/exclude rules
			if !shouldAuditPath(r.URL.Path, options.EndpointPatterns, options.ExcludePatterns) {
				next.ServeHTTP(w, r)
				return
			}

			// Detect Server-Sent Events and avoid capturing streaming bodies
			acceptHeader := r.Header.Get("Accept")
			isSSE := strings.Contains(acceptHeader, "text/event-stream")

			// Initialize event and wrap writer for status and body capture
			event, wrappedWriter := initializeAuditEvent(w, r, options)

			// Disable response capture for SSE to prevent interference with streaming
			if isSSE {
				wrappedWriter.captured = false
			}

			// Ensure event is logged even if downstream panics or early returns
			defer logAuditEvent(event, wrappedWriter, options)

			// Continue to next handler with wrapped writer
			next.ServeHTTP(wrappedWriter, r)
		})
	}
}

// validateOptions ensures the options contain at least one include or exclude pattern
func validateOptions(options *AuditMiddlewareOptions) error {
	// Must define what to include or exclude to avoid auditing everything by accident
	if len(options.EndpointPatterns) == 0 && len(options.ExcludePatterns) == 0 {
		return fmt.Errorf("at least one endpoint pattern or exclude pattern must be specified")
	}
	return nil
}

// auditResponseWriter wraps ResponseWriter to capture status and response body
type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       []byte
	captured   bool
}

// WriteHeader intercepts status codes for auditing
func (w *auditResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Write intercepts response body bytes when capture is enabled
func (w *auditResponseWriter) Write(b []byte) (int, error) {
	// Default status to 200 OK if not set
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	// Append to buffer only when capture is enabled
	if w.captured || w.body != nil {
		w.body = append(w.body, b...)
	}
	return w.ResponseWriter.Write(b)
}

// Flush forwards flush calls for streaming responses
func (w *auditResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter
func (w *auditResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// OAuthInfo carries OAuth 2.1 specific request attributes extracted for auditing
type OAuthInfo struct {
	ClientID     string
	Subject      string
	Scopes       []string
	GrantType    string
	ResponseType string
	RedirectURI  string
	Resource     string
	Token        string
	Code         string
}

// extractOAuthInfo pulls OAuth related fields from URL query, form body, headers, and context
func extractOAuthInfo(r *http.Request) OAuthInfo {
	info := OAuthInfo{}

	// Extract from query parameters
	if r.URL != nil {
		query := r.URL.Query()
		info.ClientID = query.Get("client_id")
		info.ResponseType = query.Get("response_type")
		info.RedirectURI = query.Get("redirect_uri")
		info.Resource = query.Get("resource")
		if scope := query.Get("scope"); scope != "" {
			info.Scopes = strings.Split(scope, " ")
		}
	}

	// Extract from form body when present
	if err := r.ParseForm(); err == nil {
		if info.GrantType == "" {
			info.GrantType = r.FormValue("grant_type")
		}
		info.Code = r.FormValue("code")
		if scope := r.FormValue("scope"); scope != "" && len(info.Scopes) == 0 {
			info.Scopes = strings.Split(scope, " ")
		}
	}

	// Extract bearer token from Authorization header
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		info.Token = strings.TrimPrefix(auth, "Bearer ")
	}

	if authInfo, ok := server.GetAuthInfo(r.Context()); ok {
		// Extract subject from the Extra claims
		if authInfo.Extra != nil {
			if sub, ok := authInfo.Extra["sub"].(string); ok {
				info.Subject = sub
			}
		}

		// Use scopes from authInfo if not already populated
		if len(info.Scopes) == 0 && len(authInfo.Scopes) > 0 {
			info.Scopes = authInfo.Scopes
		}

		// Extract client_id from Extra claims first
		if cid, ok := authInfo.Extra["client_id"].(string); ok && info.ClientID == "" {
			info.ClientID = cid
		}

		// Fallback to AuthInfo.ClientID field if Extra doesn't contain client_id
		if info.ClientID == "" && authInfo.ClientID != "" {
			info.ClientID = authInfo.ClientID
		}
	}
	return info
}

// shouldAuditPath checks include and exclude regex patterns to decide auditing
func shouldAuditPath(path string, includePatterns, excludePatterns []string) bool {
	// Exclude takes precedence when matched
	for _, pattern := range excludePatterns {
		if matched, _ := regexp.MatchString(pattern, path); matched {
			return false
		}
	}
	// If no include patterns set then audit all non excluded paths
	if len(includePatterns) == 0 {
		return true
	}
	// Audit when any include pattern matches
	for _, pattern := range includePatterns {
		if matched, _ := regexp.MatchString(pattern, path); matched {
			return true
		}
	}
	return false
}

// determineEventType maps path and method to a coarse event category
func determineEventType(path, method string) string {
	switch {
	case strings.Contains(path, "/authorize"):
		return "oauth_authorization"
	case strings.Contains(path, "/token"):
		return "oauth_token"
	case strings.Contains(path, "/revoke"):
		return "oauth_revocation"
	case strings.Contains(path, "/register"):
		return "oauth_registration"
	case strings.Contains(path, "/metadata"):
		return "oauth_metadata"
	default:
		return "oauth_request"
	}
}

// generateEventID builds a unique event identifier based on time and random suffix
func generateEventID() string {
	return fmt.Sprintf("audit_%d_%s", time.Now().UnixNano(), randomString(8))
}

// randomString generates a pseudo random lowercase alphanumeric string
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)

	// Try cryptographic randomness first and fall back to time based selection
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
	} else {
		for i := range b {
			b[i] = charset[int(b[i])%len(charset)]
		}
	}
	return string(b)
}

// sanitizeMap redacts configured sensitive keys and normalizes values to a single string
func sanitizeMap[T string | []string](data map[string]T, sensitiveKeys []string) map[string]string {
	sanitized := make(map[string]string)

	// Iterate keys and redact any that match configured sensitive keys
	for key, value := range data {
		isSensitive := false
		for _, sensitiveKey := range sensitiveKeys {
			if strings.EqualFold(key, sensitiveKey) {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			sanitized[key] = "[REDACTED]"
		} else {
			// Normalize to first value for slice and direct string otherwise
			switch v := any(value).(type) {
			case string:
				sanitized[key] = v
			case []string:
				if len(v) > 0 {
					sanitized[key] = v[0]
				}
			}
		}
	}
	return sanitized
}

// sanitizeQueryParams applies redaction and normalization to URL query parameters
func sanitizeQueryParams(query map[string][]string, sensitiveKeys []string) map[string]string {
	return sanitizeMap(query, sensitiveKeys)
}

// sanitizeHeaders applies redaction and normalization to HTTP headers
func sanitizeHeaders(headers map[string][]string, sensitiveKeys []string) map[string]string {
	return sanitizeMap(headers, sensitiveKeys)
}

// hashSensitiveData returns a hex encoded SHA256 hash for a sensitive string
func hashSensitiveData(data string) string {
	if data == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// defaultRiskAssessment computes a simple risk score and contributing factors
func defaultRiskAssessment(event AuditEvent) (string, []string) {
	var riskFactors []string
	riskLevel := "low"

	// Client side error increases risk
	if event.StatusCode >= 400 {
		riskFactors = append(riskFactors, "client_error")
	}

	// Server side error increases risk more
	if event.StatusCode >= 500 {
		riskFactors = append(riskFactors, "server_error")
		riskLevel = "medium"
	}

	// Slow responses may indicate issues
	if event.ResponseTime > 5*time.Second {
		riskFactors = append(riskFactors, "slow_response")
		riskLevel = "medium"
	}

	// Missing client identifier is suspicious
	if event.ClientID == "" {
		riskFactors = append(riskFactors, "missing_client_id")
		riskLevel = "high"
	}

	// Specific endpoint categories slightly elevate risk
	if strings.Contains(event.Path, "/revoke") {
		riskFactors = append(riskFactors, "token_revocation")
		riskLevel = "medium"
	}
	if strings.Contains(event.Path, "/register") {
		riskFactors = append(riskFactors, "client_registration")
		riskLevel = "medium"
	}
	return riskLevel, riskFactors
}

// determineErrorCode maps HTTP status codes to OAuth style error codes
func determineErrorCode(statusCode int) string {
	switch {
	case statusCode == 400:
		return "invalid_request"
	case statusCode == 401:
		return "invalid_token"
	case statusCode == 403:
		return "insufficient_scope"
	case statusCode == 404:
		return "not_found"
	case statusCode == 429:
		return "too_many_requests"
	case statusCode >= 500:
		return "server_error"
	default:
		return "unknown_error"
	}
}

// determineErrorMessage extracts error text from a JSON error response or falls back to status text
func determineErrorMessage(statusCode int, body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var errorResponse struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	// Try decode standard OAuth error response
	if err := json.Unmarshal(body, &errorResponse); err == nil {
		if errorResponse.ErrorDescription != "" {
			return errorResponse.ErrorDescription
		}
		if errorResponse.Error != "" {
			return errorResponse.Error
		}
	}
	// Fallback to generic status text when payload is not structured
	return http.StatusText(statusCode)
}

// extractSubject reads the subject claim from AuthInfo.Extra
func extractSubject(authInfo server.AuthInfo) string {
	if authInfo.Extra != nil {
		if sub, ok := authInfo.Extra["sub"].(string); ok {
			return sub
		}
	}
	return ""
}

// GetAuthInfo extracts AuthInfo from the request context
func GetAuthInfo(ctx context.Context) (server.AuthInfo, bool) {
	// The context key is expected to be provided by upstream auth middleware
	if authInfo, ok := ctx.Value(AuthInfoKey).(server.AuthInfo); ok {
		return authInfo, true
	}
	return server.AuthInfo{}, false
}

// initializeAuditEvent constructs an AuditEvent and wraps the ResponseWriter for capture
func initializeAuditEvent(w http.ResponseWriter, r *http.Request, options *AuditMiddlewareOptions) (AuditEvent, *auditResponseWriter) {
	start := time.Now()

	// Optionally read and restore the request body for logging
	var reqBody []byte
	if (options.Level >= AuditLevelFull || options.IncludeRequestBody) && r.Body != nil {
		reqBody, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
	}

	// Configure wrapped writer to capture response body when enabled
	wrappedWriter := &auditResponseWriter{
		ResponseWriter: w,
		captured:       (options.Level >= AuditLevelFull || options.IncludeResponseBody),
	}

	// Gather OAuth specific attributes for context
	oauthInfo := extractOAuthInfo(r)

	// Seed the event with request metadata and extracted OAuth fields
	event := AuditEvent{
		EventID:      generateEventID(),
		Timestamp:    start,
		EventType:    determineEventType(r.URL.Path, r.Method),
		AuditLevel:   options.Level,
		Method:       r.Method,
		Path:         r.URL.Path,
		RemoteAddr:   r.RemoteAddr,
		UserAgent:    r.UserAgent(),
		RequestID:    r.Header.Get("X-Request-ID"),
		ClientID:     oauthInfo.ClientID,
		Subject:      oauthInfo.Subject,
		Scopes:       oauthInfo.Scopes,
		GrantType:    oauthInfo.GrantType,
		ResponseType: oauthInfo.ResponseType,
		RedirectURI:  oauthInfo.RedirectURI,
		Resource:     oauthInfo.Resource,
		Metadata:     make(map[string]interface{}),
	}

	// At detailed level and above include sanitized query params and headers
	if options.Level >= AuditLevelDetailed {
		event.QueryParams = sanitizeQueryParams(r.URL.Query(), options.SensitiveKeys)
		event.Headers = sanitizeHeaders(r.Header, options.SensitiveKeys)
	}

	// Hash tokens and IP address when configured to avoid leaking PII
	if options.HashSensitiveData {
		event.TokenHash = hashSensitiveData(oauthInfo.Token)
		event.CodeHash = hashSensitiveData(oauthInfo.Code)
		event.IPHash = hashSensitiveData(r.RemoteAddr)
	}

	// Include request body when configured
	if (options.Level >= AuditLevelFull || options.IncludeRequestBody) && len(reqBody) > 0 {
		event.RequestBody = string(reqBody)
	}

	// Extract custom metadata when a provider is supplied
	if options.MetadataExtractor != nil {
		event.Metadata = options.MetadataExtractor(r)
	}

	// Assess risk using custom function or defaults
	if options.RiskAssessor != nil {
		event.RiskLevel, event.RiskFactors = options.RiskAssessor(event)
	} else {
		event.RiskLevel, event.RiskFactors = defaultRiskAssessment(event)
	}

	return event, wrappedWriter
}

// logAuditEvent finalizes timing and status then emits the audit event via the configured logger
func logAuditEvent(event AuditEvent, w *auditResponseWriter, options *AuditMiddlewareOptions) {
	// Compute latency and attach final status code
	event.ResponseTime = time.Since(event.Timestamp)
	event.StatusCode = w.statusCode

	// Optionally include captured response body
	if w.captured && len(w.body) > 0 && (options.Level >= AuditLevelFull || options.IncludeResponseBody) {
		event.ResponseBody = string(w.body)
	}

	// Derive error details for non successful responses
	if event.StatusCode >= 400 {
		event.ErrorCode = determineErrorCode(event.StatusCode)
		event.ErrorMessage = determineErrorMessage(event.StatusCode, w.body)
	}

	// Emit the event and log any failure to stdout as a last resort
	if err := options.Logger.LogEvent(event); err != nil {
		fmt.Printf("[AUDIT ERROR] Failed to log audit event: %v\n", err)
	}
}

// WithOAuthAudit returns an OAuth specific audit middleware using provided options
func WithOAuthAudit(options *AuditMiddlewareOptions) func(http.Handler) http.Handler {
	return AuditMiddleware(options)
}

// WithBasicAudit returns a middleware configured for basic auditing
func WithBasicAudit() func(http.Handler) http.Handler {
	return AuditMiddleware(NewAuditOptionsBuilder().
		WithLevel(AuditLevelBasic).
		Build())
}

// WithDetailedAudit returns a middleware configured for detailed auditing
func WithDetailedAudit() func(http.Handler) http.Handler {
	return AuditMiddleware(NewAuditOptionsBuilder().
		WithLevel(AuditLevelDetailed).
		Build())
}

// WithFullAudit returns a middleware configured for full auditing including bodies
func WithFullAudit() func(http.Handler) http.Handler {
	return AuditMiddleware(NewAuditOptionsBuilder().
		WithLevel(AuditLevelFull).
		WithRequestBody(true).
		WithResponseBody(true).
		Build())
}

// WithZapLogger returns options pre configured with a custom zap logger
func WithZapLogger(logger *zap.Logger) *AuditMiddlewareOptions {
	return NewAuditOptionsBuilder().
		WithLogger(logger).
		Build()
}

// WithCustomZapLogger returns options configured with a custom zap logger and core toggles
func WithCustomZapLogger(logger *zap.Logger, level AuditLevel, hashSensitive bool) *AuditMiddlewareOptions {
	return NewAuditOptionsBuilder().
		WithLogger(logger).
		WithLevel(level).
		WithHashSensitiveData(hashSensitive).
		Build()
}
