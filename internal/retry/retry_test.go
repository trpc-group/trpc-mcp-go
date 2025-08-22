// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package retry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "connection timeout",
			err:      errors.New("connection timeout occurred"),
			expected: true,
		},
		{
			name:     "EOF error",
			err:      errors.New("EOF"),
			expected: true,
		},
		{
			name:     "HTTP 500 error",
			err:      errors.New("HTTP 500 Internal Server Error"),
			expected: true,
		},
		{
			name:     "HTTP 409 error",
			err:      errors.New("status 409 Conflict"),
			expected: true,
		},
		{
			name:     "HTTP 404 error (non-retryable)",
			err:      errors.New("HTTP 404 Not Found"),
			expected: false,
		},
		{
			name:     "authentication error (non-retryable)",
			err:      errors.New("authentication failed"),
			expected: false,
		},
		{
			name:     "unknown error",
			err:      errors.New("some random error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestExecute_Success(t *testing.T) {
	config := &Config{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		BackoffFactor:  2.0,
		MaxBackoff:     100 * time.Millisecond,
	}

	callCount := 0
	operation := func() error {
		callCount++
		return nil // Success on first try
	}

	ctx := context.Background()
	err := Execute(ctx, operation, config, "test_operation")

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestExecute_SuccessAfterRetries(t *testing.T) {
	config := &Config{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		BackoffFactor:  2.0,
		MaxBackoff:     100 * time.Millisecond,
	}

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("connection timeout") // Retryable error
		}
		return nil // Success on 3rd try
	}

	ctx := context.Background()
	start := time.Now()
	err := Execute(ctx, operation, config, "test_operation")
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
	// Should have some delay due to backoff
	if duration < 10*time.Millisecond {
		t.Errorf("Expected some delay due to backoff, got %v", duration)
	}
}

func TestExecute_NonRetryableError(t *testing.T) {
	config := &Config{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		BackoffFactor:  2.0,
		MaxBackoff:     100 * time.Millisecond,
	}

	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("authentication failed") // Non-retryable error
	}

	ctx := context.Background()
	err := Execute(ctx, operation, config, "test_operation")

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("Expected authentication error, got: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call (no retries), got %d", callCount)
	}
}

func TestExecute_ExhaustRetries(t *testing.T) {
	config := &Config{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		BackoffFactor:  2.0,
		MaxBackoff:     100 * time.Millisecond,
	}

	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("connection timeout") // Always retryable error
	}

	ctx := context.Background()
	err := Execute(ctx, operation, config, "test_operation")

	if err == nil {
		t.Error("Expected error after exhausting retries, got nil")
	}
	if !strings.Contains(err.Error(), "connection timeout") {
		t.Errorf("Expected connection timeout error, got: %v", err)
	}
	if callCount != 3 { // 1 initial + 2 retries
		t.Errorf("Expected 3 calls (1 initial + 2 retries), got %d", callCount)
	}
}

func TestExecute_ContextCancellation(t *testing.T) {
	config := &Config{
		MaxRetries:     5,
		InitialBackoff: 100 * time.Millisecond,
		BackoffFactor:  2.0,
		MaxBackoff:     1 * time.Second,
	}

	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("connection timeout") // Always retryable error
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := Execute(ctx, operation, config, "test_operation")

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded, got: %v", err)
	}
	// Should have been called at least once
	if callCount < 1 {
		t.Errorf("Expected at least 1 call, got %d", callCount)
	}
}

func TestExecute_NoRetryConfig(t *testing.T) {
	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("connection timeout")
	}

	ctx := context.Background()
	err := Execute(ctx, operation, nil, "test_operation")

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call (no retries), got %d", callCount)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name: "valid config",
			input: Config{
				MaxRetries:     3,
				InitialBackoff: 1 * time.Second,
				BackoffFactor:  2.0,
				MaxBackoff:     30 * time.Second,
			},
			expected: Config{
				MaxRetries:     3,
				InitialBackoff: 1 * time.Second,
				BackoffFactor:  2.0,
				MaxBackoff:     30 * time.Second,
			},
		},
		{
			name: "negative max retries",
			input: Config{
				MaxRetries:     -1,
				InitialBackoff: 1 * time.Second,
				BackoffFactor:  2.0,
				MaxBackoff:     30 * time.Second,
			},
			expected: Config{
				MaxRetries:     0,
				InitialBackoff: 1 * time.Second,
				BackoffFactor:  2.0,
				MaxBackoff:     30 * time.Second,
			},
		},
		{
			name: "too high max retries",
			input: Config{
				MaxRetries:     15,
				InitialBackoff: 1 * time.Second,
				BackoffFactor:  2.0,
				MaxBackoff:     30 * time.Second,
			},
			expected: Config{
				MaxRetries:     10,
				InitialBackoff: 1 * time.Second,
				BackoffFactor:  2.0,
				MaxBackoff:     30 * time.Second,
			},
		},
		{
			name: "too small initial backoff",
			input: Config{
				MaxRetries:     3,
				InitialBackoff: 0,
				BackoffFactor:  2.0,
				MaxBackoff:     30 * time.Second,
			},
			expected: Config{
				MaxRetries:     3,
				InitialBackoff: 1 * time.Millisecond,
				BackoffFactor:  2.0,
				MaxBackoff:     30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.Validate()
			if result != tt.expected {
				t.Errorf("Config.Validate() = %+v, expected %+v", result, tt.expected)
			}
		})
	}
}
