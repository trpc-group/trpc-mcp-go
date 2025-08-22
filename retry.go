// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/retry"
)

// RetryConfig defines configuration for MCP client retry behavior.
type RetryConfig struct {
	// MaxRetries specifies the maximum number of retry attempts for requests.
	MaxRetries int `json:"max_retries"`
	// InitialBackoff specifies the initial backoff duration before the first retry.
	InitialBackoff time.Duration `json:"initial_backoff"`
	// BackoffFactor specifies the factor to multiply the backoff duration for each retry.
	// For example, with factor 2.0: 100ms -> 200ms -> 400ms -> 800ms
	BackoffFactor float64 `json:"backoff_factor"`
	// MaxBackoff specifies the maximum backoff duration to cap exponential growth.
	MaxBackoff time.Duration `json:"max_backoff"`
}

// defaultRetryConfig provides sensible defaults for retry configuration.
// Uses industry standard values: simple and conservative settings.
var defaultRetryConfig = RetryConfig{
	MaxRetries:     2,                      // Conservative retry count
	InitialBackoff: 500 * time.Millisecond, // 0.5s initial delay
	BackoffFactor:  2.0,                    // Standard exponential backoff
	MaxBackoff:     8 * time.Second,        // Maximum delay cap
}

// WithSimpleRetry enables retry with the specified maximum number of attempts.
// Uses default backoff configuration (500ms initial, 2.0 factor, 8s max).
func WithSimpleRetry(maxRetries int) ClientOption {
	config := defaultRetryConfig
	config.MaxRetries = maxRetries
	return func(c *Client) {
		internalConfig := retry.Config{
			MaxRetries:     config.MaxRetries,
			InitialBackoff: config.InitialBackoff,
			BackoffFactor:  config.BackoffFactor,
			MaxBackoff:     config.MaxBackoff,
		}
		validated := internalConfig.Validate()
		c.retryConfig = &validated
		// Set retry config on transport if it exists
		if c.transport != nil {
			c.transport.setRetryConfig(c.retryConfig)
		}
	}
}

// WithRetry enables retry with custom configuration.
func WithRetry(config RetryConfig) ClientOption {
	return func(c *Client) {
		internalConfig := retry.Config{
			MaxRetries:     config.MaxRetries,
			InitialBackoff: config.InitialBackoff,
			BackoffFactor:  config.BackoffFactor,
			MaxBackoff:     config.MaxBackoff,
		}
		validated := internalConfig.Validate()
		c.retryConfig = &validated
		// Set retry config on transport if it exists
		if c.transport != nil {
			c.transport.setRetryConfig(c.retryConfig)
		}
	}
}
