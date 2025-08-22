// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"testing"
	"time"
)

func TestWithSimpleRetry(t *testing.T) {
	client := &Client{}
	option := WithSimpleRetry(5)
	option(client)

	if client.retryConfig == nil {
		t.Fatal("Expected retry config to be set")
	}
	if client.retryConfig.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries=5, got %d", client.retryConfig.MaxRetries)
	}
	if client.retryConfig.InitialBackoff != 500*time.Millisecond {
		t.Errorf("Expected InitialBackoff=500ms, got %v", client.retryConfig.InitialBackoff)
	}
}

func TestWithRetry(t *testing.T) {
	client := &Client{}
	config := RetryConfig{
		MaxRetries:     10,
		InitialBackoff: 2 * time.Second,
		BackoffFactor:  3.0,
		MaxBackoff:     1 * time.Minute,
	}
	option := WithRetry(config)
	option(client)

	if client.retryConfig == nil {
		t.Fatal("Expected retry config to be set")
	}
	if client.retryConfig.MaxRetries != 10 {
		t.Errorf("Expected MaxRetries=10, got %d", client.retryConfig.MaxRetries)
	}
	if client.retryConfig.InitialBackoff != 2*time.Second {
		t.Errorf("Expected InitialBackoff=2s, got %v", client.retryConfig.InitialBackoff)
	}
	if client.retryConfig.BackoffFactor != 3.0 {
		t.Errorf("Expected BackoffFactor=3.0, got %f", client.retryConfig.BackoffFactor)
	}
}
