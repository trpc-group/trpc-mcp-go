// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/reconnect"
)

// ReconnectConfig defines configuration for MCP client reconnection behavior.
// Reconnection handles connection-level failures such as stream disconnections,
// which are different from request-level retry failures.
type ReconnectConfig struct {
	// MaxReconnectAttempts specifies the maximum number of reconnection attempts.
	// Valid range: 0-5, default: 2
	MaxReconnectAttempts int `json:"max_reconnect_attempts"`
	// ReconnectDelay specifies the initial delay before the first reconnection attempt.
	// Valid range: 100ms-30s, default: 1s
	ReconnectDelay time.Duration `json:"reconnect_delay"`
	// ReconnectBackoffFactor specifies the factor to multiply the delay for each reconnection attempt.
	// For example, with factor 1.5: 1s -> 1.5s -> 2.25s -> 3.375s
	// Valid range: 1.0-3.0, default: 1.5
	ReconnectBackoffFactor float64 `json:"reconnect_backoff_factor"`
	// MaxReconnectDelay specifies the maximum delay between reconnection attempts.
	// Valid range: minimum is ReconnectDelay, maximum: 5 minutes, default: 30s
	MaxReconnectDelay time.Duration `json:"max_reconnect_delay"`
}

// defaultReconnectConfig provides sensible defaults for reconnection configuration.
// Uses conservative values optimized for connection stability over speed.
var defaultReconnectConfig = ReconnectConfig{
	MaxReconnectAttempts:   2,                // Conservative reconnection count
	ReconnectDelay:         1 * time.Second,  // 1s initial delay
	ReconnectBackoffFactor: 1.5,              // Gentle exponential backoff
	MaxReconnectDelay:      30 * time.Second, // Maximum delay cap
}

// WithSimpleReconnect enables reconnection with the specified maximum number of attempts.
// Uses default backoff configuration (1s initial, 1.5 factor, 30s max).
func WithSimpleReconnect(maxAttempts int) ClientOption {
	config := defaultReconnectConfig
	config.MaxReconnectAttempts = maxAttempts
	return func(c *Client) {
		internalConfig := reconnect.Config{
			MaxReconnectAttempts:   config.MaxReconnectAttempts,
			ReconnectDelay:         config.ReconnectDelay,
			ReconnectBackoffFactor: config.ReconnectBackoffFactor,
			MaxReconnectDelay:      config.MaxReconnectDelay,
		}
		internalConfig.Validate()
		c.setReconnectConfig(&internalConfig)
	}
}

// WithReconnect enables reconnection with custom configuration.
// All configuration parameters are validated and clamped to acceptable ranges.
func WithReconnect(config ReconnectConfig) ClientOption {
	return func(c *Client) {
		internalConfig := reconnect.Config{
			MaxReconnectAttempts:   config.MaxReconnectAttempts,
			ReconnectDelay:         config.ReconnectDelay,
			ReconnectBackoffFactor: config.ReconnectBackoffFactor,
			MaxReconnectDelay:      config.MaxReconnectDelay,
		}
		internalConfig.Validate()
		c.setReconnectConfig(&internalConfig)
	}
}
