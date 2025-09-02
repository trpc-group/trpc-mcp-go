// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package retry

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Validation range constants for retry configuration parameters.
const (
	// MaxRetries validation range
	MinMaxRetries = 0
	MaxMaxRetries = 10

	// InitialBackoff validation range
	MinInitialBackoff = time.Millisecond
	MaxInitialBackoff = 30 * time.Second

	// BackoffFactor validation range
	MinBackoffFactor = 1.0
	MaxBackoffFactor = 10.0

	// MaxBackoff validation range
	MaxMaxBackoff = 5 * time.Minute
)

// retryableStatusCodes contains HTTP status codes that should be retried.
// Pre-computed at package level for optimal performance.
var retryableStatusCodes = []string{
	// 4xx codes that are retryable
	strconv.Itoa(http.StatusRequestTimeout),  // 408 - Request Timeout
	strconv.Itoa(http.StatusConflict),        // 409 - Conflict
	strconv.Itoa(http.StatusTooManyRequests), // 429 - Too Many Requests

	// All 5xx server errors are retryable
	strconv.Itoa(http.StatusInternalServerError),           // 500 - Internal Server Error
	strconv.Itoa(http.StatusNotImplemented),                // 501 - Not Implemented
	strconv.Itoa(http.StatusBadGateway),                    // 502 - Bad Gateway
	strconv.Itoa(http.StatusServiceUnavailable),            // 503 - Service Unavailable
	strconv.Itoa(http.StatusGatewayTimeout),                // 504 - Gateway Timeout
	strconv.Itoa(http.StatusHTTPVersionNotSupported),       // 505 - HTTP Version Not Supported
	strconv.Itoa(http.StatusVariantAlsoNegotiates),         // 506 - Variant Also Negotiates
	strconv.Itoa(http.StatusInsufficientStorage),           // 507 - Insufficient Storage
	strconv.Itoa(http.StatusLoopDetected),                  // 508 - Loop Detected
	"509",                                                  // 509 - Bandwidth Limit Exceeded (non-standard, not defined in net/http)
	strconv.Itoa(http.StatusNotExtended),                   // 510 - Not Extended
	strconv.Itoa(http.StatusNetworkAuthenticationRequired), // 511 - Network Authentication Required
}

// Config defines configuration for retry behavior.
type Config struct {
	// MaxRetries specifies the maximum number of retry attempts for requests.
	MaxRetries int
	// InitialBackoff specifies the initial backoff duration before the first retry.
	InitialBackoff time.Duration
	// BackoffFactor specifies the factor to multiply the backoff duration for each retry.
	// For example, with factor 2.0: 100ms -> 200ms -> 400ms -> 800ms
	BackoffFactor float64
	// MaxBackoff specifies the maximum backoff duration to cap exponential growth.
	MaxBackoff time.Duration
}

// Validate validates and clamps retry configuration parameters to sensible ranges.
func (c Config) Validate() Config {
	validated := c

	// Clamp MaxRetries to reasonable range
	if validated.MaxRetries < MinMaxRetries {
		validated.MaxRetries = MinMaxRetries
	} else if validated.MaxRetries > MaxMaxRetries {
		validated.MaxRetries = MaxMaxRetries
	}

	// Clamp InitialBackoff to reasonable range
	if validated.InitialBackoff < MinInitialBackoff {
		validated.InitialBackoff = MinInitialBackoff
	} else if validated.InitialBackoff > MaxInitialBackoff {
		validated.InitialBackoff = MaxInitialBackoff
	}

	// Clamp BackoffFactor to reasonable range
	if validated.BackoffFactor < MinBackoffFactor {
		validated.BackoffFactor = MinBackoffFactor
	} else if validated.BackoffFactor > MaxBackoffFactor {
		validated.BackoffFactor = MaxBackoffFactor
	}

	// Clamp MaxBackoff to reasonable range, ensure it's >= InitialBackoff
	if validated.MaxBackoff < validated.InitialBackoff {
		validated.MaxBackoff = validated.InitialBackoff
	} else if validated.MaxBackoff > MaxMaxBackoff {
		validated.MaxBackoff = MaxMaxBackoff
	}

	return validated
}

// IsRetryableError determines if an error is retryable based on its characteristics.
// This function uses precise pattern matching to avoid false positives.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Network connection errors - use precise matching to avoid false positives
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection timeout") ||
		strings.Contains(errStr, "connection lost") ||
		strings.Contains(errStr, "connection aborted") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "read timeout") ||
		strings.Contains(errStr, "write timeout") ||
		strings.Contains(errStr, "dial timeout") ||
		errStr == "eof" || // Exact match to avoid false positives
		strings.HasSuffix(errStr, ": eof") { // EOF at end of error chain
		return true
	}

	// HTTP status code errors
	if isHTTPStatusRetryable(errStr) {
		return true
	}

	// Default to non-retryable for unknown errors to avoid infinite retry loops
	return false
}

// isHTTPStatusRetryable checks if an error contains a retryable HTTP status code.
// Uses precise patterns to avoid false positives (e.g., "port 5001" won't match "501").
func isHTTPStatusRetryable(errStr string) bool {
	for _, code := range retryableStatusCodes {
		// Match patterns like "HTTP 500", "status 500", "500 Internal Server Error"
		if strings.Contains(errStr, "http "+code) ||
			strings.Contains(errStr, "status "+code) ||
			strings.Contains(errStr, "status: "+code) ||
			strings.Contains(errStr, "code "+code) ||
			strings.Contains(errStr, "code: "+code) ||
			strings.Contains(errStr, code+" ") { // Status code followed by space (e.g., "500 Internal")
			return true
		}
	}

	return false
}

// Execute executes a function with exponential backoff retry logic.
// It implements the retry strategy defined in the Config.
func Execute(
	ctx context.Context,
	operation func() error,
	config *Config,
	operationName string,
) error {
	if config == nil || config.MaxRetries == 0 {
		return operation()
	}

	var lastErr error
	maxAttempts := config.MaxRetries + 1 // +1 for initial attempt

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check context cancellation before each attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lastErr = operation()
		if lastErr == nil {
			return nil // Success
		}

		// Check if error is retryable
		if !IsRetryableError(lastErr) {
			return lastErr // Non-retryable error, fail immediately
		}

		// Don't sleep after the last attempt
		if attempt == maxAttempts {
			break
		}

		// Calculate backoff duration with exponential growth
		var multiplier float64 = 1
		for i := 1; i < attempt; i++ {
			multiplier *= config.BackoffFactor
		}
		backoff := time.Duration(float64(config.InitialBackoff) * multiplier)
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	// All retry attempts exhausted
	// Return the original error without additional wrapping to avoid deep error chains
	return lastErr
}
