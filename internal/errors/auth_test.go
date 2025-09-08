// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package errors_test

import (
	"testing"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

func TestNewOAuthError(t *testing.T) {
	err := errors.NewOAuthError(errors.ErrInvalidRequest, "missing parameter", "https://example.com/docs")

	if err.ErrorCode != "invalid request" {
		t.Errorf("expected error code 'invalid request', got %s", err.ErrorCode)
	}
	if err.Message != "missing parameter" {
		t.Errorf("expected message 'missing parameter', got %s", err.Message)
	}
	if err.ErrorURI != "https://example.com/docs" {
		t.Errorf("expected URI 'https://example.com/docs', got %s", err.ErrorURI)
	}
}

func TestToResponseStruct(t *testing.T) {
	err := errors.NewOAuthError(errors.ErrInvalidClient, "bad client id", "")
	resp := err.ToResponseStruct()

	if resp.Error != "invalid client" {
		t.Errorf("expected 'invalid client', got %s", resp.Error)
	}
	if resp.ErrorDescription != "bad client id" {
		t.Errorf("expected description 'bad client id', got %s", resp.ErrorDescription)
	}
	if resp.ErrorURI != "" {
		t.Errorf("expected empty URI, got %s", resp.ErrorURI)
	}
}

func TestErrorMethod(t *testing.T) {
	err := errors.NewOAuthError(errors.ErrServerError, "internal failure", "")
	if err.Error() != "server error" {
		t.Errorf("expected 'server error', got %s", err.Error())
	}
}
