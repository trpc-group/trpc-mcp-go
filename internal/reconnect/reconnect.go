// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// Package reconnect provides connection-level reconnection functionality for MCP transports.
// This package handles stream disconnections and connection recovery, distinct from request-level retries.
package reconnect

import (
	"math"
	"strings"
	"time"
)

// Validation range constants for reconnect configuration parameters.
const (
	// MaxReconnectAttempts validation range
	MinMaxReconnectAttempts = 0
	MaxMaxReconnectAttempts = 5

	// ReconnectDelay validation range
	MinReconnectDelay = 100 * time.Millisecond
	MaxReconnectDelay = 30 * time.Second

	// ReconnectBackoffFactor validation range
	MinReconnectBackoffFactor = 1.0
	MaxReconnectBackoffFactor = 3.0

	// MaxReconnectDelay validation range
	MaxMaxReconnectDelay = 5 * time.Minute
)

// Config represents the configuration for connection-level reconnection.
// Reconnection differs from retry: retry handles temporary request failures,
// while reconnection handles stream/connection breaks that require re-establishment.
type Config struct {
	MaxReconnectAttempts   int           `json:"max_reconnect_attempts"`   // Maximum number of reconnection attempts (default: 2, range: 0-5)
	ReconnectDelay         time.Duration `json:"reconnect_delay"`          // Initial delay before reconnection (default: 1s, range: 100ms-30s)
	ReconnectBackoffFactor float64       `json:"reconnect_backoff_factor"` // Exponential backoff factor (default: 1.5, range: 1.0-3.0)
	MaxReconnectDelay      time.Duration `json:"max_reconnect_delay"`      // Maximum delay between attempts (default: 30s, range: up to 5min)
}

// Validate validates and clamps the reconnect configuration parameters to acceptable ranges.
// Invalid values are automatically corrected to the nearest valid value.
func (c *Config) Validate() {
	// Clamp MaxReconnectAttempts to valid range
	if c.MaxReconnectAttempts < MinMaxReconnectAttempts {
		c.MaxReconnectAttempts = MinMaxReconnectAttempts
	} else if c.MaxReconnectAttempts > MaxMaxReconnectAttempts {
		c.MaxReconnectAttempts = MaxMaxReconnectAttempts
	}

	// Clamp ReconnectDelay to valid range
	if c.ReconnectDelay < MinReconnectDelay {
		c.ReconnectDelay = MinReconnectDelay
	} else if c.ReconnectDelay > MaxReconnectDelay {
		c.ReconnectDelay = MaxReconnectDelay
	}

	// Clamp ReconnectBackoffFactor to valid range
	if c.ReconnectBackoffFactor < MinReconnectBackoffFactor {
		c.ReconnectBackoffFactor = MinReconnectBackoffFactor
	} else if c.ReconnectBackoffFactor > MaxReconnectBackoffFactor {
		c.ReconnectBackoffFactor = MaxReconnectBackoffFactor
	}

	// Clamp MaxReconnectDelay to valid range
	if c.MaxReconnectDelay > MaxMaxReconnectDelay {
		c.MaxReconnectDelay = MaxMaxReconnectDelay
	}

	// Ensure MaxReconnectDelay is at least equal to ReconnectDelay
	if c.MaxReconnectDelay < c.ReconnectDelay {
		c.MaxReconnectDelay = c.ReconnectDelay
	}
}

// CalculateDelay calculates the delay for a specific reconnection attempt using exponential backoff.
// The delay grows exponentially with each attempt but is capped at MaxReconnectDelay.
func (c *Config) CalculateDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return 0 // No delay for first attempt
	}

	// Calculate exponential backoff: delay * factor^(attempt-2)
	delay := float64(c.ReconnectDelay) * math.Pow(c.ReconnectBackoffFactor, float64(attempt-2))

	// Apply maximum delay cap
	if time.Duration(delay) > c.MaxReconnectDelay {
		return c.MaxReconnectDelay
	}

	return time.Duration(delay)
}

// IsStreamDisconnectedError checks if an error indicates a stream disconnection that can be reconnected.
// These are connection-level issues that don't require session re-initialization.
func IsStreamDisconnectedError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Stream-specific disconnection patterns
	streamPatterns := []string{
		"stream closed",
		"stream disconnected",
		"connection lost",
		"sse connection",
		"broken pipe",
		"connection reset",
	}

	for _, pattern := range streamPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// IsSessionExpiredError checks if an error indicates session expiration that requires Agent-level handling.
// These errors should be wrapped and propagated to the Agent layer for session recreation.
func IsSessionExpiredError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Session expiration patterns
	sessionPatterns := []string{
		"404",          // HTTP 404 Not Found (session expired)
		"unauthorized", // Authentication expired
		"session not found",
		"invalid session",
		"session expired",
	}

	for _, pattern := range sessionPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
