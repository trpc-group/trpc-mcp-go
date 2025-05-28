// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package httputil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAcceptHeader(t *testing.T) {
	testCases := []struct {
		name     string
		header   string
		expected []string
	}{
		{
			name:     "Empty header",
			header:   "",
			expected: []string{},
		},
		{
			name:     "Single content type",
			header:   "application/json",
			expected: []string{"application/json"},
		},
		{
			name:     "Multiple content types",
			header:   "application/json, text/html, text/plain",
			expected: []string{"application/json", "text/html", "text/plain"},
		},
		{
			name:     "With quality values",
			header:   "application/json;q=1.0, text/html;q=0.9, text/plain;q=0.8",
			expected: []string{"application/json", "text/html", "text/plain"},
		},
		{
			name:     "With extra parameters",
			header:   "application/json;version=1, text/html;charset=utf-8",
			expected: []string{"application/json", "text/html"},
		},
		{
			name:     "With whitespace",
			header:   " application/json ,  text/html ",
			expected: []string{"application/json", "text/html"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseAcceptHeader(tc.header)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestContainsContentType(t *testing.T) {
	testCases := []struct {
		name        string
		accepts     []string
		contentType string
		expected    bool
	}{
		{
			name:        "Empty accepts",
			accepts:     []string{},
			contentType: "application/json",
			expected:    false,
		},
		{
			name:        "Direct match",
			accepts:     []string{"application/json", "text/html"},
			contentType: "application/json",
			expected:    true,
		},
		{
			name:        "No match",
			accepts:     []string{"application/xml", "text/html"},
			contentType: "application/json",
			expected:    false,
		},
		{
			name:        "Wildcard match",
			accepts:     []string{"text/html", "*/*"},
			contentType: "application/json",
			expected:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ContainsContentType(tc.accepts, tc.contentType)
			assert.Equal(t, tc.expected, result)
		})
	}
}
